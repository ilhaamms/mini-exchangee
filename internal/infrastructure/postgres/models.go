package postgres

import "time"

type OrderModel struct {
	ID           string    `gorm:"primaryKey;column:id;type:varchar(20)"`
	StockCode    string    `gorm:"column:stock_code;not null;type:varchar(10);index:idx_orders_stock_code;index:idx_orders_stock_side_status,composite:stock_code"`
	Side         string    `gorm:"column:side;not null;type:varchar(4);index:idx_orders_stock_side_status,composite:side"`
	Price        float64   `gorm:"column:price;not null"`
	Quantity     int64     `gorm:"column:quantity;not null"`
	FilledQty    int64     `gorm:"column:filled_qty;not null;default:0"`
	RemainingQty int64     `gorm:"column:remaining_qty;not null"`
	Status       string    `gorm:"column:status;not null;type:varchar(10);default:'OPEN';index:idx_orders_status;index:idx_orders_stock_side_status,composite:status"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt    time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (OrderModel) TableName() string { return "orders" }

type TradeModel struct {
	ID          string    `gorm:"primaryKey;column:id;type:varchar(20)"`
	StockCode   string    `gorm:"column:stock_code;not null;type:varchar(10);index:idx_trades_stock_code"`
	BuyOrderID  string    `gorm:"column:buy_order_id;not null;type:varchar(20)"`
	SellOrderID string    `gorm:"column:sell_order_id;not null;type:varchar(20)"`
	Price       float64   `gorm:"column:price;not null"`
	Quantity    int64     `gorm:"column:quantity;not null"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime;index:idx_trades_created_at,sort:desc"`
}

func (TradeModel) TableName() string { return "trades" }

type UserModel struct {
	ID        string    `gorm:"primaryKey;column:id;type:varchar(20)"`
	Username  string    `gorm:"column:username;uniqueIndex:idx_users_username;not null;type:varchar(50)"`
	Email     string    `gorm:"column:email;uniqueIndex:idx_users_email;not null;type:varchar(100)"`
	Password  string    `gorm:"column:password;not null;type:varchar(64)"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (UserModel) TableName() string { return "users" }
