package oauth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"indian-store-mcp-server/internal/config"
)

type OryClient struct {
	config *config.Config
	client *http.Client
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

type UserInfo struct {
	Sub   string `json:"sub"`
	Email string `json:"email,omitempty"`
	Name  string `json:"name,omitempty"`
}

type IntrospectionResponse struct {
	Active bool   `json:"active"`
	Sub    string `json:"sub,omitempty"`
	Email  string `json:"email,omitempty"`
	Scope  string `json:"scope,omitempty"`
	Exp    int64  `json:"exp,omitempty"`
}

func NewOryClient(cfg *config.Config) *OryClient {
	return &OryClient{
		config: cfg,
		client: &http.Client{},
	}
}

// GetAuthorizationURL builds the Ory authorization URL
func (o *OryClient) GetAuthorizationURL(state string) string {
	params := url.Values{}
	params.Add("client_id", o.config.OryClientID)
	params.Add("redirect_uri", o.config.OryCallbackURL)
	params.Add("response_type", "code")
	params.Add("scope", o.config.OryScopes)
	params.Add("state", state)

	authURL := fmt.Sprintf("%s/oauth2/auth?%s", o.config.OryURL, params.Encode())
	return authURL
}

// ExchangeCodeForToken exchanges authorization code for access token
func (o *OryClient) ExchangeCodeForToken(code string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", o.config.OryCallbackURL)

	// Use internal URL if available, otherwise fallback to external
	tokenURL := fmt.Sprintf("%s/oauth2/token", o.config.OryURL)
	if o.config.OryInternalURL != "" {
		tokenURL = fmt.Sprintf("%s/oauth2/token", o.config.OryInternalURL)
	}
	
	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// Use Basic Auth (client_secret_basic) instead of POST body
	req.SetBasicAuth(o.config.OryClientID, o.config.OryClientSecret)

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed: %s - %s", resp.Status, string(body))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	return &tokenResp, nil
}

// IntrospectToken validates a token with Ory
func (o *OryClient) IntrospectToken(token string) (*IntrospectionResponse, error) {
	introspectionURL := o.config.OryIntrospectionURL
	if introspectionURL == "" {
		// Use internal admin URL for server-to-server introspection
		introspectionURL = fmt.Sprintf("%s/admin/oauth2/introspect", o.config.OryAdminURL)
	}

	data := url.Values{}
	data.Set("token", token)

	req, err := http.NewRequest("POST", introspectionURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create introspection request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(o.config.OryClientID, o.config.OryClientSecret)

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to introspect token: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("introspection failed: %s - %s", resp.Status, string(body))
	}

	var introResp IntrospectionResponse
	if err := json.Unmarshal(body, &introResp); err != nil {
		return nil, fmt.Errorf("failed to parse introspection response: %w", err)
	}

	return &introResp, nil
}

// GetUserInfo fetches user information from Ory
func (o *OryClient) GetUserInfo(accessToken string) (*UserInfo, error) {
	userInfoURL := o.config.OryUserInfoURL
	if userInfoURL == "" {
		userInfoURL = fmt.Sprintf("%s/userinfo", o.config.OryURL)
	}

	req, err := http.NewRequest("GET", userInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create userinfo request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo failed: %s - %s", resp.Status, string(body))
	}

	var userInfo UserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, fmt.Errorf("failed to parse userinfo response: %w", err)
	}

	return &userInfo, nil
}

// RefreshToken uses refresh token to get new access token
func (o *OryClient) RefreshToken(refreshToken string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)

	// Use internal URL if available, otherwise fallback to external
	tokenURL := fmt.Sprintf("%s/oauth2/token", o.config.OryURL)
	if o.config.OryInternalURL != "" {
		tokenURL = fmt.Sprintf("%s/oauth2/token", o.config.OryInternalURL)
	}

	req, err := http.NewRequest("POST", tokenURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create refresh request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// Use Basic Auth (client_secret_basic) instead of POST body
	req.SetBasicAuth(o.config.OryClientID, o.config.OryClientSecret)

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token refresh failed: %s - %s", resp.Status, string(body))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse refresh response: %w", err)
	}

	return &tokenResp, nil
}
