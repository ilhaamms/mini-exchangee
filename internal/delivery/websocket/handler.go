package websocket

import (
	"fmt"
	"log"
	"net/http"
	"sync/atomic"

	"github.com/ilhaamms/ybtech/internal/delivery/http/middleware"
	ws "github.com/gorilla/websocket"
)

var upgrader = ws.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow all origins for development/testing
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var clientSeq int64

// HandleWebSocket upgrades an HTTP connection to WebSocket.
// JWT token is validated via the "token" query parameter:
//
//	ws://localhost:8080/ws?token=<jwt>
//
// If no token is provided, the connection is still allowed but
// the client ID will be "anon-X" instead of "user-X (username)".
func HandleWebSocket(hub *Hub, jwtConfig middleware.JWTConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Optional JWT authentication via query parameter
		tokenStr := r.URL.Query().Get("token")
		clientLabel := ""

		if tokenStr != "" {
			claims, err := middleware.ValidateWSToken(jwtConfig, tokenStr)
			if err != nil {
				log.Printf("WS: invalid token from %s: %v", r.RemoteAddr, err)
				http.Error(w, "invalid or expired token", http.StatusUnauthorized)
				return
			}
			clientLabel = fmt.Sprintf("user-%s (%s)", claims.UserID, claims.Username)
		} else {
			clientLabel = fmt.Sprintf("anon-%d", atomic.AddInt64(&clientSeq, 1))
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("WS: upgrade error: %v", err)
			return
		}

		client := NewClient(hub, conn, clientLabel)

		hub.Register(client)

		// Start read and write pumps in separate goroutines
		go client.WritePump()
		go client.ReadPump()

		// Send welcome message
		welcomeMsg := "Welcome to Mini Exchange WebSocket."
		if tokenStr != "" {
			welcomeMsg += " Authenticated as " + clientLabel + "."
		} else {
			welcomeMsg += " Connected as anonymous. Pass ?token=<jwt> to authenticate."
		}
		welcomeMsg += " Send {\"action\":\"subscribe\",\"channel\":\"market.ticker\",\"stock_code\":\"BBCA\"} to start receiving updates."

		client.sendResponse(WSResponse{
			Type:    "connected",
			Message: welcomeMsg,
		})
	}
}
