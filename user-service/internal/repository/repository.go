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

// EnsureAdminExists creates a default admin account if none exists yet with
// this username, so the very first admin can log in without a public
// "create an admin" endpoint (which would let anyone grant themselves
// admin). userName/hashedPassword are expected to come from startup
// configuration (environment variables) rather than being hardcoded.
func (r *Repository) EnsureAdminExists(userName string, hashedPassword string) error {
	admin := domain.User{
		UserName: userName,
		Password: hashedPassword,
		Role:     domain.AdminRole,
	}
	return r.db.Where(domain.User{UserName: userName}).FirstOrCreate(&admin).Error
}
