package agg

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog/log"

	"github.com/newrelic/nrdot-internal-devlab/pkg/fb"
	"github.com/newrelic/nrdot-internal-devlab/pkg/metrics"
	"github.com/newrelic/nrdot-internal-devlab/pkg/telemetry"
)

// Config holds the configuration for the FB-AGG function block
type Config struct {
	// WindowSeconds is the aggregation window in seconds
	WindowSeconds int `json:"windowSeconds"`

	// Aggregations defines the aggregation rules
	Aggregations []AggregationRule `json:"aggregations"`

	// BufferSize is the size of the buffer for incoming metrics
	BufferSize int `json:"bufferSize"`
}

// AggregationRule defines a rule for aggregating metrics
type AggregationRule struct {
	// Metric is the name of the metric to aggregate
	Metric string `json:"metric"`

	// Type is the type of aggregation (sum, avg, min, max, histogram)
	Type string `json:"type"`

	// Labels are the labels to group by
	Labels []string `json:"labels"`

	// Buckets are the histogram buckets (only for histogram aggregation)
	Buckets []float64 `json:"buckets,omitempty"`
}

// AggregationFunctionBlock implements the function block for metric aggregation
type AggregationFunctionBlock struct {
	fb.BaseFunctionBlock
	config         Config
	aggregators    map[string]Aggregator
	metricCh       chan *telemetry.Metric
	forwarder      telemetry.Forwarder
	shutdownCh     chan struct{}
	wg             sync.WaitGroup
	mu             sync.RWMutex
	aggregatorsMu  sync.RWMutex
	flushTimersMu  sync.Mutex
	flushTimers    map[string]*time.Timer
	metricsFactory metrics.Factory
}

// Metrics for monitoring the aggregation function block
var (
	metricsBatchesProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "fb_agg_batches_processed_total",
		Help: "The total number of metric batches processed",
	})

	metricsAggregated = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "fb_agg_metrics_aggregated_total",
		Help: "The total number of metrics aggregated by type",
	}, []string{"type"})

	aggregationFlushes = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "fb_agg_flushes_total",
		Help: "The total number of aggregation flushes by aggregation type",
	}, []string{"type"})

	aggregationErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "fb_agg_errors_total",
		Help: "The total number of aggregation errors",
	})

	aggregationLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "fb_agg_latency_seconds",
		Help:    "Latency of metric aggregation operations",
		Buckets: prometheus.DefBuckets,
	})
)

// Aggregator defines the interface for metric aggregators
type Aggregator interface {
	// AddMetric adds a metric to the aggregator
	AddMetric(metric *telemetry.Metric) error

	// Flush flushes the aggregator and returns the resulting metrics
	Flush() ([]*telemetry.Metric, error)

	// Reset resets the aggregator state
	Reset()
}

// NewAggregationFunctionBlock creates a new aggregation function block
func NewAggregationFunctionBlock(name string, forwarder telemetry.Forwarder, metricsFactory metrics.Factory) *AggregationFunctionBlock {
	return &AggregationFunctionBlock{
		BaseFunctionBlock: fb.NewBaseFunctionBlock(name),
		aggregators:       make(map[string]Aggregator),
		shutdownCh:        make(chan struct{}),
		forwarder:         forwarder,
		flushTimers:       make(map[string]*time.Timer),
		metricsFactory:    metricsFactory,
	}
}

// Initialize initializes the aggregation function block
func (a *AggregationFunctionBlock) Initialize(ctx context.Context) error {
	log.Info().Str("function_block", a.Name()).Msg("Initializing aggregation function block")

	// Setup default configuration if none exists
	if a.config.WindowSeconds == 0 {
		a.config.WindowSeconds = 60 // Default to 60 seconds
	}

	if a.config.BufferSize == 0 {
		a.config.BufferSize = 1000 // Default buffer size
	}

	// Create metric channel
	a.metricCh = make(chan *telemetry.Metric, a.config.BufferSize)

	// Start the processing goroutine
	a.wg.Add(1)
	go a.processMetrics()

	a.SetReady(true)
	log.Info().Str("function_block", a.Name()).Msg("Aggregation function block initialized successfully")
	return nil
}

// ProcessBatch processes a batch of metrics
func (a *AggregationFunctionBlock) ProcessBatch(ctx context.Context, batch *fb.MetricBatch) (*fb.ProcessResult, error) {
	start := time.Now()
	defer func() {
		aggregationLatency.Observe(time.Since(start).Seconds())
	}()

	// Increment processed batches counter
	metricsBatchesProcessed.Inc()

	// Deserialize metrics from batch
	var metrics []*telemetry.Metric
	if err := json.Unmarshal(batch.Data, &metrics); err != nil {
		aggregationErrors.Inc()
		log.Error().Err(err).Str("function_block", a.Name()).Str("batch_id", batch.BatchID).Msg("Failed to deserialize metrics")
		return fb.NewErrorResult(batch.BatchID, fb.ErrorCodeInvalidInput, err, false), err
	}

	// Send metrics to the processing channel
	for _, metric := range metrics {
		select {
		case a.metricCh <- metric:
			// Successfully sent to channel
		default:
			// Channel is full, log warning and continue
			log.Warn().Str("function_block", a.Name()).Str("batch_id", batch.BatchID).Msg("Metric channel is full, dropping metric")
		}
	}

	return fb.NewSuccessResult(batch.BatchID), nil
}

