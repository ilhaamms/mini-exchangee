package http

import (
	"log"
	"net/http"
	"time"

	"github.com/ilhaamms/ybtech/internal/delivery/http/middleware"
	"github.com/ilhaamms/ybtech/internal/delivery/websocket"
	"github.com/ilhaamms/ybtech/internal/engine"
	"github.com/ilhaamms/ybtech/internal/repository"
)

// corsMiddleware adds CORS headers to allow cross-origin requests
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware logs HTTP requests
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("HTTP: %s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

// RouterConfig holds dependencies needed by the router
type RouterConfig struct {
	OrderRepo      repository.OrderRepository
	TradeRepo      repository.TradeRepository
	MarketRepo     *repository.MarketRepository
	UserRepo       repository.UserRepository
	MatchingEngine *engine.MatchingEngine
	Hub            *websocket.Hub
	JWTConfig      middleware.JWTConfig
	RateLimiter    *middleware.RateLimiter
}

// NewRouter creates and configures the HTTP router with all endpoints
func NewRouter(cfg RouterConfig) http.Handler {
	mux := http.NewServeMux()

	// Initialize handlers
	orderHandler := NewOrderHandler(cfg.OrderRepo, cfg.MarketRepo, cfg.MatchingEngine)
	tradeHandler := NewTradeHandler(cfg.TradeRepo)
	marketHandler := NewMarketHandler(cfg.MarketRepo, cfg.TradeRepo, cfg.MatchingEngine)
	authHandler := NewAuthHandler(cfg.UserRepo, cfg.JWTConfig)

	// ─── Public Routes (no auth required) ───

	// Auth endpoints
	mux.HandleFunc("/api/register", authHandler.Register)
	mux.HandleFunc("/api/login", authHandler.Login)

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","service":"mini-exchange"}`))
	})

	// ─── Protected Routes (JWT auth required) ───

	authMw := middleware.AuthMiddleware(cfg.JWTConfig)

	// Orders - protected
	mux.Handle("/api/orders", authMw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			orderHandler.CreateOrder(w, r)
		case http.MethodGet:
			orderHandler.GetOrders(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})))

	// Trades - protected
	mux.Handle("/api/trades", authMw(http.HandlerFunc(tradeHandler.GetTradeHistory)))

	// Market data - public (read-only market data doesn't need auth)
	mux.HandleFunc("/api/market/ticker", marketHandler.GetTicker)
	mux.HandleFunc("/api/market/orderbook", marketHandler.GetOrderBook)
	mux.HandleFunc("/api/market/trades", marketHandler.GetRecentTrades)

	// WebSocket endpoint (token validated inside handler via query param)
	mux.HandleFunc("/ws", websocket.HandleWebSocket(cfg.Hub, cfg.JWTConfig))

	// ─── Apply Global Middlewares ───
	// Order: CORS → Logging → Rate Limiting → Route Handler
	handler := corsMiddleware(
		loggingMiddleware(
			middleware.RateLimitMiddleware(cfg.RateLimiter)(mux),
		),
	)

	return handler
}
