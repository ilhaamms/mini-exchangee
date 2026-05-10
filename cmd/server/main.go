package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ilhaamms/ybtech/internal/config"
	httpDelivery "github.com/ilhaamms/ybtech/internal/delivery/http"
	"github.com/ilhaamms/ybtech/internal/delivery/http/middleware"
	wsDelivery "github.com/ilhaamms/ybtech/internal/delivery/websocket"
	"github.com/ilhaamms/ybtech/internal/domain"
	"github.com/ilhaamms/ybtech/internal/engine"
	infraNats "github.com/ilhaamms/ybtech/internal/infrastructure/nats"
	infraPostgres "github.com/ilhaamms/ybtech/internal/infrastructure/postgres"
	infraRedis "github.com/ilhaamms/ybtech/internal/infrastructure/redis"
	"github.com/ilhaamms/ybtech/internal/repository"
	"github.com/ilhaamms/ybtech/internal/simulator"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Println("=== Mini Exchange - Realtime Trading System ===")

	
	cfg := config.Load()

	
	var orderRepo repository.OrderRepository = repository.NewOrderRepository()
	var tradeRepo repository.TradeRepository = repository.NewTradeRepository()
	marketRepo := repository.NewMarketRepository()
	var userRepo repository.UserRepository = repository.NewUserRepository()

	
	var pgDB *infraPostgres.DB
	if cfg.PostgresEnabled {
		var err error
		pgDB, err = infraPostgres.NewDB(cfg.PostgresDSN)
		if err != nil {
			log.Printf("WARNING: PostgreSQL connection failed: %v (continuing with in-memory storage)", err)
		} else {
			orderRepo = infraPostgres.NewOrderRepository(pgDB)
			tradeRepo = infraPostgres.NewTradeRepository(pgDB)
			userRepo = infraPostgres.NewUserRepository(pgDB)
			log.Println("POSTGRES: using PostgreSQL for order, trade, and user storage")
		}
	}

	
	hub := wsDelivery.NewHub()
	go hub.Run()

	
	
	onEvent := func(event domain.Event) {
		hub.BroadcastEvent(event)
	}

	
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
			_ = redisCache 

			
			redisPubSub = infraRedis.NewPubSub(redisClient, func(event domain.Event) {
				hub.BroadcastEvent(event)
			})
			if err := redisPubSub.Start(context.Background()); err != nil {
				log.Printf("WARNING: Redis Pub/Sub failed: %v", err)
			}

			
			originalOnEvent := onEvent
			onEvent = func(event domain.Event) {
				originalOnEvent(event) 
				if err := redisPubSub.Publish(context.Background(), event); err != nil {
					log.Printf("WARNING: Redis publish failed: %v", err)
				}
				
				if event.Type == domain.EventTicker {
					if ticker, ok := event.Data.(domain.Ticker); ok {
						redisCache.SetTicker(context.Background(), ticker)
					}
				}
			}

			log.Println("REDIS: integration enabled (Pub/Sub + Cache)")
		}
	}

	
	var natsBroker *infraNats.Broker

	if cfg.NatsEnabled {
		var err error
		natsBroker, err = infraNats.NewBroker(cfg.NatsURL)
		if err != nil {
			log.Printf("WARNING: NATS connection failed: %v (continuing without NATS)", err)
		} else {
			
			natsBroker.Subscribe(func(event domain.Event) {
				hub.BroadcastEvent(event)
			})

			
			onEvent = func(event domain.Event) {
				if err := natsBroker.Publish(event); err != nil {
					log.Printf("WARNING: NATS publish failed: %v", err)
					
					hub.BroadcastEvent(event)
				}
			}

			log.Println("NATS: message broker enabled")
		}
	}

	
	matchingEngine := engine.NewMatchingEngine(orderRepo, tradeRepo, marketRepo, onEvent)

	
	var priceFeeder interface{ Stop() }

	if cfg.BinanceEnabled {
		
		feed := simulator.NewBinanceFeed(marketRepo, onEvent)
		if err := feed.Start(); err != nil {
			log.Printf("WARNING: Binance feed failed: %v (falling back to simulator)", err)
			
			stocks := simulator.DefaultStocks()
			sim := simulator.NewPriceSimulator(marketRepo, onEvent, stocks)
			sim.Start()
			priceFeeder = sim
		} else {
			priceFeeder = feed
			log.Println("BINANCE: real market data feed enabled")
		}
	} else {
		
		stocks := simulator.DefaultStocks()
		sim := simulator.NewPriceSimulator(marketRepo, onEvent, stocks)
		sim.Start()
		priceFeeder = sim
	}

	
	jwtConfig := middleware.DefaultJWTConfig()
	if cfg.JWTSecret != "" {
		jwtConfig.SecretKey = cfg.JWTSecret
	}

	
	
	rateLimiter := middleware.NewRateLimiter(100, 100)

	
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

	
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	
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

	
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Printf("Received signal %v, shutting down...", sig)

	
	priceFeeder.Stop()

	
	rateLimiter.Stop()

	
	if natsBroker != nil {
		natsBroker.Close()
	}

	
	if redisPubSub != nil {
		redisPubSub.Stop()
	}
	if redisClient != nil {
		redisClient.Close()
	}

	
	if pgDB != nil {
		pgDB.Close()
	}

	
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
