package cache

import "github.com/Hiroki111/go-carshop-backend/car-service/internal/domain"

type ProductsPage struct {
	Products []domain.Product
	Total    int64
}
