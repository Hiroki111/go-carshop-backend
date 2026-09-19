package service

import (
	"github.com/Hiroki111/go-carshop-backend/car-service/internal/cache"
	"github.com/Hiroki111/go-carshop-backend/car-service/internal/repository"
)

type Service struct {
	repo                *repository.Repository
	productsCache       cache.ProductsCache
	productsCacheWarmer cache.ProductsCacheWarmer
}

func NewService(
	repo *repository.Repository,
	productsCache cache.ProductsCache,
	productsCacheWarmer cache.ProductsCacheWarmer,
) *Service {
	return &Service{
		repo:                repo,
		productsCache:       productsCache,
		productsCacheWarmer: productsCacheWarmer,
	}
}
