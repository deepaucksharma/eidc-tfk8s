package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/newrelic/nrdot-internal-devlab/internal/common/logging"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

// Build information, injected at build time
var (
	Version   string = "2.1.2-dev"
	BuildTime string
	CommitSHA string
)

// Metrics
var (
	configPushesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cc_config_pushes_total",
		Help: "Total number of configuration pushes",
	})
	configAcksTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cc_config_acks_total",
		Help: "Total number of configuration acknowledgments",
	}, []string{"status"})
	configStreamTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cc_stream_total",
		Help: "Total number of config stream connections",
	})
	configStreamActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cc_stream_active",
		Help: "Number of active config stream connections",
	})
	configStreamPushTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cc_stream_push_total",
		Help: "Total number of config stream pushes",
	})
	configStreamAckTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cc_stream_ack_total",
		Help: "Total number of config stream acknowledgments",
	}, []string{"status"})
	configGeneration = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cc_config_generation",
		Help: "Current configuration generation",
	})
)

// FB-specific configuration type for each FB
type FBConfig map[string]interface{}

// Pipeline configuration map - maps FB name to its configuration
type PipelineConfig map[string]FBConfig

// Client subscription for streaming config updates
type ClientSubscription struct {
	fbName         string
	instanceID     string
	stream         ConfigService_StreamConfigServer
	lastGeneration int64
	active         bool
	lastAck        time.Time
	mu             sync.Mutex
}

// ConfigController manages the configuration for Function Blocks
type ConfigController struct {
	logger            *logging.Logger
	dynamicClient     dynamic.Interface
	pipelineGVR       schema.GroupVersionResource
	namespace         string
	subscriptions     map[string][]*ClientSubscription
	subscriptionsMu   sync.RWMutex
	currentConfig     PipelineConfig
	currentGeneration int64
	configMu          sync.RWMutex
}

// NewConfigController creates a new ConfigController
func NewConfigController(logger *logging.Logger, config *rest.Config, namespace string) (*ConfigController, error) {
	// Create dynamic client
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// Create controller
	controller := &ConfigController{
		logger:        logger,
		dynamicClient: dynamicClient,
		pipelineGVR: schema.GroupVersionResource{
			Group:    "nrdot.newrelic.com",
			Version:  "v1",
			Resource: "nrdotpluspipelines",
		},
		namespace:       namespace,
		subscriptions:   make(map[string][]*ClientSubscription),
		currentConfig:   make(PipelineConfig),
		currentGeneration: 0,
	}

	return controller, nil
}

// Start starts the controller
func (c *ConfigController) Start(ctx context.Context) error {
	// Start watching for pipeline resource changes
	go c.watchPipelines(ctx)

	return nil
}

// watchPipelines watches for changes to NRDotPlusPipeline resources
func (c *ConfigController) watchPipelines(ctx context.Context) {
	c.logger.Info("Starting to watch NRDotPlusPipeline resources", map[string]interface{}{
		"namespace": c.namespace,
	})

	for {
		// Watch for changes to pipeline resources
		watcher, err := c.dynamicClient.
			Resource(c.pipelineGVR).
			Namespace(c.namespace).
			Watch(ctx, metav1.ListOptions{})
		if err != nil {
			c.logger.Error("Failed to watch pipelines", err, map[string]interface{}{
				"namespace": c.namespace,
			})
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
				continue
			}
		}

		// Process watch events
		watchCh := watcher.ResultChan()
		for {
			select {
			case <-ctx.Done():
				watcher.Stop()
				return
			case event, ok := <-watchCh:
				if !ok {
					c.logger.Warn("Watch channel closed", map[string]interface{}{
						"namespace": c.namespace,
					})
					break
				}

				// Process the event
				c.handleWatchEvent(ctx, event)
			}
		}

		watcher.Stop()
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
			// Retry watching
		}
	}
}

