package gw

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"eidc-tfk8s/internal/common/logging"
	"eidc-tfk8s/internal/common/metrics"
	"eidc-tfk8s/internal/common/resilience"
	"eidc-tfk8s/internal/common/schema"
	"eidc-tfk8s/internal/common/tracing"
	"eidc-tfk8s/internal/config"
	"eidc-tfk8s/pkg/fb"

	"go.opentelemetry.io/otel/codes"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GWConfig represents the configuration for the Gateway function block
type GWConfig struct {
	// Common configuration for all function blocks
	Common config.FBConfig `json:"common"`

	// Whether to enforce schema validation
	SchemaEnforce bool `json:"schema_enforce"`

	// Export endpoint URL
	ExportEndpoint string `json:"export_endpoint"`

	// PII fields to check for
	PiiFields []string `json:"pii_fields"`

	// Whether to enable PII detection
	EnablePiiDetection bool `json:"enable_pii_detection"`
}

// GW is the Gateway function block for exporting metrics
type GW struct {
	fb.BaseFunctionBlock
	logger          logging.Logger
	metrics         *metrics.FBMetrics
	tracer          *tracing.Tracer
	config          GWConfig
	exportClient    *grpc.ClientConn
	nextFBClient    fb.ChainPushServiceClient
	nextFBConn      *grpc.ClientConn
	dlqClient       fb.ChainPushServiceClient
	dlqConn         *grpc.ClientConn
	circuitBreaker  *resilience.CircuitBreaker
	schemaValidator schema.SchemaValidator
}

// NewGW creates a new Gateway function block
func NewGW() *GW {
	return &GW{
		BaseFunctionBlock: fb.BaseFunctionBlock{},
		logger:  logging.NewLogger("fb-gw"),
		metrics: metrics.NewFBMetrics("fb-gw"),
		tracer:  tracing.NewTracer("fb-gw"),
	}
}

// Initialize initializes the Gateway function block
func (g *GW) Initialize(ctx context.Context) error {
	// Set the name and ready state
	baseFB := fb.NewBaseFunctionBlock("fb-gw")
	g.BaseFunctionBlock = baseFB
	g.logger.Info("Initializing Gateway function block", map[string]interface{}{})
	g.SetReady(false)
	g.metrics.SetReady(0)

	// Initialize schema validator with default settings
	g.schemaValidator = schema.NewDefaultValidator()

	// Success
	g.logger.Info("Gateway function block initialized", map[string]interface{}{})
	g.SetReady(true)
	g.metrics.SetReady(1)
	return nil
}

// ProcessBatch processes a batch of metrics
func (g *GW) ProcessBatch(ctx context.Context, batch *fb.MetricBatch) (*fb.ProcessResult, error) {
	startTime := time.Now()
	
	// Start span for processing
	ctx, span := g.tracer.StartSpan(ctx, "GW.ProcessBatch")
	defer span.End()
	
	g.metrics.RecordBatchReceived()
	
	g.logger.Info("Processing batch", map[string]interface{}{
		"batch_id": batch.BatchID,
		"format":   batch.Format,
		"replay":   batch.Replay,
	})
	
	// Validate schema if enabled
	if g.config.SchemaEnforce {
		if err := g.validateSchema(ctx, batch); err != nil {
			// Schema validation failed, send to DLQ
			g.metrics.RecordBatchRejected()
			g.tracer.SetStatus(ctx, codes.Error, "Schema validation failed")
			
			// Send to DLQ if possible
			dlqResult, dlqErr := g.sendToDLQ(ctx, batch, fb.ErrorCodeInvalidInput, err)
			
			// Return error with info about DLQ
			return fb.NewErrorResult(
				batch.BatchID,
				fb.ErrorCodeInvalidInput,
				err,
				dlqResult != nil && dlqErr == nil,
			), err
		}
	}
	
	// Process the batch
	g.metrics.RecordBatchProcessed(time.Since(startTime).Seconds())
	
	// Forward to next FB (if configured)
	if g.config.Common.NextFB != "" {
		// Start span for forwarding
		forwardCtx, forwardSpan := g.tracer.StartSpan(ctx, "GW.ForwardBatch")
		defer forwardSpan.End()
		
		forwardStartTime := time.Now()
		result, err := g.forwardBatch(forwardCtx, batch)
		g.metrics.RecordBatchForwarded(time.Since(forwardStartTime).Seconds())
		
		if err != nil {
			g.logger.Error("Failed to forward batch", err, map[string]interface{}{
				"batch_id": batch.BatchID,
			})
			g.tracer.SetStatus(ctx, codes.Error, "Failed to forward batch")
			return result, err
		}
	}
	
	// Success
	g.logger.Info("Batch processed successfully", map[string]interface{}{
		"batch_id": batch.BatchID,
	})
	g.tracer.SetStatus(ctx, codes.Ok, "Batch processed successfully")
	
	return fb.NewSuccessResult(batch.BatchID), nil
}

