package service

import (
	"errors"
	"fmt"

	"github.com/Hiroki111/go-carshop-backend/user-service/internal/auth"
	"github.com/Hiroki111/go-carshop-backend/user-service/internal/domain"
	"github.com/go-playground/validator/v10"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func (s *Service) CreateUser(input domain.User) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	input.Email = NormalizeEmail(input.Email)

	return s.repo.CreateUser(input, string(hashedPassword))
}

const emailRule = "required,email,max=254"

func (s *Service) EnsureAdminExists(userName, email, password string) error {
	email = NormalizeEmail(email)
	if err := validator.New().Var(email, emailRule); err != nil {
		return fmt.Errorf("invalid admin email %q: %w", email, err)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	return s.repo.EnsureAdminExists(userName, email, string(hashedPassword))
}

func (s *Service) Login(userName string, password string) (string, error) {
	user, err := s.repo.GetUserByUserName(userName)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrInvalidCredentials
		}
		return "", err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return "", ErrInvalidCredentials
	}

	return auth.GenerateJWTToken(s.privateKey, user.ID, user.UserName, user.Role)
}
