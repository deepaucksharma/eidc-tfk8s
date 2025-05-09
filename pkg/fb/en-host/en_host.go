package enhost

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
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

// EnHostConfig contains configuration for the EN-HOST function block
type EnHostConfig struct {
	// Common configuration
	Common config.FBConfig `json:"common"`

	// EN-HOST-specific configuration
	CacheTTL         string `json:"cache_ttl"`
	ProcStatsEnabled bool   `json:"proc_stats_enabled"`
}

// EnHost implements the FB-EN-HOST function block
type EnHost struct {
	fb.BaseFunctionBlock
	logger          *logging.Logger
	metrics         *metrics.FBMetrics
	tracer          *tracing.Tracer
	config          *EnHostConfig
	configMu        sync.RWMutex
	nextFBClient    fb.ChainPushServiceClient
	nextFBConn      *grpc.ClientConn
	dlqClient       fb.ChainPushServiceClient
	dlqConn         *grpc.ClientConn
	circuitBreaker  *resilience.CircuitBreaker
	cache           *HostInfoCache
}

// NewEnHost creates a new EN-HOST function block
func NewEnHost(logger *logging.Logger, metrics *metrics.FBMetrics, tracer *tracing.Tracer) *EnHost {
	return &EnHost{
		BaseFunctionBlock: fb.BaseFunctionBlock{
			name:  "fb-en-host",
			ready: false,
		},
		logger:  logger,
		metrics: metrics,
		tracer:  tracer,
	}
}

// Initialize initializes the EN-HOST function block
func (h *EnHost) Initialize(ctx context.Context) error {
	h.logger.Info("Initializing FB-EN-HOST", nil)

	// Initialize circuit breaker
	h.circuitBreaker = resilience.NewCircuitBreaker("fb-en-host", resilience.DefaultCircuitBreakerConfig())

	// Initialize host info cache with default TTL
	h.cache = NewHostInfoCache(10 * time.Minute)

	// Mark as ready (full readiness will be set after config is loaded)
	h.BaseFunctionBlock.ready = true

	return nil
}

// ProcessBatch processes a batch of metrics
func (h *EnHost) ProcessBatch(ctx context.Context, batch *fb.MetricBatch) (*fb.ProcessResult, error) {
	// Create child span for the batch processing
	ctx, span := h.tracer.StartSpan(ctx, "process-batch", nil)
	defer span.End()

	// Record metric
	h.metrics.RecordBatchReceived()

	startTime := time.Now()

	// Process batch
	processingErr := h.processBatch(ctx, batch)
	if processingErr != nil {
		h.metrics.RecordProcessingError()
		return fb.NewErrorResult(batch.BatchID, fb.ErrorCodeProcessingFailed, processingErr, false), processingErr
	}

	// Record processing metrics
	h.metrics.RecordBatchProcessed(time.Since(startTime).Seconds())

	// Forward to next FB
	forwardingResult, forwardingErr := h.forwardToNextFB(ctx, batch)
	if forwardingErr != nil {
		// If forwarding fails but processing succeeded, attempt to send to DLQ
		dlqErr := h.sendToDLQ(ctx, batch, forwardingErr)
		if dlqErr != nil {
			h.logger.Error("Failed to send to DLQ after forwarding failure", dlqErr, map[string]interface{}{
				"batch_id": batch.BatchID,
			})
			return fb.NewErrorResult(batch.BatchID, fb.ErrorCodeDLQSendFailed, dlqErr, false), dlqErr
		}
		
		// Return error with DLQ status
		return fb.NewErrorResult(batch.BatchID, fb.ErrorCodeForwardingFailed, forwardingErr, true), forwardingErr
	}

	return forwardingResult, nil
}

// processBatch performs host enrichment on the batch
func (h *EnHost) processBatch(ctx context.Context, batch *fb.MetricBatch) error {
	// TODO: In a real implementation, this would:
	// 1. Parse the batch data based on format (OTLP, Prometheus, etc.)
	// 2. Enrich each metric with host information from /proc, hostname, etc.
	// 3. Cache host information for performance
	// 4. Update the batch data with the enriched metrics
	
	// For now, we'll just simulate the enrichment
	// This would be replaced with actual enrichment logic in a real implementation
	time.Sleep(5 * time.Millisecond) // Simulate processing time
	
	return nil
}

