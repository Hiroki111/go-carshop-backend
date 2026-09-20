package cache

import (
	"context"
	"time"

	"github.com/Hiroki111/go-carshop-backend/car-service/internal/domain"
)

type CarsCache interface {
	GetCar(ctx context.Context, key string) (*domain.Car, bool, error)
	GetPage(ctx context.Context, key string) (*CarsPage, bool, error)

	SetCar(ctx context.Context, key string, car *domain.Car, ttl time.Duration) error
	SetPage(ctx context.Context, key string, page *CarsPage, ttl time.Duration) error

	InvalidateCars(ctx context.Context) error
	InvalidateCar(ctx context.Context, cacheKey string) error
}
