package dp

import (
	"context"
	"encoding/json"
	"fmt"
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

// DPConfig contains configuration for the Deduplication function block
type DPConfig struct {
	// Common configuration
	Common config.FBConfig `json:"common"`

	// DP-specific configuration
	Enabled     bool   `json:"enabled"`
	GCInterval  string `json:"gcInterval"`
	StoragePath string `json:"storagePath"`
}

// DP implements the FB-DP (Deduplication) function block
type DP struct {
	fb.BaseFunctionBlock
	logger          *logging.Logger
	metrics         *metrics.FBMetrics
	tracer          *tracing.Tracer
	config          *DPConfig
	configMu        sync.RWMutex
	nextFBClient    fb.ChainPushServiceClient
	nextFBConn      *grpc.ClientConn
	dlqClient       fb.ChainPushServiceClient
	dlqConn         *grpc.ClientConn
	circuitBreaker  *resilience.CircuitBreaker
}

// NewDP creates a new Deduplication function block
func NewDP() *DP {
	return &DP{
		BaseFunctionBlock: fb.BaseFunctionBlock{
			name:  "fb-dp",
			ready: false,
		},
		logger:  logging.NewLogger("fb-dp"),
		metrics: metrics.NewFBMetrics("fb-dp"),
		tracer:  tracing.NewTracer("fb-dp"),
	}
}

// Initialize initializes the Deduplication function block
func (d *DP) Initialize(ctx context.Context) error {
	d.logger.Info("Initializing FB-DP", nil)

	// Initialize circuit breaker with default config
	d.circuitBreaker = resilience.NewCircuitBreaker("fb-dp", resilience.DefaultCircuitBreakerConfig())

	// Mark as ready (full readiness will be set after config is loaded)
	d.BaseFunctionBlock.ready = true

	return nil
}

// ProcessBatch processes a batch of metrics
func (d *DP) ProcessBatch(ctx context.Context, batch *fb.MetricBatch) (*fb.ProcessResult, error) {
	// Create child span for the batch processing
	ctx, span := d.tracer.StartSpan(ctx, "process-batch", nil)
	defer span.End()

	// Record metric
	d.metrics.RecordBatchReceived()

	startTime := time.Now()

	// Process batch
	processingErr := d.processBatch(ctx, batch)
	if processingErr != nil {
		d.metrics.RecordProcessingError()
		return fb.NewErrorResult(batch.BatchID, fb.ErrorCodeProcessingFailed, processingErr, false), processingErr
	}

	// Record processing metrics
	d.metrics.RecordBatchProcessed(time.Since(startTime).Seconds())

	// Forward to next FB
	forwardingResult, forwardingErr := d.forwardToNextFB(ctx, batch)
	if forwardingErr != nil {
		// If forwarding fails but processing succeeded, attempt to send to DLQ
		dlqErr := d.sendToDLQ(ctx, batch, forwardingErr)
		if dlqErr != nil {
			d.logger.Error("Failed to send to DLQ after forwarding failure", dlqErr, map[string]interface{}{
				"batch_id": batch.BatchID,
			})
			return fb.NewErrorResult(batch.BatchID, fb.ErrorCodeDLQSendFailed, dlqErr, false), dlqErr
		}
		
		// Return error with DLQ status
		return fb.NewErrorResult(batch.BatchID, fb.ErrorCodeForwardingFailed, forwardingErr, true), forwardingErr
	}

	return forwardingResult, nil
}

// processBatch performs the actual batch processing
func (d *DP) processBatch(ctx context.Context, batch *fb.MetricBatch) error {
	// Create child span for deduplication
	ctx, span := d.tracer.StartSpan(ctx, "deduplication", nil)
	defer span.End()

	// TODO: Implement deduplication logic here
	// This would involve:
	// 1. Calculating hash for each metric in the batch
	// 2. Checking against BadgerDB store if we've seen this hash before
	// 3. Filtering out duplicates from the batch

	return nil
}

// forwardToNextFB forwards the batch to the next function block
func (d *DP) forwardToNextFB(ctx context.Context, batch *fb.MetricBatch) (*fb.ProcessResult, error) {
	startTime := time.Now()

	// Use circuit breaker to protect against downstream failures
	err := d.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		// Get the current config
		d.configMu.RLock()
		nextFB := d.config.Common.NextFB
		d.configMu.RUnlock()

		// Ensure we have a connection to the next FB
		if d.nextFBClient == nil {
			return fmt.Errorf("no connection to next FB: %s", nextFB)
		}

		// Create child span for forwarding
		ctx, span := d.tracer.StartSpan(ctx, "forward-to-next-fb", nil)
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
		res, err := d.nextFBClient.PushMetrics(ctx, req)
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
	d.metrics.RecordBatchForwarded(time.Since(startTime).Seconds())

	if err != nil {
		if err == resilience.ErrCircuitOpen {
			return fb.NewErrorResult(batch.BatchID, fb.ErrorCodeCircuitBreakerOpen, err, false), err
		}
		return fb.NewErrorResult(batch.BatchID, fb.ErrorCodeForwardingFailed, err, false), err
	}

	return fb.NewSuccessResult(batch.BatchID), nil
}

// sendToDLQ sends a batch to the Dead Letter Queue
func (d *DP) sendToDLQ(ctx context.Context, batch *fb.MetricBatch, originalErr error) error {
	// Create child span for DLQ
	ctx, span := d.tracer.StartSpan(ctx, "send-to-dlq", nil)
	defer span.End()

	// Ensure we have a connection to the DLQ
	if d.dlqClient == nil {
		return fmt.Errorf("no connection to DLQ")
	}

	// Add error info to internal labels
	if batch.InternalLabels == nil {
		batch.InternalLabels = make(map[string]string)
	}
	batch.InternalLabels["error"] = originalErr.Error()
	batch.InternalLabels["fb_sender"] = d.Name()

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
	res, err := d.dlqClient.PushMetrics(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to push metrics to DLQ: %w", err)
	}

	// Check response
	if res.Status != fb.StatusSuccess {
		return fmt.Errorf("DLQ returned error: %s (code: %s)", res.ErrorMessage, res.ErrorCode)
	}

	// Record metric
	d.metrics.RecordBatchDLQ()

	return nil
}

// UpdateConfig updates the Deduplication function block's configuration
func (d *DP) UpdateConfig(ctx context.Context, configBytes []byte, generation int64) error {
	// Create child span for config update
	ctx, span := d.tracer.StartSpan(ctx, "update-config", nil)
	defer span.End()

	// Parse configuration
	var newConfig DPConfig
	if err := json.Unmarshal(configBytes, &newConfig); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	// Validate configuration
	if err := d.validateConfig(&newConfig); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// Apply configuration
	d.configMu.Lock()
	d.config = &newConfig
	d.configGeneration = generation
	d.configMu.Unlock()

	// Update circuit breaker configuration
	d.circuitBreaker = resilience.NewCircuitBreaker("fb-dp", resilience.CircuitBreakerConfig{
		ErrorThresholdPercentage: newConfig.Common.CircuitBreaker.ErrorThresholdPercentage,
		OpenStateSeconds:         newConfig.Common.CircuitBreaker.OpenStateSeconds,
		HalfOpenRequestThreshold: newConfig.Common.CircuitBreaker.HalfOpenRequestThreshold,
	})

	// Connect to next FB and DLQ if not already connected
	if d.nextFBClient == nil {
		if err := d.connectToNextFB(ctx, newConfig.Common.NextFB); err != nil {
			d.logger.Error("Failed to connect to next FB", err, map[string]interface{}{
				"next_fb": newConfig.Common.NextFB,
			})
			// Don't fail config update on connection error - we'll retry on next batch
		}
	}

	if d.dlqClient == nil {
		if err := d.connectToDLQ(ctx, newConfig.Common.DLQ); err != nil {
			d.logger.Error("Failed to connect to DLQ", err, map[string]interface{}{
				"dlq": newConfig.Common.DLQ,
			})
			// Don't fail config update on connection error - we'll retry when needed
		}
	}

	// Update metrics
	d.metrics.SetConfigGeneration(generation)
	d.metrics.SetReady(true)

	d.logger.Info("Config updated", map[string]interface{}{
		"generation":  generation,
		"enabled":     newConfig.Enabled,
		"gc_interval": newConfig.GCInterval,
		"storage_path": newConfig.StoragePath,
	})

	return nil
}

// validateConfig validates the Deduplication function block's configuration
func (d *DP) validateConfig(config *DPConfig) error {
	// Check if next FB is configured
	if config.Common.NextFB == "" {
		return fmt.Errorf("next FB not configured")
	}

	// Check if DLQ is configured
	if config.Common.DLQ == "" {
		return fmt.Errorf("DLQ not configured")
	}

	// Check storage path
	if config.StoragePath == "" {
		return fmt.Errorf("storage path not configured")
	}

	return nil
}

// connectToNextFB establishes a connection to the next function block
func (d *DP) connectToNextFB(ctx context.Context, nextFB string) error {
	// Close existing connection if any
	if d.nextFBConn != nil {
		d.nextFBConn.Close()
		d.nextFBConn = nil
		d.nextFBClient = nil
	}

	// Create new connection
	conn, err := grpc.DialContext(ctx, nextFB,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to next FB: %w", err)
	}

	d.nextFBConn = conn
	d.nextFBClient = fb.NewChainPushServiceClient(conn)
	
	return nil
}

// connectToDLQ establishes a connection to the DLQ function block
func (d *DP) connectToDLQ(ctx context.Context, dlqAddr string) error {
	// Close existing connection if any
	if d.dlqConn != nil {
		d.dlqConn.Close()
		d.dlqConn = nil
		d.dlqClient = nil
	}

	// Create new connection
	conn, err := grpc.DialContext(ctx, dlqAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to DLQ: %w", err)
	}

	d.dlqConn = conn
	d.dlqClient = fb.NewChainPushServiceClient(conn)
	
	return nil
}

// Shutdown shuts down the Deduplication function block
func (d *DP) Shutdown(ctx context.Context) error {
	d.logger.Info("Shutting down FB-DP", nil)

	// Close connections
	if d.nextFBConn != nil {
		d.nextFBConn.Close()
		d.nextFBConn = nil
		d.nextFBClient = nil
	}

	if d.dlqConn != nil {
		d.dlqConn.Close()
		d.dlqConn = nil
		d.dlqClient = nil
	}

	// Mark as not ready
	d.BaseFunctionBlock.ready = false

	return nil
}

// Testing helpers

// SetNextFBClientForTesting sets the next FB client for testing purposes
func (d *DP) SetNextFBClientForTesting(client fb.ChainPushServiceClient) {
	d.nextFBClient = client
}

// SetDLQClientForTesting sets the DLQ client for testing purposes
func (d *DP) SetDLQClientForTesting(client fb.ChainPushServiceClient) {
	d.dlqClient = client
}