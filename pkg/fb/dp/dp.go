package dp

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

// DPConfig contains configuration for the Deduplication function block
type DPConfig struct {
	// Common configuration
	Common config.FBConfig `json:"common"`

	// DP-specific configuration
	Enabled      bool     `json:"enabled"`
	StorageType  string   `json:"storageType"`
	TTLMinutes   int      `json:"ttlMinutes"`
	GCInterval   string   `json:"gcInterval"`
	DeduplicationKey []string `json:"deduplicationKey"`
	
	// Persistent storage configuration
	PersistentStorage struct {
		Enabled         bool   `json:"enabled"`
		Path            string `json:"path"`
		VolumeClaimName string `json:"volumeClaimName"`
	} `json:"persistentStorage"`
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
	store           DeduplicationStore
	storeMu         sync.RWMutex
	gcCtx           context.Context
	gcCancel        context.CancelFunc
	dedupCounter    metrics.Counter
}

// NewDP creates a new Deduplication function block
func NewDP() *DP {
	// Create cancellable context for GC process
	gcCtx, gcCancel := context.WithCancel(context.Background())
	
	return &DP{
		BaseFunctionBlock: fb.BaseFunctionBlock{
			name:  "fb-dp",
			ready: false,
		},
		logger:    logging.NewLogger("fb-dp"),
		metrics:   metrics.NewFBMetrics("fb-dp"),
		tracer:    tracing.NewTracer("fb-dp"),
		gcCtx:     gcCtx,
		gcCancel:  gcCancel,
		dedupCounter: metrics.NewCounter("fb_dp_deduplicated_total", "Total number of deduplicated telemetry items"),
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

	// Skip deduplication if not enabled
	d.configMu.RLock()
	enabled := d.config.Enabled
	deduplicationKeys := d.config.DeduplicationKey
	ttlMinutes := d.config.TTLMinutes
	d.configMu.RUnlock()

	if !enabled || len(deduplicationKeys) == 0 {
		d.logger.Debug("Deduplication disabled or no deduplication keys configured", nil)
		return nil
	}

	// Get the store
	d.storeMu.RLock()
	store := d.store
	d.storeMu.RUnlock()
	
	// Ensure we have a store
	if store == nil {
		return fmt.Errorf("deduplication store not initialized")
	}

	// Deserialize the batch data
	var metrics []map[string]interface{}
	if err := json.Unmarshal(batch.Data, &metrics); err != nil {
		return fmt.Errorf("failed to deserialize metrics: %w", err)
	}

	// Process each metric
	var uniqueMetrics []map[string]interface{}
	var dedupCount int

	for _, metric := range metrics {
		// Create a deduplication key from the metric using the configured keys
		dedupKey, err := createDeduplicationKey(metric, deduplicationKeys)
		if err != nil {
			d.logger.Warn("Failed to create deduplication key, including metric", err, map[string]interface{}{
				"metric": metric,
			})
			uniqueMetrics = append(uniqueMetrics, metric)
			continue
		}

		// Check if we've seen this metric before
		exists, err := store.Has(dedupKey)
		if err != nil {
			d.logger.Warn("Failed to check deduplication key, including metric", err, map[string]interface{}{
				"metric": metric,
			})
			uniqueMetrics = append(uniqueMetrics, metric)
			continue
		}

		if exists {
			// Metric is a duplicate, skip it
			dedupCount++
			d.logger.Debug("Deduplicated metric", map[string]interface{}{
				"dedup_key": string(dedupKey),
			})
			continue
		}

		// Metric is unique, add it to the store
		ttl := time.Duration(ttlMinutes) * time.Minute
		if err := store.Put(dedupKey, ttl); err != nil {
			d.logger.Warn("Failed to store deduplication key, including metric anyway", err, map[string]interface{}{
				"metric": metric,
			})
		}

		// Add the metric to the uniqueMetrics list
		uniqueMetrics = append(uniqueMetrics, metric)
	}

	// Update deduplication counter
	if dedupCount > 0 {
		d.dedupCounter.Add(float64(dedupCount))
		d.logger.Info("Deduplicated metrics", map[string]interface{}{
			"count": dedupCount,
		})
	}

	// Replace batch data with filtered metrics
	if len(uniqueMetrics) != len(metrics) {
		filteredData, err := json.Marshal(uniqueMetrics)
		if err != nil {
			return fmt.Errorf("failed to serialize filtered metrics: %w", err)
		}
		batch.Data = filteredData
	}

	return nil
}

// createDeduplicationKey creates a unique key for a metric based on the configured deduplication keys
func createDeduplicationKey(metric map[string]interface{}, deduplicationKeys []string) ([]byte, error) {
	// Create a map with just the fields used for deduplication
	keyMap := make(map[string]interface{})
	for _, key := range deduplicationKeys {
		if val, ok := metric[key]; ok {
			keyMap[key] = val
		} else {
			return nil, fmt.Errorf("deduplication key %s not found in metric", key)
		}
	}

	// Serialize the map to JSON
	keyJSON, err := json.Marshal(keyMap)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize deduplication key: %w", err)
	}

	return keyJSON, nil
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

	// Initialize or update storage backend
	if err := d.initializeStore(&newConfig); err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
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
		"storage_type": newConfig.StorageType,
		"persistent":  newConfig.PersistentStorage.Enabled,
		"ttl_minutes": newConfig.TTLMinutes,
	})

	return nil
}

