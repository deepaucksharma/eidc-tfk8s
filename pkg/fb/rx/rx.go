package rx

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

// RXConfig contains configuration for the RX function block
type RXConfig struct {
	// Common configuration
	Common config.FBConfig `json:"common"`

	// RX-specific configuration
	Endpoints []Endpoint `json:"endpoints"`
}

// Endpoint represents a telemetry ingestion endpoint
type Endpoint struct {
	Protocol string `json:"protocol"`
	Port     int    `json:"port"`
	Enabled  bool   `json:"enabled"`
}

// RX implements the FB-RX function block
type RX struct {
	fb.BaseFunctionBlock
	logger          *logging.Logger
	metrics         *metrics.FBMetrics
	tracer          *tracing.Tracer
	config          *RXConfig
	configMu        sync.RWMutex
	nextFBClient    fb.ChainPushServiceClient
	nextFBConn      *grpc.ClientConn
	dlqClient       fb.ChainPushServiceClient
	dlqConn         *grpc.ClientConn
	circuitBreaker  *resilience.CircuitBreaker
}

// NewRX creates a new RX function block
func NewRX() *RX {
	return &RX{
		BaseFunctionBlock: fb.BaseFunctionBlock{
			name:  "fb-rx",
			ready: false,
		},
		logger:  logging.NewLogger("fb-rx"),
		metrics: metrics.NewFBMetrics("fb-rx"),
		tracer:  tracing.NewTracer("fb-rx"),
	}
}

// Initialize initializes the RX function block
func (r *RX) Initialize(ctx context.Context) error {
	r.logger.Info("Initializing FB-RX", nil)

	// Initialize circuit breaker
	r.circuitBreaker = resilience.NewCircuitBreaker("fb-rx", resilience.DefaultCircuitBreakerConfig())

	// Mark as ready (full readiness will be set after config is loaded)
	r.BaseFunctionBlock.ready = true

	return nil
}

