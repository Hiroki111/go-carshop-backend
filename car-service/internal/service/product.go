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
	productListTTLWithoutQuery = 60 * time.Minute
	productListTTLWithQuery    = 60 * time.Minute
	individualProductTTL       = 30 * time.Minute
)

type GetProductsParameters struct {
	OrderBy  string
	SortIn   string
	Name     string
	MinPrice int64
	MaxPrice int64
	Page     int
	Limit    int
}

func (s *Service) GetProductsWithTotalCount(ctx context.Context, params GetProductsParameters) ([]domain.Product, uint, error) {
	var offset int
	if params.Page > 0 {
		offset = (params.Page - 1) * params.Limit
	}
	inputs := repository.GetProductsInput{
		OrderBy:  params.OrderBy,
		SortIn:   params.SortIn,
		Name:     params.Name,
		MinPrice: params.MinPrice,
		MaxPrice: params.MaxPrice,
		Limit:    params.Limit,
		Offset:   offset,
	}

	ttl := getTTLForQuery(inputs)

	cacheKey := cache.ProductListCacheKey(inputs)
	productPage, found, err := s.productsCache.GetPage(ctx, cacheKey)
	if err != nil {
		log.Printf("cache read failed: %v", err)
	} else if found {
		return productPage.Products, uint(productPage.Total), nil
	}

	products, total, err := s.repo.GetProductsWithTotalCount(inputs)
	if err != nil {
		return nil, 0, err
	}

	newProductPage := cache.ProductsPage{
		Products: products,
		Total:    total,
	}
	if err := s.productsCache.SetPage(ctx, cacheKey, &newProductPage, ttl); err != nil {
		log.Printf("failed to set products cache: %v", err)
	}

	return products, uint(total), nil
}

func (s *Service) GetProductById(ctx context.Context, id uint) (domain.Product, error) {
	cacheKey := cache.ProductCacheKey(id)
	cachedProduct, found, err := s.productsCache.GetProduct(ctx, cacheKey)
	if err != nil {
		log.Printf("cache read failed: %v", err)
	} else if found {
		return *cachedProduct, nil
	}

	product, err := s.repo.GetProductById(id)
	if err != nil {
		return domain.Product{}, err
	}

	if err := s.productsCache.SetProduct(ctx, cacheKey, &product, individualProductTTL); err != nil {
		log.Printf("failed to set product cache: %v", err)
	}

	return product, nil
}

func (s *Service) CreateProduct(input domain.Product) (domain.Product, error) {
	product, err := s.repo.CreateProduct(domain.Product{
		Name:       input.Name,
		PriceCents: input.PriceCents,
	})
	if err != nil {
		return domain.Product{}, err
	}

	go s.productsCacheWarmer.WarmProductList(productListTTLWithoutQuery)
	go s.productsCacheWarmer.WarmProduct(product.ID, individualProductTTL)

	return product, nil
}

func (s *Service) UpdateProduct(ctx context.Context, input repository.UpdateProductsInput) (domain.Product, error) {
	product, err := s.repo.UpdateProduct(input)
	if err != nil {
		return domain.Product{}, err
	}

	if err := s.productsCache.InvalidateProducts(ctx); err != nil {
		log.Printf("cache invalidation failed: %v", err)
	}
	if err := s.productsCache.SetProduct(ctx, cache.ProductCacheKey(product.ID), &product, individualProductTTL); err != nil {
		log.Printf("cache update failed: %v", err)
	}
	go s.productsCacheWarmer.WarmProductList(productListTTLWithoutQuery)

	return product, nil
}

func (s *Service) DeleteProduct(ctx context.Context, id uint) error {
	err := s.repo.DeleteProduct(id)
	if err != nil {
		return err
	}

	if err := s.productsCache.InvalidateProducts(ctx); err != nil {
		log.Printf("cache invalidation failed: %v", err)
	}

	cacheKey := cache.ProductCacheKey(id)
	if err := s.productsCache.InvalidateProduct(ctx, cacheKey); err != nil {
		log.Printf("cache invalidation failed: %v", err)
	}

	go s.productsCacheWarmer.WarmProductList(productListTTLWithoutQuery)

	return nil
}

func getTTLForQuery(inputs repository.GetProductsInput) time.Duration {
	isDefault := inputs == repository.GetDefaultQueryForProducts()

	ttl := productListTTLWithQuery
	if isDefault {
		ttl = productListTTLWithoutQuery
	}

	return addJitter(ttl)
}

func addJitter(ttl time.Duration) time.Duration {
	jitter := time.Duration(rand.Int63n(int64(5 * time.Minute)))
	return ttl + jitter
}
