package repository

import (
	"fmt"
	"sync"

	"github.com/ilhaamms/ybtech/internal/domain"
)

// UserRepository provides thread-safe in-memory storage for users
type UserRepository struct {
	mu         sync.RWMutex
	users      map[string]*domain.User // userID -> User
	byUsername map[string]*domain.User // username -> User
	byEmail    map[string]*domain.User // email -> User
}

// NewUserRepository creates a new UserRepository
func NewUserRepository() *UserRepository {
	return &UserRepository{
		users:      make(map[string]*domain.User),
		byUsername: make(map[string]*domain.User),
		byEmail:    make(map[string]*domain.User),
	}
}

// Save persists a user to the repository
func (r *UserRepository) Save(user *domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check for duplicate username
	if _, exists := r.byUsername[user.Username]; exists {
		return fmt.Errorf("username '%s' already exists", user.Username)
	}

	// Check for duplicate email
	if _, exists := r.byEmail[user.Email]; exists {
		return fmt.Errorf("email '%s' already exists", user.Email)
	}

	r.users[user.ID] = user
	r.byUsername[user.Username] = user
	r.byEmail[user.Email] = user
	return nil
}

// GetByUsername retrieves a user by username
func (r *UserRepository) GetByUsername(username string) (*domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, ok := r.byUsername[username]
	if !ok {
		return nil, fmt.Errorf("user '%s' not found", username)
	}
	return user, nil
}

// GetByID retrieves a user by their ID
func (r *UserRepository) GetByID(id string) (*domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, ok := r.users[id]
	if !ok {
		return nil, fmt.Errorf("user with id '%s' not found", id)
	}
	return user, nil
}
