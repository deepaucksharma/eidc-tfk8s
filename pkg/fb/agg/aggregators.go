package agg

import (
	"fmt"
	"math"
	"sort"
	"sync"

	"github.com/newrelic/nrdot-internal-devlab/pkg/telemetry"
)

// SumAggregator implements sum aggregation
type SumAggregator struct {
	mu    sync.Mutex
	sum   float64
	count int
	name  string
	attrs map[string]string
}

// NewSumAggregator creates a new sum aggregator
func NewSumAggregator() *SumAggregator {
	return &SumAggregator{
		attrs: make(map[string]string),
	}
}

// AddMetric adds a metric to the aggregator
func (a *SumAggregator) AddMetric(metric *telemetry.Metric) error {
	if metric == nil {
		return fmt.Errorf("metric cannot be nil")
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// On first metric, set the name and attributes
	if a.count == 0 {
		a.name = metric.Name
		// Copy labels for the output metric
		for k, v := range metric.Labels {
			a.attrs[k] = v
		}
	}

	// Add the value
	a.sum += metric.Value
	a.count++

	return nil
}

// Flush returns the aggregated metric
func (a *SumAggregator) Flush() ([]*telemetry.Metric, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.count == 0 {
		return nil, nil
	}

	// Create the result metric
	metric := &telemetry.Metric{
		Name:   a.name,
		Value:  a.sum,
		Labels: make(map[string]string),
	}

	// Copy attributes
	for k, v := range a.attrs {
		metric.Labels[k] = v
	}

	return []*telemetry.Metric{metric}, nil
}

// Reset resets the aggregator state
func (a *SumAggregator) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.sum = 0
	a.count = 0
	// Keep the name and attributes for the next cycle
}

// AvgAggregator implements average aggregation
type AvgAggregator struct {
	mu    sync.Mutex
	sum   float64
	count int
	name  string
	attrs map[string]string
}

// NewAvgAggregator creates a new average aggregator
func NewAvgAggregator() *AvgAggregator {
	return &AvgAggregator{
		attrs: make(map[string]string),
	}
}

// AddMetric adds a metric to the aggregator
func (a *AvgAggregator) AddMetric(metric *telemetry.Metric) error {
	if metric == nil {
		return fmt.Errorf("metric cannot be nil")
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// On first metric, set the name and attributes
	if a.count == 0 {
		a.name = metric.Name
		// Copy labels for the output metric
		for k, v := range metric.Labels {
			a.attrs[k] = v
		}
	}

	// Add the value
	a.sum += metric.Value
	a.count++

	return nil
}

// Flush returns the aggregated metric
func (a *AvgAggregator) Flush() ([]*telemetry.Metric, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.count == 0 {
		return nil, nil
	}

	// Create the result metric
	metric := &telemetry.Metric{
		Name:   a.name,
		Value:  a.sum / float64(a.count),
		Labels: make(map[string]string),
	}

	// Copy attributes
	for k, v := range a.attrs {
		metric.Labels[k] = v
	}

	return []*telemetry.Metric{metric}, nil
}

// Reset resets the aggregator state
func (a *AvgAggregator) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.sum = 0
	a.count = 0
	// Keep the name and attributes for the next cycle
}

// MinAggregator implements minimum value aggregation
type MinAggregator struct {
	mu    sync.Mutex
	min   float64
	count int
	name  string
	attrs map[string]string
}

// NewMinAggregator creates a new minimum aggregator
func NewMinAggregator() *MinAggregator {
	return &MinAggregator{
		min:   math.MaxFloat64,
		attrs: make(map[string]string),
	}
}

// AddMetric adds a metric to the aggregator
func (a *MinAggregator) AddMetric(metric *telemetry.Metric) error {
	if metric == nil {
		return fmt.Errorf("metric cannot be nil")
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// On first metric, set the name and attributes
	if a.count == 0 {
		a.name = metric.Name
		// Copy labels for the output metric
		for k, v := range metric.Labels {
			a.attrs[k] = v
		}
	}

	// Update the minimum value
	if metric.Value < a.min {
		a.min = metric.Value
	}
	a.count++

	return nil
}

// Flush returns the aggregated metric
func (a *MinAggregator) Flush() ([]*telemetry.Metric, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.count == 0 {
		return nil, nil
	}

	// Create the result metric
	metric := &telemetry.Metric{
		Name:   a.name,
		Value:  a.min,
		Labels: make(map[string]string),
	}

	// Copy attributes
	for k, v := range a.attrs {
		metric.Labels[k] = v
	}

	return []*telemetry.Metric{metric}, nil
}

// Reset resets the aggregator state
func (a *MinAggregator) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.min = math.MaxFloat64
	a.count = 0
	// Keep the name and attributes for the next cycle
}

// MaxAggregator implements maximum value aggregation
type MaxAggregator struct {
	mu    sync.Mutex
	max   float64
	count int
	name  string
	attrs map[string]string
}

