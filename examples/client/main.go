package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

const (
	serverURL = "http://localhost:8080"
)

// Session response structure
type SessionResponse struct {
	Valid     bool              `json:"valid"`
	SessionID string            `json:"session_id,omitempty"`
	UserID    string            `json:"user_id,omitempty"`
	Claims    map[string]string `json:"claims,omitempty"`
	ExpiresAt time.Time         `json:"expires_at,omitempty"`
	Error     string            `json:"error,omitempty"`
}

// Token response structure
type TokenResponse struct {
	SessionID string    `json:"session_id"`
	UserID    string    `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
	Token     string    `json:"token"`
}

func main() {
	fmt.Println("Kratos Session Management Client Example")
	fmt.Println("---------------------------------------")

	// 1. Check server health
	fmt.Println("1. Checking server health...")
	health, err := getServerHealth()
	if err != nil {
		log.Fatalf("Server health check failed: %v", err)
	}
	fmt.Printf("   Server health: %s\n\n", health["status"])

	// 2. Login and create session
	fmt.Println("2. Logging in and creating session...")
	tokenResp, err := login("user123", "admin")
	if err != nil {
		log.Fatalf("Login failed: %v", err)
	}
	fmt.Printf("   Session created with ID: %s\n", tokenResp.SessionID)
	fmt.Printf("   JWT Token: %s\n\n", tokenResp.Token)

	// 3. Validate session
	fmt.Println("3. Validating session...")
	session, err := validateSession(tokenResp.SessionID)
	if err != nil {
		log.Fatalf("Session validation failed: %v", err)
	}
	fmt.Printf("   Session valid: %v\n", session.Valid)
	fmt.Printf("   User ID: %s\n", session.UserID)
	fmt.Printf("   Expires at: %s\n\n", session.ExpiresAt.Format(time.RFC3339))

	// 4. Make an authenticated request
	fmt.Println("4. Making authenticated request...")
	userSessions, err := getUserSessions(tokenResp.UserID, tokenResp.Token)
	if err != nil {
		log.Fatalf("Failed to get user sessions: %v", err)
	}
	fmt.Printf("   Retrieved %d sessions for user\n\n", len(userSessions))

	// 5. Logout
	fmt.Println("5. Logging out...")
	err = logout(tokenResp.Token)
	if err != nil {
		log.Fatalf("Logout failed: %v", err)
	}
	fmt.Println("   Successfully logged out")

	fmt.Println("\nClient example completed successfully!")
}

// getServerHealth checks if the server is healthy
func getServerHealth() (map[string]string, error) {
	resp, err := http.Get(serverURL + "/health")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("health check failed with status: %d", resp.StatusCode)
	}

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// login creates a new session
func login(userID, role string) (*TokenResponse, error) {
	// Create login request
	reqBody, err := json.Marshal(map[string]interface{}{
		"user_id": userID,
		"claims": map[string]string{
			"role": role,
		},
	})
	if err != nil {
		return nil, err
	}

	// Send login request
	resp, err := http.Post(serverURL+"/api/auth/login", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("login failed with status %d: %s", resp.StatusCode, body)
	}

	// Parse response
	var tokenResp TokenResponse
	err = json.NewDecoder(resp.Body).Decode(&tokenResp)
	if err != nil {
		return nil, err
	}

	return &tokenResp, nil
}

// validateSession validates a session
func validateSession(sessionID string) (*SessionResponse, error) {
	// Create request
	req, err := http.NewRequest("POST", serverURL+"/api/sessions/"+sessionID+"/validate", nil)
	if err != nil {
		return nil, err
	}

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("session validation failed with status %d: %s", resp.StatusCode, body)
	}

	// Parse response
	var sessionResp SessionResponse
	err = json.NewDecoder(resp.Body).Decode(&sessionResp)
	if err != nil {
		return nil, err
	}

	return &sessionResp, nil
}

// getUserSessions gets all sessions for a user
func getUserSessions(userID, token string) ([]SessionResponse, error) {
	// Create request
	req, err := http.NewRequest("GET", serverURL+"/api/sessions/user/"+userID, nil)
	if err != nil {
		return nil, err
	}

	// Add authorization header
	req.Header.Add("Authorization", "Bearer "+token)

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("get user sessions failed with status %d: %s", resp.StatusCode, body)
	}

	// Parse response
	var sessions []SessionResponse
	err = json.NewDecoder(resp.Body).Decode(&sessions)
	if err != nil {
		return nil, err
	}

	return sessions, nil
}

// logout invalidates the session
func logout(token string) error {
	// Create request
	reqBody, err := json.Marshal(map[string]interface{}{
		"token": token,
	})
	if err != nil {
		return err
	}

	// Create request
	req, err := http.NewRequest("POST", serverURL+"/api/auth/logout", bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}

	// Add authorization header
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("Content-Type", "application/json")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("logout failed with status %d: %s", resp.StatusCode, body)
	}

	return nil
}
