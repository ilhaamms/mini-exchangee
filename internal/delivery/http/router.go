package http

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/ilhaamms/ybtech/internal/delivery/http/middleware"
	"github.com/ilhaamms/ybtech/internal/delivery/websocket"
	"github.com/ilhaamms/ybtech/internal/engine"
	"github.com/ilhaamms/ybtech/internal/repository"
	"github.com/ilhaamms/ybtech/pkg/metrics"
)

// statusRecorder wraps http.ResponseWriter to capture the status code written.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

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

// loggingMiddleware logs every request with structured fields via slog
// and records Prometheus HTTP metrics.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		duration := time.Since(start)
		status := rec.status

		slog.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"duration_ms", duration.Milliseconds(),
			"remote_addr", r.RemoteAddr,
		)

		metrics.HTTPRequestsTotal.WithLabelValues(
			r.Method,
			r.URL.Path,
			fmt.Sprintf("%d", status),
		).Inc()

		metrics.HTTPRequestDuration.WithLabelValues(
			r.Method,
			r.URL.Path,
		).Observe(duration.Seconds())
	})
}

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

func NewRouter(cfg RouterConfig) http.Handler {
	mux := http.NewServeMux()

	orderHandler := NewOrderHandler(cfg.OrderRepo, cfg.MarketRepo, cfg.MatchingEngine)
	tradeHandler := NewTradeHandler(cfg.TradeRepo)
	marketHandler := NewMarketHandler(cfg.MarketRepo, cfg.TradeRepo, cfg.MatchingEngine)
	authHandler := NewAuthHandler(cfg.UserRepo, cfg.JWTConfig)

	mux.HandleFunc("/api/register", authHandler.Register)
	mux.HandleFunc("/api/login", authHandler.Login)

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","service":"mini-exchange"}`))
	})

	mux.Handle("/metrics", metrics.Handler())

	authMw := middleware.AuthMiddleware(cfg.JWTConfig)

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

	mux.Handle("/api/trades", authMw(http.HandlerFunc(tradeHandler.GetTradeHistory)))

	mux.HandleFunc("/api/market/ticker", marketHandler.GetTicker)
	mux.HandleFunc("/api/market/orderbook", marketHandler.GetOrderBook)
	mux.HandleFunc("/api/market/trades", marketHandler.GetRecentTrades)

	mux.HandleFunc("/ws", websocket.HandleWebSocket(cfg.Hub, cfg.JWTConfig))

	handler := corsMiddleware(
		loggingMiddleware(
			middleware.RateLimitMiddleware(cfg.RateLimiter)(mux),
		),
	)

	return handler
}
