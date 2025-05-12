package http

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/harry-urek/urek/v/internal/logger"
	"github.com/harry-urek/urek/v/internal/models"
	"github.com/harry-urek/urek/v/internal/session"
	"go.uber.org/zap"
)

// Server represents the HTTP server for session management
type Server struct {
	router  *mux.Router
	manager *session.Manager
	log     *zap.Logger
}

// NewServer creates a new HTTP server
func NewServer(manager *session.Manager) *Server {
	s := &Server{
		router:  mux.NewRouter(),
		manager: manager,
		log:     logger.Named("http-server"),
	}

	s.setupRoutes()
	return s
}

// setupRoutes configures the HTTP routes
func (s *Server) setupRoutes() {
	// Health check
	s.router.HandleFunc("/health", s.handleHealthCheck).Methods(http.MethodGet)

	// API routes
	api := s.router.PathPrefix("/api").Subrouter()

	// Session management routes
	sessions := api.PathPrefix("/sessions").Subrouter()
	sessions.HandleFunc("", s.handleCreateSession).Methods(http.MethodPost)
	sessions.HandleFunc("/{id}", s.handleGetSession).Methods(http.MethodGet)
	sessions.HandleFunc("/{id}", s.handleDeleteSession).Methods(http.MethodDelete)
	sessions.HandleFunc("/{id}/validate", s.handleValidateSession).Methods(http.MethodPost)
	sessions.HandleFunc("/user/{userId}", s.handleListUserSessions).Methods(http.MethodGet)

	// Authentication routes
	auth := api.PathPrefix("/auth").Subrouter()
	auth.HandleFunc("/login", s.handleLogin).Methods(http.MethodPost)
	auth.HandleFunc("/logout", s.handleLogout).Methods(http.MethodPost)
	auth.HandleFunc("/refresh", s.handleRefreshToken).Methods(http.MethodPost)

	// OAuth routes
	oauth := auth.PathPrefix("/oauth").Subrouter()
	oauth.HandleFunc("/authorize/{provider}", s.handleOAuthAuthorize).Methods(http.MethodGet)
	oauth.HandleFunc("/callback/{provider}", s.handleOAuthCallback).Methods(http.MethodGet)

	// WebSocket route for session events
	s.router.HandleFunc("/ws/sessions", s.handleWebSocketConnection)
}

// Handler returns the HTTP handler for the server
func (s *Server) Handler() http.Handler {
	return s.router
}

// handleHealthCheck handles health check requests
func (s *Server) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleCreateSession handles session creation requests
func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var req models.CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.log.Warn("Invalid request body", zap.Error(err))
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Set IP and user agent if not provided
	if req.IP == "" {
		req.IP = getClientIP(r)
	}
	if req.UserAgent == "" {
		req.UserAgent = r.UserAgent()
	}

	session, err := s.manager.CreateSession(r.Context(), &req)
	if err != nil {
		s.log.Error("Failed to create session", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "Failed to create session")
		return
	}

	// Generate JWT for the session
	token, err := s.manager.GenerateJWT(session, req.Claims["role"])
	if err != nil {
		s.log.Error("Failed to generate JWT", zap.Error(err))
	}

	response := models.SessionResponse{
		Valid:     true,
		SessionID: session.ID,
		UserID:    session.UserID,
		Claims:    make(map[string]string),
		ExpiresAt: session.ExpiresAt,
	}

	// Copy existing claims if any
	if session.Claims != nil {
		for k, v := range session.Claims {
			response.Claims[k] = v
		}
	}

	// Set session cookie
	s.manager.SetSessionCookie(w, session.ID)

	// If JWT was generated, include it in the response
	if token != "" {
		response.Claims["jwt"] = token
	}

	writeJSON(w, http.StatusCreated, response)
}

// handleGetSession handles session retrieval requests
func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["id"]

	session, err := s.manager.ValidateSession(r.Context(), sessionID)
	if err != nil {
		s.log.Debug("Session validation failed", zap.Error(err), zap.String("session_id", sessionID))
		writeError(w, http.StatusNotFound, "Session not found or invalid")
		return
	}

	writeJSON(w, http.StatusOK, session)
}

// handleDeleteSession handles session deletion requests
func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["id"]

	if err := s.manager.DeleteSession(r.Context(), sessionID); err != nil {
		s.log.Error("Failed to delete session", zap.Error(err), zap.String("session_id", sessionID))
		writeError(w, http.StatusInternalServerError, "Failed to delete session")
		return
	}

	// If this was the session identified by the cookie, remove it
	if cookie, err := s.manager.GetSessionFromCookie(r); err == nil && cookie == sessionID {
		s.manager.DeleteSessionCookie(w)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleValidateSession handles session validation requests
func (s *Server) handleValidateSession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["id"]

	var req models.SessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// If body is empty, use just the session ID from path
		req.SessionID = sessionID
	} else if req.SessionID == "" {
		req.SessionID = sessionID
	}

	session, err := s.manager.ValidateSession(r.Context(), req.SessionID)
	response := models.SessionResponse{}

	if err != nil {
		s.log.Debug("Session validation failed", zap.Error(err), zap.String("session_id", req.SessionID))
		response.Valid = false
		response.Error = err.Error()
	} else {
		response.Valid = true
		response.SessionID = session.ID
		response.UserID = session.UserID
		response.Claims = session.Claims
		response.ExpiresAt = session.ExpiresAt
	}

	writeJSON(w, http.StatusOK, response)
}

