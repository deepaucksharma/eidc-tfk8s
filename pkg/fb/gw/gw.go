package gw

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/newrelic/nrdot-internal-devlab/internal/common/logging"
	"github.com/newrelic/nrdot-internal-devlab/internal/common/metrics"
	"github.com/newrelic/nrdot-internal-devlab/internal/common/resilience"
	"github.com/newrelic/nrdot-internal-devlab/internal/common/schema"
	"github.com/newrelic/nrdot-internal-devlab/internal/common/tracing"
	"github.com/newrelic/nrdot-internal-devlab/internal/config"
	"github.com/newrelic/nrdot-internal-devlab/pkg/fb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GWConfig contains configuration for the Gateway function block
type GWConfig struct {
	// Common configuration
	Common config.FBConfig `json:"common"`

	// GW-specific configuration
	SchemaEnforce   bool   `json:"schemaEnforce"`
	ExportEndpoint  string `json:"exportEndpoint"`
	PiiFields       []string `json:"piiFields"`
	EnablePiiDetection bool `json:"enablePiiDetection"`
}

// GW implements the FB-GW (Gateway) function block
type GW struct {
	fb.BaseFunctionBlock
	logger          *logging.Logger
	metrics         *metrics.FBMetrics
	tracer          *tracing.Tracer
	config          *GWConfig
	configMu        sync.RWMutex
	dlqClient       fb.ChainPushServiceClient
	dlqConn         *grpc.ClientConn
	circuitBreaker  *resilience.CircuitBreaker
	schemaValidator schema.SchemaValidator
}

// NewGW creates a new Gateway function block
func NewGW() *GW {
	return &GW{
		BaseFunctionBlock: fb.BaseFunctionBlock{
			name:  "fb-gw",
			ready: false,
		},
		logger:  logging.NewLogger("fb-gw"),
		metrics: metrics.NewFBMetrics("fb-gw"),
		tracer:  tracing.NewTracer("fb-gw"),
	}
}

// Initialize initializes the Gateway function block
func (g *GW) Initialize(ctx context.Context) error {
	g.logger.Info("Initializing FB-GW", nil)

	// Initialize circuit breaker with default config
	g.circuitBreaker = resilience.NewCircuitBreaker("fb-gw", resilience.DefaultCircuitBreakerConfig())

	// Mark as ready (full readiness will be set after config is loaded)
	g.BaseFunctionBlock.ready = true

	return nil
}

// ProcessBatch processes a batch of metrics
func (g *GW) ProcessBatch(ctx context.Context, batch *fb.MetricBatch) (*fb.ProcessResult, error) {
	// Create child span for the batch processing
	ctx, span := g.tracer.StartSpan(ctx, "process-batch", nil)
	defer span.End()

	// Record metric
	g.metrics.RecordBatchReceived()

	startTime := time.Now()

	// Process batch
	processingErr := g.processBatch(ctx, batch)
	if processingErr != nil {
		g.metrics.RecordProcessingError()
		
		// If schema validation fails, send to DLQ
		if processingErr.Error() == schema.ErrPIIDetected.Error() || 
		   processingErr.Error() == schema.ErrInvalidFieldValue.Error() ||
		   processingErr.Error() == schema.ErrInvalidFieldType.Error() ||
		   processingErr.Error() == schema.ErrMissingRequiredField.Error() {
			
			dlqErr := g.sendToDLQ(ctx, batch, processingErr)
			if dlqErr != nil {
				g.logger.Error("Failed to send to DLQ after schema validation failure", dlqErr, map[string]interface{}{
					"batch_id": batch.BatchID,
				})
				return fb.NewErrorResult(batch.BatchID, fb.ErrorCodeDLQSendFailed, dlqErr, false), dlqErr
			}
			
			// Return error with DLQ status
			return fb.NewErrorResult(batch.BatchID, fb.ErrorCodeInvalidInput, processingErr, true), processingErr
		}
		
		return fb.NewErrorResult(batch.BatchID, fb.ErrorCodeProcessingFailed, processingErr, false), processingErr
	}

	// Record processing metrics
	g.metrics.RecordBatchProcessed(time.Since(startTime).Seconds())

	return fb.NewSuccessResult(batch.BatchID), nil
}

// processBatch performs the actual batch processing
func (g *GW) processBatch(ctx context.Context, batch *fb.MetricBatch) error {
	// Create child span for processing
	ctx, span := g.tracer.StartSpan(ctx, "gw-process", nil)
	defer span.End()

	// Get current config
	g.configMu.RLock()
	config := g.config
	g.configMu.RUnlock()

	// Schema validation if enabled
	if config != nil && config.SchemaEnforce && g.schemaValidator != nil {
		var data interface{}
		if err := json.Unmarshal(batch.Data, &data); err != nil {
			return fmt.Errorf("failed to parse batch data: %w", err)
		}
		
		// Validate against schema
		result := g.schemaValidator.Validate(data)
		if !result.Valid {
			g.logger.Error("Schema validation failed", result.Error, map[string]interface{}{
				"batch_id": batch.BatchID,
				"path":     result.Path,
			})
			g.metrics.RecordBatchValidationError()
			return result.Error
		}
	}

	// Export to the configured endpoint
	// TODO: Implement the actual export logic
	
	return nil
}

