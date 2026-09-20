package cache

import "github.com/Hiroki111/go-carshop-backend/car-service/internal/domain"

type CarsPage struct {
	Cars []domain.Car
	Total    int64
}
