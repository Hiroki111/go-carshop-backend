package repository

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/Hiroki111/go-carshop-backend/car-service/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GetCarsInput struct {
	OrderBy                string
	SortIn                 string
	Name                   string
	MinPrice               int64
	MaxPrice               int64
	IncludeUnavailableCars bool
	Offset                 int
	Limit                  int
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

	if !inputs.IncludeUnavailableCars {
		query = query.Where("status = ?", string(domain.CarStatusAvailable))
	}

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
	car := domain.Car{Name: data.Name, PriceCents: data.PriceCents, Status: domain.CarStatusAvailable}
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

	if car.Status != domain.CarStatusAvailable {
		return domain.Car{}, ErrItemNotAvailable
	}

	return car, nil
}

// ChangeCarStatus moves a car to a new status. allowed decides which
// from -> to moves the caller is permitted to make (e.g.
// domain.CarStatus.CanBeChangedByDealerTo); a move it rejects fails with
// ErrInvalidStatusTransition.
func (r *Repository) ChangeCarStatus(id uint, next domain.CarStatus, allowed func(from, to domain.CarStatus) bool) (domain.Car, error) {
	var car domain.Car
	if err := r.db.First(&car, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Car{}, ErrItemNotFound
		}
		return domain.Car{}, err
	}

	if !allowed(car.Status, next) {
		return domain.Car{}, fmt.Errorf("%w: %s -> %s", ErrInvalidStatusTransition, car.Status, next)
	}

	result := r.db.
		Model(&car).
		Where("id = ? AND version = ?", car.ID, car.Version).
		Updates(map[string]interface{}{
			"status":  string(next),
			"version": car.Version + 1,
		})
	if err := result.Error; err != nil {
		return domain.Car{}, err
	}
	if result.RowsAffected == 0 {
		return domain.Car{}, ErrOptimisticLockFailed
	}

	car.Status = next
	car.Version++
	return car, nil
}

func (r *Repository) ReserveCar(id, orderID uint) (domain.Car, error) {
	result := r.db.
		Model(&domain.Car{}).
		Where("id = ? AND status = ?", id, string(domain.CarStatusAvailable)).
		Updates(map[string]interface{}{
			"status":               string(domain.CarStatusReserved),
			"reserved_by_order_id": orderID,
			"version":              gorm.Expr("version + 1"),
		})
	if err := result.Error; err != nil {
		return domain.Car{}, err
	}

	car, err := r.GetCarById(id)
	if err != nil {
		return domain.Car{}, err
	}

	if result.RowsAffected == 1 {
		return car, nil
	}

	if car.Status == domain.CarStatusReserved && car.ReservedByOrderID != nil && *car.ReservedByOrderID == orderID {
		return car, nil
	}

	return domain.Car{}, fmt.Errorf("%w: car is %s", ErrItemNotAvailable, car.Status)
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
