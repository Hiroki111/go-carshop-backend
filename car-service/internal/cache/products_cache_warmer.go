package cache

import (
	"time"
)

type ProductsCacheWarmer interface {
	WarmProduct(id uint, ttl time.Duration)
	WarmProductList(ttl time.Duration)
}
