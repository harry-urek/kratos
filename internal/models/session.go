package models

import (
	"time"

	"github.com/google/uuid"
)

// Session represents a user session
type Session struct {
	ID        string            `json:"id"`
	UserID    string            `json:"user_id"`
	ClientID  string            `json:"client_id"`
	Claims    map[string]string `json:"claims"`
	CreatedAt time.Time         `json:"created_at"`
	ExpiresAt time.Time         `json:"expires_at"`
	LastSeen  time.Time         `json:"last_seen"`
	IP        string            `json:"ip,omitempty"`
	UserAgent string            `json:"user_agent,omitempty"`
	Active    bool              `json:"active"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// creates a new session
func NewSession(userID, clientID string, claims map[string]string, duration time.Duration) *Session {
	now := time.Now()
	return &Session{
		ID:        uuid.New().String(),
		UserID:    userID,
		ClientID:  clientID,
		Claims:    claims,
		CreatedAt: now,
		ExpiresAt: now.Add(duration),
		LastSeen:  now,
		Active:    true,
		Metadata:  make(map[string]string),
	}
}

// Expire , Valid , UpdateLastSeen , Extend , Invalidate , AddMetadata , GetMetadata
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

func (s *Session) IsValid() bool {
	return !s.IsExpired() && s.Active
}

func (s *Session) UpdateLastSeen() {
	s.LastSeen = time.Now()
}

func (s *Session) Extend(duration time.Duration) {
	s.ExpiresAt = time.Now().Add(duration)
}

func (s *Session) Invalidate() {
	s.Active = false
}

func (s *Session) AddMetadata(key, value string) {
	if s.Metadata == nil {
		s.Metadata = make(map[string]string)
	}
	s.Metadata[key] = value
}

func (s *Session) GetMetadata(key string) (string, bool) {
	if s.Metadata == nil {
		return "", false
	}
	val, ok := s.Metadata[key]
	return val, ok
}

type SessionRequest struct {
	SessionID string            `json:"session_id"`
	ClientID  string            `json:"client_id"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type CreateSessionRequest struct {
	UserID    string            `json:"user_id"`
	ClientID  string            `json:"client_id"`
	Claims    map[string]string `json:"claims,omitempty"`
	IP        string            `json:"ip,omitempty"`
	UserAgent string            `json:"user_agent,omitempty"`
	Duration  time.Duration     `json:"duration,omitempty"`
}

type SessionResponse struct {
	Valid     bool              `json:"valid"`
	SessionID string            `json:"session_id,omitempty"`
	UserID    string            `json:"user_id,omitempty"`
	Claims    map[string]string `json:"claims,omitempty"`
	ExpiresAt time.Time         `json:"expires_at,omitempty"`
	Error     string            `json:"error,omitempty"`
}
