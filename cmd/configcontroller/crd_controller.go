package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"time"

	"google.golang.org/protobuf/proto"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	pb "github.com/newrelic/nrdot-internal-devlab/pkg/api/protobuf"
)

// CRDController manages the NRDotPlusPipeline CRD
type CRDController struct {
	logger              *log.Logger
	configController    *ConfigController
	clientset           *kubernetes.Clientset
	dynamicClient       dynamic.Interface
	namespace           string
	resourceGVR         schema.GroupVersionResource
	informer            cache.SharedIndexInformer
	lastResourceVersion string
}

// NewCRDController creates a new CRD controller
func NewCRDController(logger *log.Logger, configController *ConfigController, clientset *kubernetes.Clientset, namespace string) (*CRDController, error) {
	// Create dynamic client for CRD operations
	config, err := clientset.RESTClient().Config.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get REST config: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// Define GVR for NRDotPlusPipeline
	resourceGVR := schema.GroupVersionResource{
		Group:    "nrdot.newrelic.com",
		Version:  "v1",
		Resource: "nrdotpluspipelines",
	}

	controller := &CRDController{
		logger:           logger,
		configController: configController,
		clientset:        clientset,
		dynamicClient:    dynamicClient,
		namespace:        namespace,
		resourceGVR:      resourceGVR,
	}

	return controller, nil
}

// Start starts the CRD controller
func (c *CRDController) Start(ctx context.Context) error {
	c.logger.Printf(`{"level":"info","timestamp":"%s","message":"Starting CRD controller","namespace":"%s"}`,
		time.Now().Format(time.RFC3339), c.namespace)

	// Create informer factory
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		c.dynamicClient,
		30*time.Second,
		c.namespace,
		nil,
	)

	// Create informer for NRDotPlusPipeline
	informer := factory.ForResource(c.resourceGVR).Informer()

	// Add event handlers
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.onAdd,
		UpdateFunc: c.onUpdate,
		DeleteFunc: c.onDelete,
	})

	c.informer = informer

	// Start informer
	factory.Start(ctx.Done())

	// Wait for the first list to complete
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		return fmt.Errorf("failed to sync informer cache")
	}

	c.logger.Printf(`{"level":"info","timestamp":"%s","message":"CRD controller started"}`, time.Now().Format(time.RFC3339))

	// Run a periodic status update
	go c.runStatusUpdater(ctx)

	// Wait for context cancellation
	<-ctx.Done()
	return nil
}

// onAdd handles CRD add events
func (c *CRDController) onAdd(obj interface{}) {
	unstructured, ok := obj.(*unstructured.Unstructured)
	if !ok {
		c.logger.Printf(`{"level":"error","timestamp":"%s","message":"Failed to cast object to Unstructured"}`, time.Now().Format(time.RFC3339))
		return
	}

	c.logger.Printf(`{"level":"info","timestamp":"%s","message":"NRDotPlusPipeline added","name":"%s","namespace":"%s"}`,
		time.Now().Format(time.RFC3339), unstructured.GetName(), unstructured.GetNamespace())

	// Process the CRD
	c.processCRD(unstructured)
}

// onUpdate handles CRD update events
func (c *CRDController) onUpdate(oldObj, newObj interface{}) {
	oldUnstructured, ok := oldObj.(*unstructured.Unstructured)
	if !ok {
		c.logger.Printf(`{"level":"error","timestamp":"%s","message":"Failed to cast old object to Unstructured"}`, time.Now().Format(time.RFC3339))
		return
	}

	newUnstructured, ok := newObj.(*unstructured.Unstructured)
	if !ok {
		c.logger.Printf(`{"level":"error","timestamp":"%s","message":"Failed to cast new object to Unstructured"}`, time.Now().Format(time.RFC3339))
		return
	}

	// Skip processing if the spec hasn't changed
	oldSpec, _, _ := unstructured.NestedMap(oldUnstructured.Object, "spec")
	newSpec, _, _ := unstructured.NestedMap(newUnstructured.Object, "spec")

	if reflect.DeepEqual(oldSpec, newSpec) {
		c.logger.Printf(`{"level":"debug","timestamp":"%s","message":"NRDotPlusPipeline updated but spec unchanged","name":"%s","namespace":"%s"}`,
			time.Now().Format(time.RFC3339), newUnstructured.GetName(), newUnstructured.GetNamespace())
		return
	}

	c.logger.Printf(`{"level":"info","timestamp":"%s","message":"NRDotPlusPipeline updated","name":"%s","namespace":"%s","generation":%d,"resource_version":"%s"}`,
		time.Now().Format(time.RFC3339), newUnstructured.GetName(), newUnstructured.GetNamespace(), newUnstructured.GetGeneration(), newUnstructured.GetResourceVersion())

	// Process the CRD
	c.processCRD(newUnstructured)
}

