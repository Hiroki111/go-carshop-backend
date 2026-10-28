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

var (
	// ErrOrderNotCancellable means the order is in a status that can't be
	// cancelled (failed: it never held a car).
	ErrOrderNotCancellable = errors.New("order cannot be cancelled")

	// ErrCarNotReleasable means car-service refused to release the car: it is
	// sold, or held by a different order.
	ErrCarNotReleasable = errors.New("car-service refused to release the car")

	// ErrCarReleaseUnconfirmed means car-service didn't answer definitely
	// (timeout, outage, unexpected status). The order is left as it was, so
	// the cancellation can be repeated.
	ErrCarReleaseUnconfirmed = errors.New("car-service did not confirm the release")
)

type PendingOrderError struct {
	OrderID uint
	Err     error
}

func (e *PendingOrderError) Error() string {
	return fmt.Sprintf("order %d is pending: %v", e.OrderID, e.Err)
}

func (s *Service) CreateOrder(ctx context.Context, userId uint, userName string, carId uint) (domain.Order, bool, error) {
	existing, err := s.repo.GetLiveOrderForUser(carId, userId)
	if err == nil {
		order, err := s.settleOrder(ctx, existing)
		return order, false, err
	}
	if !errors.Is(err, repository.ErrItemNotFound) {
		return domain.Order{}, false, err
	}

	car, err := s.carClient.GetCarByID(ctx, carId)
	if err != nil {
		if errors.Is(err, carclient.ErrCarNotFound) {
			return domain.Order{}, false, repository.ErrItemNotFound
		}
		return domain.Order{}, false, err
	}

	if car.Status != carclient.StatusAvailable {
		return domain.Order{}, false, repository.ErrItemNotAvailable
	}

	order, err := s.repo.CreateOrder(domain.Order{
		UserID:     userId,
		UserName:   userName,
		CarID:      carId,
		CarName:    car.Name,
		PriceCents: car.PriceCents,
		Status:     domain.OrderStatusPending,
	})
	if errors.Is(err, repository.ErrItemNotAvailable) {
		existing, lookupErr := s.repo.GetLiveOrderForUser(carId, userId)
		// Another customer ordered the same car after this customer retrieved the car from car-service.
		if errors.Is(lookupErr, repository.ErrItemNotFound) {
			return domain.Order{}, false, repository.ErrItemNotAvailable
		}
		if lookupErr != nil {
			return domain.Order{}, false, lookupErr
		}
		// The car has a live order. If it is this customer's own - a
		// double-click whose twin got there first - this request is a retry of it.
		order, err := s.settleOrder(ctx, existing)
		return order, false, err
	}
	if err != nil {
		return domain.Order{}, false, err
	}

	order, err = s.settleOrder(ctx, order)
	if err != nil {
		return domain.Order{}, false, err
	}

	return order, true, nil
}

func (s *Service) settleOrder(ctx context.Context, order domain.Order) (domain.Order, error) {
	if order.Status == domain.OrderStatusConfirmed {
		return order, nil
	}

	_, reserveErr := s.carClient.ReserveCar(ctx, order.CarID, order.ID)
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

	err := s.repo.ChangeOrderStatus(order.ID, domain.OrderStatusPending, domain.OrderStatusConfirmed)
	if errors.Is(err, repository.ErrOrderStatusConflict) {
		current, getErr := s.repo.GetOrderById(order.ID)
		if getErr == nil && current.Status == domain.OrderStatusConfirmed {
			return current, nil
		}
	}
	if err != nil {
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

func (s *Service) CancelOrder(ctx context.Context, id uint) (domain.Order, error) {
	order, err := s.repo.GetOrderById(id)
	if err != nil {
		return domain.Order{}, err
	}

	switch order.Status {
	case domain.OrderStatusCancelled:
		return order, nil
	case domain.OrderStatusPending, domain.OrderStatusConfirmed:
		// cancel below
	default:
		return domain.Order{}, ErrOrderNotCancellable
	}

	err = s.carClient.ReleaseCar(ctx, order.CarID, order.ID)
	switch {
	case err == nil:
		// released
	case errors.Is(err, carclient.ErrCarNotFound):
		// The car no longer exists, so there is nothing left to release.
	case errors.Is(err, carclient.ErrCarNotAvailable):
		return domain.Order{}, ErrCarNotReleasable
	default:
		return domain.Order{}, fmt.Errorf("%w: %w", ErrCarReleaseUnconfirmed, err)
	}

	err = s.repo.ChangeOrderStatus(order.ID, order.Status, domain.OrderStatusCancelled)
	if errors.Is(err, repository.ErrOrderStatusConflict) {
		// Another admin cancelled it at the same moment: same result.
		current, getErr := s.repo.GetOrderById(order.ID)
		if getErr == nil && current.Status == domain.OrderStatusCancelled {
			return current, nil
		}
	}
	if err != nil {
		return domain.Order{}, err
	}
	order.Status = domain.OrderStatusCancelled

	return order, nil
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
