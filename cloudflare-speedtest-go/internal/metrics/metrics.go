package metrics

import (
	"sync"
	"time"
)

// MetricType represents different types of metrics
type MetricType string

const (
	MetricTypeCounter   MetricType = "counter"
	MetricTypeGauge     MetricType = "gauge"
	MetricTypeHistogram MetricType = "histogram"
	MetricTypeTimer     MetricType = "timer"
)

// Metric represents a single metric
type Metric struct {
	Name        string            `json:"name"`
	Type        MetricType        `json:"type"`
	Value       float64           `json:"value"`
	Count       int64             `json:"count"`
	LastUpdated time.Time         `json:"last_updated"`
	Tags        map[string]string `json:"tags,omitempty"`
}

// SpeedSample represents a speed measurement sample for sliding window
type SpeedSample struct {
	Timestamp time.Time `json:"timestamp"`
	Speed     float64   `json:"speed"`    // Speed in Mbps
	Bytes     int64     `json:"bytes"`    // Bytes downloaded
	Duration  float64   `json:"duration"` // Duration in seconds
}

// SlidingWindow maintains a sliding window of speed samples
type SlidingWindow struct {
	samples    []SpeedSample
	maxSize    int
	windowSize time.Duration
	mu         sync.RWMutex
}

// PerformanceStats represents overall performance statistics
type PerformanceStats struct {
	TestsStarted      int64     `json:"tests_started"`
	TestsCompleted    int64     `json:"tests_completed"`
	TestsSuccessful   int64     `json:"tests_successful"`
	TestsFailed       int64     `json:"tests_failed"`
	AverageSpeed      float64   `json:"average_speed"`
	PeakSpeed         float64   `json:"peak_speed"`
	AverageLatency    float64   `json:"average_latency"`
	MinLatency        float64   `json:"min_latency"`
	MaxLatency        float64   `json:"max_latency"`
	TotalDataTransfer int64     `json:"total_data_transfer"`
	TestDuration      float64   `json:"test_duration_seconds"`
	LastUpdated       time.Time `json:"last_updated"`
}

// Metrics manages performance metrics and statistics
type Metrics struct {
	mu               sync.RWMutex
	metrics          map[string]*Metric
	speedWindow      *SlidingWindow
	performanceStats *PerformanceStats
	startTime        time.Time
	speedSamples     []float64 // For calculating averages
	latencySamples   []float64 // For calculating averages
}

// New creates a new metrics manager
func New() *Metrics {
	return &Metrics{
		metrics:     make(map[string]*Metric),
		speedWindow: NewSlidingWindow(100, 30*time.Second), // 100 samples, 30 second window
		performanceStats: &PerformanceStats{
			MinLatency: 999999, // Initialize to high value
		},
		startTime:      time.Now(),
		speedSamples:   make([]float64, 0),
		latencySamples: make([]float64, 0),
	}
}

// NewSlidingWindow creates a new sliding window
func NewSlidingWindow(maxSize int, windowSize time.Duration) *SlidingWindow {
	return &SlidingWindow{
		samples:    make([]SpeedSample, 0, maxSize),
		maxSize:    maxSize,
		windowSize: windowSize,
	}
}

// RecordCounter increments a counter metric
func (m *Metrics) RecordCounter(name string, value float64, tags map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	metric, exists := m.metrics[name]
	if !exists {
		metric = &Metric{
			Name: name,
			Type: MetricTypeCounter,
			Tags: tags,
		}
		m.metrics[name] = metric
	}

	metric.Value += value
	metric.Count++
	metric.LastUpdated = time.Now()
}

// RecordGauge sets a gauge metric value
func (m *Metrics) RecordGauge(name string, value float64, tags map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	metric, exists := m.metrics[name]
	if !exists {
		metric = &Metric{
			Name: name,
			Type: MetricTypeGauge,
			Tags: tags,
		}
		m.metrics[name] = metric
	}

	metric.Value = value
	metric.Count++
	metric.LastUpdated = time.Now()
}

// RecordTimer records a timing metric
func (m *Metrics) RecordTimer(name string, duration time.Duration, tags map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	metric, exists := m.metrics[name]
	if !exists {
		metric = &Metric{
			Name: name,
			Type: MetricTypeTimer,
			Tags: tags,
		}
		m.metrics[name] = metric
	}

	// Calculate running average
	totalValue := metric.Value * float64(metric.Count)
	metric.Count++
	metric.Value = (totalValue + duration.Seconds()) / float64(metric.Count)
	metric.LastUpdated = time.Now()
}