// UpdateConfig updates the function block's configuration
func (a *AggregationFunctionBlock) UpdateConfig(ctx context.Context, configBytes []byte, generation int64) error {
	var newConfig Config
	if err := json.Unmarshal(configBytes, &newConfig); err != nil {
		log.Error().Err(err).Str("function_block", a.Name()).Msg("Failed to deserialize configuration")
		return fb.ErrConfigInvalid
	}

	log.Info().Str("function_block", a.Name()).Int64("generation", generation).Msg("Updating configuration")

	// Validate configuration
	if newConfig.WindowSeconds <= 0 {
		return fmt.Errorf("%w: windowSeconds must be positive", fb.ErrConfigInvalid)
	}

	if len(newConfig.Aggregations) == 0 {
		return fmt.Errorf("%w: at least one aggregation rule must be defined", fb.ErrConfigInvalid)
	}

	for i, rule := range newConfig.Aggregations {
		if rule.Metric == "" {
			return fmt.Errorf("%w: aggregation rule %d has empty metric name", fb.ErrConfigInvalid, i)
		}

		switch rule.Type {
		case "sum", "avg", "min", "max", "histogram":
			// These are valid
		default:
			return fmt.Errorf("%w: aggregation rule %d has invalid type: %s", fb.ErrConfigInvalid, i, rule.Type)
		}

		if rule.Type == "histogram" && (len(rule.Buckets) == 0) {
			return fmt.Errorf("%w: histogram aggregation rule %d has no buckets", fb.ErrConfigInvalid, i)
		}
	}

	// Update configuration and reset aggregators
	a.mu.Lock()
	a.config = newConfig
	a.mu.Unlock()

	// Create new aggregators based on the new configuration
	a.resetAggregators()

	// Update config generation
	a.SetConfigGeneration(generation)

	log.Info().Str("function_block", a.Name()).Int64("generation", generation).Msg("Configuration updated successfully")
	return nil
}

// Shutdown shuts down the function block
func (a *AggregationFunctionBlock) Shutdown(ctx context.Context) error {
	log.Info().Str("function_block", a.Name()).Msg("Shutting down aggregation function block")

	// Stop accepting new metrics
	a.SetReady(false)

	// Signal the processing goroutine to stop
	close(a.shutdownCh)

	// Wait for goroutines to finish with a timeout
	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Goroutines finished
	case <-ctx.Done():
		return fb.ErrShutdownTimeout
	}

	// Flush all aggregators one last time
	if err := a.flushAllAggregators(); err != nil {
		log.Error().Err(err).Str("function_block", a.Name()).Msg("Error flushing aggregators during shutdown")
		return err
	}

	log.Info().Str("function_block", a.Name()).Msg("Aggregation function block shut down successfully")
	return nil
}

// processMetrics processes incoming metrics and sends them to the appropriate aggregator
func (a *AggregationFunctionBlock) processMetrics() {
	defer a.wg.Done()

	log.Info().Str("function_block", a.Name()).Msg("Starting metric processing")

	for {
		select {
		case <-a.shutdownCh:
			log.Info().Str("function_block", a.Name()).Msg("Stopping metric processing")
			return
		case metric := <-a.metricCh:
			a.processMetric(metric)
		}
	}
}

// processMetric processes a single metric
func (a *AggregationFunctionBlock) processMetric(metric *telemetry.Metric) {
	// Find applicable aggregation rules
	a.mu.RLock()
	defer a.mu.RUnlock()

	for _, rule := range a.config.Aggregations {
		if rule.Metric == metric.Name {
			// Create a key for this metric + rule combination
			key := a.createAggregatorKey(rule, metric)

			// Get or create aggregator
			agg, err := a.getOrCreateAggregator(key, rule)
			if err != nil {
				log.Error().Err(err).Str("function_block", a.Name()).Str("metric", metric.Name).Msg("Failed to create aggregator")
				aggregationErrors.Inc()
				continue
			}

			// Add metric to aggregator
			if err := agg.AddMetric(metric); err != nil {
				log.Error().Err(err).Str("function_block", a.Name()).Str("metric", metric.Name).Msg("Failed to add metric to aggregator")
				aggregationErrors.Inc()
				continue
			}

			// Increment the counter for this type of aggregation
			metricsAggregated.WithLabelValues(rule.Type).Inc()

			// Ensure there's a flush timer for this aggregator
			a.ensureFlushTimer(key, rule.Type)
		}
	}
}

// createAggregatorKey creates a unique key for an aggregator based on the rule and metric labels
func (a *AggregationFunctionBlock) createAggregatorKey(rule AggregationRule, metric *telemetry.Metric) string {
	key := fmt.Sprintf("%s:%s", metric.Name, rule.Type)

	// Add relevant labels to the key
	for _, labelName := range rule.Labels {
		if labelValue, ok := metric.Labels[labelName]; ok {
			key = fmt.Sprintf("%s:%s=%s", key, labelName, labelValue)
		}
	}

	return key
}

