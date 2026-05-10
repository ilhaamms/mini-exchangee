package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// HTTP layer
	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mini_exchange_http_requests_total",
		Help: "Total number of HTTP requests.",
	}, []string{"method", "path", "status"})

	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "mini_exchange_http_request_duration_seconds",
		Help:    "HTTP request latency in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})

	// WebSocket layer
	WSActiveConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "mini_exchange_ws_active_connections",
		Help: "Number of currently active WebSocket connections.",
	})

	// Business layer
	OrdersCreatedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "mini_exchange_orders_created_total",
		Help: "Total number of orders successfully created.",
	})

	TradesExecutedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "mini_exchange_trades_executed_total",
		Help: "Total number of trades executed by the matching engine.",
	})
)

// Handler returns the Prometheus HTTP handler for the /metrics endpoint.
func Handler() http.Handler {
	return promhttp.Handler()
}
