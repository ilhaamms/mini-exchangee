package postgres

import (
	"database/sql"
	"log"

	"github.com/ilhaamms/ybtech/internal/domain"
)

// TradeRepository provides PostgreSQL-backed storage for trades
type TradeRepository struct {
	db *sql.DB
}

// NewTradeRepository creates a new PostgreSQL trade repository
func NewTradeRepository(db *DB) *TradeRepository {
	return &TradeRepository{db: db.GetConn()}
}

// Save inserts a trade into the database
func (r *TradeRepository) Save(trade *domain.Trade) error {
	query := `
		INSERT INTO trades (id, stock_code, buy_order_id, sell_order_id, price, quantity, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO NOTHING
	`
	_, err := r.db.Exec(query,
		trade.ID, trade.StockCode, trade.BuyOrderID, trade.SellOrderID,
		trade.Price, trade.Quantity, trade.CreatedAt,
	)
	if err != nil {
		log.Printf("POSTGRES: failed to save trade %s: %v", trade.ID, err)
	}
	return err
}

// GetAll returns all trades sorted by time descending
func (r *TradeRepository) GetAll() ([]domain.Trade, error) {
	query := `SELECT id, stock_code, buy_order_id, sell_order_id, price, quantity, created_at FROM trades ORDER BY created_at DESC`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trades []domain.Trade
	for rows.Next() {
		var t domain.Trade
		if err := rows.Scan(&t.ID, &t.StockCode, &t.BuyOrderID, &t.SellOrderID,
			&t.Price, &t.Quantity, &t.CreatedAt); err != nil {
			continue
		}
		trades = append(trades, t)
	}

	return trades, nil
}

// GetByStock returns trades for a specific stock code
func (r *TradeRepository) GetByStock(stockCode string) ([]domain.Trade, error) {
	query := `SELECT id, stock_code, buy_order_id, sell_order_id, price, quantity, created_at
	          FROM trades WHERE stock_code = $1 ORDER BY created_at DESC`

	rows, err := r.db.Query(query, stockCode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trades []domain.Trade
	for rows.Next() {
		var t domain.Trade
		if err := rows.Scan(&t.ID, &t.StockCode, &t.BuyOrderID, &t.SellOrderID,
			&t.Price, &t.Quantity, &t.CreatedAt); err != nil {
			continue
		}
		trades = append(trades, t)
	}

	return trades, nil
}

// GetRecentByStock returns the last N trades for a stock
func (r *TradeRepository) GetRecentByStock(stockCode string, limit int) ([]domain.Trade, error) {
	query := `SELECT id, stock_code, buy_order_id, sell_order_id, price, quantity, created_at
	          FROM trades WHERE stock_code = $1 ORDER BY created_at DESC LIMIT $2`

	rows, err := r.db.Query(query, stockCode, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trades []domain.Trade
	for rows.Next() {
		var t domain.Trade
		if err := rows.Scan(&t.ID, &t.StockCode, &t.BuyOrderID, &t.SellOrderID,
			&t.Price, &t.Quantity, &t.CreatedAt); err != nil {
			continue
		}
		trades = append(trades, t)
	}

	return trades, nil
}
