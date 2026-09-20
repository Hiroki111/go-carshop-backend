package cache

import (
	"time"
)

type CarsCacheWarmer interface {
	WarmCar(id uint, ttl time.Duration)
	WarmCarList(ttl time.Duration)
}
