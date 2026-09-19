package cache

import (
	"time"
)

type NoopProductsCacheWarmer struct {
}

func NewNoopProductsCacheWarmer() *NoopProductsCacheWarmer {
	return &NoopProductsCacheWarmer{}
}

func (w *NoopProductsCacheWarmer) WarmProduct(id uint, ttl time.Duration) {}

func (w *NoopProductsCacheWarmer) WarmProductList(ttl time.Duration) {}
