package cache

import (
	"context"
	"time"

	"github.com/Hiroki111/go-carshop-backend/car-service/internal/domain"
)

type NoopProductsCache struct{}

func NewNoopProductsCache() *NoopProductsCache {
	return &NoopProductsCache{}
}

func (c *NoopProductsCache) GetProduct(
	ctx context.Context,
	key string,
) (*domain.Product, bool, error) {
	return nil, false, nil
}

func (c *NoopProductsCache) GetPage(
	ctx context.Context,
	key string,
) (*ProductsPage, bool, error) {
	return nil, false, nil
}

func (c *NoopProductsCache) SetProduct(
	ctx context.Context,
	key string,
	product *domain.Product,
	ttl time.Duration,
) error {
	return nil
}

func (c *NoopProductsCache) SetPage(
	ctx context.Context,
	key string,
	page *ProductsPage,
	ttl time.Duration,
) error {
	return nil
}

func (c *NoopProductsCache) InvalidateProducts(
	ctx context.Context,
) error {
	return nil
}

func (c *NoopProductsCache) InvalidateProduct(
	ctx context.Context,
	cacheKey string,
) error {
	return nil
}
