package postgres

import (
	"log"

	"github.com/ilhaamms/ybtech/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TradeRepository struct {
	db *gorm.DB
}

func NewTradeRepository(db *DB) *TradeRepository {
	return &TradeRepository{db: db.GetConn()}
}

func (r *TradeRepository) Save(trade *domain.Trade) error {
	m := TradeModel{
		ID:          trade.ID,
		StockCode:   trade.StockCode,
		BuyOrderID:  trade.BuyOrderID,
		SellOrderID: trade.SellOrderID,
		Price:       trade.Price,
		Quantity:    trade.Quantity,
		CreatedAt:   trade.CreatedAt,
	}
	result := r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&m)
	if result.Error != nil {
		log.Printf("POSTGRES: failed to save trade %s: %v", trade.ID, result.Error)
	}
	return result.Error
}

func (r *TradeRepository) GetAll() ([]domain.Trade, error) {
	var models []TradeModel
	if result := r.db.Order("created_at DESC").Find(&models); result.Error != nil {
		return nil, result.Error
	}
	return tradeModelsToDomain(models), nil
}

func (r *TradeRepository) GetByStock(stockCode string) ([]domain.Trade, error) {
	var models []TradeModel
	result := r.db.Where("stock_code = ?", stockCode).Order("created_at DESC").Find(&models)
	if result.Error != nil {
		return nil, result.Error
	}
	return tradeModelsToDomain(models), nil
}

func (r *TradeRepository) GetRecentByStock(stockCode string, limit int) ([]domain.Trade, error) {
	var models []TradeModel
	result := r.db.Where("stock_code = ?", stockCode).Order("created_at DESC").Limit(limit).Find(&models)
	if result.Error != nil {
		return nil, result.Error
	}
	return tradeModelsToDomain(models), nil
}

func tradeModelsToDomain(models []TradeModel) []domain.Trade {
	trades := make([]domain.Trade, len(models))
	for i, m := range models {
		trades[i] = domain.Trade{
			ID:          m.ID,
			StockCode:   m.StockCode,
			BuyOrderID:  m.BuyOrderID,
			SellOrderID: m.SellOrderID,
			Price:       m.Price,
			Quantity:    m.Quantity,
			CreatedAt:   m.CreatedAt,
		}
	}
	return trades
}
