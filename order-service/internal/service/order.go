package service

import (
	"context"
	"errors"

	"github.com/Hiroki111/go-carshop-backend/order-service/internal/carclient"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/domain"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/repository"
)

type GetOrderParameters struct {
	OrderBy string
	SortIn  string
	CarIDs  []uint
	Page    int
	Limit   int
}

func (s *Service) CreateOrder(ctx context.Context, userId uint, userName string, carId uint) error {
	car, err := s.carClient.GetCarByID(ctx, carId)
	if err != nil {
		if errors.Is(err, carclient.ErrCarNotFound) {
			return repository.ErrItemNotFound
		}
		return err
	}

	if !car.IsAvailable {
		return repository.ErrItemNotAvailable
	}

	order := domain.Order{
		UserID:     userId,
		UserName:   userName,
		CarID:      carId,
		CarName:    car.Name,
		PriceCents: car.PriceCents,
	}

	// No explicit transaction needed here: order-service only writes to its
	// own single row now that car availability lives in car-service's own
	// database. Making the "check availability, then place the order"
	// sequence safe against two customers racing for the same car is a
	// separate concern (see the concurrency-safe reservation item in the
	// project README) - not solved by a local DB transaction anymore.
	return s.repo.CreateOrderWithTx(nil, order)
}

func (s *Service) GetOrdersWithTotalCount(ctx context.Context, params GetOrderParameters) ([]domain.Order, uint, error) {
	var offset int
	if params.Page > 0 {
		offset = (params.Page - 1) * params.Limit
	}
	inputs := repository.GetOrdersInput{
		OrderBy: params.OrderBy,
		SortIn:  params.SortIn,
		CarIDs:  params.CarIDs,
		Limit:   params.Limit,
		Offset:  offset,
	}
	orders, total, err := s.repo.GetOrdersWithTotalCount(inputs)
	if err != nil {
		return make([]domain.Order, 0), 0, err
	}

	return orders, uint(total), err
}

func (s *Service) GetOrderById(id uint) (domain.Order, error) {
	return s.repo.GetOrderById(id)
}

func (s *Service) UpdateOrder(input repository.UpdateOrderInput) (domain.Order, error) {
	return s.repo.UpdateOrder(input)
}

func (s *Service) DeleteOrder(id uint) error {
	return s.repo.DeleteOrder(id)
}
