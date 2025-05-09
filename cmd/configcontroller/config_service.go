// ConfigServiceServer implementation for the ConfigController
package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	pb "github.com/newrelic/nrdot-internal-devlab/pkg/api/protobuf"
)

// ConfigController implements the ConfigService gRPC service
type ConfigController struct {
	pb.UnimplementedConfigServiceServer
	logger *log.Logger
	
	// K8s client
	clientset *kubernetes.Clientset
	namespace string
	
	// Configuration tracking
	configMu          sync.RWMutex
	currentGeneration int64
	pipelineConfig    *pb.PipelineConfig
	
	// Connected clients tracking
	clientsMu sync.RWMutex
	clients   map[string]map[string]*connectedClient // Map of fb_id -> instance_id -> client
}

// connectedClient tracks a connected function block instance
type connectedClient struct {
	fbID        string
	instanceID  string
	stream      pb.ConfigService_StreamConfigServer
	lastUpdated time.Time
	genAcked    int64
}

// NewConfigController creates a new ConfigController
func NewConfigController(logger *log.Logger, clientset *kubernetes.Clientset, namespace string) *ConfigController {
	return &ConfigController{
		logger:    logger,
		clientset: clientset,
		namespace: namespace,
		clients:   make(map[string]map[string]*connectedClient),
	}
}

// GetConfig implements the GetConfig method of the ConfigService
func (c *ConfigController) GetConfig(ctx context.Context, req *pb.ConfigRequest) (*pb.ConfigResponse, error) {
	c.logger.Printf(`{"level":"info","timestamp":"%s","message":"GetConfig request","fb_id":"%s","instance_id":"%s","current_generation":%d}`,
		time.Now().Format(time.RFC3339), req.FbId, req.InstanceId, req.CurrentGeneration)
	
	c.configMu.RLock()
	defer c.configMu.RUnlock()
	
	if c.pipelineConfig == nil {
		return nil, status.Errorf(codes.Unavailable, "configuration not yet loaded")
	}
	
	return &pb.ConfigResponse{
		Status:       0,
		Generation:   c.currentGeneration,
		PipelineConfig: c.pipelineConfig,
	}, nil
}

// StreamConfig implements the StreamConfig method of the ConfigService
func (c *ConfigController) StreamConfig(req *pb.ConfigRequest, stream pb.ConfigService_StreamConfigServer) error {
	c.logger.Printf(`{"level":"info","timestamp":"%s","message":"StreamConfig connected","fb_id":"%s","instance_id":"%s","current_generation":%d}`,
		time.Now().Format(time.RFC3339), req.FbId, req.InstanceId, req.CurrentGeneration)
	
	// Register client
	client := &connectedClient{
		fbID:        req.FbId,
		instanceID:  req.InstanceId,
		stream:      stream,
		lastUpdated: time.Now(),
		genAcked:    req.CurrentGeneration,
	}
	
	c.clientsMu.Lock()
	if _, exists := c.clients[req.FbId]; !exists {
		c.clients[req.FbId] = make(map[string]*connectedClient)
	}
	c.clients[req.FbId][req.InstanceId] = client
	c.clientsMu.Unlock()
	
	// Send initial config
	c.configMu.RLock()
	currentConfig := c.pipelineConfig
	currentGen := c.currentGeneration
	c.configMu.RUnlock()
	
	if currentConfig != nil && currentGen > req.CurrentGeneration {
		if err := stream.Send(&pb.ConfigResponse{
			Status:       0,
			Generation:   currentGen,
			PipelineConfig: currentConfig,
		}); err != nil {
			c.logger.Printf(`{"level":"error","timestamp":"%s","message":"Failed to send initial config","fb_id":"%s","instance_id":"%s","error":"%s"}`,
				time.Now().Format(time.RFC3339), req.FbId, req.InstanceId, err)
			
			// Unregister client on error
			c.clientsMu.Lock()
			if fbClients, exists := c.clients[req.FbId]; exists {
				delete(fbClients, req.InstanceId)
				if len(fbClients) == 0 {
					delete(c.clients, req.FbId)
				}
			}
			c.clientsMu.Unlock()
			
			return err
		}
	}
	
	// Keep connection open until client disconnects or context is cancelled
	<-stream.Context().Done()
	
	c.logger.Printf(`{"level":"info","timestamp":"%s","message":"StreamConfig disconnected","fb_id":"%s","instance_id":"%s"}`,
		time.Now().Format(time.RFC3339), req.FbId, req.InstanceId)
	
	// Unregister client
	c.clientsMu.Lock()
	if fbClients, exists := c.clients[req.FbId]; exists {
		delete(fbClients, req.InstanceId)
		if len(fbClients) == 0 {
			delete(c.clients, req.FbId)
		}
	}
	c.clientsMu.Unlock()
	
	return nil
}

