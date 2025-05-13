package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/harry-urek/urek/v/internal/config"
	"github.com/harry-urek/urek/v/internal/logger"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

var (
	// ErrOAuthDisabled is returned when OAuth is disabled in config
	ErrOAuthDisabled = errors.New("oauth is disabled")
	// ErrInvalidCode is returned when the authorization code is invalid
	ErrInvalidCode = errors.New("invalid authorization code")
	// ErrFailedToFetchUserInfo is returned when user info fetch fails
	ErrFailedToFetchUserInfo = errors.New("failed to fetch user info")
)

// Manager handles OAuth authentication flows
type Manager struct {
	config       *config.OAuthConfig
	oauthConfig  *oauth2.Config
	httpClient   *http.Client
	log          *zap.Logger
	userInfoFunc func(ctx context.Context, client *http.Client, userInfoURL string) (map[string]interface{}, error)
}

// UserInfo contains user information fetched from OAuth provider
type UserInfo struct {
	ID            string
	Email         string
	Name          string
	EmailVerified bool
	RawData       map[string]interface{}
}

// New creates a new OAuth manager
func New(cfg *config.OAuthConfig) *Manager {
	if !cfg.Enabled {
		return &Manager{
			config: cfg,
			log:    logger.Named("oauth"),
		}
	}

	oauthConfig := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  cfg.AuthURL,
			TokenURL: cfg.TokenURL,
		},
		Scopes: []string{"profile", "email"},
	}

	return &Manager{
		config:      cfg,
		oauthConfig: oauthConfig,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		log:          logger.Named("oauth"),
		userInfoFunc: fetchUserInfo,
	}
}

// Enabled returns whether OAuth is enabled
func (m *Manager) Enabled() bool {
	return m.config != nil && m.config.Enabled
}

// GetAuthURL returns the OAuth authorization URL
func (m *Manager) GetAuthURL(state string) (string, error) {
	if !m.Enabled() {
		return "", ErrOAuthDisabled
	}
	return m.oauthConfig.AuthCodeURL(state), nil
}

// Exchange exchanges the authorization code for an access token
func (m *Manager) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	if !m.Enabled() {
		return nil, ErrOAuthDisabled
	}

	token, err := m.oauthConfig.Exchange(ctx, code)
	if err != nil {
		m.log.Error("Failed to exchange code for token", zap.Error(err))
		return nil, ErrInvalidCode
	}
	return token, nil
}

// GetUserInfo fetches the user info from the OAuth provider
func (m *Manager) GetUserInfo(ctx context.Context, token *oauth2.Token) (*UserInfo, error) {
	if !m.Enabled() {
		return nil, ErrOAuthDisabled
	}

	client := m.oauthConfig.Client(ctx, token)
	data, err := m.userInfoFunc(ctx, client, m.config.UserInfoURL)
	if err != nil {
		m.log.Error("Failed to fetch user info", zap.Error(err))
		return nil, ErrFailedToFetchUserInfo
	}

	// Map the user info - this is generic and might need customization per provider
	userInfo := &UserInfo{
		RawData: data,
	}

	if id, ok := data["id"]; ok {
		userInfo.ID, _ = id.(string)
	} else if id, ok := data["sub"]; ok {
		userInfo.ID, _ = id.(string)
	}

	if email, ok := data["email"]; ok {
		userInfo.Email, _ = email.(string)
	}

	if name, ok := data["name"]; ok {
		userInfo.Name, _ = name.(string)
	}

	if verified, ok := data["email_verified"]; ok {
		userInfo.EmailVerified, _ = verified.(bool)
	}

	return userInfo, nil
}

// Helper function to fetch user info from provider
func fetchUserInfo(ctx context.Context, client *http.Client, userInfoURL string) (map[string]interface{}, error) {
	resp, err := client.Get(userInfoURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("non-200 response from user info endpoint")
	}

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	return data, nil
}

// HandleCallback processes the OAuth callback
func (m *Manager) HandleCallback(w http.ResponseWriter, r *http.Request) (*UserInfo, error) {
	if !m.Enabled() {
		return nil, ErrOAuthDisabled
	}

	// Extract the code from the request
	code := r.URL.Query().Get("code")
	if code == "" {
		return nil, errors.New("no code in request")
	}

	// Exchange code for token
	token, err := m.Exchange(r.Context(), code)
	if err != nil {
		return nil, err
	}

	// Get user info
	userInfo, err := m.GetUserInfo(r.Context(), token)
	if err != nil {
		return nil, err
	}

	return userInfo, nil
}
