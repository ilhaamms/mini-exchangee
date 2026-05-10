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

// OrderHandler handles order-related HTTP requests
type OrderHandler struct {
	orderRepo  repository.OrderRepository
	marketRepo *repository.MarketRepository
	engine     *engine.MatchingEngine
}

// NewOrderHandler creates a new OrderHandler
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

// CreateOrderRequest represents the request body for creating an order
type CreateOrderRequest struct {
	StockCode string  `json:"stock_code"`
	Side      string  `json:"side"`
	Price     float64 `json:"price"`
	Quantity  int64   `json:"quantity"`
}

// CreateOrder handles POST /api/orders
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

	// Validate input
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

	// Generate unique order ID (atomic for concurrency safety)
	orderID := fmt.Sprintf("ORD%010d", atomic.AddInt64(&orderSeq, 1))

	// Ensure market data exists for this stock
	h.marketRepo.GetOrCreate(req.StockCode, req.Price)

	// Create order
	order := domain.NewOrder(orderID, req.StockCode, domain.Side(req.Side), req.Price, req.Quantity)

	// Save order first
	if err := h.orderRepo.Save(order); err != nil {
		response.InternalError(w, "failed to save order: "+err.Error())
		return
	}

	// Process matching in a goroutine for non-blocking response
	// The matching engine has per-stock locking so concurrent orders are safe
	go h.engine.ProcessOrder(order)

	response.Created(w, "order created successfully", order.ToSnapshot())
}

// GetOrders handles GET /api/orders?stock=BBCA&status=OPEN
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
