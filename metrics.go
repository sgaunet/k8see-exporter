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
)
