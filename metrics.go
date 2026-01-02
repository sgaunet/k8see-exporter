package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Prometheus metrics for k8see-exporter observability.
var (
	// Counter metrics.
	eventsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "k8see_events_total",
			Help: "Total number of Kubernetes events processed",
		},
		[]string{"type", "namespace"},
	)

	eventsWrittenTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "k8see_events_written_total",
			Help: "Total number of events successfully written to Redis",
		},
	)

	eventsFailedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "k8see_events_failed_total",
			Help: "Total number of events that failed to write after retries",
		},
	)

	redisReconnectionsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "k8see_redis_reconnections_total",
			Help: "Total number of Redis reconnection attempts",
		},
	)

	// Gauge metrics.
	redisConnected = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "k8see_redis_connected",
			Help: "Redis connection status (1=connected, 0=disconnected)",
		},
	)

	informerSynced = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "k8see_informer_synced",
			Help: "Informer cache sync status (1=synced, 0=not synced)",
		},
	)

	// Histogram metrics.
	eventWriteDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "k8see_event_write_duration_seconds",
			Help:    "Duration of event write operations to Redis",
			Buckets: prometheus.DefBuckets,
		},
	)

	redisOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "k8see_redis_operation_duration_seconds",
			Help:    "Duration of Redis operations",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation"},
	)

	// Buffer metrics.
	eventBufferSize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "k8see_event_buffer_size",
			Help: "Current number of events in the buffer",
		},
	)

	eventBufferCapacity = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "k8see_event_buffer_capacity",
			Help: "Maximum capacity of the event buffer",
		},
	)

	eventBufferUtilization = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "k8see_event_buffer_utilization_ratio",
			Help: "Event buffer utilization ratio (0-1)",
		},
	)

	eventsDroppedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "k8see_events_dropped_total",
			Help: "Total number of events dropped due to full buffer",
		},
	)

	eventsEnqueuedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "k8see_events_enqueued_total",
			Help: "Total number of events enqueued to buffer",
		},
	)

	// Circuit breaker metrics.
	circuitBreakerState = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "k8see_circuit_breaker_state",
			Help: "Circuit breaker state (0=closed, 1=half-open, 2=open)",
		},
	)

	circuitBreakerTrips = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "k8see_circuit_breaker_trips_total",
			Help: "Total number of circuit breaker trips",
		},
	)

	// Retry metrics.
	redisRetryAttemptsHistogram = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "k8see_redis_retry_attempts",
			Help:    "Number of retry attempts per operation",
			Buckets: prometheus.LinearBuckets(0, 1, maxRetryAttemptsHistogram),
		},
	)

	redisBackoffDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "k8see_redis_backoff_duration_seconds",
			Help:    "Duration of backoff delays",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60}, // seconds
		},
	)

	// Worker pool metrics.
	activeWorkers = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "k8see_active_workers",
			Help: "Number of active event processing workers",
		},
	)
)
