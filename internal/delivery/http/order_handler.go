package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/ilhaamms/ybtech/internal/domain"
	"github.com/ilhaamms/ybtech/internal/engine"
	"github.com/ilhaamms/ybtech/internal/repository"
	"github.com/ilhaamms/ybtech/pkg/response"
)

var orderSeq int64

type OrderHandler struct {
	orderRepo  repository.OrderRepository
	marketRepo *repository.MarketRepository
	engine     *engine.MatchingEngine
}

func NewOrderHandler(
	orderRepo repository.OrderRepository,
	marketRepo *repository.MarketRepository,
	engine *engine.MatchingEngine,
) *OrderHandler {
	return &OrderHandler{
		orderRepo:  orderRepo,
		marketRepo: marketRepo,
		engine:     engine,
	}
}

type CreateOrderRequest struct {
	StockCode string  `json:"stock_code"`
	Side      string  `json:"side"`
	Price     float64 `json:"price"`
	Quantity  int64   `json:"quantity"`
}

func (h *OrderHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.BadRequest(w, "method not allowed, use POST")
		return
	}

	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body: "+err.Error())
		return
	}

	
	if req.StockCode == "" {
		response.BadRequest(w, "stock_code is required")
		return
	}
	if req.Side != "BUY" && req.Side != "SELL" {
		response.BadRequest(w, "side must be BUY or SELL")
		return
	}
	if req.Price <= 0 {
		response.BadRequest(w, "price must be greater than 0")
		return
	}
	if req.Quantity <= 0 {
		response.BadRequest(w, "quantity must be greater than 0")
		return
	}

	
	orderID := fmt.Sprintf("ORD%010d", atomic.AddInt64(&orderSeq, 1))

	
	h.marketRepo.GetOrCreate(req.StockCode, req.Price)

	
	order := domain.NewOrder(orderID, req.StockCode, domain.Side(req.Side), req.Price, req.Quantity)

	
	if err := h.orderRepo.Save(order); err != nil {
		response.InternalError(w, "failed to save order: "+err.Error())
		return
	}

	
	
	go h.engine.ProcessOrder(order)

	response.Created(w, "order created successfully", order.ToSnapshot())
}

func (h *OrderHandler) GetOrders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.BadRequest(w, "method not allowed, use GET")
		return
	}

	stock := r.URL.Query().Get("stock")
	status := r.URL.Query().Get("status")

	orders, err := h.orderRepo.GetAll(stock, status)
	if err != nil {
		response.InternalError(w, "failed to retrieve orders: "+err.Error())
		return
	}

	response.Success(w, "orders retrieved successfully", map[string]interface{}{
		"orders": orders,
		"count":  len(orders),
	})
}
