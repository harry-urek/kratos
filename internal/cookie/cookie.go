package cookie

import (
	"net/http"

	"github.com/harry-urek/urek/v/internal/logger"
	"go.uber.org/zap"
)

// Manager handles cookie operations
type Manager struct {
	domain     string
	secure     bool
	httpOnly   bool
	sameSite   http.SameSite
	path       string
	maxAge     int
	log        *zap.Logger
	cookieName string
}

type Config struct {
	Domain     string
	Secure     bool
	HTTPOnly   bool
	SameSite   http.SameSite
	Path       string
	MaxAge     int
	CookieName string
}

// DefaultConfig returns default cookie configuration
func DefaultConfig() Config {
	return Config{
		Domain:     "",
		Secure:     true,
		HTTPOnly:   true,
		SameSite:   http.SameSiteStrictMode,
		Path:       "/",
		MaxAge:     86400, // 24 hours
		CookieName: "kratos_session",
	}
}

func New(cfg Config) *Manager {
	return &Manager{
		domain:     cfg.Domain,
		secure:     cfg.Secure,
		httpOnly:   cfg.HTTPOnly,
		sameSite:   cfg.SameSite,
		path:       cfg.Path,
		maxAge:     cfg.MaxAge,
		log:        logger.Named("cookie"),
		cookieName: cfg.CookieName,
	}
}

// Set sets a cookie with the given name and value
func (m *Manager) Set(w http.ResponseWriter, name, value string) {
	cookie := &http.Cookie{
		Name:     name,
		Value:    value,
		Domain:   m.domain,
		Path:     m.path,
		MaxAge:   m.maxAge,
		Secure:   m.secure,
		HttpOnly: m.httpOnly,
		SameSite: m.sameSite,
	}
	http.SetCookie(w, cookie)
	m.log.Debug("Cookie set", zap.String("name", name))
}

// session cookie
func (m *Manager) SetSession(w http.ResponseWriter, value string) {
	m.Set(w, m.cookieName, value)
}

// cookie with the given name
func (m *Manager) Get(r *http.Request, name string) (string, error) {
	cookie, err := r.Cookie(name)
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}

func (m *Manager) GetSession(r *http.Request) (string, error) {
	return m.Get(r, m.cookieName)
}

func (m *Manager) Delete(w http.ResponseWriter, name string) {
	cookie := &http.Cookie{
		Name:     name,
		Value:    "",
		Domain:   m.domain,
		Path:     m.path,
		MaxAge:   -1,
		Secure:   m.secure,
		HttpOnly: m.httpOnly,
		SameSite: m.sameSite,
	}
	http.SetCookie(w, cookie)
	m.log.Debug("Cookie deleted", zap.String("name", name))
}

func (m *Manager) DeleteSession(w http.ResponseWriter) {
	m.Delete(w, m.cookieName)
}

func (m *Manager) Refresh(w http.ResponseWriter, r *http.Request, name string) error {
	value, err := m.Get(r, name)
	if err != nil {
		return err
	}
	m.Set(w, name, value)
	return nil
}

// RefreshSession refreshes the session cookie
func (m *Manager) RefreshSession(w http.ResponseWriter, r *http.Request) error {
	return m.Refresh(w, r, m.cookieName)
}
