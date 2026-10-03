package repository

import (
	"errors"

	"github.com/Hiroki111/go-carshop-backend/user-service/internal/domain"
	"gorm.io/gorm"
)

func (r *Repository) CreateUser(data domain.User, hashedPassword string) error {
	result := r.db.Create(&domain.User{
		UserName: data.UserName,
		Password: hashedPassword,
		Role:     data.Role,
	})

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
			return ErrUserAlreadyExists
		}
		return result.Error
	}

	return nil
}

func (r *Repository) GetUserByUserName(userName string) (domain.User, error) {
	var user domain.User

	if err := r.db.Where(domain.User{UserName: userName}).First(&user).Error; err != nil {
		return domain.User{}, err
	}

	return user, nil
}
