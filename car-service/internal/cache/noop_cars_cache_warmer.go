package cache

import (
	"time"
)

type NoopCarsCacheWarmer struct {
}

func NewNoopCarsCacheWarmer() *NoopCarsCacheWarmer {
	return &NoopCarsCacheWarmer{}
}

func (w *NoopCarsCacheWarmer) WarmCar(id uint, ttl time.Duration) {}

func (w *NoopCarsCacheWarmer) WarmCarList(ttl time.Duration) {}