// RecordSpeedSample records a speed measurement sample
func (m *Metrics) RecordSpeedSample(speed float64, bytes int64, duration float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Add to sliding window
	sample := SpeedSample{
		Timestamp: time.Now(),
		Speed:     speed,
		Bytes:     bytes,
		Duration:  duration,
	}
	m.speedWindow.AddSample(sample)

	// Update performance stats
	m.speedSamples = append(m.speedSamples, speed)
	if speed > m.performanceStats.PeakSpeed {
		m.performanceStats.PeakSpeed = speed
	}

	// Calculate average speed
	total := 0.0
	for _, s := range m.speedSamples {
		total += s
	}
	m.performanceStats.AverageSpeed = total / float64(len(m.speedSamples))
	m.performanceStats.TotalDataTransfer += bytes
	m.performanceStats.LastUpdated = time.Now()
}

// RecordLatencySample records a latency measurement
func (m *Metrics) RecordLatencySample(latency float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.latencySamples = append(m.latencySamples, latency)

	// Update min/max latency
	if latency < m.performanceStats.MinLatency {
		m.performanceStats.MinLatency = latency
	}
	if latency > m.performanceStats.MaxLatency {
		m.performanceStats.MaxLatency = latency
	}

	// Calculate average latency
	total := 0.0
	for _, l := range m.latencySamples {
		total += l
	}
	m.performanceStats.AverageLatency = total / float64(len(m.latencySamples))
	m.performanceStats.LastUpdated = time.Now()
}

// RecordTestStart records the start of a test
func (m *Metrics) RecordTestStart() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.performanceStats.TestsStarted++
	m.performanceStats.LastUpdated = time.Now()
}

// RecordTestComplete records the completion of a test
func (m *Metrics) RecordTestComplete(successful bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.performanceStats.TestsCompleted++
	if successful {
		m.performanceStats.TestsSuccessful++
	} else {
		m.performanceStats.TestsFailed++
	}

	// Update test duration
	m.performanceStats.TestDuration = time.Since(m.startTime).Seconds()
	m.performanceStats.LastUpdated = time.Now()
}

// GetMetric returns a specific metric
func (m *Metrics) GetMetric(name string) *Metric {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if metric, exists := m.metrics[name]; exists {
		// Return a copy to avoid race conditions
		return &Metric{
			Name:        metric.Name,
			Type:        metric.Type,
			Value:       metric.Value,
			Count:       metric.Count,
			LastUpdated: metric.LastUpdated,
			Tags:        metric.Tags,
		}
	}
	return nil
}

// GetAllMetrics returns all metrics
func (m *Metrics) GetAllMetrics() map[string]*Metric {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*Metric)
	for name, metric := range m.metrics {
		result[name] = &Metric{
			Name:        metric.Name,
			Type:        metric.Type,
			Value:       metric.Value,
			Count:       metric.Count,
			LastUpdated: metric.LastUpdated,
			Tags:        metric.Tags,
		}
	}
	return result
}

// GetPerformanceStats returns current performance statistics
func (m *Metrics) GetPerformanceStats() *PerformanceStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return &PerformanceStats{
		TestsStarted:      m.performanceStats.TestsStarted,
		TestsCompleted:    m.performanceStats.TestsCompleted,
		TestsSuccessful:   m.performanceStats.TestsSuccessful,
		TestsFailed:       m.performanceStats.TestsFailed,
		AverageSpeed:      m.performanceStats.AverageSpeed,
		PeakSpeed:         m.performanceStats.PeakSpeed,
		AverageLatency:    m.performanceStats.AverageLatency,
		MinLatency:        m.performanceStats.MinLatency,
		MaxLatency:        m.performanceStats.MaxLatency,
		TotalDataTransfer: m.performanceStats.TotalDataTransfer,
		TestDuration:      m.performanceStats.TestDuration,
		LastUpdated:       m.performanceStats.LastUpdated,
	}
}

// GetSmoothedSpeed returns smoothed speed using sliding window algorithm
func (m *Metrics) GetSmoothedSpeed() float64 {
	return m.speedWindow.GetSmoothedSpeed()
}