// handleWatchEvent handles a watch event for a NRDotPlusPipeline resource
func (c *ConfigController) handleWatchEvent(ctx context.Context, event watch.Event) {
	// Process based on event type
	switch event.Type {
	case watch.Added, watch.Modified:
		// Get the pipeline resource
		pipeline, ok := event.Object.(*unstructured.Unstructured)
		if !ok {
			c.logger.Error("Failed to convert to unstructured", fmt.Errorf("invalid object type"), map[string]interface{}{
				"type": fmt.Sprintf("%T", event.Object),
			})
			return
		}

		// Process the pipeline update
		c.processPipelineUpdate(ctx, pipeline)

	case watch.Deleted:
		// Handle pipeline deletion
		pipeline, ok := event.Object.(*unstructured.Unstructured)
		if !ok {
			c.logger.Error("Failed to convert to unstructured", fmt.Errorf("invalid object type"), map[string]interface{}{
				"type": fmt.Sprintf("%T", event.Object),
			})
			return
		}

		c.logger.Info("Pipeline deleted", map[string]interface{}{
			"name": pipeline.GetName(),
		})

		// We might want to handle cleanup or notify subscribers
	}
}

// processPipelineUpdate processes a pipeline update event
func (c *ConfigController) processPipelineUpdate(ctx context.Context, pipeline *unstructured.Unstructured) {
	pipelineName := pipeline.GetName()
	generation := pipeline.GetGeneration()

	c.logger.Info("Processing pipeline update", map[string]interface{}{
		"name":       pipelineName,
		"generation": generation,
	})

	// Extract the spec from the pipeline resource
	spec, found, err := unstructured.NestedMap(pipeline.Object, "spec")
	if err != nil {
		c.logger.Error("Failed to get spec", err, map[string]interface{}{
			"name": pipelineName,
		})
		return
	}
	if !found {
		c.logger.Error("Spec not found", fmt.Errorf("spec not found"), map[string]interface{}{
			"name": pipelineName,
		})
		return
	}

	// Convert spec to pipeline config
	newConfig := make(PipelineConfig)
	for fbName, fbSpec := range spec {
		// Skip non-FB fields in the spec
		if fbName == "globalSettings" {
			// Process global settings if needed
			continue
		}

		// Convert to FB config
		fbConfig, ok := fbSpec.(map[string]interface{})
		if !ok {
			c.logger.Error("Invalid FB config", fmt.Errorf("invalid config type"), map[string]interface{}{
				"name":    pipelineName,
				"fb_name": fbName,
			})
			continue
		}

		// Store FB config
		newConfig[fbName] = fbConfig
	}

	// Update current config
	c.configMu.Lock()
	c.currentConfig = newConfig
	c.currentGeneration = generation
	configGeneration.Set(float64(generation))
	c.configMu.Unlock()

	// Push new config to subscribers
	c.pushConfigToSubscribers(ctx)

	// Update pipeline status
	c.updatePipelineStatus(ctx, pipeline, generation)
}

// pushConfigToSubscribers pushes configuration updates to all subscribers
func (c *ConfigController) pushConfigToSubscribers(ctx context.Context) {
	c.configMu.RLock()
	config := c.currentConfig
	generation := c.currentGeneration
	c.configMu.RUnlock()

	c.subscriptionsMu.RLock()
	defer c.subscriptionsMu.RUnlock()

	// Push to each FB's subscribers
	for fbName, subs := range c.subscriptions {
		// Get FB-specific config
		fbConfig, ok := config[fbName]
		if !ok {
			// No config for this FB
			continue
		}

		// Serialize FB config
		configBytes, err := json.Marshal(fbConfig)
		if err != nil {
			c.logger.Error("Failed to marshal config", err, map[string]interface{}{
				"fb_name": fbName,
			})
			continue
		}

		// Push to all subscribers for this FB
		for _, sub := range subs {
			if !sub.active {
				continue
			}

			// Skip if already at this generation
			sub.mu.Lock()
			if sub.lastGeneration >= generation {
				sub.mu.Unlock()
				continue
			}

			// Send config update
			res := &ConfigResponse{
				Generation: generation,
				Config:     configBytes,
				Timestamp:  time.Now().Unix(),
			}

			err := sub.stream.Send(res)
			if err != nil {
				c.logger.Error("Failed to send config", err, map[string]interface{}{
					"fb_name":     fbName,
					"instance_id": sub.instanceID,
				})
				sub.active = false
			} else {
				configStreamPushTotal.Inc()
				c.logger.Info("Sent config update", map[string]interface{}{
					"fb_name":     fbName,
					"instance_id": sub.instanceID,
					"generation":  generation,
				})
			}
			sub.mu.Unlock()
		}
	}
}

