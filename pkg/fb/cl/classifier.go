package cl

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/newrelic/nrdot-internal-devlab/internal/common/logging"
	"github.com/newrelic/nrdot-internal-devlab/internal/common/metrics"
	"github.com/newrelic/nrdot-internal-devlab/internal/common/resilience"
	"github.com/newrelic/nrdot-internal-devlab/internal/common/tracing"
	"github.com/newrelic/nrdot-internal-devlab/internal/config"
	"github.com/newrelic/nrdot-internal-devlab/pkg/fb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ClassifierConfig contains configuration for the CL function block
type ClassifierConfig struct {
	// Common configuration
	Common config.FBConfig `json:"common"`

	// CL-specific configuration
	PIIFields        []string `json:"pii_fields"`
	SaltSecretName   string   `json:"salt_secret_name"`
	SaltSecretKey    string   `json:"salt_secret_key"`
	HashAlgorithm    string   `json:"hash_algorithm"`
}

// Classifier implements the FB-CL function block
type Classifier struct {
	fb.BaseFunctionBlock
	logger          *logging.Logger
	metrics         *metrics.FBMetrics
	tracer          *tracing.Tracer
	config          *ClassifierConfig
	configMu        sync.RWMutex
	nextFBClient    fb.ChainPushServiceClient
	nextFBConn      *grpc.ClientConn
	dlqClient       fb.ChainPushServiceClient
	dlqConn         *grpc.ClientConn
	circuitBreaker  *resilience.CircuitBreaker
	salt            string
	saltSecretName  string
	saltSecretKey   string
	saltMu          sync.RWMutex
}

// NewClassifier creates a new CL function block
func NewClassifier(logger *logging.Logger, metrics *metrics.FBMetrics, tracer *tracing.Tracer, saltSecretName, saltSecretKey string) *Classifier {
	return &Classifier{
		BaseFunctionBlock: fb.BaseFunctionBlock{
			name:  "fb-cl",
			ready: false,
		},
		logger:         logger,
		metrics:        metrics,
		tracer:         tracer,
		saltSecretName: saltSecretName,
		saltSecretKey:  saltSecretKey,
	}
}

// Initialize initializes the CL function block
func (c *Classifier) Initialize(ctx context.Context) error {
	c.logger.Info("Initializing FB-CL", nil)

	// Initialize circuit breaker
	c.circuitBreaker = resilience.NewCircuitBreaker("fb-cl", resilience.DefaultCircuitBreakerConfig())

	// Load initial salt value
	if err := c.loadSalt(ctx); err != nil {
		c.logger.Error("Failed to load salt", err, nil)
		// Don't fail initialization on salt load failure
		// We'll use a default salt and retry loading later
		c.saltMu.Lock()
		c.salt = "default-salt-value-replace-in-production"
		c.saltMu.Unlock()
	}

	// Mark as ready (full readiness will be set after config is loaded)
	c.BaseFunctionBlock.ready = true

	return nil
}

// loadSalt loads the salt value from a Kubernetes secret
func (c *Classifier) loadSalt(ctx context.Context) error {
	// In a real implementation, this would read from a Kubernetes secret
	// For now, we'll just use a simulated value
	c.saltMu.Lock()
	defer c.saltMu.Unlock()
	
	c.salt = "simulated-salt-value-" + time.Now().Format(time.RFC3339)
	c.logger.Info("Loaded salt value", map[string]interface{}{
		"salt_secret_name": c.saltSecretName,
		"salt_secret_key":  c.saltSecretKey,
		// Don't log the actual salt value in production!
	})
	
	return nil
}

// ProcessBatch processes a batch of metrics
func (c *Classifier) ProcessBatch(ctx context.Context, batch *fb.MetricBatch) (*fb.ProcessResult, error) {
	// Create child span for the batch processing
	ctx, span := c.tracer.StartSpan(ctx, "process-batch", nil)
	defer span.End()

	// Record metric
	c.metrics.RecordBatchReceived()

	startTime := time.Now()

	// Process batch
	processingErr := c.processBatch(ctx, batch)
	if processingErr != nil {
		// Check if it's a PII leak error, which is a special case
		if strings.Contains(processingErr.Error(), "PII leak detected") {
			c.metrics.RecordProcessingError()
			// Send to DLQ immediately for PII leaks
			dlqErr := c.sendToDLQ(ctx, batch, processingErr)
			if dlqErr != nil {
				c.logger.Error("Failed to send to DLQ after PII leak detection", dlqErr, map[string]interface{}{
					"batch_id": batch.BatchID,
				})
				return fb.NewErrorResult(batch.BatchID, fb.ErrorCodeDLQSendFailed, dlqErr, false), dlqErr
			}
			return fb.NewErrorResult(batch.BatchID, fb.ErrorCodePIILeak, processingErr, true), processingErr
		}
		
		c.metrics.RecordProcessingError()
		return fb.NewErrorResult(batch.BatchID, fb.ErrorCodeProcessingFailed, processingErr, false), processingErr
	}

	// Record processing metrics
	c.metrics.RecordBatchProcessed(time.Since(startTime).Seconds())

	// Forward to next FB
	forwardingResult, forwardingErr := c.forwardToNextFB(ctx, batch)
	if forwardingErr != nil {
		// If forwarding fails but processing succeeded, attempt to send to DLQ
		dlqErr := c.sendToDLQ(ctx, batch, forwardingErr)
		if dlqErr != nil {
			c.logger.Error("Failed to send to DLQ after forwarding failure", dlqErr, map[string]interface{}{
				"batch_id": batch.BatchID,
			})
			return fb.NewErrorResult(batch.BatchID, fb.ErrorCodeDLQSendFailed, dlqErr, false), dlqErr
		}
		
		// Return error with DLQ status
		return fb.NewErrorResult(batch.BatchID, fb.ErrorCodeForwardingFailed, forwardingErr, true), forwardingErr
	}

	return forwardingResult, nil
}

