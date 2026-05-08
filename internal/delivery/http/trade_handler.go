package http

import (
	"net/http"

	"github.com/ilhaamms/ybtech/internal/repository"
	"github.com/ilhaamms/ybtech/pkg/response"
)

// TradeHandler handles trade-related HTTP requests
type TradeHandler struct {
	tradeRepo *repository.TradeRepository
}

// NewTradeHandler creates a new TradeHandler
func NewTradeHandler(tradeRepo *repository.TradeRepository) *TradeHandler {
	return &TradeHandler{
		tradeRepo: tradeRepo,
	}
}

// GetTradeHistory handles GET /api/trades?stock=BBCA
func (h *TradeHandler) GetTradeHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.BadRequest(w, "method not allowed, use GET")
		return
	}

	stock := r.URL.Query().Get("stock")

	var trades interface{}
	if stock != "" {
		t := h.tradeRepo.GetByStock(stock)
		trades = map[string]interface{}{
			"trades": t,
			"count":  len(t),
		}
	} else {
		t := h.tradeRepo.GetAll()
		trades = map[string]interface{}{
			"trades": t,
			"count":  len(t),
		}
	}

	response.Success(w, "trade history retrieved successfully", trades)
}
