package repository

import (
	"errors"
	"math"
	"strings"

	"github.com/Hiroki111/go-carshop-backend/car-service/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GetCarsInput struct {
	OrderBy  string
	SortIn   string
	Name     string
	MinPrice int64
	MaxPrice int64
	Offset   int
	Limit    int
}

type UpdateCarsInput struct {
	ID         uint
	Name       *string
	PriceCents *uint
}

type UpdateOrderInput struct {
	ID         uint
	PriceCents *uint
}

func (r *Repository) GetCarsWithTotalCount(inputs GetCarsInput) ([]domain.Car, int64, error) {
	var result []domain.Car
	var total int64

	query := r.db.Model(&domain.Car{})
	if inputs.Name != "" {
		query = query.Where("LOWER(name) LIKE ?", "%"+strings.ToLower(inputs.Name)+"%")
	}

	query = query.
		Where("price_cents >= ?", inputs.MinPrice).
		Where("price_cents <= ?", inputs.MaxPrice)

	sortIn := "asc"
	if inputs.SortIn == "desc" {
		sortIn = "desc"
	}

	orderBy := "created_at"
	if inputs.OrderBy == "name" || inputs.OrderBy == "price_cents" {
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

func (r *Repository) GetCarById(id uint) (domain.Car, error) {
	var car domain.Car

	err := r.db.First(&car, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Car{}, ErrItemNotFound
		}
		return domain.Car{}, err
	}

	return car, nil
}

func (r *Repository) CreateCar(data domain.Car) (domain.Car, error) {
	car := domain.Car{Name: data.Name, PriceCents: data.PriceCents}
	result := r.db.Create(&car)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
			return domain.Car{}, ErrCarAlreadyExists
		}
		return domain.Car{}, result.Error
	}

	return car, nil
}

func (r *Repository) UpdateCar(data UpdateCarsInput) (domain.Car, error) {
	var car domain.Car
	if err := r.db.First(&car, data.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Car{}, ErrItemNotFound
		}
		return domain.Car{}, err
	}

	updates := map[string]interface{}{
		"version": car.Version + 1,
	}

	if data.Name != nil {
		updates["name"] = *data.Name
	}
	if data.PriceCents != nil {
		updates["price_cents"] = *data.PriceCents
	}

	result := r.db.
		Model(&car).
		Where("id = ? AND version = ?", car.ID, car.Version).
		Updates(updates)

	if err := result.Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return domain.Car{}, ErrCarAlreadyExists
		}
		return domain.Car{}, err
	}

	if result.RowsAffected == 0 {
		return domain.Car{}, ErrOptimisticLockFailed
	}

	return car, nil
}

func (r *Repository) DeleteCar(id uint) error {
	result := r.db.Delete(&domain.Car{}, id)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrItemNotFound
	}

	return nil
}

// NOTE: Currently unused
func (r *Repository) GetCarForUpdate(tx *gorm.DB, id uint) (domain.Car, error) {
	var car domain.Car

	err := r.withTx(tx).db.Clauses(clause.Locking{Strength: "NO KEY UPDATE"}).First(&car, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Car{}, ErrItemNotFound
		}
		return domain.Car{}, err
	}

	if !car.IsAvailable {
		return domain.Car{}, ErrItemNotAvailable
	}

	return car, nil
}

// NOTE: Currently unused
func (r *Repository) UpdateCarAvailability(tx *gorm.DB, id uint, available bool) error {
	return r.withTx(tx).db.Model(&domain.Car{}).Where("id = ?", id).Update("is_available", available).Error
}

func GetDefaultQueryForCars() GetCarsInput {
	return GetCarsInput{
		OrderBy:  "",
		SortIn:   "",
		Name:     "",
		MinPrice: 0,
		MaxPrice: math.MaxInt64,
		Offset:   0,
		Limit:    20,
	}
}
