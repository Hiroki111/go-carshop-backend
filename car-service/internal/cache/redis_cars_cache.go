package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Hiroki111/go-carshop-backend/car-service/internal/domain"
	"github.com/Hiroki111/go-carshop-backend/car-service/internal/metrics"
	"github.com/redis/go-redis/v9"
)

type RedisCarsCache struct {
	client *redis.Client
}

func NewRedisCarsCache(
	client *redis.Client,
) *RedisCarsCache {
	return &RedisCarsCache{
		client: client,
	}
}

func (c *RedisCarsCache) GetCar(
	ctx context.Context,
	key string,
) (*domain.Car, bool, error) {
	val, err := c.client.Get(ctx, key).Result()

	if err == redis.Nil {
		return nil, false, nil
	}

	if err != nil {
		return nil, false, err
	}

	var car domain.Car
	if err := json.Unmarshal([]byte(val), &car); err != nil {
		return nil, false, err
	}

	return &car, true, nil
}

func (c *RedisCarsCache) GetPage(
	ctx context.Context,
	key string,
) (*CarsPage, bool, error) {
	start := time.Now()
	metrics.RedisCacheReads.Inc()
	val, err := c.client.Get(ctx, key).Result()
	metrics.RedisCacheReadDuration.Observe(time.Since(start).Seconds())

	if err == redis.Nil {
		metrics.CarsCacheMisses.Inc()
		return nil, false, nil
	}

	if err != nil {
		return nil, false, err
	}

	metrics.CarsCacheHits.Inc()

	var cars CarsPage
	if err := json.Unmarshal([]byte(val), &cars); err != nil {
		return nil, false, err
	}

	return &cars, true, nil
}

func (c *RedisCarsCache) SetCar(
	ctx context.Context,
	key string,
	car *domain.Car,
	ttl time.Duration,
) error {
	data, err := json.Marshal(car)
	if err != nil {
		return err
	}

	err = c.client.Set(ctx, key, data, ttl).Err()
	return err
}

func (c *RedisCarsCache) SetPage(
	ctx context.Context,
	key string,
	page *CarsPage,
	ttl time.Duration,
) error {
	data, err := json.Marshal(page)
	if err != nil {
		return err
	}

	start := time.Now()
	metrics.RedisCacheWrites.Inc()

	err = c.client.Set(ctx, key, data, ttl).Err()
	metrics.RedisCacheWriteDuration.Observe(time.Since(start).Seconds())

	return err
}

func (c *RedisCarsCache) InvalidateCars(ctx context.Context) error {
	iter := c.client.Scan(ctx, 0, "cars:*", 0).Iterator()
	for iter.Next(ctx) {
		if err := c.client.Del(ctx, iter.Val()).Err(); err != nil {
			return err
		}
	}
	return iter.Err()
}

func (c *RedisCarsCache) InvalidateCar(ctx context.Context, cacheKey string) error {
	return c.client.Del(ctx, cacheKey).Err()
}