// getOrCreateAggregator gets an existing aggregator or creates a new one
func (a *AggregationFunctionBlock) getOrCreateAggregator(key string, rule AggregationRule) (Aggregator, error) {
	a.aggregatorsMu.RLock()
	agg, ok := a.aggregators[key]
	a.aggregatorsMu.RUnlock()

	if !ok {
		// Create a new aggregator
		var newAgg Aggregator
		var err error

		switch rule.Type {
		case "sum":
			newAgg = NewSumAggregator()
		case "avg":
			newAgg = NewAvgAggregator()
		case "min":
			newAgg = NewMinAggregator()
		case "max":
			newAgg = NewMaxAggregator()
		case "histogram":
			newAgg, err = NewHistogramAggregator(rule.Buckets)
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unknown aggregation type: %s", rule.Type)
		}

		a.aggregatorsMu.Lock()
		a.aggregators[key] = newAgg
		a.aggregatorsMu.Unlock()

		return newAgg, nil
	}

	return agg, nil
}

// ensureFlushTimer ensures there's a flush timer for an aggregator
func (a *AggregationFunctionBlock) ensureFlushTimer(key string, aggType string) {
	a.flushTimersMu.Lock()
	defer a.flushTimersMu.Unlock()

	if _, ok := a.flushTimers[key]; !ok {
		// Create a new timer
		timer := time.AfterFunc(time.Duration(a.config.WindowSeconds)*time.Second, func() {
			if err := a.flushAggregator(key, aggType); err != nil {
				log.Error().Err(err).Str("function_block", a.Name()).Str("key", key).Msg("Failed to flush aggregator")
			}
		})

		a.flushTimers[key] = timer
	}
}

// flushAggregator flushes a specific aggregator
func (a *AggregationFunctionBlock) flushAggregator(key string, aggType string) error {
	a.aggregatorsMu.RLock()
	agg, ok := a.aggregators[key]
	a.aggregatorsMu.RUnlock()

	if !ok {
		return fmt.Errorf("aggregator not found: %s", key)
	}

	// Flush the aggregator
	metrics, err := agg.Flush()
	if err != nil {
		aggregationErrors.Inc()
		return err
	}

	// Reset the aggregator
	agg.Reset()

	// Forward the metrics
	if len(metrics) > 0 {
		if err := a.forwarder.Forward(metrics); err != nil {
			log.Error().Err(err).Str("function_block", a.Name()).Str("key", key).Msg("Failed to forward aggregated metrics")
			return err
		}
	}

	// Increment the flush counter
	aggregationFlushes.WithLabelValues(aggType).Inc()

	// Reset the timer
	a.flushTimersMu.Lock()
	if timer, ok := a.flushTimers[key]; ok {
		timer.Reset(time.Duration(a.config.WindowSeconds) * time.Second)
	} else {
		// Create a new timer if it doesn't exist
		a.flushTimers[key] = time.AfterFunc(time.Duration(a.config.WindowSeconds)*time.Second, func() {
			if err := a.flushAggregator(key, aggType); err != nil {
				log.Error().Err(err).Str("function_block", a.Name()).Str("key", key).Msg("Failed to flush aggregator")
			}
		})
	}
	a.flushTimersMu.Unlock()

	return nil
}

// flushAllAggregators flushes all aggregators
func (a *AggregationFunctionBlock) flushAllAggregators() error {
	a.aggregatorsMu.RLock()
	keys := make([]string, 0, len(a.aggregators))
	for key := range a.aggregators {
		keys = append(keys, key)
	}
	a.aggregatorsMu.RUnlock()

	var firstErr error
	for _, key := range keys {
		// Extract the type from the key (format: "metric:type:labels")
		parts := splitKey(key)
		if len(parts) < 2 {
			log.Warn().Str("function_block", a.Name()).Str("key", key).Msg("Invalid aggregator key")
			continue
		}

		err := a.flushAggregator(key, parts[1])
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// resetAggregators removes all existing aggregators and flush timers
func (a *AggregationFunctionBlock) resetAggregators() {
	// Cancel all existing flush timers
	a.flushTimersMu.Lock()
	for _, timer := range a.flushTimers {
		timer.Stop()
	}
	a.flushTimers = make(map[string]*time.Timer)
	a.flushTimersMu.Unlock()

	// Clear all aggregators
	a.aggregatorsMu.Lock()
	a.aggregators = make(map[string]Aggregator)
	a.aggregatorsMu.Unlock()
}

// Helper function to split an aggregator key
func splitKey(key string) []string {
	var result []string
	var current string
	var inLabel bool

	for _, r := range key {
		switch r {
		case ':':
			if !inLabel {
				result = append(result, current)
				current = ""
			} else {
				current += string(r)
			}
		case '=':
			current += string(r)
			inLabel = true
		default:
			current += string(r)
		}
	}

	if current != "" {
		result = append(result, current)
	}

	return result
}