// processBatch performs classification and PII handling on the batch
func (c *Classifier) processBatch(ctx context.Context, batch *fb.MetricBatch) error {
	// Get the current config and salt
	c.configMu.RLock()
	piiFields := c.config.PIIFields
	c.configMu.RUnlock()
	
	c.saltMu.RLock()
	salt := c.salt
	c.saltMu.RUnlock()
	
	// TODO: In a real implementation, this would:
	// 1. Parse the batch data based on format (OTLP, Prometheus, etc.)
	// 2. Scan for PII fields based on configuration
	// 3. Hash PII fields with the salt value
	// 4. Update the batch data with the hashed values
	
	// For now, we'll just simulate the process
	// This would be replaced with actual classification logic in a real implementation
	time.Sleep(5 * time.Millisecond) // Simulate processing time
	
	// Check for PII leaks (simulated)
	// In a real implementation, this would be a more sophisticated check
	if strings.Contains(string(batch.Data), "command_line:") && !strings.Contains(string(batch.Data), "command_line_hash:") {
		return fmt.Errorf("PII leak detected: unhashed command_line field found")
	}
	
	return nil
}

// hashPIIValue hashes a PII value using the configured algorithm and salt
func (c *Classifier) hashPIIValue(value, salt string) string {
	// For now, we only support SHA-256
	hasher := sha256.New()
	hasher.Write([]byte(salt + value))
	return hex.EncodeToString(hasher.Sum(nil))
}

// forwardToNextFB forwards the batch to the next function block
func (c *Classifier) forwardToNextFB(ctx context.Context, batch *fb.MetricBatch) (*fb.ProcessResult, error) {
	startTime := time.Now()

	// Use circuit breaker to protect against downstream failures
	err := c.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		// Get the current config
		c.configMu.RLock()
		nextFB := c.config.Common.NextFB
		c.configMu.RUnlock()

		// Ensure we have a connection to the next FB
		if c.nextFBClient == nil {
			return fmt.Errorf("no connection to next FB: %s", nextFB)
		}

		// Create child span for forwarding
		ctx, span := c.tracer.StartSpan(ctx, "forward-to-next-fb", nil)
		defer span.End()

		// Convert to ChainPushService request
		req := &fb.MetricBatchRequest{
			BatchId:          batch.BatchID,
			Data:             batch.Data,
			Format:           batch.Format,
			Replay:           batch.Replay,
			ConfigGeneration: batch.ConfigGeneration,
			Metadata:         batch.Metadata,
			InternalLabels:   batch.InternalLabels,
		}

		// Forward to next FB
		res, err := c.nextFBClient.PushMetrics(ctx, req)
		if err != nil {
			return fmt.Errorf("failed to push metrics to next FB: %w", err)
		}

		// Check response
		if res.Status != fb.StatusSuccess {
			return fmt.Errorf("next FB returned error: %s (code: %s)", res.ErrorMessage, res.ErrorCode)
		}

		return nil
	})

	// Record metrics
	c.metrics.RecordBatchForwarded(time.Since(startTime).Seconds())

	if err != nil {
		if err == resilience.ErrCircuitOpen {
			return fb.NewErrorResult(batch.BatchID, fb.ErrorCodeCircuitBreakerOpen, err, false), err
		}
		return fb.NewErrorResult(batch.BatchID, fb.ErrorCodeForwardingFailed, err, false), err
	}

	return fb.NewSuccessResult(batch.BatchID), nil
}

