package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	ProductsCacheHits = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "app_cache_products_hits_total",
			Help: "Total number of product cache hits",
		},
	)

	ProductsCacheMisses = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "app_cache_products_misses_total",
			Help: "Total number of product cache misses",
		},
	)
)
