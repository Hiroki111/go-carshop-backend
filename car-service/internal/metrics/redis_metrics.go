package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	RedisCacheReads = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "redis_cache_reads_total",
			Help: "Total number of Redis cache reads",
		},
	)

	RedisCacheWrites = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "redis_cache_writes_total",
			Help: "Total number of Redis cache writes",
		},
	)

	RedisCacheReadDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "redis_cache_read_duration_seconds",
			Help:    "Redis cache read latency",
			Buckets: prometheus.DefBuckets,
		},
	)

	RedisCacheWriteDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "redis_cache_write_duration_seconds",
			Help:    "Redis cache write latency",
			Buckets: prometheus.DefBuckets,
		},
	)
)
