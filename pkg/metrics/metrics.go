package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTP request metrics
	RequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "heif_conversion_requests_total",
			Help: "Total number of conversion requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "heif_conversion_request_duration_seconds",
			Help:    "Request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"endpoint"},
	)

	// Conversion metrics
	ConversionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "heif_conversions_total",
			Help: "Total number of image conversions",
		},
		[]string{"status"}, // success, error, cancelled
	)

	ConversionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "heif_conversion_duration_seconds",
			Help:    "Conversion duration in seconds",
			Buckets: []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"mode"}, // adaptive, fixed_quality, fast, fast_quality
	)

	ConversionBytes = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "heif_conversion_bytes",
			Help:    "Conversion input/output bytes",
			Buckets: []float64{1024, 10240, 102400, 512000, 1048576, 5242880, 10485760},
		},
		[]string{"direction"}, // input, output
	)

	// Queue/Pool metrics
	WorkerPoolQueueSize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "heif_worker_pool_queue_size",
			Help: "Current number of jobs in worker pool queue",
		},
	)

	WorkerPoolActiveJobs = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "heif_worker_pool_active_jobs",
			Help: "Current number of active conversion jobs",
		},
	)

	// Rate limiting metrics
	RateLimitExceeded = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "heif_rate_limit_exceeded_total",
			Help: "Total number of requests rejected due to rate limiting",
		},
		[]string{"ip_prefix"}, // First octet for privacy
	)

	// Concurrency metrics
	ConcurrentRequests = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "heif_concurrent_requests",
			Help: "Current number of concurrent requests being processed",
		},
	)

	ConcurrencyLimitExceeded = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "heif_concurrency_limit_exceeded_total",
			Help: "Total number of requests rejected due to concurrency limit",
		},
	)

	// Memory metrics
	MemoryPoolHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "heif_memory_pool_hits_total",
			Help: "Total number of buffer pool hits",
		},
		[]string{"size"}, // small, medium, large, xlarge
	)

	MemoryPoolMisses = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "heif_memory_pool_misses_total",
			Help: "Total number of buffer pool misses",
		},
		[]string{"size"}, // small, medium, large, xlarge
	)
)

// RecordRequest records an HTTP request
func RecordRequest(method, endpoint, status string, duration float64) {
	RequestsTotal.WithLabelValues(method, endpoint, status).Inc()
	RequestDuration.WithLabelValues(endpoint).Observe(duration)
}

// RecordConversion records a conversion operation
func RecordConversion(status, mode string, duration float64, inputBytes, outputBytes int) {
	ConversionsTotal.WithLabelValues(status).Inc()
	ConversionDuration.WithLabelValues(mode).Observe(duration)
	ConversionBytes.WithLabelValues("input").Observe(float64(inputBytes))
	ConversionBytes.WithLabelValues("output").Observe(float64(outputBytes))
}

// UpdateWorkerPoolMetrics updates worker pool metrics
func UpdateWorkerPoolMetrics(queueSize, activeJobs int) {
	WorkerPoolQueueSize.Set(float64(queueSize))
	WorkerPoolActiveJobs.Set(float64(activeJobs))
}

// RecordRateLimitExceeded records a rate limit rejection
func RecordRateLimitExceeded(ipPrefix string) {
	RateLimitExceeded.WithLabelValues(ipPrefix).Inc()
}

// UpdateConcurrency updates concurrent request gauge
func UpdateConcurrency(count int) {
	ConcurrentRequests.Set(float64(count))
}

// RecordConcurrencyLimitExceeded records a concurrency limit rejection
func RecordConcurrencyLimitExceeded() {
	ConcurrencyLimitExceeded.Inc()
}

// RecordPoolHit records a buffer pool hit
func RecordPoolHit(size string) {
	MemoryPoolHits.WithLabelValues(size).Inc()
}

// RecordPoolMiss records a buffer pool miss
func RecordPoolMiss(size string) {
	MemoryPoolMisses.WithLabelValues(size).Inc()
}
