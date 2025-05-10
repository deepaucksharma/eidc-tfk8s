package config

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// ConfigClient is a client for the Config service
type ConfigClient struct {
	client          ConfigServiceClient
	conn            *grpc.ClientConn
	fbName          string
	instanceID      string
	config          []byte
	configGeneration int64
	configMu        sync.RWMutex
	callbacks       []func([]byte, int64) error
	logger          Logger
}

// Logger interface for logging
type Logger interface {
	Info(msg string, keyValues map[string]interface{})
	Error(msg string, err error, keyValues map[string]interface{})
	Warn(msg string, keyValues map[string]interface{})
	Debug(msg string, keyValues map[string]interface{})
}

// NewConfigClient creates a new Config client
func NewConfigClient(fbName, instanceID, configServiceAddr string, logger Logger) (*ConfigClient, error) {
	// Connect to the config service
	conn, err := grpc.Dial(configServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to config service: %w", err)
	}

	// Create the client
	client := ConfigServiceClient(NewConfigServiceClient(conn))

	return &ConfigClient{
		client:     client,
		conn:       conn,
		fbName:     fbName,
		instanceID: instanceID,
		logger:     logger,
		callbacks:  make([]func([]byte, int64) error, 0),
	}, nil
}

// Start starts the config client
func (c *ConfigClient) Start(ctx context.Context) error {
	// Get initial config
	res, err := c.getConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to get initial config: %w", err)
	}

	// Update local config
	c.updateConfig(res.Config, res.Generation)

	// Start watching for config updates
	go c.watchConfig(ctx)

	return nil
}

// RegisterCallback registers a callback to be called when the config is updated
func (c *ConfigClient) RegisterCallback(callback func([]byte, int64) error) {
	c.configMu.Lock()
	defer c.configMu.Unlock()

	c.callbacks = append(c.callbacks, callback)
}

// GetConfig returns the current config
func (c *ConfigClient) GetConfig() []byte {
	c.configMu.RLock()
	defer c.configMu.RUnlock()

	return c.config
}

// GetCurrentGeneration returns the current config generation
func (c *ConfigClient) GetCurrentGeneration() int64 {
	c.configMu.RLock()
	defer c.configMu.RUnlock()

	return c.configGeneration
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

	// DLQ endpoint
	DLQ string `json:"dlq"`

	// Circuit breaker configuration
	CircuitBreaker CircuitBreakerConfig `json:"circuit_breaker"`
}

// CircuitBreakerConfig represents circuit breaker configuration
type CircuitBreakerConfig struct {
	ErrorThresholdPercentage int `json:"error_threshold_percentage"`
	OpenStateSeconds         int `json:"open_state_seconds"`
	HalfOpenRequestThreshold int `json:"half_open_request_threshold"`
}