package service

import (
	"crypto/rsa"

	"github.com/Hiroki111/go-carshop-backend/user-service/internal/repository"
)

type Service struct {
	repo       *repository.Repository
	privateKey *rsa.PrivateKey
}

func NewService(repo *repository.Repository, privateKey *rsa.PrivateKey) *Service {
	return &Service{
		repo:       repo,
		privateKey: privateKey,
	}
}