// sendToDLQ sends a batch to the Dead Letter Queue
func (c *Classifier) sendToDLQ(ctx context.Context, batch *fb.MetricBatch, originalErr error) error {
	// Create child span for DLQ
	ctx, span := c.tracer.StartSpan(ctx, "send-to-dlq", nil)
	defer span.End()

	// Ensure we have a connection to the DLQ
	if c.dlqClient == nil {
		return fmt.Errorf("no connection to DLQ")
	}

	// Add error info to internal labels
	if batch.InternalLabels == nil {
		batch.InternalLabels = make(map[string]string)
	}
	batch.InternalLabels["error"] = originalErr.Error()
	batch.InternalLabels["fb_sender"] = c.Name()
	
	// Add the error code for PII leaks
	if strings.Contains(originalErr.Error(), "PII leak detected") {
		batch.InternalLabels["error_code"] = string(fb.ErrorCodePIILeak)
	}

	// Convert to ChainPushService request
	req := &fb.MetricBatchRequest{
		BatchId:          batch.BatchID,
		Data:             batch.Data,
		Format:           batch.Format,
		Replay:           batch.Replay,
		ConfigGeneration: batch.ConfigGeneration,
		Metadata:         batch.Metadata,
		InternalLabels:   batch.InternalLabels,
	}

	// Send to DLQ
	res, err := c.dlqClient.PushMetrics(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to push metrics to DLQ: %w", err)
	}

	// Check response
	if res.Status != fb.StatusSuccess {
		return fmt.Errorf("DLQ returned error: %s (code: %s)", res.ErrorMessage, res.ErrorCode)
	}

	// Record metric
	c.metrics.RecordBatchDLQ()

	return nil
}

// UpdateConfig updates the CL function block's configuration
func (c *Classifier) UpdateConfig(ctx context.Context, configBytes []byte, generation int64) error {
	// Create child span for config update
	ctx, span := c.tracer.StartSpan(ctx, "update-config", nil)
	defer span.End()

	// Parse configuration
	var newConfig ClassifierConfig
	if err := json.Unmarshal(configBytes, &newConfig); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	// Validate configuration
	if err := c.validateConfig(&newConfig); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// Get the current config to check for salt changes
	c.configMu.RLock()
	oldSaltSecretName := ""
	oldSaltSecretKey := ""
	if c.config != nil {
		oldSaltSecretName = c.config.SaltSecretName
		oldSaltSecretKey = c.config.SaltSecretKey
	}
	c.configMu.RUnlock()
	
	// Apply configuration
	c.configMu.Lock()
	c.config = &newConfig
	c.configGeneration = generation
	c.configMu.Unlock()

	// Update circuit breaker configuration
	c.circuitBreaker = resilience.NewCircuitBreaker("fb-cl", resilience.CircuitBreakerConfig{
		ErrorThresholdPercentage: newConfig.Common.CircuitBreaker.ErrorThresholdPercentage,
		OpenStateSeconds:         newConfig.Common.CircuitBreaker.OpenStateSeconds,
		HalfOpenRequestThreshold: newConfig.Common.CircuitBreaker.HalfOpenRequestThreshold,
	})

	// Update salt if the secret name or key changed
	if newConfig.SaltSecretName != oldSaltSecretName || newConfig.SaltSecretKey != oldSaltSecretKey {
		c.saltSecretName = newConfig.SaltSecretName
		c.saltSecretKey = newConfig.SaltSecretKey
		if err := c.loadSalt(ctx); err != nil {
			c.logger.Error("Failed to load salt after config update", err, map[string]interface{}{
				"salt_secret_name": newConfig.SaltSecretName,
				"salt_secret_key":  newConfig.SaltSecretKey,
			})
			// Don't fail config update on salt load failure
		}
	}

	// Connect to next FB and DLQ
	if err := c.connectToNextFB(ctx, newConfig.Common.NextFB); err != nil {
		c.logger.Error("Failed to connect to next FB", err, map[string]interface{}{
			"next_fb": newConfig.Common.NextFB,
		})
		// Don't fail config update on connection error - we'll retry on next batch
	}

	// Update metrics
	c.metrics.SetConfigGeneration(generation)
	c.metrics.SetReady(true)

	c.logger.Info("Config updated", map[string]interface{}{
		"generation":       generation,
		"next_fb":          newConfig.Common.NextFB,
		"pii_fields_count": len(newConfig.PIIFields),
		"salt_secret_name": newConfig.SaltSecretName,
	})

	return nil
}

// validateConfig validates the CL function block's configuration
func (c *Classifier) validateConfig(config *ClassifierConfig) error {
	// Check if next FB is configured
	if config.Common.NextFB == "" {
		return fmt.Errorf("next FB not configured")
	}

	// Check if salt secret is configured
	if config.SaltSecretName == "" || config.SaltSecretKey == "" {
		return fmt.Errorf("salt secret not configured")
	}

	// Check if hash algorithm is valid
	if config.HashAlgorithm != "" && config.HashAlgorithm != "sha256" {
		return fmt.Errorf("invalid hash algorithm: %s", config.HashAlgorithm)
	}

	return nil
}

