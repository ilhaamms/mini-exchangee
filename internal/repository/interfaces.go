package repository

import "github.com/ilhaamms/ybtech/internal/domain"

type OrderRepository interface {
	Save(order *domain.Order) error
	GetByID(id string) (*domain.Order, error)
	GetAll(stockCode, status string) ([]domain.Order, error)
	GetOpenOrdersByStock(stockCode string, side domain.Side) ([]*domain.Order, error)
	UpdateStatus(order *domain.Order) error
}

type TradeRepository interface {
	Save(trade *domain.Trade) error
	GetAll() ([]domain.Trade, error)
	GetByStock(stockCode string) ([]domain.Trade, error)
	GetRecentByStock(stockCode string, limit int) ([]domain.Trade, error)
}

type UserRepository interface {
	Save(user *domain.User) error
	GetByUsername(username string) (*domain.User, error)
	GetByID(id string) (*domain.User, error)
}
