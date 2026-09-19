package repository

import (
	"errors"
	"math"
	"strings"

	"github.com/Hiroki111/go-carshop-backend/car-service/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GetProductsInput struct {
	OrderBy  string
	SortIn   string
	Name     string
	MinPrice int64
	MaxPrice int64
	Offset   int
	Limit    int
}

type UpdateProductsInput struct {
	ID         uint
	Name       *string
	PriceCents *uint
}

type UpdateOrderInput struct {
	ID         uint
	PriceCents *uint
}

func (r *Repository) GetProductsWithTotalCount(inputs GetProductsInput) ([]domain.Product, int64, error) {
	var result []domain.Product
	var total int64

	query := r.db.Model(&domain.Product{})
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

func (r *Repository) GetProductById(id uint) (domain.Product, error) {
	var product domain.Product

	err := r.db.First(&product, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Product{}, ErrItemNotFound
		}
		return domain.Product{}, err
	}

	return product, nil
}

func (r *Repository) CreateProduct(data domain.Product) (domain.Product, error) {
	product := domain.Product{Name: data.Name, PriceCents: data.PriceCents}
	result := r.db.Create(&product)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
			return domain.Product{}, ErrProductAlreadyExists
		}
		return domain.Product{}, result.Error
	}

	return product, nil
}

func (r *Repository) UpdateProduct(data UpdateProductsInput) (domain.Product, error) {
	var product domain.Product
	if err := r.db.First(&product, data.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Product{}, ErrItemNotFound
		}
		return domain.Product{}, err
	}

	updates := map[string]interface{}{
		"version": product.Version + 1,
	}

	if data.Name != nil {
		updates["name"] = *data.Name
	}
	if data.PriceCents != nil {
		updates["price_cents"] = *data.PriceCents
	}

	result := r.db.
		Model(&product).
		Where("id = ? AND version = ?", product.ID, product.Version).
		Updates(updates)

	if err := result.Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return domain.Product{}, ErrProductAlreadyExists
		}
		return domain.Product{}, err
	}

	if result.RowsAffected == 0 {
		return domain.Product{}, ErrOptimisticLockFailed
	}

	return product, nil
}

func (r *Repository) DeleteProduct(id uint) error {
	result := r.db.Delete(&domain.Product{}, id)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrItemNotFound
	}

	return nil
}

// NOTE: Currently unused
func (r *Repository) GetProductForUpdate(tx *gorm.DB, id uint) (domain.Product, error) {
	var product domain.Product

	err := r.withTx(tx).db.Clauses(clause.Locking{Strength: "NO KEY UPDATE"}).First(&product, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Product{}, ErrItemNotFound
		}
		return domain.Product{}, err
	}

	if !product.IsAvailable {
		return domain.Product{}, ErrItemNotAvailable
	}

	return product, nil
}

// NOTE: Currently unused
func (r *Repository) UpdateProductAvailability(tx *gorm.DB, id uint, available bool) error {
	return r.withTx(tx).db.Model(&domain.Product{}).Where("id = ?", id).Update("is_available", available).Error
}

func GetDefaultQueryForProducts() GetProductsInput {
	return GetProductsInput{
		OrderBy:  "",
		SortIn:   "",
		Name:     "",
		MinPrice: 0,
		MaxPrice: math.MaxInt64,
		Offset:   0,
		Limit:    20,
	}
}