// ConnectServices connects to the config service, next FB, and DLQ
func (c *Classifier) ConnectServices(ctx context.Context, configServiceAddr, nextFB, dlqAddr string) error {
	// Connect to config service
	// In a real implementation, this would:
	// 1. Connect to the config service
	// 2. Register for config updates
	// 3. Apply initial configuration
	
	// For now, we'll just create a default config
	c.config = &ClassifierConfig{
		Common: config.FBConfig{
			LogLevel:           "info",
			MetricsEnabled:     true,
			TracingEnabled:     true,
			TraceSamplingRatio: 0.1,
			NextFB:             nextFB,
			CircuitBreaker: config.CircuitBreakerConfig{
				ErrorThresholdPercentage: 50,
				OpenStateSeconds:         30,
				HalfOpenRequestThreshold: 5,
			},
		},
		PIIFields: []string{
			"command_line",
			"user_name",
			"email",
			"ip_address",
		},
		SaltSecretName: c.saltSecretName,
		SaltSecretKey:  c.saltSecretKey,
		HashAlgorithm:  "sha256",
	}
	c.configGeneration = 1
	
	// Connect to next FB
	if err := c.connectToNextFB(ctx, nextFB); err != nil {
		return fmt.Errorf("failed to connect to next FB: %w", err)
	}
	
	// Connect to DLQ
	if err := c.connectToDLQ(ctx, dlqAddr); err != nil {
		return fmt.Errorf("failed to connect to DLQ: %w", err)
	}
	
	c.logger.Info("Connected to services", map[string]interface{}{
		"config_service": configServiceAddr,
		"next_fb":        nextFB,
		"dlq":            dlqAddr,
	})
	
	return nil
}

// connectToNextFB establishes a connection to the next function block
func (c *Classifier) connectToNextFB(ctx context.Context, nextFB string) error {
	// Close existing connection if any
	if c.nextFBConn != nil {
		c.nextFBConn.Close()
		c.nextFBConn = nil
		c.nextFBClient = nil
	}

	// Create new connection
	conn, err := grpc.DialContext(ctx, nextFB,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to next FB: %w", err)
	}

	c.nextFBConn = conn
	c.nextFBClient = fb.NewChainPushServiceClient(conn)
	
	return nil
}

// connectToDLQ establishes a connection to the DLQ function block
func (c *Classifier) connectToDLQ(ctx context.Context, dlqAddr string) error {
	// Close existing connection if any
	if c.dlqConn != nil {
		c.dlqConn.Close()
		c.dlqConn = nil
		c.dlqClient = nil
	}

	// Create new connection
	conn, err := grpc.DialContext(ctx, dlqAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to DLQ: %w", err)
	}

	c.dlqConn = conn
	c.dlqClient = fb.NewChainPushServiceClient(conn)
	
	return nil
}

// Shutdown shuts down the CL function block
func (c *Classifier) Shutdown(ctx context.Context) error {
	c.logger.Info("Shutting down FB-CL", nil)

	// Close connections
	if c.nextFBConn != nil {
		c.nextFBConn.Close()
		c.nextFBConn = nil
		c.nextFBClient = nil
	}

	if c.dlqConn != nil {
		c.dlqConn.Close()
		c.dlqConn = nil
		c.dlqClient = nil
	}

	// Mark as not ready
	c.BaseFunctionBlock.ready = false

	return nil
}

// StartGRPCServer starts the gRPC server for the ChainPushService
func StartGRPCServer(ctx context.Context, fb *Classifier, port int) (*grpc.Server, error) {
	// Create gRPC server
	server := grpc.NewServer()

	// Register the ChainPushService
	fb.logger.Info("Registering ChainPushService", map[string]interface{}{"port": port})
	handler := fb.NewChainPushServiceHandler()
	fb.RegisterChainPushServiceServer(server, handler)

	// Start gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, fmt.Errorf("failed to listen on port %d: %w", port, err)
	}

	// Start server in a goroutine
	go func() {
		fb.logger.Info("Starting gRPC server", map[string]interface{}{"port": port})
		if err := server.Serve(lis); err != nil {
			fb.logger.Error("gRPC server failed", err, nil)
		}
	}()

	return server, nil
}

// NewChainPushServiceHandler creates a new ChainPushServiceHandler for this FB
func (c *Classifier) NewChainPushServiceHandler() fb.ChainPushServiceServer {
	return fb.NewChainPushServiceHandler(c)
}

// RegisterChainPushServiceServer registers the ChainPushService with the gRPC server
func (c *Classifier) RegisterChainPushServiceServer(server *grpc.Server, handler fb.ChainPushServiceServer) {
	fb.RegisterChainPushServiceServer(server, handler)
}