// validateSchema validates the schema of a batch
func (g *GW) validateSchema(ctx context.Context, batch *fb.MetricBatch) error {
	ctx, span := g.tracer.StartSpan(ctx, "GW.ValidateSchema")
	defer span.End()
	
	g.logger.Debug("Validating schema", map[string]interface{}{
		"batch_id": batch.BatchID,
	})
	
	// Parse the data
	var data interface{}
	if err := json.Unmarshal(batch.Data, &data); err != nil {
		g.logger.Error("Failed to parse batch data", err, map[string]interface{}{
			"batch_id": batch.BatchID,
		})
		g.metrics.RecordBatchValidationError()
		return fmt.Errorf("failed to parse batch data: %w", err)
	}
	
	// Validate the schema
	result := g.schemaValidator.Validate(data)
	if !result.Valid {
		g.logger.Error("Schema validation failed", result.Error, map[string]interface{}{
			"batch_id": batch.BatchID,
			"path":     result.Path,
		})
		g.metrics.RecordBatchValidationError()
		return fmt.Errorf("schema validation failed at path '%s': %w", result.Path, result.Error)
	}
	
	return nil
}

// forwardBatch forwards a batch to the next function block
func (g *GW) forwardBatch(ctx context.Context, batch *fb.MetricBatch) (*fb.ProcessResult, error) {
	ctx, span := g.tracer.StartSpan(ctx, "GW.ForwardBatch")
	defer span.End()
	
	// Connect to next FB if not already connected
	if g.nextFBClient == nil {
		if err := g.connectToNextFB(ctx); err != nil {
			return fb.NewErrorResult(batch.BatchID, fb.ErrorCodeForwardingFailed, err, false), err
		}
	}
	
	// Use circuit breaker to protect against cascading failures
	err := g.circuitBreaker.Execute(ctx, func(execCtx context.Context) error {
		// Create request
		req := &fb.MetricBatchRequest{
			BatchId:          batch.BatchID,
			Data:             batch.Data,
			Format:           batch.Format,
			Replay:           batch.Replay,
			ConfigGeneration: batch.ConfigGeneration,
			Metadata:         batch.Metadata,
			InternalLabels:   batch.InternalLabels,
		}
		
		// Add sender label
		if req.InternalLabels == nil {
			req.InternalLabels = make(map[string]string)
		}
		req.InternalLabels["fb_sender"] = g.Name()
		
		// Forward to next FB
		res, err := g.nextFBClient.PushMetrics(execCtx, req)
		if err != nil {
			return fmt.Errorf("failed to push metrics to next FB: %w", err)
		}
		
		// Check response status
		if res.Status != fb.StatusSuccess {
			return fmt.Errorf("next FB returned error: %s - %s", res.ErrorCode, res.ErrorMessage)
		}
		
		return nil
	})
	
	// Handle error
	if err != nil {
		// Check if it's a circuit breaker error
		if errors.Is(err, resilience.ErrCircuitOpen) {
			g.logger.Warn("Circuit breaker is open", map[string]interface{}{
				"batch_id": batch.BatchID,
			})
			return fb.NewErrorResult(batch.BatchID, fb.ErrorCodeCircuitBreakerOpen, err, false), err
		}
		
		// Other error, try to send to DLQ
		g.logger.Error("Failed to forward batch", err, map[string]interface{}{
			"batch_id": batch.BatchID,
		})
		
		dlqResult, dlqErr := g.sendToDLQ(ctx, batch, fb.ErrorCodeForwardingFailed, err)
		
		// Return error with info about DLQ
		return fb.NewErrorResult(
			batch.BatchID,
			fb.ErrorCodeForwardingFailed,
			err,
			dlqResult != nil && dlqErr == nil,
		), err
	}
	
	return fb.NewSuccessResult(batch.BatchID), nil
}

