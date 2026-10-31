package repository

import (
	"github.com/Hiroki111/go-carshop-backend/user-service/internal/domain"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Migrate() error {
	return r.db.AutoMigrate(&domain.User{})
}

// EnsureAdminExists creates an admin account unless a user with this user name
// already exists, in which case it changes nothing: a different email or
// password in the configuration is ignored. This lets the very first admin log
// in without a public "create an admin" endpoint (which would let anyone grant
// themselves admin). The values are expected to come from startup configuration
// (environment variables) rather than being hardcoded. The email must already
// be normalized; Service.EnsureAdminExists shoud do that.
func (r *Repository) EnsureAdminExists(userName string, email string, hashedPassword string) error {
	admin := domain.User{
		UserName: userName,
		Email:    email,
		Password: hashedPassword,
		Role:     domain.AdminRole,
	}
	return r.db.Where(domain.User{UserName: userName}).FirstOrCreate(&admin).Error
}