// AckConfig implements the AckConfig method of the ConfigService
func (c *ConfigController) AckConfig(ctx context.Context, req *pb.ConfigAckRequest) (*pb.ConfigAckResponse, error) {
	c.logger.Printf(`{"level":"info","timestamp":"%s","message":"AckConfig","fb_id":"%s","instance_id":"%s","applied_generation":%d,"success":%t}`,
		time.Now().Format(time.RFC3339), req.FbId, req.InstanceId, req.AppliedGeneration, req.Success)
	
	// Update client's acked generation
	c.clientsMu.Lock()
	if fbClients, exists := c.clients[req.FbId]; exists {
		if client, exists := fbClients[req.InstanceId]; exists {
			client.genAcked = req.AppliedGeneration
			client.lastUpdated = time.Now()
		}
	}
	c.clientsMu.Unlock()
	
	// TODO: Update CRD status subresource with applied generation and FB status
	
	return &pb.ConfigAckResponse{
		Status: 0,
	}, nil
}

// BroadcastConfig sends a configuration update to all connected clients
func (c *ConfigController) BroadcastConfig(newConfig *pb.PipelineConfig, generation int64) {
	c.logger.Printf(`{"level":"info","timestamp":"%s","message":"Broadcasting new config","generation":%d}`,
		time.Now().Format(time.RFC3339), generation)
	
	// Update current config
	c.configMu.Lock()
	c.pipelineConfig = newConfig
	c.currentGeneration = generation
	c.configMu.Unlock()
	
	// Prepare response
	resp := &pb.ConfigResponse{
		Status:       0,
		Generation:   generation,
		PipelineConfig: newConfig,
	}
	
	// Send to all clients
	c.clientsMu.RLock()
	defer c.clientsMu.RUnlock()
	
	var clientSendErrors int
	for fbID, fbClients := range c.clients {
		for instanceID, client := range fbClients {
			// Skip if client already has this or newer generation
			if client.genAcked >= generation {
				continue
			}
			
			if err := client.stream.Send(resp); err != nil {
				c.logger.Printf(`{"level":"error","timestamp":"%s","message":"Failed to send config update","fb_id":"%s","instance_id":"%s","error":"%s"}`,
					time.Now().Format(time.RFC3339), fbID, instanceID, err)
				clientSendErrors++
			}
		}
	}
	
	c.logger.Printf(`{"level":"info","timestamp":"%s","message":"Config broadcast complete","generation":%d,"clients_with_errors":%d}`,
		time.Now().Format(time.RFC3339), generation, clientSendErrors)
}

// GetClientStatus returns the status of all connected clients
func (c *ConfigController) GetClientStatus() map[string][]map[string]interface{} {
	status := make(map[string][]map[string]interface{})
	
	c.clientsMu.RLock()
	defer c.clientsMu.RUnlock()
	
	for fbID, fbClients := range c.clients {
		fbStatus := make([]map[string]interface{}, 0, len(fbClients))
		
		for instanceID, client := range fbClients {
			fbStatus = append(fbStatus, map[string]interface{}{
				"instance_id":   instanceID,
				"gen_acked":     client.genAcked,
				"last_updated":  client.lastUpdated.Format(time.RFC3339),
				"age_seconds":   int(time.Since(client.lastUpdated).Seconds()),
			})
		}
		
		status[fbID] = fbStatus
	}
	
	return status
}

// UpdateCRDStatus updates the status subresource of the NRDotPlusPipeline CRD
func (c *ConfigController) UpdateCRDStatus(crdName string) error {
	// TODO: Get current status
	// TODO: Update with client status
	// TODO: Update observedGeneration
	// TODO: Update configGenerationApplied
	// TODO: Update per-FB status
	
	return fmt.Errorf("not implemented yet")
}
