package session

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/go-redis/redis/v8"
)

type store struct {
	client *redis.Client
}

func NewStore(redisURL string) (*store, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	client := redis.NewClient(opt)
	return &store{client: client}, nil

}

func (s *store) getSession(ctx context.Context, sessID string) (*session, error) {
	data, err := s.client.Get(ctx, "session"+sessID).Bytes()

	if err != nil {
		if err == redis.Nil {
			return nil, errors.New("Session not found 404")
		}
		return nil, err
	}
	var sess session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, err
	}
	return &sess, nil
}
