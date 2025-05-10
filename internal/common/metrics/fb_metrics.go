package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// FBMetrics contains the standard metrics that all Function Blocks should expose
type FBMetrics struct {
	// FB identification
	FBName string

	// Counters
	BatchesReceivedTotal   prometheus.Counter
	BatchesProcessedTotal  prometheus.Counter
	BatchesForwardedTotal  prometheus.Counter
	BatchesRejectedTotal   prometheus.Counter
	BatchesDLQTotal        prometheus.Counter
	ProcessingErrorsTotal  prometheus.Counter
	ValidationErrorsTotal  prometheus.Counter  // Added for validation errors

	// Gauges
	ActiveConnections      prometheus.Gauge
	IsReady                prometheus.Gauge
	ConfigGeneration       prometheus.Gauge

	// Histograms
	ProcessingLatency      prometheus.Histogram
	ForwardingLatency      prometheus.Histogram
}

// NewFBMetrics creates a new set of standard metrics for a Function Block
func NewFBMetrics(fbName string) *FBMetrics {
	m := &FBMetrics{
		FBName: fbName,
	}

	// Common labels for all metrics
	labels := prometheus.Labels{
		"fb_name": fbName,
	}

	// Counters
	m.BatchesReceivedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "fb_batches_received_total",
		Help: "Total number of batches received by the function block",
		ConstLabels: labels,
	})

	m.BatchesProcessedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "fb_batches_processed_total",
		Help: "Total number of batches successfully processed by the function block",
		ConstLabels: labels,
	})

	m.BatchesForwardedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "fb_batches_forwarded_total",
		Help: "Total number of batches successfully forwarded to the next function block",
		ConstLabels: labels,
	})

	m.BatchesRejectedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "fb_batches_rejected_total",
		Help: "Total number of batches rejected by the function block",
		ConstLabels: labels,
	})

	m.BatchesDLQTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "fb_batches_dlq_total",
		Help: "Total number of batches sent to the dead letter queue",
		ConstLabels: labels,
	})

	m.ProcessingErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "fb_processing_errors_total",
		Help: "Total number of errors that occurred during processing",
		ConstLabels: labels,
	})

	m.ValidationErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "fb_validation_errors_total",
		Help: "Total number of validation errors",
		ConstLabels: labels,
	})

	// Gauges
	m.ActiveConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "fb_active_connections",
		Help: "Number of active connections to other function blocks",
		ConstLabels: labels,
	})

	m.IsReady = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "fb_is_ready",
		Help: "Whether the function block is ready to process batches (1=ready, 0=not ready)",
		ConstLabels: labels,
	})

	m.ConfigGeneration = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "fb_config_generation",
		Help: "The current configuration generation the function block is using",
		ConstLabels: labels,
	})

	// Histograms
	m.ProcessingLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "fb_processing_latency_seconds",
		Help: "Latency of batch processing in seconds",
		ConstLabels: labels,
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	})

	m.ForwardingLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "fb_forwarding_latency_seconds",
		Help: "Latency of batch forwarding to the next function block in seconds",
		ConstLabels: labels,
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	})

	return m
}

// RecordBatchReceived records that a batch was received
func (m *FBMetrics) RecordBatchReceived() {
	m.BatchesReceivedTotal.Inc()
}

// RecordBatchProcessed records that a batch was processed
func (m *FBMetrics) RecordBatchProcessed(processingTimeSeconds float64) {
	m.BatchesProcessedTotal.Inc()
	m.ProcessingLatency.Observe(processingTimeSeconds)
}

// RecordBatchForwarded records that a batch was forwarded
func (m *FBMetrics) RecordBatchForwarded(forwardingTimeSeconds float64) {
	m.BatchesForwardedTotal.Inc()
	m.ForwardingLatency.Observe(forwardingTimeSeconds)
}

// RecordBatchRejected records that a batch was rejected
func (m *FBMetrics) RecordBatchRejected() {
	m.BatchesRejectedTotal.Inc()
}

// RecordBatchDLQ records that a batch was sent to the DLQ
func (m *FBMetrics) RecordBatchDLQ() {
	m.BatchesDLQTotal.Inc()
}

// RecordProcessingError records that an error occurred during processing
func (m *FBMetrics) RecordProcessingError() {
	m.ProcessingErrorsTotal.Inc()
}

// RecordBatchValidationError records that a validation error occurred
func (m *FBMetrics) RecordBatchValidationError() {
	m.ValidationErrorsTotal.Inc()
}

// SetActiveConnections sets the number of active connections
func (m *FBMetrics) SetActiveConnections(count int) {
	m.ActiveConnections.Set(float64(count))
}

// SetReady sets whether the function block is ready
func (m *FBMetrics) SetReady(isReady bool) {
	if isReady {
		m.IsReady.Set(1)
	} else {
		m.IsReady.Set(0)
	}
}

// SetConfigGeneration sets the current configuration generation
func (m *FBMetrics) SetConfigGeneration(generation int64) {
	m.ConfigGeneration.Set(float64(generation))
}