// onDelete handles CRD delete events
func (c *CRDController) onDelete(obj interface{}) {
	unstructured, ok := obj.(*unstructured.Unstructured)
	if !ok {
		c.logger.Printf(`{"level":"error","timestamp":"%s","message":"Failed to cast object to Unstructured"}`, time.Now().Format(time.RFC3339))
		return
	}

	c.logger.Printf(`{"level":"info","timestamp":"%s","message":"NRDotPlusPipeline deleted","name":"%s","namespace":"%s"}`,
		time.Now().Format(time.RFC3339), unstructured.GetName(), unstructured.GetNamespace())

	// TODO: Handle deletion (clean up resources, etc.)
}

// processCRD processes a NRDotPlusPipeline CRD
func (c *CRDController) processCRD(unstructured *unstructured.Unstructured) {
	// Extract spec
	spec, exists, err := unstructured.NestedMap(unstructured.Object, "spec")
	if err != nil {
		c.logger.Printf(`{"level":"error","timestamp":"%s","message":"Failed to extract spec","error":"%s"}`,
			time.Now().Format(time.RFC3339), err)
		return
	}

	if !exists {
		c.logger.Printf(`{"level":"error","timestamp":"%s","message":"Spec not found in CRD"}`, time.Now().Format(time.RFC3339))
		return
	}

	// Extract fields from spec
	pipelineVersion, _ := unstructured.NestedString(unstructured.Object, "spec", "pipelineVersion")
	globalSettings, _, _ := unstructured.NestedMap(unstructured.Object, "spec", "globalSettings")
	functionBlocks, exists, _ := unstructured.NestedMap(unstructured.Object, "spec", "functionBlocks")
	
	if !exists {
		c.logger.Printf(`{"level":"error","timestamp":"%s","message":"functionBlocks not found in CRD"}`, time.Now().Format(time.RFC3339))
		return
	}

	// Build PipelineConfig
	pipelineConfig := &pb.PipelineConfig{
		Generation:      unstructured.GetGeneration(),
		PipelineVersion: pipelineVersion,
		GlobalSettings:  convertGlobalSettings(globalSettings),
		FunctionBlocks:  make(map[string]*pb.FBConfig),
	}

	// Convert functionBlocks to map[string]*pb.FBConfig
	for fbName, fbConfigRaw := range functionBlocks {
		fbConfigMap, ok := fbConfigRaw.(map[string]interface{})
		if !ok {
			c.logger.Printf(`{"level":"error","timestamp":"%s","message":"Invalid FB config","fb_name":"%s"}`,
				time.Now().Format(time.RFC3339), fbName)
			continue
		}

		enabled, _ := getNestedBool(fbConfigMap, "enabled")
		imageTag, _ := getNestedString(fbConfigMap, "imageTag")
		parametersRaw, exists, _ := getNestedMap(fbConfigMap, "parameters")
		
		if !exists {
			parametersRaw = make(map[string]interface{})
		}
		
		// Convert parameters to JSON bytes
		parametersBytes, err := json.Marshal(parametersRaw)
		if err != nil {
			c.logger.Printf(`{"level":"error","timestamp":"%s","message":"Failed to marshal parameters","fb_name":"%s","error":"%s"}`,
				time.Now().Format(time.RFC3339), fbName, err)
			continue
		}

		// Create FBConfig
		fbConfig := &pb.FBConfig{
			Enabled:    enabled,
			ImageTag:   imageTag,
			Parameters: parametersBytes,
		}

		// Add circuit breaker config if present
		circuitBreakerRaw, exists, _ := getNestedMap(parametersRaw, "circuitBreaker")
		if exists {
			circuitBreaker := &pb.CircuitBreakerConfig{}
			
			if errorThresholdRaw, ok := circuitBreakerRaw["errorThresholdPercentage"]; ok {
				if errorThreshold, ok := errorThresholdRaw.(int32); ok {
					circuitBreaker.ErrorThresholdPercentage = errorThreshold
				}
			}
			
			if openStateRaw, ok := circuitBreakerRaw["openStateSeconds"]; ok {
				if openState, ok := openStateRaw.(int32); ok {
					circuitBreaker.OpenStateSeconds = openState
				}
			}
			
			if halfOpenRaw, ok := circuitBreakerRaw["halfOpenRequestThreshold"]; ok {
				if halfOpen, ok := halfOpenRaw.(int32); ok {
					circuitBreaker.HalfOpenRequestThreshold = halfOpen
				}
			}
			
			fbConfig.CircuitBreaker = circuitBreaker
		} else {
			// Use defaults
			fbConfig.CircuitBreaker = &pb.CircuitBreakerConfig{
				ErrorThresholdPercentage: 50,
				OpenStateSeconds:         30,
				HalfOpenRequestThreshold: 5,
			}
		}

		pipelineConfig.FunctionBlocks[fbName] = fbConfig
	}

	// Save last resource version
	c.lastResourceVersion = unstructured.GetResourceVersion()

	// Broadcast config to connected clients
	c.configController.BroadcastConfig(pipelineConfig, unstructured.GetGeneration())

	// Update status
	c.updateStatus(unstructured)
}

