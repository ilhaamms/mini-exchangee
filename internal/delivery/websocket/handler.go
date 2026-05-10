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
	
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var clientSeq int64

func HandleWebSocket(hub *Hub, jwtConfig middleware.JWTConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		
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

		
		go client.WritePump()
		go client.ReadPump()

		
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
