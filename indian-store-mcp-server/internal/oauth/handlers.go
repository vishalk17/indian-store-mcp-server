package oauth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"sync"
)

type OAuthHandler struct {
	oryClient *OryClient
	states    map[string]bool // Simple state storage (use Redis in production)
	stateMux  sync.RWMutex
}

func NewOAuthHandler(oryClient *OryClient) *OAuthHandler {
	return &OAuthHandler{
		oryClient: oryClient,
		states:    make(map[string]bool),
	}
}

// HandleAuthorize redirects to Ory authorization endpoint
func (h *OAuthHandler) HandleAuthorize(w http.ResponseWriter, r *http.Request) {
	// Generate state for CSRF protection
	state := generateRandomString(32)
	
	h.stateMux.Lock()
	h.states[state] = true
	h.stateMux.Unlock()

	// Get authorization URL from Ory
	authURL := h.oryClient.GetAuthorizationURL(state)
	
	log.Printf("Redirecting to Ory authorization: %s", authURL)
	http.Redirect(w, r, authURL, http.StatusFound)
}

// HandleCallback handles OAuth callback from Ory
func (h *OAuthHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	errorParam := r.URL.Query().Get("error")

	// Check for errors
	if errorParam != "" {
		errorDesc := r.URL.Query().Get("error_description")
		log.Printf("OAuth error: %s - %s", errorParam, errorDesc)
		http.Error(w, "OAuth authorization failed: "+errorParam, http.StatusBadRequest)
		return
	}

	// Validate state
	h.stateMux.Lock()
	valid := h.states[state]
	delete(h.states, state)
	h.stateMux.Unlock()

	if !valid {
		log.Println("Invalid state parameter")
		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		return
	}

	// Exchange code for token
	tokenResp, err := h.oryClient.ExchangeCodeForToken(code)
	if err != nil {
		log.Printf("Failed to exchange code for token: %v", err)
		http.Error(w, "Failed to obtain access token", http.StatusInternalServerError)
		return
	}

	// Return tokens as JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"access_token":  tokenResp.AccessToken,
		"refresh_token": tokenResp.RefreshToken,
		"token_type":    tokenResp.TokenType,
		"expires_in":    tokenResp.ExpiresIn,
		"message":       "Authentication successful! Use the access_token for API requests.",
	})
}

// HandleToken handles token endpoint (for refresh tokens, etc.)
func (h *OAuthHandler) HandleToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	grantType := r.FormValue("grant_type")

	switch grantType {
	case "refresh_token":
		refreshToken := r.FormValue("refresh_token")
		if refreshToken == "" {
			http.Error(w, "refresh_token is required", http.StatusBadRequest)
			return
		}

		tokenResp, err := h.oryClient.RefreshToken(refreshToken)
		if err != nil {
			log.Printf("Failed to refresh token: %v", err)
			http.Error(w, "Failed to refresh token", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tokenResp)

	default:
		http.Error(w, "Unsupported grant_type", http.StatusBadRequest)
	}
}

// HandleUserInfo returns user information
func (h *OAuthHandler) HandleUserInfo(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "Authorization header required", http.StatusUnauthorized)
		return
	}

	// Extract token
	token := ""
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		token = authHeader[7:]
	} else {
		http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
		return
	}

	// Get user info from Ory
	userInfo, err := h.oryClient.GetUserInfo(token)
	if err != nil {
		log.Printf("Failed to get user info: %v", err)
		http.Error(w, "Failed to get user info", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userInfo)
}

// HandleIntrospect validates and returns token information
func (h *OAuthHandler) HandleIntrospect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	token := r.FormValue("token")
	if token == "" {
		http.Error(w, "token is required", http.StatusBadRequest)
		return
	}

	introResp, err := h.oryClient.IntrospectToken(token)
	if err != nil {
		log.Printf("Failed to introspect token: %v", err)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"active": false})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(introResp)
}

// generateRandomString generates a random string for state parameter
func generateRandomString(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)[:length]
}