// connectToNextFB connects to the next function block
func (g *GW) connectToNextFB(ctx context.Context) error {
	g.logger.Info("Connecting to next function block", map[string]interface{}{
		"next_fb": g.config.Common.NextFB,
	})
	
	// Create connection
	conn, err := grpc.Dial(g.config.Common.NextFB, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to next FB: %w", err)
	}
	
	// Create client
	g.nextFBConn = conn
	g.nextFBClient = fb.NewChainPushServiceClient(conn)
	
	// Create circuit breaker
	g.circuitBreaker = resilience.NewCircuitBreaker(
		"next-fb",
		resilience.CircuitBreakerConfig{
			ErrorThresholdPercentage: g.config.Common.CircuitBreaker.ErrorThresholdPercentage,
			OpenStateSeconds:         g.config.Common.CircuitBreaker.OpenStateSeconds,
			HalfOpenRequestThreshold: g.config.Common.CircuitBreaker.HalfOpenRequestThreshold,
		},
	)
	
	return nil
}

// connectToDLQ connects to the DLQ
func (g *GW) connectToDLQ(ctx context.Context) error {
	g.logger.Info("Connecting to DLQ", map[string]interface{}{
		"dlq": g.config.Common.DLQ,
	})
	
	// Create connection
	conn, err := grpc.Dial(g.config.Common.DLQ, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to DLQ: %w", err)
	}
	
	// Create client
	g.dlqConn = conn
	g.dlqClient = fb.NewChainPushServiceClient(conn)
	
	return nil
}

// sendToDLQ sends a batch to the DLQ
func (g *GW) sendToDLQ(ctx context.Context, batch *fb.MetricBatch, errorCode fb.ErrorCode, err error) (*fb.MetricBatchResponse, error) {
	// Connect to DLQ if not already connected
	if g.dlqClient == nil {
		if dlqErr := g.connectToDLQ(ctx); dlqErr != nil {
			g.logger.Error("Failed to connect to DLQ", dlqErr, map[string]interface{}{
				"batch_id": batch.BatchID,
			})
			return nil, fmt.Errorf("failed to connect to DLQ: %w", dlqErr)
		}
	}
	
	// Create request
	req := &fb.MetricBatchRequest{
		BatchId:          batch.BatchID,
		Data:             batch.Data,
		Format:           batch.Format,
		Replay:           batch.Replay,
		ConfigGeneration: batch.ConfigGeneration,
		Metadata:         batch.Metadata,
		InternalLabels:   make(map[string]string),
	}
	
	// Copy internal labels
	for k, v := range batch.InternalLabels {
		req.InternalLabels[k] = v
	}
	
	// Add error info
	req.InternalLabels["error_code"] = string(errorCode)
	if err != nil {
		req.InternalLabels["error"] = err.Error()
	}
	req.InternalLabels["fb_sender"] = g.Name()
	req.InternalLabels["dlq_timestamp"] = fmt.Sprintf("%d", time.Now().Unix())
	
	// Send to DLQ
	res, err := g.dlqClient.PushMetrics(ctx, req)
	if err != nil {
		g.logger.Error("Failed to send batch to DLQ", err, map[string]interface{}{
			"batch_id": batch.BatchID,
		})
		return nil, fmt.Errorf("failed to send batch to DLQ: %w", err)
	}
	
	// Check response status
	if res.Status != fb.StatusSuccess {
		g.logger.Error("DLQ returned error", fmt.Errorf(res.ErrorMessage), map[string]interface{}{
			"batch_id":      batch.BatchID,
			"error_code":    res.ErrorCode,
			"error_message": res.ErrorMessage,
		})
		return res, fmt.Errorf("DLQ returned error: %s - %s", res.ErrorCode, res.ErrorMessage)
	}
	
	// Success
	g.metrics.RecordBatchDLQ()
	g.logger.Info("Batch sent to DLQ", map[string]interface{}{
		"batch_id": batch.BatchID,
	})
	
	return res, nil
}