// convertGlobalSettings converts globalSettings map to pb.GlobalSettings
func convertGlobalSettings(globalSettings map[string]interface{}) *pb.GlobalSettings {
	if globalSettings == nil {
		return &pb.GlobalSettings{}
	}

	result := &pb.GlobalSettings{}

	if deterministicSeedEnvVar, ok := globalSettings["deterministicSeedEnvVar"].(string); ok {
		result.DeterministicSeedEnvVar = deterministicSeedEnvVar
	}

	if internalLabelPolicy, ok := globalSettings["internalLabelPolicy"].(string); ok {
		result.InternalLabelPolicy = internalLabelPolicy
	}

	return result
}

// getNestedBool extracts a boolean value from a nested map
func getNestedBool(obj map[string]interface{}, key string) (bool, bool) {
	value, exists := obj[key]
	if !exists {
		return false, false
	}

	boolValue, ok := value.(bool)
	return boolValue, ok
}

// getNestedString extracts a string value from a nested map
func getNestedString(obj map[string]interface{}, key string) (string, bool) {
	value, exists := obj[key]
	if !exists {
		return "", false
	}

	stringValue, ok := value.(string)
	return stringValue, ok
}

// getNestedMap extracts a map value from a nested map
func getNestedMap(obj map[string]interface{}, key string) (map[string]interface{}, bool, error) {
	value, exists := obj[key]
	if !exists {
		return nil, false, nil
	}

	mapValue, ok := value.(map[string]interface{})
	if !ok {
		return nil, false, fmt.Errorf("value is not a map")
	}

	return mapValue, true, nil
}

// updateStatus updates the status subresource of the NRDotPlusPipeline CRD
func (c *CRDController) updateStatus(crd *unstructured.Unstructured) {
	// Get client status
	clientStatus := c.configController.GetClientStatus()

	// Create status
	status := map[string]interface{}{
		"observedGeneration":      crd.GetGeneration(),
		"configGenerationApplied": crd.GetGeneration(),
	}

	// Create fbStatus array
	fbStatus := make([]interface{}, 0)
	for fbID, instances := range clientStatus {
		for _, instance := range instances {
			fbStatus = append(fbStatus, map[string]interface{}{
				"name":              fbID,
				"ready":             true,
				"configApplied":     true,
				"configGeneration":  instance["gen_acked"],
				"lastTransitionTime": instance["last_updated"],
			})
		}
	}
	status["fbStatus"] = fbStatus

	// Create conditions
	allReady := len(fbStatus) > 0
	for _, fb := range fbStatus {
		fbMap, ok := fb.(map[string]interface{})
		if !ok {
			continue
		}

		ready, ok := fbMap["ready"].(bool)
		if !ok || !ready {
			allReady = false
			break
		}
	}

	conditions := []interface{}{
		map[string]interface{}{
			"type":               "Ready",
			"status":             iff(allReady, "True", "False"),
			"lastTransitionTime": time.Now().Format(time.RFC3339),
			"reason":             iff(allReady, "AllFBsReady", "NotAllFBsReady"),
			"message":            iff(allReady, "All function blocks are ready", "Not all function blocks are ready"),
		},
	}
	status["conditions"] = conditions

	// Update status
	_, err := c.dynamicClient.Resource(c.resourceGVR).Namespace(crd.GetNamespace()).UpdateStatus(
		context.Background(),
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": crd.GetAPIVersion(),
				"kind":       crd.GetKind(),
				"metadata": map[string]interface{}{
					"name":            crd.GetName(),
					"namespace":       crd.GetNamespace(),
					"resourceVersion": crd.GetResourceVersion(),
				},
				"status": status,
			},
		},
		metav1.UpdateOptions{},
	)

	if err != nil {
		c.logger.Printf(`{"level":"error","timestamp":"%s","message":"Failed to update status","error":"%s"}`,
			time.Now().Format(time.RFC3339), err)
	} else {
		c.logger.Printf(`{"level":"info","timestamp":"%s","message":"Status updated","crd":"%s"}`,
			time.Now().Format(time.RFC3339), crd.GetName())
	}
}

// runStatusUpdater periodically updates the status of all NRDotPlusPipeline CRDs
func (c *CRDController) runStatusUpdater(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Get all NRDotPlusPipeline CRDs
			list, err := c.dynamicClient.Resource(c.resourceGVR).Namespace(c.namespace).List(ctx, metav1.ListOptions{})
			if err != nil {
				c.logger.Printf(`{"level":"error","timestamp":"%s","message":"Failed to list CRDs","error":"%s"}`,
					time.Now().Format(time.RFC3339), err)
				continue
			}

			// Update status for each CRD
			for _, item := range list.Items {
				c.updateStatus(&item)
			}
		}
	}
}

// iff is a helper function for ternary operations
func iff(condition bool, trueVal, falseVal string) string {
	if condition {
		return trueVal
	}
	return falseVal
}