// ProcessBatch processes a batch of metrics
func (r *RX) ProcessBatch(ctx context.Context, batch *fb.MetricBatch) (*fb.ProcessResult, error) {
	// Create child span for the batch processing
	ctx, span := r.tracer.StartSpan(ctx, "process-batch", nil)
	defer span.End()

	// Record metric
	r.metrics.RecordBatchReceived()

	startTime := time.Now()

	// Process batch
	processingErr := r.processBatch(ctx, batch)
	if processingErr != nil {
		r.metrics.RecordProcessingError()
		return fb.NewErrorResult(batch.BatchID, fb.ErrorCodeProcessingFailed, processingErr, false), processingErr
	}

	// Record processing metrics
	r.metrics.RecordBatchProcessed(time.Since(startTime).Seconds())

	// Forward to next FB
	forwardingResult, forwardingErr := r.forwardToNextFB(ctx, batch)
	if forwardingErr != nil {
		// If forwarding fails but processing succeeded, attempt to send to DLQ
		dlqErr := r.sendToDLQ(ctx, batch, forwardingErr)
		if dlqErr != nil {
			r.logger.Error("Failed to send to DLQ after forwarding failure", dlqErr, map[string]interface{}{
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
func (r *RX) processBatch(ctx context.Context, batch *fb.MetricBatch) error {
	// RX doesn't do much processing, it mostly forwards to the next FB
	// Here we'd implement telemetry parsing, normalization, etc.
	return nil
}

// forwardToNextFB forwards the batch to the next function block
func (r *RX) forwardToNextFB(ctx context.Context, batch *fb.MetricBatch) (*fb.ProcessResult, error) {
	startTime := time.Now()

	// Use circuit breaker to protect against downstream failures
	err := r.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		// Get the current config
		r.configMu.RLock()
		nextFB := r.config.Common.NextFB
		r.configMu.RUnlock()

		// Ensure we have a connection to the next FB
		if r.nextFBClient == nil {
			return fmt.Errorf("no connection to next FB: %s", nextFB)
		}

		// Create child span for forwarding
		ctx, span := r.tracer.StartSpan(ctx, "forward-to-next-fb", nil)
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
		res, err := r.nextFBClient.PushMetrics(ctx, req)
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
	r.metrics.RecordBatchForwarded(time.Since(startTime).Seconds())

	if err != nil {
		if err == resilience.ErrCircuitOpen {
			return fb.NewErrorResult(batch.BatchID, fb.ErrorCodeCircuitBreakerOpen, err, false), err
		}
		return fb.NewErrorResult(batch.BatchID, fb.ErrorCodeForwardingFailed, err, false), err
	}

	return fb.NewSuccessResult(batch.BatchID), nil
}

// sendToDLQ sends a batch to the Dead Letter Queue
func (r *RX) sendToDLQ(ctx context.Context, batch *fb.MetricBatch, originalErr error) error {
	// Create child span for DLQ
	ctx, span := r.tracer.StartSpan(ctx, "send-to-dlq", nil)
	defer span.End()

	// Ensure we have a connection to the DLQ
	if r.dlqClient == nil {
		return fmt.Errorf("no connection to DLQ")
	}

	// Add error info to internal labels
	if batch.InternalLabels == nil {
		batch.InternalLabels = make(map[string]string)
	}
	batch.InternalLabels["error"] = originalErr.Error()
	batch.InternalLabels["fb_sender"] = r.Name()

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
	res, err := r.dlqClient.PushMetrics(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to push metrics to DLQ: %w", err)
	}

	// Check response
	if res.Status != fb.StatusSuccess {
		return fmt.Errorf("DLQ returned error: %s (code: %s)", res.ErrorMessage, res.ErrorCode)
	}

	// Record metric
	r.metrics.RecordBatchDLQ()

	return nil
}

// UpdateConfig updates the RX function block's configuration
func (r *RX) UpdateConfig(ctx context.Context, configBytes []byte, generation int64) error {
	// Create child span for config update
	ctx, span := r.tracer.StartSpan(ctx, "update-config", nil)
	defer span.End()

	// Parse configuration
	var newConfig RXConfig
	if err := json.Unmarshal(configBytes, &newConfig); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	// Validate configuration
	if err := r.validateConfig(&newConfig); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// Apply configuration
	r.configMu.Lock()
	r.config = &newConfig
	r.configGeneration = generation
	r.configMu.Unlock()

	// Update circuit breaker configuration
	r.circuitBreaker = resilience.NewCircuitBreaker("fb-rx", resilience.CircuitBreakerConfig{
		ErrorThresholdPercentage: newConfig.Common.CircuitBreaker.ErrorThresholdPercentage,
		OpenStateSeconds:         newConfig.Common.CircuitBreaker.OpenStateSeconds,
		HalfOpenRequestThreshold: newConfig.Common.CircuitBreaker.HalfOpenRequestThreshold,
	})

	// Connect to next FB and DLQ
	if err := r.connectToNextFB(ctx, newConfig.Common.NextFB); err != nil {
		r.logger.Error("Failed to connect to next FB", err, map[string]interface{}{
			"next_fb": newConfig.Common.NextFB,
		})
		// Don't fail config update on connection error - we'll retry on next batch
	}

	// Update metrics
	r.metrics.SetConfigGeneration(generation)
	r.metrics.SetReady(true)

	r.logger.Info("Config updated", map[string]interface{}{
		"generation": generation,
		"next_fb":    newConfig.Common.NextFB,
		"endpoints":  len(newConfig.Endpoints),
	})

	return nil
}

// validateConfig validates the RX function block's configuration
func (r *RX) validateConfig(config *RXConfig) error {
	// Check if at least one endpoint is configured
	if len(config.Endpoints) == 0 {
		return fmt.Errorf("no endpoints configured")
	}

	// Check if next FB is configured
	if config.Common.NextFB == "" {
		return fmt.Errorf("next FB not configured")
	}

	return nil
}

// connectToNextFB establishes a connection to the next function block
func (r *RX) connectToNextFB(ctx context.Context, nextFB string) error {
	// Close existing connection if any
	if r.nextFBConn != nil {
		r.nextFBConn.Close()
		r.nextFBConn = nil
		r.nextFBClient = nil
	}

	// Create new connection
	conn, err := grpc.DialContext(ctx, nextFB,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to next FB: %w", err)
	}

	r.nextFBConn = conn
	r.nextFBClient = fb.NewChainPushServiceClient(conn)
	
	return nil
}

// connectToDLQ establishes a connection to the DLQ function block
func (r *RX) connectToDLQ(ctx context.Context, dlqAddr string) error {
	// Close existing connection if any
	if r.dlqConn != nil {
		r.dlqConn.Close()
		r.dlqConn = nil
		r.dlqClient = nil
	}

	// Create new connection
	conn, err := grpc.DialContext(ctx, dlqAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to DLQ: %w", err)
	}

	r.dlqConn = conn
	r.dlqClient = fb.NewChainPushServiceClient(conn)
	
	return nil
}

// Shutdown shuts down the RX function block
func (r *RX) Shutdown(ctx context.Context) error {
	r.logger.Info("Shutting down FB-RX", nil)

	// Close connections
	if r.nextFBConn != nil {
		r.nextFBConn.Close()
		r.nextFBConn = nil
		r.nextFBClient = nil
	}

	if r.dlqConn != nil {
		r.dlqConn.Close()
		r.dlqConn = nil
		r.dlqClient = nil
	}

	// Mark as not ready
	r.BaseFunctionBlock.ready = false

	return nil
}
