package repository

import (
	"errors"

	"github.com/Hiroki111/go-carshop-backend/order-service/internal/domain"
	"gorm.io/gorm"
)

type GetOrdersInput struct {
	OrderBy string
	SortIn  string
	CarIDs  []uint
	Offset  int
	Limit   int
}

type UpdateOrderInput struct {
	ID         uint
	PriceCents *uint
}

func (r *Repository) GetOrdersWithTotalCount(inputs GetOrdersInput) ([]domain.Order, int64, error) {
	var result []domain.Order
	var total int64

	query := r.db.Model(&domain.Order{})

	if len(inputs.CarIDs) > 0 {
		query = query.Where("car_id IN ?", inputs.CarIDs)
	}

	sortIn := "asc"
	if inputs.SortIn == "desc" {
		sortIn = "desc"
	}

	orderBy := "created_at"
	if inputs.OrderBy == "car_id" || inputs.OrderBy == "user_id" {
		orderBy = inputs.OrderBy
	}

	query = query.Order(orderBy + " " + sortIn)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// NOTE: Don't use query.Offset before query.Count
	query = query.Limit(inputs.Limit).Offset(inputs.Offset)

	if err := query.Find(&result).Error; err != nil {
		return nil, 0, err
	}

	return result, total, nil
}

func (r *Repository) GetOrderById(id uint) (domain.Order, error) {
	var order domain.Order

	err := r.db.First(&order, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Order{}, ErrItemNotFound
		}
		return domain.Order{}, err
	}

	return order, nil
}

func (r *Repository) CreateOrderWithTx(tx *gorm.DB, order domain.Order) error {
	return r.withTx(tx).db.Create(&order).Error
}

func (r *Repository) UpdateOrder(data UpdateOrderInput) (domain.Order, error) {
	var order domain.Order
	if err := r.db.First(&order, data.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Order{}, ErrItemNotFound
		}
		return domain.Order{}, err
	}

	if data.PriceCents != nil {
		order.PriceCents = *data.PriceCents
	}

	if err := r.db.Save(&order).Error; err != nil {
		return domain.Order{}, err
	}

	return order, nil
}

func (r *Repository) DeleteOrder(id uint) error {
	result := r.db.Delete(&domain.Order{}, id)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrItemNotFound
	}

	return nil
}
