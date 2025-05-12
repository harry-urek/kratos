package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/harry-urek/urek/v/internal/logger"
	"github.com/harry-urek/urek/v/internal/models"
	"go.uber.org/zap"
)

// Store errors
var (
	ErrSessionNotFound = errors.New("session not found")
	ErrInvalidSession  = errors.New("invalid session data")
	ErrStorageFailure  = errors.New("session storage failure")
)

type Store interface {
	SaveSession(ctx context.Context, session *models.Session) error
	GetSession(ctx context.Context, sessionID string) (*models.Session, error)
	DeleteSession(ctx context.Context, sessionID string) error
	ListSessionsByUserID(ctx context.Context, userID string) ([]*models.Session, error)
}

type RedisStore struct {
	client *redis.Client
	log    *zap.Logger
	prefix string
}

func NewRedisStore(redisURL string, prefix string) (*RedisStore, error) {
	if redisURL == "" {
		return nil, errors.New("empty Redis URL provided")
	}

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	client := redis.NewClient(opt)

	// Test connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	if prefix == "" {
		prefix = "session:"
	}

	store := &RedisStore{
		client: client,
		log:    logger.Named("redis-store"),
		prefix: prefix,
	}

	return store, nil
}

// SaveSession saves a session to Redis
func (s *RedisStore) SaveSession(ctx context.Context, session *models.Session) error {
	data, err := json.Marshal(session)
	if err != nil {
		s.log.Error("Failed to marshal session", zap.Error(err), zap.String("session_id", session.ID))
		return ErrInvalidSession
	}

	expiry := time.Until(session.ExpiresAt)
	if expiry <= 0 {
		// Session already expired
		return ErrInvalidSession
	}

	key := s.prefix + session.ID
	if err := s.client.Set(ctx, key, data, expiry).Err(); err != nil {
		s.log.Error("Failed to save session", zap.Error(err), zap.String("session_id", session.ID))
		return ErrStorageFailure
	}

	// Add to user's session set
	userKey := s.prefix + "user:" + session.UserID
	if err := s.client.SAdd(ctx, userKey, session.ID).Err(); err != nil {
		s.log.Warn("Failed to add session to user set", zap.Error(err),
			zap.String("session_id", session.ID), zap.String("user_id", session.UserID))
	}

	s.log.Debug("Session saved", zap.String("session_id", session.ID))
	return nil
}

// GetSession retrieves a session from Redis
func (s *RedisStore) GetSession(ctx context.Context, sessionID string) (*models.Session, error) {
	key := s.prefix + sessionID
	data, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, ErrSessionNotFound
		}
		s.log.Error("Failed to retrieve session", zap.Error(err), zap.String("session_id", sessionID))
		return nil, ErrStorageFailure
	}

	var session models.Session
	if err := json.Unmarshal(data, &session); err != nil {
		s.log.Error("Failed to unmarshal session", zap.Error(err), zap.String("session_id", sessionID))
		return nil, ErrInvalidSession
	}

	return &session, nil
}

// DeleteSession deletes a session from Redis
func (s *RedisStore) DeleteSession(ctx context.Context, sessionID string) error {
	// First get the session to know which user it belongs to
	session, err := s.GetSession(ctx, sessionID)
	if err != nil {
		if err == ErrSessionNotFound {
			return nil // Already gone
		}
		return err
	}

	key := s.prefix + sessionID
	if err := s.client.Del(ctx, key).Err(); err != nil {
		s.log.Error("Failed to delete session", zap.Error(err), zap.String("session_id", sessionID))
		return ErrStorageFailure
	}

	// Remove from user's session set
	userKey := s.prefix + "user:" + session.UserID
	if err := s.client.SRem(ctx, userKey, sessionID).Err(); err != nil {
		s.log.Warn("Failed to remove session from user set", zap.Error(err),
			zap.String("session_id", sessionID), zap.String("user_id", session.UserID))
	}

	s.log.Debug("Session deleted", zap.String("session_id", sessionID))
	return nil
}

// ListSessionsByUserID lists all sessions for a user
func (s *RedisStore) ListSessionsByUserID(ctx context.Context, userID string) ([]*models.Session, error) {
	userKey := s.prefix + "user:" + userID

	// Get the session IDs for this user
	sessionIDs, err := s.client.SMembers(ctx, userKey).Result()
	if err != nil {
		if err == redis.Nil {
			return []*models.Session{}, nil
		}
		s.log.Error("Failed to list user sessions", zap.Error(err), zap.String("user_id", userID))
		return nil, ErrStorageFailure
	}

	// If no sessions, return an empty slice
	if len(sessionIDs) == 0 {
		return []*models.Session{}, nil
	}

	// Get all the sessions
	sessions := make([]*models.Session, 0, len(sessionIDs))
	for _, id := range sessionIDs {
		session, err := s.GetSession(ctx, id)
		if err == nil {
			sessions = append(sessions, session)
		} else if err != ErrSessionNotFound {
			// Log non-not-found errors but continue
			s.log.Warn("Failed to get session in list operation",
				zap.Error(err), zap.String("session_id", id), zap.String("user_id", userID))
		}
	}

	return sessions, nil
}
