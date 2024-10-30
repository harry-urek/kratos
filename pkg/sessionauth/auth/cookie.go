package auth

import (
	"net/http"
	"time"
)

type Manager struct {
	secure   bool
	domain   string
	sameSite http.SameSite
}

func NewManager(domain string, secure bool) *Manager {
	return &Manager{
		secure:   secure,
		domain:   domain,
		sameSite: http.SameSiteLaxMode,
	}
}

func (m *Manager) SetSessionCookie(w http.ResponseWriter, sessionID string, expires time.Time) {
	cookie := &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Domain:   m.domain,
		Path:     "/",
		Expires:  expires,
		Secure:   m.secure,
		HttpOnly: true,
		SameSite: m.sameSite,
	}
	http.SetCookie(w, cookie)
}
