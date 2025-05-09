package config

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/newrelic/nrdot-internal-devlab/internal/common/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ConfigClient manages the connection to the config controller and streams config updates
type ConfigClient struct {
	fbName          string
	instanceID      string
	configGeneration int64
	configMu        sync.RWMutex
	config          []byte
	conn            *grpc.ClientConn
	client          ConfigServiceClient
	logger          *logging.Logger
	callbacks       []func([]byte, int64) error
}

// NewConfigClient creates a new config client for the specified function block
func NewConfigClient(fbName, instanceID string, logger *logging.Logger) *ConfigClient {
	return &ConfigClient{
		fbName:          fbName,
		instanceID:      instanceID,
		configGeneration: 0,
		logger:          logger,
		callbacks:       make([]func([]byte, int64) error, 0),
	}
}

// Connect establishes a connection to the config controller
func (c *ConfigClient) Connect(ctx context.Context, configServiceAddr string) error {
	// Create gRPC connection
	conn, err := grpc.DialContext(ctx, configServiceAddr, 
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(5*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to config service: %w", err)
	}
	
	c.conn = conn
	c.client = NewConfigServiceClient(conn)
	return nil
}

// GetCurrentConfig returns the current configuration
func (c *ConfigClient) GetCurrentConfig() []byte {
	c.configMu.RLock()
	defer c.configMu.RUnlock()
	return c.config
}

// GetCurrentGeneration returns the current configuration generation
func (c *ConfigClient) GetCurrentGeneration() int64 {
	c.configMu.RLock()
	defer c.configMu.RUnlock()
	return c.configGeneration
}

// RegisterCallback registers a callback function to be called when config is updated
func (c *ConfigClient) RegisterCallback(callback func([]byte, int64) error) {
	c.callbacks = append(c.callbacks, callback)
}

// StartConfigWatcher starts watching for configuration updates
func (c *ConfigClient) StartConfigWatcher(ctx context.Context) error {
	if c.client == nil {
		return fmt.Errorf("not connected to config service")
	}

	// First fetch initial config
	initialConfig, err := c.getConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to get initial config: %w", err)
	}

	// Update local config
	c.updateConfig(initialConfig.Config, initialConfig.Generation)

	// Start watching for config updates
	go c.watchConfig(ctx)

	return nil
}

// getConfig gets the latest configuration from the config service
func (c *ConfigClient) getConfig(ctx context.Context) (*ConfigResponse, error) {
	req := &ConfigRequest{
		FbName:             c.fbName,
		InstanceId:         c.instanceID,
		LastKnownGeneration: c.GetCurrentGeneration(),
	}

	res, err := c.client.GetConfig(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get config: %w", err)
	}

	return res, nil
}

// watchConfig watches for configuration updates
func (c *ConfigClient) watchConfig(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			c.logger.Info("Starting config watch stream", map[string]interface{}{
				"config_generation": c.GetCurrentGeneration(),
			})

			// Create stream
			req := &ConfigRequest{
				FbName:             c.fbName,
				InstanceId:         c.instanceID,
				LastKnownGeneration: c.GetCurrentGeneration(),
			}

			stream, err := c.client.StreamConfig(ctx, req)
			if err != nil {
				c.logger.Error("Failed to create config stream", err, map[string]interface{}{})
				time.Sleep(5 * time.Second)
				continue
			}

			// Process config updates
			for {
				res, err := stream.Recv()
				if err != nil {
					c.logger.Error("Config stream error", err, map[string]interface{}{})
					break
				}

				c.logger.Info("Received config update", map[string]interface{}{
					"old_generation": c.GetCurrentGeneration(),
					"new_generation": res.Generation,
					"requires_restart": res.RequiresRestart,
				})

				// Update local config
				c.updateConfig(res.Config, res.Generation)

				// Send acknowledgement
				ackReq := &ConfigAckRequest{
					FbName:     c.fbName,
					InstanceId: c.instanceID,
					Generation: res.Generation,
					Success:    true,
				}

				_, ackErr := c.client.AckConfig(ctx, ackReq)
				if ackErr != nil {
					c.logger.Error("Failed to acknowledge config update", ackErr, map[string]interface{}{
						"generation": res.Generation,
					})
				}
			}

			// If stream ends, wait and retry
			time.Sleep(5 * time.Second)
		}
	}
}

// updateConfig updates the local configuration and calls registered callbacks
func (c *ConfigClient) updateConfig(configBytes []byte, generation int64) {
	c.configMu.Lock()
	defer c.configMu.Unlock()

	// Skip if already at this generation or higher
	if c.configGeneration >= generation {
		return
	}

	// Update local config
	c.config = configBytes
	c.configGeneration = generation

	// Call registered callbacks
	for _, callback := range c.callbacks {
		if err := callback(configBytes, generation); err != nil {
			c.logger.Error("Config update callback failed", err, map[string]interface{}{
				"generation": generation,
			})
		}
	}
}

// Close closes the connection to the config service
func (c *ConfigClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// LoadConfigFromBytes loads a configuration from a byte slice
func LoadConfigFromBytes(configBytes []byte, config interface{}) error {
	return json.Unmarshal(configBytes, config)
}

// Common configuration types

// FBConfig represents common configuration for all function blocks
type FBConfig struct {
	// Common configuration fields
	LogLevel           string `json:"log_level"`
	MetricsEnabled     bool   `json:"metrics_enabled"`
	TracingEnabled     bool   `json:"tracing_enabled"`
	TraceSamplingRatio float64 `json:"trace_sampling_ratio"`

	// Next FB in the chain
	NextFB string `json:"next_fb"`

	// Circuit breaker configuration
	CircuitBreaker CircuitBreakerConfig `json:"circuit_breaker"`
}

// CircuitBreakerConfig represents circuit breaker configuration
type CircuitBreakerConfig struct {
	ErrorThresholdPercentage int `json:"error_threshold_percentage"`
	OpenStateSeconds         int `json:"open_state_seconds"`
	HalfOpenRequestThreshold int `json:"half_open_request_threshold"`
}

// Mock gRPC client types for compilation (would be generated from protobuf)
type ConfigServiceClient interface {
	GetConfig(ctx context.Context, in *ConfigRequest, opts ...grpc.CallOption) (*ConfigResponse, error)
	StreamConfig(ctx context.Context, in *ConfigRequest, opts ...grpc.CallOption) (ConfigService_StreamConfigClient, error)
	AckConfig(ctx context.Context, in *ConfigAckRequest, opts ...grpc.CallOption) (*ConfigAckResponse, error)
}

type ConfigService_StreamConfigClient interface {
	Recv() (*ConfigResponse, error)
	grpc.ClientStream
}

type ConfigRequest struct {
	FbName             string `json:"fb_name"`
	InstanceId         string `json:"instance_id"`
	LastKnownGeneration int64  `json:"last_known_generation"`
}

type ConfigResponse struct {
	Generation      int64  `json:"generation"`
	Config          []byte `json:"config"`
	RequiresRestart bool   `json:"requires_restart"`
	Timestamp       int64  `json:"timestamp"`
}

type ConfigAckRequest struct {
	FbName     string `json:"fb_name"`
	InstanceId string `json:"instance_id"`
	Generation int64  `json:"generation"`
	Success    bool   `json:"success"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type ConfigAckResponse struct {
	Recorded bool `json:"recorded"`
}

// NewConfigServiceClient creates a new client for the ConfigService
func NewConfigServiceClient(cc *grpc.ClientConn) ConfigServiceClient {
	// This is a mock implementation that would be generated from protobuf
	return nil
}
