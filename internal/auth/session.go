package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type SessionValidator struct {
	sfsURL     string
	httpClient *http.Client
}

type sessionResponse struct {
	Username string `json:"username"`
}

func NewSessionValidator(sfsURL string) *SessionValidator {
	return &SessionValidator{
		sfsURL: sfsURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// ValidateSession validates a session cookie by calling SFS /api/session
// Returns the username if valid, empty string if invalid
func (v *SessionValidator) ValidateSession(sessionToken string) (string, error) {
	req, err := http.NewRequest("GET", v.sfsURL+"/api/session", nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.AddCookie(&http.Cookie{
		Name:  "sfs_session",
		Value: sessionToken,
	})

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("call sfs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return "", nil
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("sfs returned status %d", resp.StatusCode)
	}

	var session sessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	return session.Username, nil
}

// GetSessionToken extracts the session token from the request cookie
func GetSessionToken(r *http.Request) string {
	cookie, err := r.Cookie("sfs_session")
	if err != nil {
		return ""
	}
	return cookie.Value
}
