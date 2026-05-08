package postgres

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/ilhaamms/ybtech/internal/domain"
)

// OrderRepository provides PostgreSQL-backed storage for orders.
// It mirrors the in-memory OrderRepository interface but persists data.
type OrderRepository struct {
	db *sql.DB
}

// NewOrderRepository creates a new PostgreSQL order repository
func NewOrderRepository(db *DB) *OrderRepository {
	return &OrderRepository{db: db.GetConn()}
}

// Save inserts or updates an order in the database
func (r *OrderRepository) Save(order *domain.Order) error {
	snap := order.ToSnapshot()
	query := `
		INSERT INTO orders (id, stock_code, side, price, quantity, filled_qty, remaining_qty, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO UPDATE SET
			filled_qty = $6,
			remaining_qty = $7,
			status = $8,
			updated_at = $10
	`
	_, err := r.db.Exec(query,
		snap.ID, snap.StockCode, string(snap.Side), snap.Price,
		snap.Quantity, snap.FilledQty, snap.RemainingQty,
		string(snap.Status), snap.CreatedAt, snap.UpdatedAt,
	)
	if err != nil {
		log.Printf("POSTGRES: failed to save order %s: %v", snap.ID, err)
	}
	return err
}

// UpdateStatus updates the fill status of an order in the database
func (r *OrderRepository) UpdateStatus(order *domain.Order) error {
	snap := order.ToSnapshot()
	query := `UPDATE orders SET filled_qty = $1, remaining_qty = $2, status = $3, updated_at = $4 WHERE id = $5`
	_, err := r.db.Exec(query, snap.FilledQty, snap.RemainingQty, string(snap.Status), snap.UpdatedAt, snap.ID)
	if err != nil {
		log.Printf("POSTGRES: failed to update order %s: %v", snap.ID, err)
	}
	return err
}

// GetByID retrieves an order by its ID
func (r *OrderRepository) GetByID(id string) (*domain.Order, error) {
	query := `SELECT id, stock_code, side, price, quantity, filled_qty, remaining_qty, status, created_at, updated_at FROM orders WHERE id = $1`
	row := r.db.QueryRow(query, id)

	var order domain.Order
	var side, status string
	err := row.Scan(&order.ID, &order.StockCode, &side, &order.Price,
		&order.Quantity, &order.FilledQty, &order.RemainingQty,
		&status, &order.CreatedAt, &order.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("order %s not found: %v", id, err)
	}

	order.Side = domain.Side(side)
	order.Status = domain.OrderStatus(status)
	return &order, nil
}

// GetAll returns orders, optionally filtered by stock and status
func (r *OrderRepository) GetAll(stockCode, status string) ([]domain.Order, error) {
	query := `SELECT id, stock_code, side, price, quantity, filled_qty, remaining_qty, status, created_at, updated_at FROM orders WHERE 1=1`
	args := make([]interface{}, 0)
	argIdx := 1

	if stockCode != "" {
		query += fmt.Sprintf(" AND stock_code = $%d", argIdx)
		args = append(args, stockCode)
		argIdx++
	}
	if status != "" {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}

	query += " ORDER BY created_at DESC"

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []domain.Order
	for rows.Next() {
		var o domain.Order
		var side, st string
		if err := rows.Scan(&o.ID, &o.StockCode, &side, &o.Price,
			&o.Quantity, &o.FilledQty, &o.RemainingQty,
			&st, &o.CreatedAt, &o.UpdatedAt); err != nil {
			continue
		}
		o.Side = domain.Side(side)
		o.Status = domain.OrderStatus(st)
		// Use ToSnapshot to get a clean copy without mutex concerns
		orders = append(orders, o.ToSnapshot())
	}

	return orders, nil
}