// updatePipelineStatus updates the status of a pipeline resource
func (c *ConfigController) updatePipelineStatus(ctx context.Context, pipeline *unstructured.Unstructured, generation int64) {
	// Create status update
	statusUpdate := map[string]interface{}{
		"observedGeneration":     generation,
		"configGenerationApplied": generation,
		"conditions": []map[string]interface{}{
			{
				"type":               "Ready",
				"status":             "True",
				"lastTransitionTime": time.Now().Format(time.RFC3339),
				"reason":             "ConfigApplied",
				"message":            "Configuration applied",
			},
		},
	}

	// Apply status update
	statusPatch := map[string]interface{}{
		"status": statusUpdate,
	}
	patchBytes, err := json.Marshal(statusPatch)
	if err != nil {
		c.logger.Error("Failed to marshal status patch", err, map[string]interface{}{
			"name": pipeline.GetName(),
		})
		return
	}

	// Update pipeline status
	_, err = c.dynamicClient.
		Resource(c.pipelineGVR).
		Namespace(c.namespace).
		Patch(ctx, pipeline.GetName(), types.MergePatchType, patchBytes, metav1.PatchOptions{}, "status")
	if err != nil {
		c.logger.Error("Failed to update pipeline status", err, map[string]interface{}{
			"name": pipeline.GetName(),
		})
		return
	}

	c.logger.Info("Updated pipeline status", map[string]interface{}{
		"name":             pipeline.GetName(),
		"generation":       generation,
		"status.observed": generation,
	})
}

// GetConfig implements the GetConfig RPC method
func (c *ConfigController) GetConfig(ctx context.Context, req *ConfigRequest) (*ConfigResponse, error) {
	// Validate request
	if req.FbName == "" {
		return nil, status.Errorf(codes.InvalidArgument, "fb_name is required")
	}

	c.logger.Info("GetConfig request", map[string]interface{}{
		"fb_name":     req.FbName,
		"instance_id": req.InstanceId,
		"last_known": req.LastKnownGeneration,
	})

	// Get current config
	c.configMu.RLock()
	defer c.configMu.RUnlock()

	// Check if we have config for this FB
	fbConfig, ok := c.currentConfig[req.FbName]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "no configuration for FB %s", req.FbName)
	}

	// Serialize FB config
	configBytes, err := json.Marshal(fbConfig)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to marshal config: %v", err)
	}

	// Increment metrics
	configPushesTotal.Inc()

	// Return config response
	return &ConfigResponse{
		Generation: c.currentGeneration,
		Config:     configBytes,
		Timestamp:  time.Now().Unix(),
	}, nil
}

