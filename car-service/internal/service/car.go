package service

import (
	"context"
	"log"
	"math/rand"
	"time"

	"github.com/Hiroki111/go-carshop-backend/car-service/internal/cache"
	"github.com/Hiroki111/go-carshop-backend/car-service/internal/domain"
	"github.com/Hiroki111/go-carshop-backend/car-service/internal/repository"
)

const (
	carListTTLWithoutQuery = 60 * time.Minute
	carListTTLWithQuery    = 60 * time.Minute
	individualCarTTL       = 30 * time.Minute
)

type GetCarsParameters struct {
	OrderBy  string
	SortIn   string
	Name     string
	MinPrice int64
	MaxPrice int64
	Page     int
	Limit    int
}

func (s *Service) GetCarsWithTotalCount(ctx context.Context, params GetCarsParameters) ([]domain.Car, uint, error) {
	var offset int
	if params.Page > 0 {
		offset = (params.Page - 1) * params.Limit
	}
	inputs := repository.GetCarsInput{
		OrderBy:  params.OrderBy,
		SortIn:   params.SortIn,
		Name:     params.Name,
		MinPrice: params.MinPrice,
		MaxPrice: params.MaxPrice,
		Limit:    params.Limit,
		Offset:   offset,
	}

	ttl := getTTLForQuery(inputs)

	cacheKey := cache.CarListCacheKey(inputs)
	carPage, found, err := s.carsCache.GetPage(ctx, cacheKey)
	if err != nil {
		log.Printf("cache read failed: %v", err)
	} else if found {
		return carPage.Cars, uint(carPage.Total), nil
	}

	cars, total, err := s.repo.GetCarsWithTotalCount(inputs)
	if err != nil {
		return nil, 0, err
	}

	newCarPage := cache.CarsPage{
		Cars: cars,
		Total:    total,
	}
	if err := s.carsCache.SetPage(ctx, cacheKey, &newCarPage, ttl); err != nil {
		log.Printf("failed to set cars cache: %v", err)
	}

	return cars, uint(total), nil
}

func (s *Service) GetCarById(ctx context.Context, id uint) (domain.Car, error) {
	cacheKey := cache.CarCacheKey(id)
	cachedCar, found, err := s.carsCache.GetCar(ctx, cacheKey)
	if err != nil {
		log.Printf("cache read failed: %v", err)
	} else if found {
		return *cachedCar, nil
	}

	car, err := s.repo.GetCarById(id)
	if err != nil {
		return domain.Car{}, err
	}

	if err := s.carsCache.SetCar(ctx, cacheKey, &car, individualCarTTL); err != nil {
		log.Printf("failed to set car cache: %v", err)
	}

	return car, nil
}

func (s *Service) CreateCar(input domain.Car) (domain.Car, error) {
	car, err := s.repo.CreateCar(domain.Car{
		Name:       input.Name,
		PriceCents: input.PriceCents,
	})
	if err != nil {
		return domain.Car{}, err
	}

	go s.carsCacheWarmer.WarmCarList(carListTTLWithoutQuery)
	go s.carsCacheWarmer.WarmCar(car.ID, individualCarTTL)

	return car, nil
}

func (s *Service) UpdateCar(ctx context.Context, input repository.UpdateCarsInput) (domain.Car, error) {
	car, err := s.repo.UpdateCar(input)
	if err != nil {
		return domain.Car{}, err
	}

	if err := s.carsCache.InvalidateCars(ctx); err != nil {
		log.Printf("cache invalidation failed: %v", err)
	}
	if err := s.carsCache.SetCar(ctx, cache.CarCacheKey(car.ID), &car, individualCarTTL); err != nil {
		log.Printf("cache update failed: %v", err)
	}
	go s.carsCacheWarmer.WarmCarList(carListTTLWithoutQuery)

	return car, nil
}

func (s *Service) DeleteCar(ctx context.Context, id uint) error {
	err := s.repo.DeleteCar(id)
	if err != nil {
		return err
	}

	if err := s.carsCache.InvalidateCars(ctx); err != nil {
		log.Printf("cache invalidation failed: %v", err)
	}

	cacheKey := cache.CarCacheKey(id)
	if err := s.carsCache.InvalidateCar(ctx, cacheKey); err != nil {
		log.Printf("cache invalidation failed: %v", err)
	}

	go s.carsCacheWarmer.WarmCarList(carListTTLWithoutQuery)

	return nil
}

func getTTLForQuery(inputs repository.GetCarsInput) time.Duration {
	isDefault := inputs == repository.GetDefaultQueryForCars()

	ttl := carListTTLWithQuery
	if isDefault {
		ttl = carListTTLWithoutQuery
	}

	return addJitter(ttl)
}

func addJitter(ttl time.Duration) time.Duration {
	jitter := time.Duration(rand.Int63n(int64(5 * time.Minute)))
	return ttl + jitter
}
