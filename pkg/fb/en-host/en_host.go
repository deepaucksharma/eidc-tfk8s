package enhost

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"eidc-tfk8s/internal/common/logging"
	"eidc-tfk8s/internal/common/metrics"
	"eidc-tfk8s/internal/common/resilience"
	"eidc-tfk8s/internal/common/tracing"
	"eidc-tfk8s/internal/config"
	"eidc-tfk8s/pkg/fb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ENHostConfig contains configuration for the Host Enrichment function block
type ENHostConfig struct {
	// Common configuration
	Common config.FBConfig `json:"common"`

	// EN-HOST-specific configuration
	Enabled  bool   `json:"enabled"`
	CacheTTL string `json:"cacheTTL"`
}

// ENHost implements the FB-EN-HOST (Host Enrichment) function block
type ENHost struct {
	fb.BaseFunctionBlock
	logger          *logging.Logger
	metrics         *metrics.FBMetrics
	tracer          *tracing.Tracer
	config          *ENHostConfig
	configMu        sync.RWMutex
	nextFBClient    fb.ChainPushServiceClient
	nextFBConn      *grpc.ClientConn
	dlqClient       fb.ChainPushServiceClient
	dlqConn         *grpc.ClientConn
	circuitBreaker  *resilience.CircuitBreaker
}

// NewENHost creates a new Host Enrichment function block
func NewENHost() *ENHost {
	return &ENHost{
		BaseFunctionBlock: fb.BaseFunctionBlock{
			name:  "fb-en-host",
			ready: false,
		},
		logger:  logging.NewLogger("fb-en-host"),
		metrics: metrics.NewFBMetrics("fb-en-host"),
		tracer:  tracing.NewTracer("fb-en-host"),
	}
}

// Initialize initializes the Host Enrichment function block
func (e *ENHost) Initialize(ctx context.Context) error {
	e.logger.Info("Initializing FB-EN-HOST", nil)

	// Initialize circuit breaker with default config
	e.circuitBreaker = resilience.NewCircuitBreaker("fb-en-host", resilience.DefaultCircuitBreakerConfig())

	// Mark as ready (full readiness will be set after config is loaded)
	e.BaseFunctionBlock.ready = true

	return nil
}

// ProcessBatch processes a batch of metrics
func (e *ENHost) ProcessBatch(ctx context.Context, batch *fb.MetricBatch) (*fb.ProcessResult, error) {
	// Create child span for the batch processing
	ctx, span := e.tracer.StartSpan(ctx, "process-batch", map[string]string{
		"batch_id": batch.BatchID,
	})
	defer span.End()

	// Record metric
	e.metrics.RecordBatchReceived()

	startTime := time.Now()

	// Ensure batch_id is in the span context
	ctx = e.tracer.ContextWithAttributes(ctx, map[string]string{
		"batch_id": batch.BatchID,
		"fb.name":  e.Name(),
	})

	// Process batch
	processingErr := e.processBatch(ctx, batch)
	if processingErr != nil {
		e.metrics.RecordProcessingError()
		e.tracer.RecordError(ctx, processingErr)
		return fb.NewErrorResult(batch.BatchID, fb.ErrorCodeProcessingFailed, processingErr, false), processingErr
	}

	// Record processing metrics
	e.metrics.RecordBatchProcessed(time.Since(startTime).Seconds())

	// Forward to next FB
	forwardingResult, forwardingErr := e.forwardToNextFB(ctx, batch)
	if forwardingErr != nil {
		e.tracer.RecordError(ctx, forwardingErr)

		// If forwarding fails but processing succeeded, attempt to send to DLQ
		dlqErr := e.sendToDLQ(ctx, batch, forwardingErr)
		if dlqErr != nil {
			e.logger.Error("Failed to send to DLQ after forwarding failure", dlqErr, map[string]interface{}{
				"batch_id": batch.BatchID,
			})
			e.tracer.RecordError(ctx, dlqErr)
			return fb.NewErrorResult(batch.BatchID, fb.ErrorCodeDLQSendFailed, dlqErr, false), dlqErr
		}
		
		// Return error with DLQ status
		return fb.NewErrorResult(batch.BatchID, fb.ErrorCodeForwardingFailed, forwardingErr, true), forwardingErr
	}

	return forwardingResult, nil
}