// forwardToNextFB forwards the batch to the next function block
func (h *EnHost) forwardToNextFB(ctx context.Context, batch *fb.MetricBatch) (*fb.ProcessResult, error) {
	startTime := time.Now()

	// Use circuit breaker to protect against downstream failures
	err := h.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		// Get the current config
		h.configMu.RLock()
		nextFB := h.config.Common.NextFB
		h.configMu.RUnlock()

		// Ensure we have a connection to the next FB
		if h.nextFBClient == nil {
			return fmt.Errorf("no connection to next FB: %s", nextFB)
		}

		// Create child span for forwarding
		ctx, span := h.tracer.StartSpan(ctx, "forward-to-next-fb", nil)
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
		res, err := h.nextFBClient.PushMetrics(ctx, req)
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
	h.metrics.RecordBatchForwarded(time.Since(startTime).Seconds())

	if err != nil {
		if err == resilience.ErrCircuitOpen {
			return fb.NewErrorResult(batch.BatchID, fb.ErrorCodeCircuitBreakerOpen, err, false), err
		}
		return fb.NewErrorResult(batch.BatchID, fb.ErrorCodeForwardingFailed, err, false), err
	}

	return fb.NewSuccessResult(batch.BatchID), nil
}

// sendToDLQ sends a batch to the Dead Letter Queue
func (h *EnHost) sendToDLQ(ctx context.Context, batch *fb.MetricBatch, originalErr error) error {
	// Create child span for DLQ
	ctx, span := h.tracer.StartSpan(ctx, "send-to-dlq", nil)
	defer span.End()

	// Ensure we have a connection to the DLQ
	if h.dlqClient == nil {
		return fmt.Errorf("no connection to DLQ")
	}

	// Add error info to internal labels
	if batch.InternalLabels == nil {
		batch.InternalLabels = make(map[string]string)
	}
	batch.InternalLabels["error"] = originalErr.Error()
	batch.InternalLabels["fb_sender"] = h.Name()

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
	res, err := h.dlqClient.PushMetrics(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to push metrics to DLQ: %w", err)
	}

	// Check response
	if res.Status != fb.StatusSuccess {
		return fmt.Errorf("DLQ returned error: %s (code: %s)", res.ErrorMessage, res.ErrorCode)
	}

	// Record metric
	h.metrics.RecordBatchDLQ()

	return nil
}

// UpdateConfig updates the EN-HOST function block's configuration
func (h *EnHost) UpdateConfig(ctx context.Context, configBytes []byte, generation int64) error {
	// Create child span for config update
	ctx, span := h.tracer.StartSpan(ctx, "update-config", nil)
	defer span.End()

	// Parse configuration
	var newConfig EnHostConfig
	if err := json.Unmarshal(configBytes, &newConfig); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	// Validate configuration
	if err := h.validateConfig(&newConfig); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// Apply configuration
	h.configMu.Lock()
	h.config = &newConfig
	h.configGeneration = generation
	h.configMu.Unlock()

	// Update circuit breaker configuration
	h.circuitBreaker = resilience.NewCircuitBreaker("fb-en-host", resilience.CircuitBreakerConfig{
		ErrorThresholdPercentage: newConfig.Common.CircuitBreaker.ErrorThresholdPercentage,
		OpenStateSeconds:         newConfig.Common.CircuitBreaker.OpenStateSeconds,
		HalfOpenRequestThreshold: newConfig.Common.CircuitBreaker.HalfOpenRequestThreshold,
	})

	// Update cache TTL if specified
	if newConfig.CacheTTL != "" {
		ttl, err := time.ParseDuration(newConfig.CacheTTL)
		if err != nil {
			h.logger.Error("Invalid cache TTL, using default", err, map[string]interface{}{
				"ttl": newConfig.CacheTTL,
			})
		} else {
			h.cache.SetTTL(ttl)
		}
	}

	// Connect to next FB and DLQ
	if err := h.connectToNextFB(ctx, newConfig.Common.NextFB); err != nil {
		h.logger.Error("Failed to connect to next FB", err, map[string]interface{}{
			"next_fb": newConfig.Common.NextFB,
		})
		// Don't fail config update on connection error - we'll retry on next batch
	}

	// Update metrics
	h.metrics.SetConfigGeneration(generation)
	h.metrics.SetReady(true)

	h.logger.Info("Config updated", map[string]interface{}{
		"generation": generation,
		"next_fb":    newConfig.Common.NextFB,
		"cache_ttl":  newConfig.CacheTTL,
	})

	return nil
}

// validateConfig validates the EN-HOST function block's configuration
func (h *EnHost) validateConfig(config *EnHostConfig) error {
	// Check if next FB is configured
	if config.Common.NextFB == "" {
		return fmt.Errorf("next FB not configured")
	}

	// Check if cache TTL is valid if specified
	if config.CacheTTL != "" {
		if _, err := time.ParseDuration(config.CacheTTL); err != nil {
			return fmt.Errorf("invalid cache TTL: %w", err)
		}
	}

	return nil
}

