package http

import (
	"net/http"
	"strconv"

	"github.com/ilhaamms/ybtech/internal/engine"
	"github.com/ilhaamms/ybtech/internal/repository"
	"github.com/ilhaamms/ybtech/pkg/response"
)

type MarketHandler struct {
	marketRepo *repository.MarketRepository
	tradeRepo  repository.TradeRepository
	engine     *engine.MatchingEngine
}

func NewMarketHandler(
	marketRepo *repository.MarketRepository,
	tradeRepo repository.TradeRepository,
	engine *engine.MatchingEngine,
) *MarketHandler {
	return &MarketHandler{
		marketRepo: marketRepo,
		tradeRepo:  tradeRepo,
		engine:     engine,
	}
}

func (h *MarketHandler) GetTicker(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.BadRequest(w, "method not allowed, use GET")
		return
	}

	stock := r.URL.Query().Get("stock")

	if stock != "" {
		md := h.marketRepo.Get(stock)
		if md == nil {
			response.NotFound(w, "stock not found: "+stock)
			return
		}
		response.Success(w, "ticker retrieved successfully", md.GetTicker())
		return
	}

	
	tickers := h.marketRepo.GetAll()
	response.Success(w, "all tickers retrieved successfully", tickers)
}

func (h *MarketHandler) GetOrderBook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.BadRequest(w, "method not allowed, use GET")
		return
	}

	stock := r.URL.Query().Get("stock")
	if stock == "" {
		response.BadRequest(w, "stock query parameter is required")
		return
	}

	orderBook := h.engine.GetOrderBook(stock)
	response.Success(w, "order book retrieved successfully", orderBook)
}

func (h *MarketHandler) GetRecentTrades(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.BadRequest(w, "method not allowed, use GET")
		return
	}

	stock := r.URL.Query().Get("stock")
	if stock == "" {
		response.BadRequest(w, "stock query parameter is required")
		return
	}

	limit := 20 
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	trades, err := h.tradeRepo.GetRecentByStock(stock, limit)
	if err != nil {
		response.InternalError(w, "failed to retrieve trades: "+err.Error())
		return
	}
	response.Success(w, "recent trades retrieved successfully", map[string]interface{}{
		"trades": trades,
		"count":  len(trades),
	})
}
