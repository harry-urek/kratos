package session

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/harry-urek/urek/v/internal/auth/jwt"
	"github.com/harry-urek/urek/v/internal/config"
	"github.com/harry-urek/urek/v/internal/cookie"
	"github.com/harry-urek/urek/v/internal/logger"
	"github.com/harry-urek/urek/v/internal/models"
	"go.uber.org/zap"
)

// Manager errors
var (
	ErrInvalidSessionID = errors.New("invalid session ID")
	ErrSessionExpired   = errors.New("session expired")
	ErrSessionInactive  = errors.New("session inactive")
	ErrMissingUserID    = errors.New("missing user ID")
)

// Manager manages sessions
type Manager struct {
	store           Store
	cookieManager   *cookie.Manager
	jwtManager      *jwt.Manager
	sessionDuration time.Duration
	log             *zap.Logger
	wsServer        *wsServer
}

// NewManager creates a new session manager
func NewManager(cfg *config.Config) (*Manager, error) {
	// Initialize Redis store
	store, err := NewRedisStore(cfg.Redis.URL, "")
	if err != nil {
		return nil, err
	}

	// Initialize cookie manager
	cookieConfig := cookie.DefaultConfig()
	cookieConfig.MaxAge = int(cfg.Auth.SessionExpiryDuration.Seconds())
	cookieConfig.Secure = true // Set based on environment if needed
	cookieManager := cookie.New(cookieConfig)

	// Initialize JWT manager
	jwtManager := jwt.New(&cfg.Auth)

	manager := &Manager{
		store:           store,
		cookieManager:   cookieManager,
		jwtManager:      jwtManager,
		sessionDuration: cfg.Auth.SessionExpiryDuration,
		log:             logger.Named("session-manager"),
	}

	// Initialize WebSocket server
	manager.wsServer = NewWebSocketServer(manager)

	return manager, nil
}

// CreateSession creates a new session
func (m *Manager) CreateSession(ctx context.Context, req *models.CreateSessionRequest) (*models.Session, error) {
	if req.UserID == "" {
		return nil, ErrMissingUserID
	}

	duration := m.sessionDuration
	if req.Duration > 0 {
		duration = req.Duration
	}

	// Create the session object
	session := models.NewSession(req.UserID, req.ClientID, req.Claims, duration)

	// Add additional info if available
	if req.IP != "" {
		session.IP = req.IP
	}

	if req.UserAgent != "" {
		session.UserAgent = req.UserAgent
	}

	// Save the session to the store
	if err := m.store.SaveSession(ctx, session); err != nil {
		m.log.Error("Failed to save session", zap.Error(err), zap.String("user_id", req.UserID))
		return nil, err
	}

	m.log.Info("Session created",
		zap.String("session_id", session.ID),
		zap.String("user_id", req.UserID),
		zap.String("client_id", req.ClientID))

	return session, nil
}

// ValidateSession validates a session
func (m *Manager) ValidateSession(ctx context.Context, sessionID string) (*models.Session, error) {
	if sessionID == "" {
		return nil, ErrInvalidSessionID
	}

	// Get the session from the store
	session, err := m.store.GetSession(ctx, sessionID)
	if err != nil {
		if err == ErrSessionNotFound {
			m.log.Debug("Session not found", zap.String("session_id", sessionID))
		} else {
			m.log.Error("Failed to get session", zap.Error(err), zap.String("session_id", sessionID))
		}
		return nil, err
	}

	// Check if the session is valid
	if session.IsExpired() {
		m.log.Debug("Session expired", zap.String("session_id", sessionID))
		return nil, ErrSessionExpired
	}

	if !session.Active {
		m.log.Debug("Session inactive", zap.String("session_id", sessionID))
		return nil, ErrSessionInactive
	}

	// Update last seen time
	session.UpdateLastSeen()
	err = m.store.SaveSession(ctx, session)
	if err != nil {
		m.log.Warn("Failed to update session last seen time", zap.Error(err), zap.String("session_id", sessionID))
	}

	return session, nil
}

// InvalidateSession invalidates a session
func (m *Manager) InvalidateSession(ctx context.Context, sessionID string) error {
	session, err := m.store.GetSession(ctx, sessionID)
	if err != nil {
		if err == ErrSessionNotFound {
			return nil // Already gone
		}
		return err
	}

	// Invalidate the session
	session.Invalidate()
	if err := m.store.SaveSession(ctx, session); err != nil {
		return err
	}

	m.log.Info("Session invalidated", zap.String("session_id", sessionID), zap.String("user_id", session.UserID))
	return nil
}

// DeleteSession completely removes a session
func (m *Manager) DeleteSession(ctx context.Context, sessionID string) error {
	err := m.store.DeleteSession(ctx, sessionID)
	if err != nil {
		m.log.Error("Failed to delete session", zap.Error(err), zap.String("session_id", sessionID))
		return err
	}

	m.log.Info("Session deleted", zap.String("session_id", sessionID))
	return nil
}

// ListSessionsByUser lists all sessions for a user
func (m *Manager) ListSessionsByUser(ctx context.Context, userID string) ([]*models.Session, error) {
	sessions, err := m.store.ListSessionsByUserID(ctx, userID)
	if err != nil {
		m.log.Error("Failed to list user sessions", zap.Error(err), zap.String("user_id", userID))
		return nil, err
	}
	return sessions, nil
}

// GetWebSocketServer returns the WebSocket server
func (m *Manager) GetWebSocketServer() *wsServer {
	return m.wsServer
}

// GenerateJWT generates a JWT token for a session
func (m *Manager) GenerateJWT(session *models.Session, role string) (string, error) {
	return m.jwtManager.GenerateToken(session.ID, session.UserID, role, session.Claims)
}

// ValidateJWT validates a JWT token
func (m *Manager) ValidateJWT(tokenString string) (*jwt.Claims, error) {
	return m.jwtManager.ValidateToken(tokenString)
}

// SetSessionCookie sets the session cookie
func (m *Manager) SetSessionCookie(w http.ResponseWriter, sessionID string) {
	m.cookieManager.SetSession(w, sessionID)
}

// GetSessionFromCookie gets the session ID from the cookie
func (m *Manager) GetSessionFromCookie(r *http.Request) (string, error) {
	return m.cookieManager.GetSession(r)
}

// DeleteSessionCookie deletes the session cookie
func (m *Manager) DeleteSessionCookie(w http.ResponseWriter) {
	m.cookieManager.DeleteSession(w)
}
