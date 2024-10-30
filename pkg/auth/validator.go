package auth

import (
	"crypto/rand"
	"encoding/base64"

	"github.com/harry-urek/urek/v/internal/session"
)

type Validator struct {
	sauce   []byte
	manager *session.Manager
}

func NewValidator(sKey []byte, manager *session.Manager) *Validator {
	return &Validator{
		sauce:   sKey,
		manager: manager,
	}
}

func (v *Validator) GenrateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil

}
