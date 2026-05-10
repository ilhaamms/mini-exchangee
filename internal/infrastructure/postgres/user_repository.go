package postgres

import (
	"fmt"
	"strings"

	"github.com/ilhaamms/ybtech/internal/domain"
	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *DB) *UserRepository {
	return &UserRepository{db: db.GetConn()}
}

func (r *UserRepository) Save(user *domain.User) error {
	m := UserModel{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		Password:  user.Password,
		CreatedAt: user.CreatedAt,
	}
	result := r.db.Create(&m)
	if result.Error != nil {
		errStr := result.Error.Error()
		if strings.Contains(errStr, "idx_users_username") || strings.Contains(errStr, "users_username_key") {
			return fmt.Errorf("username '%s' already exists", user.Username)
		}
		if strings.Contains(errStr, "idx_users_email") || strings.Contains(errStr, "users_email_key") {
			return fmt.Errorf("email '%s' already exists", user.Email)
		}
		return result.Error
	}
	return nil
}

func (r *UserRepository) GetByUsername(username string) (*domain.User, error) {
	var m UserModel
	result := r.db.First(&m, "username = ?", username)
	if result.Error != nil {
		return nil, fmt.Errorf("user '%s' not found", username)
	}
	return userModelToDomain(m), nil
}

func (r *UserRepository) GetByID(id string) (*domain.User, error) {
	var m UserModel
	result := r.db.First(&m, "id = ?", id)
	if result.Error != nil {
		return nil, fmt.Errorf("user with id '%s' not found", id)
	}
	return userModelToDomain(m), nil
}

func userModelToDomain(m UserModel) *domain.User {
	return &domain.User{
		ID:        m.ID,
		Username:  m.Username,
		Email:     m.Email,
		Password:  m.Password,
		CreatedAt: m.CreatedAt,
	}
}
