package oauth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"indian-store-mcp-server/internal/config"
)

type RegistrationHandler struct {
	config    *config.Config
	oryClient *OryClient
}

type ClientRegistrationRequest struct {
	ClientName    string   `json:"client_name,omitempty"`
	RedirectURIs  []string `json:"redirect_uris"`
	GrantTypes    []string `json:"grant_types,omitempty"`
	ResponseTypes []string `json:"response_types,omitempty"`
	Scope         string   `json:"scope,omitempty"`
	TokenEndpointAuthMethod string `json:"token_endpoint_auth_method,omitempty"`
}

type ClientRegistrationResponse struct {
	ClientID                string   `json:"client_id"`
	ClientSecret            string   `json:"client_secret,omitempty"`
	ClientName              string   `json:"client_name,omitempty"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	Scope                   string   `json:"scope,omitempty"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method,omitempty"`
	ClientSecretExpiresAt   int64    `json:"client_secret_expires_at"`
}

func NewRegistrationHandler(cfg *config.Config, oryClient *OryClient) *RegistrationHandler {
	return &RegistrationHandler{
		config:    cfg,
		oryClient: oryClient,
	}
}

// HandleRegister implements RFC 7591 Dynamic Client Registration
func (h *RegistrationHandler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ClientRegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Failed to decode registration request: %v", err)
		jsonError(w, "invalid_request", "Invalid JSON in request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if len(req.RedirectURIs) == 0 {
		jsonError(w, "invalid_redirect_uri", "At least one redirect_uri is required", http.StatusBadRequest)
		return
	}

	// Set defaults
	if len(req.GrantTypes) == 0 {
		req.GrantTypes = []string{"authorization_code", "refresh_token"}
	}
	if len(req.ResponseTypes) == 0 {
		req.ResponseTypes = []string{"code"}
	}
	if req.Scope == "" {
		req.Scope = "openid offline_access email profile"
	}
	if req.TokenEndpointAuthMethod == "" {
		req.TokenEndpointAuthMethod = "client_secret_basic"
	}

	// Forward request to Ory Hydra admin API
	oryRequest := map[string]interface{}{
		"client_name":                req.ClientName,
		"redirect_uris":              req.RedirectURIs,
		"grant_types":                req.GrantTypes,
		"response_types":             req.ResponseTypes,
		"scope":                      req.Scope,
		"token_endpoint_auth_method": req.TokenEndpointAuthMethod,
	}

	jsonData, err := json.Marshal(oryRequest)
	if err != nil {
		log.Printf("Failed to marshal Ory request: %v", err)
		jsonError(w, "server_error", "Internal server error", http.StatusInternalServerError)
		return
	}

	// Call Ory Hydra admin API to create client
	adminURL := fmt.Sprintf("%s/admin/clients", h.config.OryAdminURL)
	if adminURL == "/admin/clients" {
		// Fallback if OryAdminURL not set
		log.Println("WARNING: ORY_ADMIN_URL not configured, using default")
		adminURL = "http://ory-hydra-admin.default.svc.cluster.local:4445/admin/clients"
	}

	httpReq, err := http.NewRequest("POST", adminURL, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Failed to create Ory request: %v", err)
		jsonError(w, "server_error", "Internal server error", http.StatusInternalServerError)
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("Failed to call Ory Hydra: %v", err)
		jsonError(w, "server_error", "Failed to register client with OAuth provider", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		log.Printf("Ory Hydra returned error: %d - %s", resp.StatusCode, string(body))
		jsonError(w, "server_error", "Failed to register client", http.StatusInternalServerError)
		return
	}

	// Parse Ory response
	var oryResponse map[string]interface{}
	if err := json.Unmarshal(body, &oryResponse); err != nil {
		log.Printf("Failed to parse Ory response: %v", err)
		jsonError(w, "server_error", "Internal server error", http.StatusInternalServerError)
		return
	}

	// Build RFC 7591 compliant response
	response := ClientRegistrationResponse{
		ClientID:                getStringFromMap(oryResponse, "client_id"),
		ClientSecret:            getStringFromMap(oryResponse, "client_secret"),
		ClientName:              req.ClientName,
		RedirectURIs:            req.RedirectURIs,
		GrantTypes:              req.GrantTypes,
		ResponseTypes:           req.ResponseTypes,
		Scope:                   req.Scope,
		TokenEndpointAuthMethod: req.TokenEndpointAuthMethod,
		ClientSecretExpiresAt:   0, // 0 means it doesn't expire
	}

	log.Printf("Successfully registered client: %s (name: %s)", response.ClientID, response.ClientName)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func getStringFromMap(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

func jsonError(w http.ResponseWriter, errorCode, errorDesc string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error":             errorCode,
		"error_description": errorDesc,
	})
}
