package postgres

import (
	"fmt"
	"log"

	"github.com/ilhaamms/ybtech/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OrderRepository struct {
	db *gorm.DB
}

func NewOrderRepository(db *DB) *OrderRepository {
	return &OrderRepository{db: db.GetConn()}
}

func (r *OrderRepository) Save(order *domain.Order) error {
	snap := order.ToSnapshot()
	m := OrderModel{
		ID:           snap.ID,
		StockCode:    snap.StockCode,
		Side:         string(snap.Side),
		Price:        snap.Price,
		Quantity:     snap.Quantity,
		FilledQty:    snap.FilledQty,
		RemainingQty: snap.RemainingQty,
		Status:       string(snap.Status),
		CreatedAt:    snap.CreatedAt,
		UpdatedAt:    snap.UpdatedAt,
	}
	result := r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"filled_qty", "remaining_qty", "status", "updated_at"}),
	}).Create(&m)
	if result.Error != nil {
		log.Printf("POSTGRES: failed to save order %s: %v", snap.ID, result.Error)
	}
	return result.Error
}

func (r *OrderRepository) UpdateStatus(order *domain.Order) error {
	snap := order.ToSnapshot()
	result := r.db.Model(&OrderModel{}).Where("id = ?", snap.ID).Updates(map[string]interface{}{
		"filled_qty":    snap.FilledQty,
		"remaining_qty": snap.RemainingQty,
		"status":        string(snap.Status),
		"updated_at":    snap.UpdatedAt,
	})
	if result.Error != nil {
		log.Printf("POSTGRES: failed to update order %s: %v", snap.ID, result.Error)
	}
	return result.Error
}

func (r *OrderRepository) GetByID(id string) (*domain.Order, error) {
	var m OrderModel
	result := r.db.First(&m, "id = ?", id)
	if result.Error != nil {
		return nil, fmt.Errorf("order %s not found: %v", id, result.Error)
	}
	return orderModelToDomain(m), nil
}

func (r *OrderRepository) GetAll(stockCode, status string) ([]domain.Order, error) {
	var models []OrderModel
	q := r.db.Order("created_at DESC")
	if stockCode != "" {
		q = q.Where("stock_code = ?", stockCode)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if result := q.Find(&models); result.Error != nil {
		return nil, result.Error
	}
	orders := make([]domain.Order, len(models))
	for i, m := range models {
		orders[i] = *orderModelToDomain(m)
	}
	return orders, nil
}

func (r *OrderRepository) GetOpenOrdersByStock(stockCode string, side domain.Side) ([]*domain.Order, error) {
	var models []OrderModel
	result := r.db.
		Where("stock_code = ? AND side = ? AND status IN ('OPEN','PARTIAL')", stockCode, string(side)).
		Order("created_at ASC").
		Find(&models)
	if result.Error != nil {
		return nil, result.Error
	}
	orders := make([]*domain.Order, len(models))
	for i, m := range models {
		orders[i] = orderModelToDomain(m)
	}
	return orders, nil
}

func orderModelToDomain(m OrderModel) *domain.Order {
	return &domain.Order{
		ID:           m.ID,
		StockCode:    m.StockCode,
		Side:         domain.Side(m.Side),
		Price:        m.Price,
		Quantity:     m.Quantity,
		FilledQty:    m.FilledQty,
		RemainingQty: m.RemainingQty,
		Status:       domain.OrderStatus(m.Status),
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}
