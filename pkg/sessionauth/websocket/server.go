package websocket

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/harry-urek/urek/v/pkg/sessionauth/auth"
)

type Server struct {
	manager  *auth.Manager
	upgrader websocket.Upgrader

	sessions map[string]*websocket.Conn
	sessMu   sync.RWMutex
}

func NewWebSocketServer(manager *auth.Manager) *Server {
	return &Server{
		manager: manager,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				// Implement origin check logic
				return true
			},
		},
		sessions: make(map[string]*websocket.Conn),
	}
}

func (s *Server) Start(port string) error {
	http.HandleFunc("/ws", s.handleConnection)
	return http.ListenAndServe(port, nil)
}

func (s *Server) handleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}
	defer conn.Close()

	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		conn.WriteJSON(map[string]string{"error": "no session ID provided"})
		return
	}

	s.addSession(sessionID, conn)
	go s.readSession(sessionID, conn)
}

func (s *Server) addSession(sessionID string, conn *websocket.Conn) {
	s.sessMu.Lock()
	defer s.sessMu.Unlock()
	s.sessions[sessionID] = conn
}

func (s *Server) removeSession(sessionID string) {
	s.sessMu.Lock()
	defer s.sessMu.Unlock()
	delete(s.sessions, sessionID)
}

type Packet struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

func (s *Server) readSession(sessionID string, conn *websocket.Conn) {
	defer func() {
		s.removeSession(sessionID)
		conn.Close()
	}()

	for {
		var packet Packet
		if err := conn.ReadJSON(&packet); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Error reading message for session %s: %v", sessionID, err)
			}
			break
		}

		switch packet.Type {
		case "validate":
			s.handleValidate(sessionID, conn)
		default:
			log.Printf("Unknown packet type: %s", packet.Type)
		}
	}
}

func (s *Server) handleValidate(sessionID string, conn *websocket.Conn) {
	session, err := s.manager.ValidateSession(context.Background(), sessionID)
	if err != nil {
		conn.WriteJSON(map[string]string{"error": err.Error()})
		return
	}

	conn.WriteJSON(map[string]interface{}{
		"type":  "validate-result",
		"valid": session != nil,
	})
}
