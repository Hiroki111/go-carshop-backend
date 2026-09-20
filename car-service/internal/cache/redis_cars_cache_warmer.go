package cache

import (
	"context"
	"log"
	"time"

	"github.com/Hiroki111/go-carshop-backend/car-service/internal/repository"
)

type RedisCarsCacheWarmer struct {
	repo    repository.Repository
	cache   CarsCache
	workers chan struct{}
}

func NewRedisCarsCacheWarmer(repo repository.Repository, cache CarsCache) *RedisCarsCacheWarmer {
	return &RedisCarsCacheWarmer{
		repo:    repo,
		cache:   cache,
		workers: make(chan struct{}, 5),
	}
}

func (w *RedisCarsCacheWarmer) WarmCar(id uint, ttl time.Duration) {
	select {
	case w.workers <- struct{}{}:
		defer func() { <-w.workers }()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		car, err := w.repo.GetCarById(id)
		if err != nil {
			log.Printf("failed to get car for cache: %v", err)
			return
		}

		cacheKey := CarCacheKey(id)
		err = w.cache.SetCar(ctx, cacheKey, &car, ttl)
		if err != nil {
			log.Printf("failed to warm car cache: %v", err)
			return
		}
	default:
	}
}

func (w *RedisCarsCacheWarmer) WarmCarList(ttl time.Duration) {
	select {
	case w.workers <- struct{}{}:
		defer func() { <-w.workers }()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		inputs := repository.GetDefaultQueryForCars()
		cars, total, err := w.repo.GetCarsWithTotalCount(inputs)
		if err != nil {
			log.Printf("failed to get cars and total count for cache: %v", err)
			return
		}

		cacheKey := CarListCacheKey(inputs)
		page := CarsPage{Cars: cars, Total: total}
		err = w.cache.SetPage(ctx, cacheKey, &page, ttl)
		if err != nil {
			log.Printf("failed to warm car page cache: %v", err)
			return
		}
	default:
	}
}
