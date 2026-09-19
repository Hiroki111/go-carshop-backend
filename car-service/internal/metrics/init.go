package metrics

import "github.com/prometheus/client_golang/prometheus"

func Register() {
	prometheus.MustRegister(
		ProductsCacheHits,
		ProductsCacheMisses,
		RedisCacheReads,
		RedisCacheWrites,
		RedisCacheReadDuration,
		RedisCacheWriteDuration,
	)
}
