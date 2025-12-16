package observability

import (
	"sync"
	"sync/atomic"
	"time"
)

// MetricType represents the type of a metric.
type MetricType int

const (
	// MetricTypeCounter represents a counter metric (monotonically increasing).
	MetricTypeCounter MetricType = iota
	// MetricTypeGauge represents a gauge metric (can go up or down).
	MetricTypeGauge
	// MetricTypeHistogram represents a histogram metric (distribution of values).
	MetricTypeHistogram
)

// Metric represents a single metric.
type Metric struct {
	Name      string
	Type      MetricType
	Value     float64
	Labels    map[string]string
	Timestamp time.Time
}

// Counter represents a counter metric.
type Counter struct {
	name   string
	labels map[string]string
	value  int64 // Atomic counter
}

// Gauge represents a gauge metric.
type Gauge struct {
	name   string
	labels map[string]string
	value  int64 // Atomic gauge value (stored as int64, converted to float64)
}

// Histogram represents a histogram metric.
type Histogram struct {
	name   string
	labels map[string]string
	buckets []float64
	values  []int64 // Atomic counters for each bucket
}

// MetricsRegistry manages all metrics.
type MetricsRegistry struct {
	mu sync.RWMutex

	counters   map[string]*Counter
	gauges     map[string]*Gauge
	histograms map[string]*Histogram
}

// NewMetricsRegistry creates a new metrics registry.
func NewMetricsRegistry() *MetricsRegistry {
	return &MetricsRegistry{
		counters:   make(map[string]*Counter),
		gauges:     make(map[string]*Gauge),
		histograms: make(map[string]*Histogram),
	}
}

// Counter creates or returns an existing counter metric.
func (r *MetricsRegistry) Counter(name string, labels map[string]string) *Counter {
	key := metricKey(name, labels)
	
	r.mu.RLock()
	if counter, exists := r.counters[key]; exists {
		r.mu.RUnlock()
		return counter
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	if counter, exists := r.counters[key]; exists {
		return counter
	}

	counter := &Counter{
		name:   name,
		labels: labels,
		value:  0,
	}
	r.counters[key] = counter
	return counter
}

// Gauge creates or returns an existing gauge metric.
func (r *MetricsRegistry) Gauge(name string, labels map[string]string) *Gauge {
	key := metricKey(name, labels)
	
	r.mu.RLock()
	if gauge, exists := r.gauges[key]; exists {
		r.mu.RUnlock()
		return gauge
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	if gauge, exists := r.gauges[key]; exists {
		return gauge
	}

	gauge := &Gauge{
		name:   name,
		labels: labels,
		value:  0,
	}
	r.gauges[key] = gauge
	return gauge
}

// Histogram creates or returns an existing histogram metric.
func (r *MetricsRegistry) Histogram(name string, labels map[string]string, buckets []float64) *Histogram {
	key := metricKey(name, labels)
	
	r.mu.RLock()
	if hist, exists := r.histograms[key]; exists {
		r.mu.RUnlock()
		return hist
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	if hist, exists := r.histograms[key]; exists {
		return hist
	}

	hist := &Histogram{
		name:    name,
		labels:  labels,
		buckets: buckets,
		values:  make([]int64, len(buckets)+1), // +1 for overflow bucket
	}
	r.histograms[key] = hist
	return hist
}

// Inc increments a counter by 1.
func (c *Counter) Inc() {
	atomic.AddInt64(&c.value, 1)
}

// Add increments a counter by the given value.
func (c *Counter) Add(delta int64) {
	atomic.AddInt64(&c.value, delta)
}

// Get returns the current counter value.
func (c *Counter) Get() int64 {
	return atomic.LoadInt64(&c.value)
}

// Set sets a gauge value.
func (g *Gauge) Set(value float64) {
	atomic.StoreInt64(&g.value, int64(value*1000)) // Store as int64 with 3 decimal precision
}

// Add adds to a gauge value.
func (g *Gauge) Add(delta float64) {
	atomic.AddInt64(&g.value, int64(delta*1000))
}

// Get returns the current gauge value.
func (g *Gauge) Get() float64 {
	return float64(atomic.LoadInt64(&g.value)) / 1000.0
}

// Observe records a value in the histogram.
func (h *Histogram) Observe(value float64) {
	// Find the appropriate bucket
	bucketIndex := len(h.buckets) // Default to overflow bucket
	for i, bucket := range h.buckets {
		if value <= bucket {
			bucketIndex = i
			break
		}
	}
	atomic.AddInt64(&h.values[bucketIndex], 1)
}

// Get returns the histogram values for each bucket.
func (h *Histogram) Get() []int64 {
	values := make([]int64, len(h.values))
	for i := range h.values {
		values[i] = atomic.LoadInt64(&h.values[i])
	}
	return values
}

// GetAllMetrics returns all metrics as a slice.
func (r *MetricsRegistry) GetAllMetrics() []Metric {
	r.mu.RLock()
	defer r.mu.RUnlock()

	metrics := make([]Metric, 0)
	now := time.Now()

	// Collect counters
	for _, counter := range r.counters {
		metrics = append(metrics, Metric{
			Name:      counter.name,
			Type:      MetricTypeCounter,
			Value:     float64(counter.Get()),
			Labels:    counter.labels,
			Timestamp: now,
		})
	}

	// Collect gauges
	for _, gauge := range r.gauges {
		metrics = append(metrics, Metric{
			Name:      gauge.name,
			Type:      MetricTypeGauge,
			Value:     gauge.Get(),
			Labels:    gauge.labels,
			Timestamp: now,
		})
	}

	// Collect histograms (sum of all buckets)
	for _, hist := range r.histograms {
		values := hist.Get()
		sum := int64(0)
		for _, v := range values {
			sum += v
		}
		metrics = append(metrics, Metric{
			Name:      hist.name,
			Type:      MetricTypeHistogram,
			Value:     float64(sum),
			Labels:    hist.labels,
			Timestamp: now,
		})
	}

	return metrics
}

// metricKey creates a unique key for a metric with labels.
func metricKey(name string, labels map[string]string) string {
	if len(labels) == 0 {
		return name
	}
	// Simple key generation - in production, use a more efficient method
	key := name
	for k, v := range labels {
		key += ":" + k + "=" + v
	}
	return key
}

// DefaultRegistry is the default metrics registry.
var defaultRegistry *MetricsRegistry
var defaultRegistryOnce sync.Once

// DefaultMetricsRegistry returns the default metrics registry.
func DefaultMetricsRegistry() *MetricsRegistry {
	defaultRegistryOnce.Do(func() {
		defaultRegistry = NewMetricsRegistry()
	})
	return defaultRegistry
}