// handleListUserSessions handles requests to list a user's sessions
func (s *Server) handleListUserSessions(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["userId"]

	sessions, err := s.manager.ListSessionsByUser(r.Context(), userID)
	if err != nil {
		s.log.Error("Failed to list user sessions", zap.Error(err), zap.String("user_id", userID))
		writeError(w, http.StatusInternalServerError, "Failed to list sessions")
		return
	}

	writeJSON(w, http.StatusOK, sessions)
}

// handleLogin handles login requests
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID   string            `json:"user_id"`
		Password string            `json:"password,omitempty"`
		Claims   map[string]string `json:"claims,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.log.Warn("Invalid login request", zap.Error(err))
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// This is a simplified login - in a real system, you would validate credentials
	// against a user store or authentication service

	sessionReq := &models.CreateSessionRequest{
		UserID:    req.UserID,
		ClientID:  getClientIP(r),
		Claims:    req.Claims,
		IP:        getClientIP(r),
		UserAgent: r.UserAgent(),
	}

	session, err := s.manager.CreateSession(r.Context(), sessionReq)
	if err != nil {
		s.log.Error("Failed to create session during login", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "Login failed")
		return
	}

	// Generate JWT
	role := ""
	if req.Claims != nil {
		role = req.Claims["role"]
	}
	token, err := s.manager.GenerateJWT(session, role)
	if err != nil {
		s.log.Error("Failed to generate JWT", zap.Error(err))
	}

	// Set session cookie
	s.manager.SetSessionCookie(w, session.ID)

	response := map[string]interface{}{
		"session_id": session.ID,
		"user_id":    session.UserID,
		"expires_at": session.ExpiresAt,
	}

	if token != "" {
		response["token"] = token
	}

	writeJSON(w, http.StatusOK, response)
}

// handleLogout handles logout requests
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	sessionID, err := s.manager.GetSessionFromCookie(r)
	if err != nil {
		s.log.Debug("No session cookie in logout request", zap.Error(err))
		writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
		return
	}

	if err := s.manager.InvalidateSession(r.Context(), sessionID); err != nil {
		s.log.Error("Failed to invalidate session during logout", zap.Error(err))
	}

	// Always delete the cookie, even if session invalidation failed
	s.manager.DeleteSessionCookie(w)

	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}

// handleRefreshToken handles token refresh requests
func (s *Server) handleRefreshToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Get JWT claims (without full validation)
	claims, err := s.manager.ValidateJWT(req.Token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Invalid token")
		return
	}

	// Validate the session exists and is still valid
	session, err := s.manager.ValidateSession(r.Context(), claims.SessionID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Invalid session")
		return
	}

	// Generate a new token
	newToken, err := s.manager.GenerateJWT(session, claims.Role)
	if err != nil {
		s.log.Error("Failed to refresh token", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "Failed to refresh token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"token":      newToken,
		"user_id":    session.UserID,
		"session_id": session.ID,
	})
}

// handleOAuthAuthorize handles OAuth authorization requests
func (s *Server) handleOAuthAuthorize(w http.ResponseWriter, r *http.Request) {
	// This is a placeholder - actual implementation would redirect to OAuth provider
	writeError(w, http.StatusNotImplemented, "OAuth not implemented yet")
}

// handleOAuthCallback handles OAuth callback requests
func (s *Server) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	// This is a placeholder - actual implementation would process the OAuth callback
	writeError(w, http.StatusNotImplemented, "OAuth not implemented yet")
}

// handleWebSocketConnection handles WebSocket connection requests
func (s *Server) handleWebSocketConnection(w http.ResponseWriter, r *http.Request) {
	// Simply delegate to the WebSocket server in the session manager
	wsServer := s.manager.GetWebSocketServer()
	if wsServer != nil {
		// Access the WebSocket server's handleConnection method
		wsServer.HandleConnection(w, r)
	} else {
		s.log.Error("WebSocket server is not initialized")
		writeError(w, http.StatusInternalServerError, "WebSocket server is not available")
	}
}

// Helper functions

// writeJSON writes a JSON response
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logger.Named("http").Error("Failed to encode JSON response", zap.Error(err))
	}
}

// writeError writes an error response
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// getClientIP gets the client IP address
func getClientIP(r *http.Request) string {
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = r.Header.Get("X-Real-IP")
	}
	if ip == "" {
		ip = r.RemoteAddr
	}
	return ip
}
