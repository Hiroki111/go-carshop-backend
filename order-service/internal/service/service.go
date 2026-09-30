package service

import (
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/carclient"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/repository"
)

type Service struct {
	repo      *repository.Repository
	carClient carclient.CarClient
}

func NewService(
	repo *repository.Repository,
	carClient carclient.CarClient,
) *Service {
	return &Service{
		repo:      repo,
		carClient: carClient,
	}
}
