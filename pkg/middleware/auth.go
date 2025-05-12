package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/harry-urek/urek/v/internal/logger"
	"github.com/harry-urek/urek/v/internal/session"
	"go.uber.org/zap"
)

// Auth errors
var (
	ErrNoToken       = errors.New("no auth token provided")
	ErrInvalidToken  = errors.New("invalid auth token")
	ErrInvalidBearer = errors.New("invalid bearer token format")
)

// JWTAuth is middleware for JWT authentication
func JWTAuth(manager *session.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log := logger.Named("middleware.auth")

			// Get token from header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				log.Debug("No authorization header")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Check bearer format
			if !strings.HasPrefix(authHeader, "Bearer ") {
				log.Debug("Invalid authorization format", zap.String("auth_header", authHeader))
				http.Error(w, "Invalid authorization format", http.StatusUnauthorized)
				return
			}

			// Extract token
			token := strings.TrimPrefix(authHeader, "Bearer ")

			// Validate token
			claims, err := manager.ValidateJWT(token)
			if err != nil {
				log.Debug("Invalid token", zap.Error(err))
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Check that session is valid
			_, err = manager.ValidateSession(r.Context(), claims.SessionID)
			if err != nil {
				log.Debug("Invalid session", zap.Error(err), zap.String("session_id", claims.SessionID))
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Set claims in context
			ctx := context.WithValue(r.Context(), "user_id", claims.UserID)
			ctx = context.WithValue(ctx, "session_id", claims.SessionID)
			ctx = context.WithValue(ctx, "role", claims.Role)

			// Call the next handler with the authenticated context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// CookieAuth is middleware for cookie-based authentication
func CookieAuth(manager *session.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log := logger.Named("middleware.auth")

			// Get session ID from cookie
			sessionID, err := manager.GetSessionFromCookie(r)
			if err != nil {
				log.Debug("No session cookie", zap.Error(err))
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Validate session
			session, err := manager.ValidateSession(r.Context(), sessionID)
			if err != nil {
				log.Debug("Invalid session", zap.Error(err), zap.String("session_id", sessionID))
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Set session info in context
			ctx := context.WithValue(r.Context(), "user_id", session.UserID)
			ctx = context.WithValue(ctx, "session_id", session.ID)

			// Call the next handler with the authenticated context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserID extracts the user ID from the request context
func GetUserID(r *http.Request) (string, bool) {
	userID, ok := r.Context().Value("user_id").(string)
	return userID, ok
}

// GetSessionID extracts the session ID from the request context
func GetSessionID(r *http.Request) (string, bool) {
	sessionID, ok := r.Context().Value("session_id").(string)
	return sessionID, ok
}

// GetRole extracts the role from the request context
func GetRole(r *http.Request) (string, bool) {
	role, ok := r.Context().Value("role").(string)
	return role, ok
}
