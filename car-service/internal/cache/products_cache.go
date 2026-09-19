package cache

import (
	"context"
	"time"

	"github.com/Hiroki111/go-carshop-backend/car-service/internal/domain"
)

type ProductsCache interface {
	GetProduct(ctx context.Context, key string) (*domain.Product, bool, error)
	GetPage(ctx context.Context, key string) (*ProductsPage, bool, error)

	SetProduct(ctx context.Context, key string, product *domain.Product, ttl time.Duration) error
	SetPage(ctx context.Context, key string, page *ProductsPage, ttl time.Duration) error

	InvalidateProducts(ctx context.Context) error
	InvalidateProduct(ctx context.Context, cacheKey string) error
}
