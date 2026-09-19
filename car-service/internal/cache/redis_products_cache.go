package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Hiroki111/go-carshop-backend/car-service/internal/domain"
	"github.com/Hiroki111/go-carshop-backend/car-service/internal/metrics"
	"github.com/redis/go-redis/v9"
)

type RedisProductsCache struct {
	client *redis.Client
}

func NewRedisProductsCache(
	client *redis.Client,
) *RedisProductsCache {
	return &RedisProductsCache{
		client: client,
	}
}

func (c *RedisProductsCache) GetProduct(
	ctx context.Context,
	key string,
) (*domain.Product, bool, error) {
	val, err := c.client.Get(ctx, key).Result()

	if err == redis.Nil {
		return nil, false, nil
	}

	if err != nil {
		return nil, false, err
	}

	var product domain.Product
	if err := json.Unmarshal([]byte(val), &product); err != nil {
		return nil, false, err
	}

	return &product, true, nil
}

func (c *RedisProductsCache) GetPage(
	ctx context.Context,
	key string,
) (*ProductsPage, bool, error) {
	start := time.Now()
	metrics.RedisCacheReads.Inc()
	val, err := c.client.Get(ctx, key).Result()
	metrics.RedisCacheReadDuration.Observe(time.Since(start).Seconds())

	if err == redis.Nil {
		metrics.ProductsCacheMisses.Inc()
		return nil, false, nil
	}

	if err != nil {
		return nil, false, err
	}

	metrics.ProductsCacheHits.Inc()

	var products ProductsPage
	if err := json.Unmarshal([]byte(val), &products); err != nil {
		return nil, false, err
	}

	return &products, true, nil
}

func (c *RedisProductsCache) SetProduct(
	ctx context.Context,
	key string,
	product *domain.Product,
	ttl time.Duration,
) error {
	data, err := json.Marshal(product)
	if err != nil {
		return err
	}

	err = c.client.Set(ctx, key, data, ttl).Err()
	return err
}

func (c *RedisProductsCache) SetPage(
	ctx context.Context,
	key string,
	page *ProductsPage,
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

func (c *RedisProductsCache) InvalidateProducts(ctx context.Context) error {
	iter := c.client.Scan(ctx, 0, "products:*", 0).Iterator()
	for iter.Next(ctx) {
		if err := c.client.Del(ctx, iter.Val()).Err(); err != nil {
			return err
		}
	}
	return iter.Err()
}

func (c *RedisProductsCache) InvalidateProduct(ctx context.Context, cacheKey string) error {
	return c.client.Del(ctx, cacheKey).Err()
}
