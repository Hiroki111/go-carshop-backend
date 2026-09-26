package service

import (
	"context"

	"github.com/Hiroki111/go-carshop-backend/order-service/internal/domain"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/repository"
	"gorm.io/gorm"
)

type GetOrderParameters struct {
	OrderBy string
	SortIn  string
	CarIDs  []uint
	Page    int
	Limit   int
}

func (s *Service) CreateOrder(ctx context.Context, userId uint, carId uint) error {
	var order domain.Order

	// TODO: Update this block.
	// car-service is now an independent service, so order-service can't get car-service's DB.
	err := s.repo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// car, err := s.repo.GetCarForUpdate(tx, carId)
		// if err != nil {
		// 	return err
		// }

		// if err := s.repo.UpdateCarAvailability(tx, carId, false); err != nil {
		// 	return err
		// }

		order = domain.Order{
			UserID: userId,
			CarID:  carId,
			// TODO: PriceCents must be retreived from a car that can be found by carId. Fix this.
			PriceCents: 100,
		}
		if err := s.repo.CreateOrderWithTx(tx, order); err != nil {
			return err
		}
		return nil
	})
	return err
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
