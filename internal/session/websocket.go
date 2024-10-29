package session

import (
	"context"
	"encoding/json"
	"log"
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

func (ws *wsServer) handleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := ws.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection : %v", err)
		return
	}
	defer conn.Close()

	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		conn.WriteJSON(map[string]string{"error": "no session ID provided"})
		return
	}

	// add connection sync it
	ws.sessMu.Lock()
	ws.sessions[sessionID] = conn
	ws.sessMu.Unlock()

	// Handle ws handle conn concurently
	go ws.readSess(sessionID, conn)
}

func (ws *wsServer) readSess(sessID string, conn *websocket.Conn) {
	defer func() {
		// delete/remove session after its is handled or closed
		ws.sessMu.Lock()
		delete(ws.sessions, sessID)
		ws.sessMu.Unlock()
		conn.Close()
	}()

	for {
		var packet wsPacket
		if err := conn.ReadJSON(&packet); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error reading message: %v", err)
			}
			break

		}
		switch packet.Type {
		case "validate":
			session, err := ws.manager.ValidateSession(context.Background(), sessID)
			if err != nil {
				conn.WriteJSON(map[string]string{"error": err.Error()})
				continue
			}
			conn.WriteJSON(map[string]interface{}{
				"type":  "validate-result",
				"valid": session != nil,
			})
		}
	}
}