// ConnectServices connects to the config service, next FB, and DLQ
func (h *EnHost) ConnectServices(ctx context.Context, configServiceAddr, nextFB, dlqAddr string) error {
	// Connect to config service
	// In a real implementation, this would:
	// 1. Connect to the config service
	// 2. Register for config updates
	// 3. Apply initial configuration
	
	// For now, we'll just create a default config
	h.config = &EnHostConfig{
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
		CacheTTL:         "10m",
		ProcStatsEnabled: true,
	}
	h.configGeneration = 1
	
	// Connect to next FB
	if err := h.connectToNextFB(ctx, nextFB); err != nil {
		return fmt.Errorf("failed to connect to next FB: %w", err)
	}
	
	// Connect to DLQ
	if err := h.connectToDLQ(ctx, dlqAddr); err != nil {
		return fmt.Errorf("failed to connect to DLQ: %w", err)
	}
	
	h.logger.Info("Connected to services", map[string]interface{}{
		"config_service": configServiceAddr,
		"next_fb":        nextFB,
		"dlq":            dlqAddr,
	})
	
	return nil
}

// connectToNextFB establishes a connection to the next function block
func (h *EnHost) connectToNextFB(ctx context.Context, nextFB string) error {
	// Close existing connection if any
	if h.nextFBConn != nil {
		h.nextFBConn.Close()
		h.nextFBConn = nil
		h.nextFBClient = nil
	}

	// Create new connection
	conn, err := grpc.DialContext(ctx, nextFB,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to next FB: %w", err)
	}

	h.nextFBConn = conn
	h.nextFBClient = fb.NewChainPushServiceClient(conn)
	
	return nil
}

// connectToDLQ establishes a connection to the DLQ function block
func (h *EnHost) connectToDLQ(ctx context.Context, dlqAddr string) error {
	// Close existing connection if any
	if h.dlqConn != nil {
		h.dlqConn.Close()
		h.dlqConn = nil
		h.dlqClient = nil
	}

	// Create new connection
	conn, err := grpc.DialContext(ctx, dlqAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to DLQ: %w", err)
	}

	h.dlqConn = conn
	h.dlqClient = fb.NewChainPushServiceClient(conn)
	
	return nil
}

// Shutdown shuts down the EN-HOST function block
func (h *EnHost) Shutdown(ctx context.Context) error {
	h.logger.Info("Shutting down FB-EN-HOST", nil)

	// Close connections
	if h.nextFBConn != nil {
		h.nextFBConn.Close()
		h.nextFBConn = nil
		h.nextFBClient = nil
	}

	if h.dlqConn != nil {
		h.dlqConn.Close()
		h.dlqConn = nil
		h.dlqClient = nil
	}

	// Mark as not ready
	h.BaseFunctionBlock.ready = false

	return nil
}

// StartGRPCServer starts the gRPC server for the ChainPushService
func StartGRPCServer(ctx context.Context, fb *EnHost, port int) (*grpc.Server, error) {
	// Create gRPC server
	server := grpc.NewServer()

	// Register the ChainPushService
	fb.logger.Info("Registering ChainPushService", map[string]interface{}{"port": port})
	RegisterChainPushService(server, fb)

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

// RegisterChainPushService registers the ChainPushService with the gRPC server
func RegisterChainPushService(server *grpc.Server, fb *EnHost) {
	// Implementation depends on the generated gRPC code
	// In a real implementation, this would register the service with the server
	// fb.RegisterChainPushServiceServer(server, NewChainPushServiceServer(fb))
}

// ChainPushServiceServer implements the ChainPushService gRPC interface
type ChainPushServiceServer struct {
	fb *EnHost
}

// NewChainPushServiceServer creates a new ChainPushServiceServer
func NewChainPushServiceServer(fb *EnHost) *ChainPushServiceServer {
	return &ChainPushServiceServer{fb: fb}
}

// PushMetrics implements the PushMetrics method of the ChainPushService
func (s *ChainPushServiceServer) PushMetrics(ctx context.Context, req *fb.MetricBatchRequest) (*fb.MetricBatchResponse, error) {
	// Convert to MetricBatch
	batch := &fb.MetricBatch{
		BatchID:          req.BatchId,
		Data:             req.Data,
		Format:           req.Format,
		Replay:           req.Replay,
		ConfigGeneration: req.ConfigGeneration,
		Metadata:         req.Metadata,
		InternalLabels:   req.InternalLabels,
	}

	// Process the batch
	result, err := s.fb.ProcessBatch(ctx, batch)
	if err != nil {
		// Return error response
		return &fb.MetricBatchResponse{
			Status:       result.Status,
			ErrorMessage: result.ErrorMessage,
			ErrorCode:    string(result.ErrorCode),
			BatchId:      req.BatchId,
		}, nil
	}

	// Return success response
	return &fb.MetricBatchResponse{
		Status:  result.Status,
		BatchId: req.BatchId,
	}, nil
}