// sendToDLQ sends a batch to the Dead Letter Queue
func (g *GW) sendToDLQ(ctx context.Context, batch *fb.MetricBatch, originalErr error) error {
	// Create child span for DLQ
	ctx, span := g.tracer.StartSpan(ctx, "send-to-dlq", nil)
	defer span.End()

	// Ensure we have a connection to the DLQ
	if g.dlqClient == nil {
		return fmt.Errorf("no connection to DLQ")
	}

	// Add error info to internal labels
	if batch.InternalLabels == nil {
		batch.InternalLabels = make(map[string]string)
	}
	batch.InternalLabels["error"] = originalErr.Error()
	batch.InternalLabels["fb_sender"] = g.Name()
	batch.InternalLabels["error_code"] = string(fb.ErrorCodeInvalidInput)

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
	res, err := g.dlqClient.PushMetrics(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to push metrics to DLQ: %w", err)
	}

	// Check response
	if res.Status != fb.StatusSuccess {
		return fmt.Errorf("DLQ returned error: %s (code: %s)", res.ErrorMessage, res.ErrorCode)
	}

	// Record metric
	g.metrics.RecordBatchDLQ()

	return nil
}

// UpdateConfig updates the Gateway function block's configuration
func (g *GW) UpdateConfig(ctx context.Context, configBytes []byte, generation int64) error {
	// Create child span for config update
	ctx, span := g.tracer.StartSpan(ctx, "update-config", nil)
	defer span.End()

	// Parse configuration
	var newConfig GWConfig
	if err := json.Unmarshal(configBytes, &newConfig); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	// Validate configuration
	if err := g.validateConfig(&newConfig); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// Apply configuration
	g.configMu.Lock()
	g.config = &newConfig
	g.configGeneration = generation
	g.configMu.Unlock()

	// Update circuit breaker configuration
	g.circuitBreaker = resilience.NewCircuitBreaker("fb-gw", resilience.CircuitBreakerConfig{
		ErrorThresholdPercentage: newConfig.Common.CircuitBreaker.ErrorThresholdPercentage,
		OpenStateSeconds:         newConfig.Common.CircuitBreaker.OpenStateSeconds,
		HalfOpenRequestThreshold: newConfig.Common.CircuitBreaker.HalfOpenRequestThreshold,
	})

	// Initialize schema validator if schema enforcement is enabled
	if newConfig.SchemaEnforce {
		// TODO: Load the actual schema JSON from a file or config
		schemaJSON := `{
			"type": "object",
			"required": ["resource_metrics"],
			"properties": {
				"resource_metrics": {
					"type": "array",
					"items": {
						"type": "object",
						"properties": {
							"resource": {
								"type": "object"
							},
							"scope_metrics": {
								"type": "array"
							}
						}
					}
				},
				"internal_labels": {
					"type": "object"
				}
			}
		}`
		
		validator, err := schema.NewJSONSchemaValidator(schemaJSON, newConfig.PiiFields, newConfig.EnablePiiDetection)
		if err != nil {
			g.logger.Error("Failed to initialize schema validator", err, nil)
			// Don't fail config update on validator init error
		} else {
			g.schemaValidator = validator
		}
	}

	// Connect to DLQ if not already connected
	if g.dlqClient == nil {
		if err := g.connectToDLQ(ctx, newConfig.Common.DLQ); err != nil {
			g.logger.Error("Failed to connect to DLQ", err, map[string]interface{}{
				"dlq_service": newConfig.Common.DLQ,
			})
			// Don't fail config update on connection error - we'll retry on next batch
		}
	}

	// Update metrics
	g.metrics.SetConfigGeneration(generation)
	g.metrics.SetReady(true)

	g.logger.Info("Config updated", map[string]interface{}{
		"generation":      generation,
		"schema_enforce":  newConfig.SchemaEnforce,
		"export_endpoint": newConfig.ExportEndpoint,
	})

	return nil
}

// validateConfig validates the Gateway function block's configuration
func (g *GW) validateConfig(config *GWConfig) error {
	// Check if export endpoint is configured
	if config.ExportEndpoint == "" {
		return fmt.Errorf("export endpoint not configured")
	}

	// Check if DLQ is configured
	if config.Common.DLQ == "" {
		return fmt.Errorf("DLQ not configured")
	}

	return nil
}

// connectToDLQ establishes a connection to the DLQ function block
func (g *GW) connectToDLQ(ctx context.Context, dlqAddr string) error {
	// Close existing connection if any
	if g.dlqConn != nil {
		g.dlqConn.Close()
		g.dlqConn = nil
		g.dlqClient = nil
	}

	// Create new connection
	conn, err := grpc.DialContext(ctx, dlqAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to DLQ: %w", err)
	}

	g.dlqConn = conn
	g.dlqClient = fb.NewChainPushServiceClient(conn)
	
	return nil
}

// Shutdown shuts down the Gateway function block
func (g *GW) Shutdown(ctx context.Context) error {
	g.logger.Info("Shutting down FB-GW", nil)

	// Close connections
	if g.dlqConn != nil {
		g.dlqConn.Close()
		g.dlqConn = nil
		g.dlqClient = nil
	}

	// Mark as not ready
	g.BaseFunctionBlock.ready = false

	return nil
}

// SetDLQClientForTesting sets the DLQ client for testing purposes
func (g *GW) SetDLQClientForTesting(client fb.ChainPushServiceClient) {
	g.dlqClient = client
}

// SetSchemaValidatorForTesting sets the schema validator for testing purposes
func (g *GW) SetSchemaValidatorForTesting(validator schema.SchemaValidator) {
	g.schemaValidator = validator
}