// StreamConfig implements the StreamConfig RPC method
func (c *ConfigController) StreamConfig(req *ConfigRequest, stream ConfigService_StreamConfigServer) error {
	// Validate request
	if req.FbName == "" {
		return status.Errorf(codes.InvalidArgument, "fb_name is required")
	}

	c.logger.Info("StreamConfig request", map[string]interface{}{
		"fb_name":     req.FbName,
		"instance_id": req.InstanceId,
		"last_known": req.LastKnownGeneration,
	})

	// Create subscription
	sub := &ClientSubscription{
		fbName:         req.FbName,
		instanceID:     req.InstanceId,
		stream:         stream,
		lastGeneration: req.LastKnownGeneration,
		active:         true,
		lastAck:        time.Now(),
	}

	// Add subscription
	c.subscriptionsMu.Lock()
	c.subscriptions[req.FbName] = append(c.subscriptions[req.FbName], sub)
	c.subscriptionsMu.Unlock()

	// Increment metrics
	configStreamTotal.Inc()
	configStreamActive.Inc()

	// Send initial config
	c.configMu.RLock()
	fbConfig, ok := c.currentConfig[req.FbName]
	currentGeneration := c.currentGeneration
	c.configMu.RUnlock()

	if ok && currentGeneration > req.LastKnownGeneration {
		// Serialize FB config
		configBytes, err := json.Marshal(fbConfig)
		if err != nil {
			c.logger.Error("Failed to marshal config", err, map[string]interface{}{
				"fb_name":     req.FbName,
				"instance_id": req.InstanceId,
			})
		} else {
			// Send config update
			res := &ConfigResponse{
				Generation: currentGeneration,
				Config:     configBytes,
				Timestamp:  time.Now().Unix(),
			}

			err := stream.Send(res)
			if err != nil {
				c.logger.Error("Failed to send initial config", err, map[string]interface{}{
					"fb_name":     req.FbName,
					"instance_id": req.InstanceId,
				})
				return err
			}

			configStreamPushTotal.Inc()
			c.logger.Info("Sent initial config", map[string]interface{}{
				"fb_name":     req.FbName,
				"instance_id": req.InstanceId,
				"generation":  currentGeneration,
			})

			// Update subscription generation
			sub.mu.Lock()
			sub.lastGeneration = currentGeneration
			sub.mu.Unlock()
		}
	}

	// Wait for context cancellation (client disconnect)
	<-stream.Context().Done()

	// Clean up subscription
	c.subscriptionsMu.Lock()
	subs := c.subscriptions[req.FbName]
	for i, s := range subs {
		if s == sub {
			// Remove this subscription
			c.subscriptions[req.FbName] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
	c.subscriptionsMu.Unlock()

	// Decrement metrics
	configStreamActive.Dec()

	c.logger.Info("StreamConfig ended", map[string]interface{}{
		"fb_name":     req.FbName,
		"instance_id": req.InstanceId,
	})

	return nil
}

// AckConfig implements the AckConfig RPC method
func (c *ConfigController) AckConfig(ctx context.Context, req *ConfigAckRequest) (*ConfigAckResponse, error) {
	// Validate request
	if req.FbName == "" {
		return nil, status.Errorf(codes.InvalidArgument, "fb_name is required")
	}

	c.logger.Info("AckConfig request", map[string]interface{}{
		"fb_name":     req.FbName,
		"instance_id": req.InstanceId,
		"generation":  req.Generation,
		"success":     req.Success,
	})

	// Find subscription
	c.subscriptionsMu.RLock()
	subs := c.subscriptions[req.FbName]
	var sub *ClientSubscription
	for _, s := range subs {
		if s.instanceID == req.InstanceId {
			sub = s
			break
		}
	}
	c.subscriptionsMu.RUnlock()

	// Update subscription if found
	if sub != nil {
		sub.mu.Lock()
		sub.lastAck = time.Now()
		if req.Success {
			sub.lastGeneration = req.Generation
		}
		sub.mu.Unlock()
	}

	// Increment metrics
	if req.Success {
		configAcksTotal.WithLabelValues("success").Inc()
		configStreamAckTotal.WithLabelValues("OK").Inc()
	} else {
		configAcksTotal.WithLabelValues("failure").Inc()
		configStreamAckTotal.WithLabelValues("ERROR").Inc()
	}

	// Return success
	return &ConfigAckResponse{
		Recorded: true,
	}, nil
}

// ConfigRequest is the request message for the GetConfig and StreamConfig methods
type ConfigRequest struct {
	FbName             string
	InstanceId         string
	LastKnownGeneration int64
}

// ConfigResponse is the response message for the GetConfig and StreamConfig methods
type ConfigResponse struct {
	Generation      int64
	Config          []byte
	RequiresRestart bool
	Timestamp       int64
}

// ConfigAckRequest is the request message for the AckConfig method
type ConfigAckRequest struct {
	FbName       string
	InstanceId   string
	Generation   int64
	Success      bool
	ErrorMessage string
}

// ConfigAckResponse is the response message for the AckConfig method
type ConfigAckResponse struct {
	Recorded bool
}

// ConfigService_StreamConfigServer is the server API for Config service.
type ConfigService_StreamConfigServer interface {
	Send(*ConfigResponse) error
	grpc.ServerStream
}

func main() {
	// Parse command-line flags
	var (
		grpcPort           = flag.Int("grpc-port", 5000, "gRPC service port")
		metricsPort        = flag.Int("metrics-port", 2112, "Prometheus metrics port")
		namespace          = flag.String("namespace", "", "Kubernetes namespace (default: auto-detect)")
		leaseLockName      = flag.String("lease-lock-name", "config-controller", "Name of the lease lock")
		leaseLockNamespace = flag.String("lease-lock-namespace", "", "Namespace of the lease lock")
		id                 = flag.String("id", "", "Leader election ID (default: pod name)")
		leaseDuration      = flag.Duration("lease-duration", 15*time.Second, "Leader lease duration")
		renewDeadline      = flag.Duration("renew-deadline", 10*time.Second, "Leader renew deadline")
		retryPeriod        = flag.Duration("retry-period", 2*time.Second, "Leader election retry period")
	)
	flag.Parse()

	// Set up logging
	logger := logging.NewLogger("config-controller")
	logger.Info("Starting ConfigController", map[string]interface{}{
		"version":    Version,
		"build_time": BuildTime,
		"commit":     CommitSHA,
	})

	// Create context that listens for termination signals
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		logger.Info("Received signal", map[string]interface{}{"signal": sig.String()})
		cancel()
	}()

	// Set up metrics server
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("healthy"))
	})
	http.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		// TODO: Implement proper readiness check
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	})

	metricsServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", *metricsPort),
		Handler: nil,
	}

	go func() {
		logger.Info("Starting metrics server", map[string]interface{}{"port": *metricsPort})
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Metrics server failed", err, nil)
			cancel()
		}
	}()

	// Initialize Kubernetes client
	config, err := rest.InClusterConfig()
	if err != nil {
		logger.Fatal("Failed to create in-cluster config", err, nil)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Fatal("Failed to create Kubernetes client", err, nil)
	}

	// Auto-detect namespace if not provided
	if *namespace == "" {
		if data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
			*namespace = string(data)
			logger.Info("Auto-detected namespace", map[string]interface{}{"namespace": *namespace})
		} else {
			logger.Fatal("Failed to auto-detect namespace", err, nil)
		}
	}

	// Auto-detect pod name for ID if not provided
	if *id == "" {
		if hostname, err := os.Hostname(); err == nil {
			*id = hostname
			logger.Info("Auto-detected hostname", map[string]interface{}{"hostname": *id})
		} else {
			logger.Fatal("Failed to auto-detect hostname", err, nil)
		}
	}

	// Use the same namespace for lease lock if not specified
	if *leaseLockNamespace == "" {
		*leaseLockNamespace = *namespace
	}

	// Create controller
	controller, err := NewConfigController(logger, config, *namespace)
	if err != nil {
		logger.Fatal("Failed to create controller", err, nil)
	}

	// Initialize gRPC server
	server := grpc.NewServer()
	// Register the ConfigService server
	// RegisterConfigServiceServer(server, controller)

	// Start gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *grpcPort))
	if err != nil {
		logger.Fatal("Failed to listen", err, map[string]interface{}{"port": *grpcPort})
	}

	go func() {
		logger.Info("Starting gRPC server", map[string]interface{}{"port": *grpcPort})
		if err := server.Serve(lis); err != nil {
			logger.Error("gRPC server failed", err, nil)
			cancel()
		}
	}()

	// Set up leader election
	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      *leaseLockName,
			Namespace: *leaseLockNamespace,
		},
		Client: clientset.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: *id,
		},
	}

	// Run leader election
	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   *leaseDuration,
		RenewDeadline:   *renewDeadline,
		RetryPeriod:     *retryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				logger.Info("Started leading", map[string]interface{}{"id": *id})
				// Start the controller
				if err := controller.Start(ctx); err != nil {
					logger.Error("Failed to start controller", err, nil)
					cancel()
				}
			},
			OnStoppedLeading: func() {
				logger.Info("Stopped leading", map[string]interface{}{"id": *id})
				// We should exit if we stop being leader
				cancel()
			},
			OnNewLeader: func(identity string) {
				if identity != *id {
					logger.Info("New leader elected", map[string]interface{}{"leader": identity})
				}
			},
		},
	})

	// Wait for termination
	<-ctx.Done()
	logger.Info("Shutting down", nil)

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("Error shutting down metrics server", err, nil)
	}

	// Gracefully stop the gRPC server
	server.GracefulStop()

	logger.Info("Shutdown complete", nil)
}