// GetRecentSamples returns recent speed samples
func (m *Metrics) GetRecentSamples(count int) []SpeedSample {
	return m.speedWindow.GetRecentSamples(count)
}

// Reset resets all metrics and statistics
func (m *Metrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.metrics = make(map[string]*Metric)
	m.speedWindow = NewSlidingWindow(100, 30*time.Second)
	m.performanceStats = &PerformanceStats{
		MinLatency: 999999,
	}
	m.startTime = time.Now()
	m.speedSamples = make([]float64, 0)
	m.latencySamples = make([]float64, 0)
}

// AddSample adds a sample to the sliding window
func (sw *SlidingWindow) AddSample(sample SpeedSample) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	// Add new sample
	sw.samples = append(sw.samples, sample)

	// Remove old samples outside the window
	cutoff := time.Now().Add(-sw.windowSize)
	for i, s := range sw.samples {
		if s.Timestamp.After(cutoff) {
			sw.samples = sw.samples[i:]
			break
		}
	}

	// Limit to max size
	if len(sw.samples) > sw.maxSize {
		sw.samples = sw.samples[len(sw.samples)-sw.maxSize:]
	}
}

// GetSmoothedSpeed calculates smoothed speed using weighted average
func (sw *SlidingWindow) GetSmoothedSpeed() float64 {
	sw.mu.RLock()
	defer sw.mu.RUnlock()

	if len(sw.samples) == 0 {
		return 0
	}

	if len(sw.samples) == 1 {
		return sw.samples[0].Speed
	}

	// Calculate weighted average with more recent samples having higher weight
	totalWeight := 0.0
	weightedSum := 0.0
	now := time.Now()

	for i, sample := range sw.samples {
		// Weight based on recency and position
		age := now.Sub(sample.Timestamp).Seconds()
		ageWeight := 1.0 / (1.0 + age/10.0)                       // Decay over 10 seconds
		positionWeight := float64(i+1) / float64(len(sw.samples)) // Later samples have higher weight

		weight := ageWeight * positionWeight
		weightedSum += sample.Speed * weight
		totalWeight += weight
	}

	if totalWeight > 0 {
		return weightedSum / totalWeight
	}

	return 0
}

// GetRecentSamples returns the most recent samples
func (sw *SlidingWindow) GetRecentSamples(count int) []SpeedSample {
	sw.mu.RLock()
	defer sw.mu.RUnlock()

	if count <= 0 || len(sw.samples) == 0 {
		return []SpeedSample{}
	}

	start := len(sw.samples) - count
	if start < 0 {
		start = 0
	}

	result := make([]SpeedSample, len(sw.samples)-start)
	copy(result, sw.samples[start:])
	return result
}

// GetSampleCount returns the number of samples in the window
func (sw *SlidingWindow) GetSampleCount() int {
	sw.mu.RLock()
	defer sw.mu.RUnlock()
	return len(sw.samples)
}

// GetWindowStats returns statistics about the sliding window
func (sw *SlidingWindow) GetWindowStats() map[string]interface{} {
	sw.mu.RLock()
	defer sw.mu.RUnlock()

	if len(sw.samples) == 0 {
		return map[string]interface{}{
			"sample_count":        0,
			"window_size_seconds": sw.windowSize.Seconds(),
			"max_size":            sw.maxSize,
		}
	}

	minSpeed := sw.samples[0].Speed
	maxSpeed := sw.samples[0].Speed
	totalSpeed := 0.0

	for _, sample := range sw.samples {
		if sample.Speed < minSpeed {
			minSpeed = sample.Speed
		}
		if sample.Speed > maxSpeed {
			maxSpeed = sample.Speed
		}
		totalSpeed += sample.Speed
	}

	avgSpeed := totalSpeed / float64(len(sw.samples))

	return map[string]interface{}{
		"sample_count":        len(sw.samples),
		"window_size_seconds": sw.windowSize.Seconds(),
		"max_size":            sw.maxSize,
		"min_speed":           minSpeed,
		"max_speed":           maxSpeed,
		"avg_speed":           avgSpeed,
		"smoothed_speed":      sw.GetSmoothedSpeed(),
		"oldest_sample":       sw.samples[0].Timestamp,
		"newest_sample":       sw.samples[len(sw.samples)-1].Timestamp,
	}
}
