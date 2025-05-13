package jwt

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/harry-urek/urek/v/internal/config"
	"github.com/harry-urek/urek/v/internal/logger"
	"go.uber.org/zap"
)

var (
	// ErrInvalidToken is returned when the token is invalid
	ErrInvalidToken = errors.New("invalid token")
	// ErrExpiredToken is returned when the token is expired
	ErrExpiredToken = errors.New("token expired")
	// ErrInvalidSigningMethod is returned when the signing method is invalid
	ErrInvalidSigningMethod = errors.New("invalid signing method")
)

// Manager handles JWT token generation and validation
type Manager struct {
	secret        string
	expirySeconds time.Duration
	log           *zap.Logger
}

// Claims represents the JWT claims structure
type Claims struct {
	jwt.RegisteredClaims
	SessionID string            `json:"sid,omitempty"`
	UserID    string            `json:"uid,omitempty"`
	Role      string            `json:"role,omitempty"`
	Custom    map[string]string `json:"custom,omitempty"`
}

// New creates a new JWT manager
func New(cfg *config.AuthConfig) *Manager {
	return &Manager{
		secret:        cfg.JWTSecret,
		expirySeconds: cfg.JWTExpiryDuration,
		log:           logger.Named("jwt"),
	}
}

// GenerateToken creates a new JWT token with the given claims
func (m *Manager) GenerateToken(sessionID, userID, role string, custom map[string]string) (string, error) {
	if m.secret == "" {
		m.log.Error("JWT secret is not configured")
		return "", fmt.Errorf("jwt secret is not configured")
	}

	if sessionID == "" {
		m.log.Error("Session ID is required for JWT token")
		return "", fmt.Errorf("session ID is required")
	}

	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(m.expirySeconds)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "kratos-session",
			Subject:   userID,
			ID:        sessionID, // Use the session ID as the JWT ID for extra security
		},
		SessionID: sessionID,
		UserID:    userID,
		Role:      role,
	}

	// Only set custom claims if they exist
	if custom != nil {
		claims.Custom = custom
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(m.secret))
	if err != nil {
		m.log.Error("Failed to sign token", zap.Error(err))
		return "", err
	}

	m.log.Debug("Token generated successfully",
		zap.String("user_id", userID),
		zap.String("session_id", sessionID),
		zap.Time("expires_at", now.Add(m.expirySeconds)))

	return tokenString, nil
}

// ValidateToken validates a JWT token and returns the claims
func (m *Manager) ValidateToken(tokenString string) (*Claims, error) {
	if tokenString == "" {
		return nil, ErrInvalidToken
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			m.log.Warn("Invalid signing method", zap.String("method", token.Method.Alg()))
			return nil, ErrInvalidSigningMethod
		}

		// Validate alg is specifically HMAC-SHA256
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			m.log.Warn("Unexpected signing method", zap.String("expected", jwt.SigningMethodHS256.Alg()),
				zap.String("actual", token.Method.Alg()))
			return nil, ErrInvalidSigningMethod
		}

		return []byte(m.secret), nil
	})

	if err != nil {
		// With jwt/v5, the error handling changed
		if errors.Is(err, jwt.ErrTokenExpired) || errors.Is(err, jwt.ErrTokenNotValidYet) {
			m.log.Debug("Token expired or not valid yet")
			return nil, ErrExpiredToken
		}
		m.log.Debug("Invalid token", zap.Error(err))
		return nil, ErrInvalidToken
	}

	if !token.Valid {
		m.log.Debug("Token marked as invalid")
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		m.log.Debug("Failed to extract claims")
		return nil, ErrInvalidToken
	}

	// Additional validation on claims if needed
	if claims.SessionID == "" || claims.UserID == "" {
		m.log.Debug("Missing required claims",
			zap.String("session_id", claims.SessionID),
			zap.String("user_id", claims.UserID))
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// RefreshToken creates a new token with the same claims but a new expiry time
func (m *Manager) RefreshToken(tokenString string) (string, error) {
	// First try to extract claims without validation to handle expired tokens
	extractedClaims, err := m.ExtractClaims(tokenString)
	if err != nil {
		return "", fmt.Errorf("cannot extract claims from token: %w", err)
	}

	// Then validate token to check other validations (signature, etc.)
	_, err = m.ValidateToken(tokenString)
	// Allow expired token for refresh, but not other errors
	if err != nil && err != ErrExpiredToken {
		return "", fmt.Errorf("cannot refresh invalid token: %w", err)
	}

	// Create a new token with the same claims but new expiry
	return m.GenerateToken(extractedClaims.SessionID, extractedClaims.UserID, extractedClaims.Role, extractedClaims.Custom)
}

// ExtractClaims extracts claims from a token without validation (useful for debugging)
func (m *Manager) ExtractClaims(tokenString string) (*Claims, error) {
	if tokenString == "" {
		m.log.Debug("Empty token string provided to ExtractClaims")
		return nil, ErrInvalidToken
	}

	// In jwt/v5, we need to use ParseUnverified directly without setting SkipClaimsValidation
	token, _, err := jwt.NewParser().ParseUnverified(tokenString, &Claims{})
	if err != nil {
		m.log.Debug("Failed to parse unverified token", zap.Error(err))
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		m.log.Debug("Failed to extract claims from unverified token")
		return nil, ErrInvalidToken
	}

	// Log the extracted claims for debugging
	m.log.Debug("Successfully extracted claims from unverified token",
		zap.String("session_id", claims.SessionID),
		zap.String("user_id", claims.UserID),
		zap.String("role", claims.Role))

	if !claims.ExpiresAt.IsZero() {
		m.log.Debug("Token expiry info", zap.Time("expires_at", claims.ExpiresAt.Time))
	}

	return claims, nil
}
