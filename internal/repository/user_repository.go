package repository

import (
	"fmt"
	"sync"

	"github.com/ilhaamms/ybtech/internal/domain"
)

type InMemoryUserRepository struct {
	mu         sync.RWMutex
	users      map[string]*domain.User 
	byUsername map[string]*domain.User 
	byEmail    map[string]*domain.User 
}

func NewUserRepository() *InMemoryUserRepository {
	return &InMemoryUserRepository{
		users:      make(map[string]*domain.User),
		byUsername: make(map[string]*domain.User),
		byEmail:    make(map[string]*domain.User),
	}
}

func (r *InMemoryUserRepository) Save(user *domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	
	if _, exists := r.byUsername[user.Username]; exists {
		return fmt.Errorf("username '%s' already exists", user.Username)
	}

	
	if _, exists := r.byEmail[user.Email]; exists {
		return fmt.Errorf("email '%s' already exists", user.Email)
	}

	r.users[user.ID] = user
	r.byUsername[user.Username] = user
	r.byEmail[user.Email] = user
	return nil
}

func (r *InMemoryUserRepository) GetByUsername(username string) (*domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, ok := r.byUsername[username]
	if !ok {
		return nil, fmt.Errorf("user '%s' not found", username)
	}
	return user, nil
}

func (r *InMemoryUserRepository) GetByID(id string) (*domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, ok := r.users[id]
	if !ok {
		return nil, fmt.Errorf("user with id '%s' not found", id)
	}
	return user, nil
}
