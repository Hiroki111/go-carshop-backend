package service

import (
	"context"
	"errors"
	"fmt"
	"log"

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

type PendingOrderError struct {
	OrderID uint
	Err     error
}

func (e *PendingOrderError) Error() string {
	return fmt.Sprintf("order %d is pending: %v", e.OrderID, e.Err)
}

func (s *Service) CreateOrder(ctx context.Context, userId uint, userName string, carId uint) (domain.Order, error) {
	car, err := s.carClient.GetCarByID(ctx, carId)
	if err != nil {
		if errors.Is(err, carclient.ErrCarNotFound) {
			return domain.Order{}, repository.ErrItemNotFound
		}
		return domain.Order{}, err
	}

	if car.Status != carclient.StatusAvailable {
		return domain.Order{}, repository.ErrItemNotAvailable
	}

	order, err := s.repo.CreateOrder(domain.Order{
		UserID:     userId,
		UserName:   userName,
		CarID:      carId,
		CarName:    car.Name,
		PriceCents: car.PriceCents,
		Status:     domain.OrderStatusPending,
	})
	if err != nil {
		return domain.Order{}, err
	}

	_, reserveErr := s.carClient.ReserveCar(ctx, carId, order.ID)
	switch {
	case reserveErr == nil:
		// reserved: confirm below
	case errors.Is(reserveErr, carclient.ErrCarNotAvailable):
		s.markOrderFailed(order.ID)
		return domain.Order{}, repository.ErrItemNotAvailable
	case errors.Is(reserveErr, carclient.ErrCarNotFound):
		s.markOrderFailed(order.ID)
		return domain.Order{}, repository.ErrItemNotFound
	default:
		return domain.Order{}, &PendingOrderError{OrderID: order.ID, Err: reserveErr}
	}

	if err := s.repo.ChangeOrderStatus(order.ID, domain.OrderStatusPending, domain.OrderStatusConfirmed); err != nil {
		return domain.Order{}, &PendingOrderError{OrderID: order.ID, Err: err}
	}
	order.Status = domain.OrderStatusConfirmed

	return order, nil
}

func (s *Service) markOrderFailed(orderID uint) {
	err := s.repo.ChangeOrderStatus(orderID, domain.OrderStatusPending, domain.OrderStatusFailed)
	if err != nil {
		log.Printf("order %d: failed to mark the order as failed: %v", orderID, err)
	}
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