// processBatch performs the actual batch processing
func (e *ENHost) processBatch(ctx context.Context, batch *fb.MetricBatch) error {
	// Create child span for enrichment
	ctx, span := e.tracer.StartSpan(ctx, "host-enrichment", nil)
	defer span.End()

	// TODO: Implement host-level enrichment logic here
	// This would involve:
	// 1. Extracting host information for each metric
	// 2. Looking up additional host metadata (OS, CPU, memory, etc.)
	// 3. Enriching metrics with this metadata

	return nil
}

// forwardToNextFB forwards the batch to the next function block
func (e *ENHost) forwardToNextFB(ctx context.Context, batch *fb.MetricBatch) (*fb.ProcessResult, error) {
	startTime := time.Now()

	// Create child span for forwarding
	ctx, span := e.tracer.StartSpan(ctx, "forward-to-next-fb", nil)
	defer span.End()

	// Use circuit breaker to protect against downstream failures
	err := e.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		// Get the current config
		e.configMu.RLock()
		nextFB := e.config.Common.NextFB
		e.configMu.RUnlock()

		// Ensure we have a connection to the next FB
		if e.nextFBClient == nil {
			return fmt.Errorf("no connection to next FB: %s", nextFB)
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

		// Forward to next FB
		res, err := e.nextFBClient.PushMetrics(ctx, req)
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
	e.metrics.RecordBatchForwarded(time.Since(startTime).Seconds())

	if err != nil {
		if err == resilience.ErrCircuitOpen {
			return fb.NewErrorResult(batch.BatchID, fb.ErrorCodeCircuitBreakerOpen, err, false), err
		}
		return fb.NewErrorResult(batch.BatchID, fb.ErrorCodeForwardingFailed, err, false), err
	}

	return fb.NewSuccessResult(batch.BatchID), nil
}

// sendToDLQ sends a batch to the Dead Letter Queue
func (e *ENHost) sendToDLQ(ctx context.Context, batch *fb.MetricBatch, originalErr error) error {
	// Create child span for DLQ
	ctx, span := e.tracer.StartSpan(ctx, "send-to-dlq", nil)
	defer span.End()

	// Ensure we have a connection to the DLQ
	if e.dlqClient == nil {
		return fmt.Errorf("no connection to DLQ")
	}

	// Add error info to internal labels
	if batch.InternalLabels == nil {
		batch.InternalLabels = make(map[string]string)
	}
	batch.InternalLabels["error"] = originalErr.Error()
	batch.InternalLabels["fb_sender"] = e.Name()

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
	res, err := e.dlqClient.PushMetrics(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to push metrics to DLQ: %w", err)
	}

	// Check response
	if res.Status != fb.StatusSuccess {
		return fmt.Errorf("DLQ returned error: %s (code: %s)", res.ErrorMessage, res.ErrorCode)
	}

	// Record metric
	e.metrics.RecordBatchDLQ()

	return nil
}

// UpdateConfig updates the Host Enrichment function block's configuration
func (e *ENHost) UpdateConfig(ctx context.Context, configBytes []byte, generation int64) error {
	// Create child span for config update
	ctx, span := e.tracer.StartSpan(ctx, "update-config", nil)
	defer span.End()

	// Parse configuration
	var newConfig ENHostConfig
	if err := json.Unmarshal(configBytes, &newConfig); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	// Validate configuration
	if err := e.validateConfig(&newConfig); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// Apply configuration
	e.configMu.Lock()
	e.config = &newConfig
	e.configGeneration = generation
	e.configMu.Unlock()

	// Update circuit breaker configuration
	e.circuitBreaker = resilience.NewCircuitBreaker("fb-en-host", resilience.CircuitBreakerConfig{
		ErrorThresholdPercentage: newConfig.Common.CircuitBreaker.ErrorThresholdPercentage,
		OpenStateSeconds:         newConfig.Common.CircuitBreaker.OpenStateSeconds,
		HalfOpenRequestThreshold: newConfig.Common.CircuitBreaker.HalfOpenRequestThreshold,
	})

	// Connect to next FB and DLQ if not already connected
	if e.nextFBClient == nil {
		if err := e.connectToNextFB(ctx, newConfig.Common.NextFB); err != nil {
			e.logger.Error("Failed to connect to next FB", err, map[string]interface{}{
				"next_fb": newConfig.Common.NextFB,
			})
			// Don't fail config update on connection error - we'll retry on next batch
		}
	}

	if e.dlqClient == nil {
		if err := e.connectToDLQ(ctx, newConfig.Common.DLQ); err != nil {
			e.logger.Error("Failed to connect to DLQ", err, map[string]interface{}{
				"dlq": newConfig.Common.DLQ,
			})
			// Don't fail config update on connection error - we'll retry when needed
		}
	}

	// Update metrics
	e.metrics.SetConfigGeneration(generation)
	e.metrics.SetReady(true)

	e.logger.Info("Config updated", map[string]interface{}{
		"generation": generation,
		"enabled":    newConfig.Enabled,
		"cache_ttl":  newConfig.CacheTTL,
	})

	return nil
}

// validateConfig validates the Host Enrichment function block's configuration
func (e *ENHost) validateConfig(config *ENHostConfig) error {
	// Check if next FB is configured
	if config.Common.NextFB == "" {
		return fmt.Errorf("next FB not configured")
	}

	// Check if DLQ is configured
	if config.Common.DLQ == "" {
		return fmt.Errorf("DLQ not configured")
	}

	return nil
}

// connectToNextFB establishes a connection to the next function block
func (e *ENHost) connectToNextFB(ctx context.Context, nextFB string) error {
	// Close existing connection if any
	if e.nextFBConn != nil {
		e.nextFBConn.Close()
		e.nextFBConn = nil
		e.nextFBClient = nil
	}

	// Create new connection
	conn, err := grpc.DialContext(ctx, nextFB,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to next FB: %w", err)
	}

	e.nextFBConn = conn
	e.nextFBClient = fb.NewChainPushServiceClient(conn)
	
	return nil
}

// connectToDLQ establishes a connection to the DLQ function block
func (e *ENHost) connectToDLQ(ctx context.Context, dlqAddr string) error {
	// Close existing connection if any
	if e.dlqConn != nil {
		e.dlqConn.Close()
		e.dlqConn = nil
		e.dlqClient = nil
	}

	// Create new connection
	conn, err := grpc.DialContext(ctx, dlqAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to DLQ: %w", err)
	}

	e.dlqConn = conn
	e.dlqClient = fb.NewChainPushServiceClient(conn)
	
	return nil
}

// Shutdown shuts down the Host Enrichment function block
func (e *ENHost) Shutdown(ctx context.Context) error {
	e.logger.Info("Shutting down FB-EN-HOST", nil)

	// Close connections
	if e.nextFBConn != nil {
		e.nextFBConn.Close()
		e.nextFBConn = nil
		e.nextFBClient = nil
	}

	if e.dlqConn != nil {
		e.dlqConn.Close()
		e.dlqConn = nil
		e.dlqClient = nil
	}

	// Mark as not ready
	e.BaseFunctionBlock.ready = false

	return nil
}

// Testing helpers

// SetNextFBClientForTesting sets the next FB client for testing purposes
func (e *ENHost) SetNextFBClientForTesting(client fb.ChainPushServiceClient) {
	e.nextFBClient = client
}

// SetDLQClientForTesting sets the DLQ client for testing purposes
func (e *ENHost) SetDLQClientForTesting(client fb.ChainPushServiceClient) {
	e.dlqClient = client
}
