package service

import (
	"github.com/Hiroki111/go-carshop-backend/car-service/internal/cache"
	"github.com/Hiroki111/go-carshop-backend/car-service/internal/repository"
)

type Service struct {
	repo                *repository.Repository
	carsCache       cache.CarsCache
	carsCacheWarmer cache.CarsCacheWarmer
}

func NewService(
	repo *repository.Repository,
	carsCache cache.CarsCache,
	carsCacheWarmer cache.CarsCacheWarmer,
) *Service {
	return &Service{
		repo:                repo,
		carsCache:       carsCache,
		carsCacheWarmer: carsCacheWarmer,
	}
}
