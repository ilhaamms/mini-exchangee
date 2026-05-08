package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	httpDelivery "github.com/ilhaamms/ybtech/internal/delivery/http"
	"github.com/ilhaamms/ybtech/internal/delivery/http/middleware"
	wsDelivery "github.com/ilhaamms/ybtech/internal/delivery/websocket"
	"github.com/ilhaamms/ybtech/internal/domain"
	"github.com/ilhaamms/ybtech/internal/engine"
	infraNats "github.com/ilhaamms/ybtech/internal/infrastructure/nats"
	infraRedis "github.com/ilhaamms/ybtech/internal/infrastructure/redis"
	"github.com/ilhaamms/ybtech/internal/config"
	"github.com/ilhaamms/ybtech/internal/repository"
	"github.com/ilhaamms/ybtech/internal/simulator"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Println("=== Mini Exchange - Realtime Trading System ===")

	// ─── Load Configuration ───
	cfg := config.Load()

	// ─── Initialize Repositories (In-Memory Storage) ───
	orderRepo := repository.NewOrderRepository()
	tradeRepo := repository.NewTradeRepository()
	marketRepo := repository.NewMarketRepository()
	userRepo := repository.NewUserRepository()

	// ─── Initialize WebSocket Hub ───
	hub := wsDelivery.NewHub()
	go hub.Run()

	// ─── Event Callback Setup ───
	// Default: route events directly to WebSocket hub
	onEvent := func(event domain.Event) {
		hub.BroadcastEvent(event)
	}

	// ─── [BONUS] Redis Integration ───
	var redisClient *infraRedis.Client
	var redisPubSub *infraRedis.PubSub
	var redisCache *infraRedis.Cache

	if cfg.RedisEnabled {
		var err error
		redisClient, err = infraRedis.NewClient(cfg.RedisAddr, cfg.RedisPassword, 0)
		if err != nil {
			log.Printf("WARNING: Redis connection failed: %v (continuing without Redis)", err)
		} else {
			redisCache = infraRedis.NewCache(redisClient)
			_ = redisCache // available for handlers if needed

			// Redis Pub/Sub: receive events from other server instances
			redisPubSub = infraRedis.NewPubSub(redisClient, func(event domain.Event) {
				hub.BroadcastEvent(event)
			})
			if err := redisPubSub.Start(context.Background()); err != nil {
				log.Printf("WARNING: Redis Pub/Sub failed: %v", err)
			}

			// Override onEvent to also publish to Redis
			originalOnEvent := onEvent
			onEvent = func(event domain.Event) {
				originalOnEvent(event) // local broadcast
				if err := redisPubSub.Publish(context.Background(), event); err != nil {
					log.Printf("WARNING: Redis publish failed: %v", err)
				}
				// Cache ticker updates
				if event.Type == domain.EventTicker {
					if ticker, ok := event.Data.(domain.Ticker); ok {
						redisCache.SetTicker(context.Background(), ticker)
					}
				}
			}

			log.Println("REDIS: integration enabled (Pub/Sub + Cache)")
		}
	}

	// ─── [BONUS] NATS Message Broker ───
	var natsBroker *infraNats.Broker

	if cfg.NatsEnabled {
		var err error
		natsBroker, err = infraNats.NewBroker(cfg.NatsURL)
		if err != nil {
			log.Printf("WARNING: NATS connection failed: %v (continuing without NATS)", err)
		} else {
			// Subscribe: NATS events → WebSocket hub
			natsBroker.Subscribe(func(event domain.Event) {
				hub.BroadcastEvent(event)
			})

			// Override onEvent to publish to NATS instead of direct broadcast
			onEvent = func(event domain.Event) {
				if err := natsBroker.Publish(event); err != nil {
					log.Printf("WARNING: NATS publish failed: %v", err)
					// Fallback: direct broadcast
					hub.BroadcastEvent(event)
				}
			}

			log.Println("NATS: message broker enabled")
		}
	}

	// ─── Initialize Matching Engine ───
	matchingEngine := engine.NewMatchingEngine(orderRepo, tradeRepo, marketRepo, onEvent)

	// ─── Initialize Price Feed (Simulator or Binance) ───
	var priceFeeder interface{ Stop() }

	if cfg.BinanceEnabled {
		// [BONUS] Binance real market data
		feed := simulator.NewBinanceFeed(marketRepo, onEvent)
		if err := feed.Start(); err != nil {
			log.Printf("WARNING: Binance feed failed: %v (falling back to simulator)", err)
			// Fallback to simulator
			stocks := simulator.DefaultStocks()
			sim := simulator.NewPriceSimulator(marketRepo, onEvent, stocks)
			sim.Start()
			priceFeeder = sim
		} else {
			priceFeeder = feed
			log.Println("BINANCE: real market data feed enabled")
		}
	} else {
		// Default: simulated price data
		stocks := simulator.DefaultStocks()
		sim := simulator.NewPriceSimulator(marketRepo, onEvent, stocks)
		sim.Start()
		priceFeeder = sim
	}

	// ─── [BONUS] JWT Configuration ───
	jwtConfig := middleware.DefaultJWTConfig()
	if cfg.JWTSecret != "" {
		jwtConfig.SecretKey = cfg.JWTSecret
	}

	// ─── [BONUS] Rate Limiter ───
	// 100 requests burst capacity, 100 requests per minute sustained rate
	rateLimiter := middleware.NewRateLimiter(100, 100)

	// ─── Setup HTTP Router ───
	router := httpDelivery.NewRouter(httpDelivery.RouterConfig{
		OrderRepo:      orderRepo,
		TradeRepo:      tradeRepo,
		MarketRepo:     marketRepo,
		UserRepo:       userRepo,
		MatchingEngine: matchingEngine,
		Hub:            hub,
		JWTConfig:      jwtConfig,
		RateLimiter:    rateLimiter,
	})

	// ─── Create HTTP Server ───
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// ─── Start Server ───
	go func() {
		log.Printf("Server starting on http://localhost:%s", cfg.Port)
		log.Printf("REST API:     http://localhost:%s/api/orders", cfg.Port)
		log.Printf("Auth:         http://localhost:%s/api/register | /api/login", cfg.Port)
		log.Printf("WebSocket:    ws://localhost:%s/ws", cfg.Port)
		log.Printf("Health:       http://localhost:%s/health", cfg.Port)
		log.Println("─────────────────────────────────────────────")
		log.Printf("Rate Limit:   100 req/min per IP")
		log.Printf("JWT Auth:     enabled (protected: /api/orders, /api/trades)")
		log.Printf("Redis:        %s", enabledStr(cfg.RedisEnabled))
		log.Printf("PostgreSQL:   %s", enabledStr(cfg.PostgresEnabled))
		log.Printf("NATS:         %s", enabledStr(cfg.NatsEnabled))
		log.Printf("Binance Feed: %s", enabledStr(cfg.BinanceEnabled))

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// ─── Graceful Shutdown ───
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Printf("Received signal %v, shutting down...", sig)

	// Stop price feeder
	priceFeeder.Stop()

	// Stop rate limiter cleanup goroutine
	rateLimiter.Stop()

	// Stop NATS broker
	if natsBroker != nil {
		natsBroker.Close()
	}

	// Stop Redis
	if redisPubSub != nil {
		redisPubSub.Stop()
	}
	if redisClient != nil {
		redisClient.Close()
	}

	// Shutdown HTTP server with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped gracefully")
}

func enabledStr(enabled bool) string {
	if enabled {
		return "ENABLED"
	}
	return "disabled (set env to enable)"
}
