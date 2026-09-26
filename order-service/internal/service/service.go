package service

import (
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/repository"
)

type Service struct {
	repo *repository.Repository
}

func NewService(
	repo *repository.Repository,
) *Service {
	return &Service{
		repo: repo,
	}
}
