package metrics

import "github.com/prometheus/client_golang/prometheus"

func Register() {
	prometheus.MustRegister(
		CarsCacheHits,
		CarsCacheMisses,
		RedisCacheReads,
		RedisCacheWrites,
		RedisCacheReadDuration,
		RedisCacheWriteDuration,
	)
}