// NewMaxAggregator creates a new maximum aggregator
func NewMaxAggregator() *MaxAggregator {
	return &MaxAggregator{
		max:   -math.MaxFloat64,
		attrs: make(map[string]string),
	}
}

// AddMetric adds a metric to the aggregator
func (a *MaxAggregator) AddMetric(metric *telemetry.Metric) error {
	if metric == nil {
		return fmt.Errorf("metric cannot be nil")
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// On first metric, set the name and attributes
	if a.count == 0 {
		a.name = metric.Name
		// Copy labels for the output metric
		for k, v := range metric.Labels {
			a.attrs[k] = v
		}
	}

	// Update the maximum value
	if metric.Value > a.max {
		a.max = metric.Value
	}
	a.count++

	return nil
}

// Flush returns the aggregated metric
func (a *MaxAggregator) Flush() ([]*telemetry.Metric, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.count == 0 {
		return nil, nil
	}

	// Create the result metric
	metric := &telemetry.Metric{
		Name:   a.name,
		Value:  a.max,
		Labels: make(map[string]string),
	}

	// Copy attributes
	for k, v := range a.attrs {
		metric.Labels[k] = v
	}

	return []*telemetry.Metric{metric}, nil
}

// Reset resets the aggregator state
func (a *MaxAggregator) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.max = -math.MaxFloat64
	a.count = 0
	// Keep the name and attributes for the next cycle
}

// HistogramAggregator implements histogram aggregation
type HistogramAggregator struct {
	mu      sync.Mutex
	buckets []float64
	counts  []int
	count   int
	name    string
	attrs   map[string]string
}

// NewHistogramAggregator creates a new histogram aggregator
func NewHistogramAggregator(buckets []float64) (*HistogramAggregator, error) {
	if len(buckets) == 0 {
		return nil, fmt.Errorf("histogram buckets cannot be empty")
	}

	// Make a copy of the buckets and sort them
	sortedBuckets := make([]float64, len(buckets))
	copy(sortedBuckets, buckets)
	sort.Float64s(sortedBuckets)

	// Initialize with counts at 0
	counts := make([]int, len(sortedBuckets)+1) // +1 for the "Inf" bucket

	return &HistogramAggregator{
		buckets: sortedBuckets,
		counts:  counts,
		attrs:   make(map[string]string),
	}, nil
}

// AddMetric adds a metric to the aggregator
func (a *HistogramAggregator) AddMetric(metric *telemetry.Metric) error {
	if metric == nil {
		return fmt.Errorf("metric cannot be nil")
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// On first metric, set the name and attributes
	if a.count == 0 {
		a.name = metric.Name
		// Copy labels for the output metric
		for k, v := range metric.Labels {
			a.attrs[k] = v
		}
	}

	// Find the appropriate bucket
	bucketIndex := len(a.buckets) // Default to the "Inf" bucket
	for i, upperBound := range a.buckets {
		if metric.Value <= upperBound {
			bucketIndex = i
			break
		}
	}

	// Increment the bucket count
	a.counts[bucketIndex]++
	a.count++

	return nil
}

// Flush returns the aggregated histogram metrics
func (a *HistogramAggregator) Flush() ([]*telemetry.Metric, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.count == 0 {
		return nil, nil
	}

	// Create a metric for each bucket
	metrics := make([]*telemetry.Metric, len(a.buckets)+1)

	cumulativeCount := 0
	for i, upperBound := range a.buckets {
		cumulativeCount += a.counts[i]
		
		metric := &telemetry.Metric{
			Name:   fmt.Sprintf("%s_bucket", a.name),
			Value:  float64(cumulativeCount),
			Labels: make(map[string]string),
		}
		
		// Copy attributes
		for k, v := range a.attrs {
			metric.Labels[k] = v
		}
		
		// Add le (less than or equal) label
		metric.Labels["le"] = fmt.Sprintf("%g", upperBound)
		
		metrics[i] = metric
	}

	// Add the +Inf bucket
	cumulativeCount += a.counts[len(a.buckets)]
	infiniteMetric := &telemetry.Metric{
		Name:   fmt.Sprintf("%s_bucket", a.name),
		Value:  float64(cumulativeCount),
		Labels: make(map[string]string),
	}
	
	// Copy attributes
	for k, v := range a.attrs {
		infiniteMetric.Labels[k] = v
	}
	
	// Add le label for Inf
	infiniteMetric.Labels["le"] = "+Inf"
	
	metrics[len(a.buckets)] = infiniteMetric

	return metrics, nil
}

// Reset resets the aggregator state
func (a *HistogramAggregator) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Reset all counts to 0
	for i := range a.counts {
		a.counts[i] = 0
	}
	a.count = 0
	// Keep the name, attributes, and buckets for the next cycle
}
