package session

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/harry-urek/urek/v/internal/cookie"
)

type Manager struct {
	store       *redis.Client
	cookieMangr *cookie.Manager

	wsServer *wsServer
}

type session struct {
	ID       string
	clientID string
	claims   map[string]string
	endTime  time.Time
}

func NewManager(redisURL string) (*Manager, error) {
	//Init everything - i.e - store, cookie , ws
	return nil, nil

}

func (m *Manager) CreateSession(ctx context.Context, req *CreateSessionRequest) (*session, error) {
	// Create new Session with new cookieMangr
	return nil, nil
}

func (m *Manager) ValidateSession(ctx context.Context, sessID string) (*session, error) {
	// Validate using gRPC protocol
	return nil, nil

}
