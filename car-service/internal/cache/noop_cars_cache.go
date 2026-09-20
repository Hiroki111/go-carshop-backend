package cache

import (
	"context"
	"time"

	"github.com/Hiroki111/go-carshop-backend/car-service/internal/domain"
)

type NoopCarsCache struct{}

func NewNoopCarsCache() *NoopCarsCache {
	return &NoopCarsCache{}
}

func (c *NoopCarsCache) GetCar(
	ctx context.Context,
	key string,
) (*domain.Car, bool, error) {
	return nil, false, nil
}

func (c *NoopCarsCache) GetPage(
	ctx context.Context,
	key string,
) (*CarsPage, bool, error) {
	return nil, false, nil
}

func (c *NoopCarsCache) SetCar(
	ctx context.Context,
	key string,
	car *domain.Car,
	ttl time.Duration,
) error {
	return nil
}

func (c *NoopCarsCache) SetPage(
	ctx context.Context,
	key string,
	page *CarsPage,
	ttl time.Duration,
) error {
	return nil
}

func (c *NoopCarsCache) InvalidateCars(
	ctx context.Context,
) error {
	return nil
}

func (c *NoopCarsCache) InvalidateCar(
	ctx context.Context,
	cacheKey string,
) error {
	return nil
}
