package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

type SessionEvent struct {
	SID        string            `json:"session_id"`
	UID        string            `json:"user_id"`
	EventType  string            `json:"Event_Type"`
	TimeStamp  time.Time         `json:"timestamp"`
	Metadata   map[string]string `json:"metadata"`
	DeviceInfo map[string]string `json:"device_info"`
	IP         string            `json:"IP_Address"`
}

type SessionAnalytics struct {
	store      *redis.Client
	timeWindow time.Duration
}

func NewSessionAnalytics(store *redis.Client, window time.Duration) *SessionAnalytics {
	return &SessionAnalytics{
		store:      store,
		timeWindow: window,
	}

}

func (sa *SessionAnalytics) TrackEvent(ctx context.Context, event *SessionEvent) error {
	eventKey := fmt.Sprintf("event : %s : %d", event.EventType, event.TimeStamp.Unix())
	eventData, err := json.Marshal(event)
	if err != nil {
		return err

	}

	pipe := sa.store.Pipeline()
	pipe.HSet(ctx, eventKey, event.SID, eventData)
	pipe.Expire(ctx, eventKey, sa.timeWindow)

	// Update user session count
	userKey := fmt.Sprintf("user_sessions:%s", event.UID)
	pipe.Incr(ctx, userKey)

	// Track device statistics
	deviceKey := fmt.Sprintf("device_stats:%s", event.DeviceInfo["platform"])
	pipe.Incr(ctx, deviceKey)

	_, err = pipe.Exec(ctx)
	return err

}
