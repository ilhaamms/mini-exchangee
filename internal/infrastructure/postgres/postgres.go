package postgres

import (
	"database/sql"
	"log"

	_ "github.com/lib/pq"
)

// DB wraps the PostgreSQL database connection
type DB struct {
	conn *sql.DB
}

// NewDB creates a new PostgreSQL connection and runs migrations
func NewDB(dsn string) (*DB, error) {
	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	// Configure connection pool
	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(5)

	// Verify connection
	if err := conn.Ping(); err != nil {
		return nil, err
	}

	log.Println("POSTGRES: connected successfully")

	db := &DB{conn: conn}

	// Auto-migrate
	if err := db.migrate(); err != nil {
		return nil, err
	}

	return db, nil
}

// GetConn returns the underlying *sql.DB
func (db *DB) GetConn() *sql.DB {
	return db.conn
}

// Close closes the database connection
func (db *DB) Close() error {
	log.Println("POSTGRES: closing connection")
	return db.conn.Close()
}

// migrate creates tables if they don't exist
func (db *DB) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS orders (
			id            VARCHAR(20) PRIMARY KEY,
			stock_code    VARCHAR(10) NOT NULL,
			side          VARCHAR(4) NOT NULL,
			price         DOUBLE PRECISION NOT NULL,
			quantity      BIGINT NOT NULL,
			filled_qty    BIGINT NOT NULL DEFAULT 0,
			remaining_qty BIGINT NOT NULL,
			status        VARCHAR(10) NOT NULL DEFAULT 'OPEN',
			created_at    TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			updated_at    TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_orders_stock_code ON orders(stock_code)`,
		`CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status)`,
		`CREATE INDEX IF NOT EXISTS idx_orders_stock_side_status ON orders(stock_code, side, status)`,

		`CREATE TABLE IF NOT EXISTS trades (
			id            VARCHAR(20) PRIMARY KEY,
			stock_code    VARCHAR(10) NOT NULL,
			buy_order_id  VARCHAR(20) NOT NULL,
			sell_order_id VARCHAR(20) NOT NULL,
			price         DOUBLE PRECISION NOT NULL,
			quantity      BIGINT NOT NULL,
			created_at    TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_trades_stock_code ON trades(stock_code)`,
		`CREATE INDEX IF NOT EXISTS idx_trades_created_at ON trades(created_at DESC)`,

		`CREATE TABLE IF NOT EXISTS users (
			id         VARCHAR(20) PRIMARY KEY,
			username   VARCHAR(50) UNIQUE NOT NULL,
			email      VARCHAR(100) UNIQUE NOT NULL,
			password   VARCHAR(64) NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users(username)`,
	}

	for _, q := range queries {
		if _, err := db.conn.Exec(q); err != nil {
			return err
		}
	}

	log.Println("POSTGRES: migration completed successfully")
	return nil
}
