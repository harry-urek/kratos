package session

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type wsServer struct {
	manager  *Manager
	upgrader websocket.Upgrader

	sessions map[string]*websocket.Conn
	sessMu   sync.RWMutex
}

type wsPacket struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

func NewWebSocketServer(manager *Manager) *wsServer {
	return &wsServer{
		manager: manager,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				// Implement your origin check logic also can add cqrs
				return true
			},
		},
		sessions: make(map[string]*websocket.Conn),
	}
}