// UpdateConfig updates the Gateway function block's configuration
func (g *GW) UpdateConfig(ctx context.Context, configBytes []byte, generation int64) error {
	g.logger.Info("Updating configuration", map[string]interface{}{
		"generation": generation,
	})
	
	// Parse config
	var newConfig GWConfig
	if err := config.LoadConfigFromBytes(configBytes, &newConfig); err != nil {
		g.logger.Error("Failed to parse configuration", err, map[string]interface{}{
			"generation": generation,
		})
		return fmt.Errorf("failed to parse configuration: %w", err)
	}
	
	// Validate config
	if newConfig.ExportEndpoint == "" {
		return fmt.Errorf("export endpoint not configured")
	}
	
	// Store config
	oldConfig := g.config
	g.config = newConfig
	g.SetConfigGeneration(generation)
	g.metrics.SetConfigGeneration(generation)
	
	// Update schema validator if PII settings changed
	if !slicesEqual(oldConfig.PiiFields, newConfig.PiiFields) || oldConfig.EnablePiiDetection != newConfig.EnablePiiDetection {
		g.schemaValidator = schema.NewDefaultValidator()
		if newConfig.EnablePiiDetection {
			g.schemaValidator.SetPIIFields(newConfig.PiiFields)
		}
	}
	
	// Check if next FB changed
	if oldConfig.Common.NextFB != newConfig.Common.NextFB && g.nextFBConn != nil {
		// Close old connection
		g.nextFBConn.Close()
		g.nextFBConn = nil
		g.nextFBClient = nil
	}
	
	// Check if DLQ changed
	if oldConfig.Common.DLQ != newConfig.Common.DLQ && g.dlqConn != nil {
		// Close old connection
		g.dlqConn.Close()
		g.dlqConn = nil
		g.dlqClient = nil
	}
	
	g.logger.Info("Configuration updated", map[string]interface{}{
		"generation": generation,
	})
	
	return nil
}

// Shutdown shuts down the Gateway function block
func (g *GW) Shutdown(ctx context.Context) error {
	g.logger.Info("Shutting down Gateway function block", map[string]interface{}{})
	g.SetReady(false)
	g.metrics.SetReady(0)
	
	// Close connections
	if g.nextFBConn != nil {
		g.nextFBConn.Close()
	}
	
	if g.dlqConn != nil {
		g.dlqConn.Close()
	}
	
	if g.exportClient != nil {
		g.exportClient.Close()
	}
	
	g.logger.Info("Gateway function block shut down", map[string]interface{}{})
	return nil
}

// slicesEqual checks if two string slices are equal
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	
	return true
}

// For testing

// SetSchemaValidatorForTesting sets the schema validator for testing
func (g *GW) SetSchemaValidatorForTesting(validator schema.SchemaValidator) {
	g.schemaValidator = validator
}

// SetDLQClientForTesting sets the DLQ client for testing
func (g *GW) SetDLQClientForTesting(client fb.ChainPushServiceClient) {
	g.dlqClient = client
}

// GetConfigGeneration returns the current configuration generation
func (g *GW) GetConfigGeneration() int64 {
	return g.BaseFunctionBlock.GetConfigGeneration()
}

