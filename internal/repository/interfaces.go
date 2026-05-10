package repository

import "github.com/ilhaamms/ybtech/internal/domain"

// OrderRepository is the common interface for order storage (in-memory or postgres).
type OrderRepository interface {
	Save(order *domain.Order) error
	GetByID(id string) (*domain.Order, error)
	GetAll(stockCode, status string) ([]domain.Order, error)
	GetOpenOrdersByStock(stockCode string, side domain.Side) ([]*domain.Order, error)
	UpdateStatus(order *domain.Order) error
}

// TradeRepository is the common interface for trade storage (in-memory or postgres).
type TradeRepository interface {
	Save(trade *domain.Trade) error
	GetAll() ([]domain.Trade, error)
	GetByStock(stockCode string) ([]domain.Trade, error)
	GetRecentByStock(stockCode string, limit int) ([]domain.Trade, error)
}

// UserRepository is the common interface for user storage (in-memory or postgres).
type UserRepository interface {
	Save(user *domain.User) error
	GetByUsername(username string) (*domain.User, error)
	GetByID(id string) (*domain.User, error)
}
