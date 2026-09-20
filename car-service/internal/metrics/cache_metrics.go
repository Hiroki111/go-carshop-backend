package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	CarsCacheHits = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "app_cache_cars_hits_total",
			Help: "Total number of car cache hits",
		},
	)

	CarsCacheMisses = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "app_cache_cars_misses_total",
			Help: "Total number of car cache misses",
		},
	)
)