// initializeStore initializes the deduplication storage backend
func (d *DP) initializeStore(config *DPConfig) error {
	d.storeMu.Lock()
	defer d.storeMu.Unlock()

	// Close existing store if any
	if d.store != nil {
		if err := d.store.Close(); err != nil {
			d.logger.Error("Failed to close existing store", err, nil)
			// Continue to initialize new store
		}
		d.store = nil
	}

	// Stop existing GC process if any
	if d.gcCancel != nil {
		d.gcCancel()
		// Create new cancellable context for GC process
		d.gcCtx, d.gcCancel = context.WithCancel(context.Background())
	}

	// Initialize new store based on configuration
	var store DeduplicationStore
	var err error

	switch config.StorageType {
	case "memory":
		d.logger.Info("Initializing in-memory deduplication store", nil)
		memStore := NewMemoryStore()
		store = memStore

		// Start garbage collection for memory store
		gcInterval, err := time.ParseDuration(config.GCInterval)
		if err != nil {
			gcInterval = 5 * time.Minute // Default to 5 minutes
		}

		// Start garbage collection in a goroutine
		go func() {
			ticker := time.NewTicker(gcInterval)
			defer ticker.Stop()

			for {
				select {
				case <-d.gcCtx.Done():
					return
				case <-ticker.C:
					memStore.runGC()
				}
			}
		}()

	case "badgerdb":
		// Determine storage path
		var storagePath string
		if config.PersistentStorage.Enabled {
			storagePath = config.PersistentStorage.Path
		} else {
			storagePath = "/tmp/dedup-badger"
		}

		d.logger.Info("Initializing BadgerDB deduplication store", map[string]interface{}{
			"path":       storagePath,
			"persistent": config.PersistentStorage.Enabled,
		})

		// Initialize BadgerDB store
		badgerStore, err := NewBadgerStore(storagePath)
		if err != nil {
			return fmt.Errorf("failed to initialize BadgerDB store: %w", err)
		}
		store = badgerStore

		// Start BadgerDB garbage collection
		gcInterval, err := time.ParseDuration(config.GCInterval)
		if err != nil {
			gcInterval = 10 * time.Minute // Default to 10 minutes for BadgerDB
		}
		badgerStore.StartGarbageCollection(d.gcCtx, gcInterval)

	default:
		return fmt.Errorf("unsupported storage type: %s", config.StorageType)
	}

	// Set the store
	d.store = store
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

	// Validate storage type
	if config.StorageType != "memory" && config.StorageType != "badgerdb" {
		return fmt.Errorf("invalid storage type: %s, must be 'memory' or 'badgerdb'", config.StorageType)
	}

	// Validate TTL
	if config.TTLMinutes <= 0 {
		return fmt.Errorf("ttlMinutes must be positive")
	}

	// Validate deduplication keys
	if len(config.DeduplicationKey) == 0 {
		return fmt.Errorf("at least one deduplication key must be configured")
	}

	// Validate persistent storage configuration
	if config.StorageType == "badgerdb" && config.PersistentStorage.Enabled {
		if config.PersistentStorage.Path == "" {
			return fmt.Errorf("persistent storage path not configured")
		}
	}

	// Validate GC interval
	if config.GCInterval == "" {
		return fmt.Errorf("GC interval not configured")
	}

	// Try to parse GC interval
	_, err := time.ParseDuration(config.GCInterval)
	if err != nil {
		return fmt.Errorf("invalid GC interval: %w", err)
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

	// Stop garbage collection
	if d.gcCancel != nil {
		d.gcCancel()
	}

	// Close store
	d.storeMu.Lock()
	if d.store != nil {
		// Flush store to ensure data is persisted
		if err := d.store.Flush(); err != nil {
			d.logger.Error("Failed to flush store during shutdown", err, nil)
		}
		
		// Close store
		if err := d.store.Close(); err != nil {
			d.logger.Error("Failed to close store during shutdown", err, nil)
		}
		d.store = nil
	}
	d.storeMu.Unlock()

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
