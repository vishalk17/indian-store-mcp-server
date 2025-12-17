package oauth

import (
	"bytes"
	"encoding/json"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
)

// LoginConsentHandler handles login and consent flows
type LoginConsentHandler struct {
	oryClient *OryClient
}

func NewLoginConsentHandler(oryClient *OryClient) *LoginConsentHandler {
	return &LoginConsentHandler{
		oryClient: oryClient,
	}
}

// HandleLogin handles the login page
func (h *LoginConsentHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	challenge := r.URL.Query().Get("login_challenge")
	if challenge == "" {
		http.Error(w, "Missing login_challenge", http.StatusBadRequest)
		return
	}

	log.Printf("Login challenge received: %s", challenge)

	// Get login request info from Hydra
	req, err := http.NewRequest("GET", h.oryClient.config.OryAdminURL+"/admin/oauth2/auth/requests/login?login_challenge="+challenge, nil)
	if err != nil {
		log.Printf("Error creating request: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	resp, err := h.oryClient.client.Do(req)
	if err != nil {
		log.Printf("Error getting login request: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Error from Hydra: %s", string(body))
		http.Error(w, "Error communicating with OAuth server", http.StatusInternalServerError)
		return
	}

	// Auto-accept the login for now (no actual user authentication)
	// In production, you'd show a login form here
	acceptData := map[string]interface{}{
		"subject": "indian-store-user", // Default user
		"remember": true,
		"remember_for": 3600,
	}

	acceptBody, _ := json.Marshal(acceptData)
	acceptReq, err := http.NewRequest("PUT",
		h.oryClient.config.OryAdminURL+"/admin/oauth2/auth/requests/login/accept?login_challenge="+challenge,
		bytes.NewReader(acceptBody))
	if err != nil {
		log.Printf("Error creating accept request: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	acceptReq.Header.Set("Content-Type", "application/json")

	acceptResp, err := h.oryClient.client.Do(acceptReq)
	if err != nil {
		log.Printf("Error accepting login: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	defer acceptResp.Body.Close()

	if acceptResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(acceptResp.Body)
		log.Printf("Error accepting login: %s", string(body))
		http.Error(w, "Error completing login", http.StatusInternalServerError)
		return
	}

	var acceptResult struct {
		RedirectTo string `json:"redirect_to"`
	}
	if err := json.NewDecoder(acceptResp.Body).Decode(&acceptResult); err != nil {
		log.Printf("Error decoding accept response: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	log.Printf("Login accepted, redirecting to: %s", acceptResult.RedirectTo)
	http.Redirect(w, r, acceptResult.RedirectTo, http.StatusFound)
}

// HandleConsent handles the consent page
func (h *LoginConsentHandler) HandleConsent(w http.ResponseWriter, r *http.Request) {
	challenge := r.URL.Query().Get("consent_challenge")
	if challenge == "" {
		http.Error(w, "Missing consent_challenge", http.StatusBadRequest)
		return
	}

	log.Printf("Consent challenge received: %s", challenge)

	// Get consent request info from Hydra
	req, err := http.NewRequest("GET", h.oryClient.config.OryAdminURL+"/admin/oauth2/auth/requests/consent?consent_challenge="+challenge, nil)
	if err != nil {
		log.Printf("Error creating request: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	resp, err := h.oryClient.client.Do(req)
	if err != nil {
		log.Printf("Error getting consent request: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Error from Hydra: %s", string(body))
		http.Error(w, "Error communicating with OAuth server", http.StatusInternalServerError)
		return
	}

	var consentInfo struct {
		RequestedScope []string `json:"requested_scope"`
		Subject        string   `json:"subject"`
		Client         struct {
			ClientID string `json:"client_id"`
		} `json:"client"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&consentInfo); err != nil {
		log.Printf("Error decoding consent info: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// Auto-accept the consent
	// In production, you'd show a consent form here
	acceptData := map[string]interface{}{
		"grant_scope": consentInfo.RequestedScope,
		"grant_access_token_audience": []string{},
		"remember": true,
		"remember_for": 3600,
		"session": map[string]interface{}{
			"id_token": map[string]interface{}{
				"email": "user@indian-store.com",
				"name":  "Indian Store User",
			},
		},
	}

	acceptBody, _ := json.Marshal(acceptData)
	acceptReq, err := http.NewRequest("PUT",
		h.oryClient.config.OryAdminURL+"/admin/oauth2/auth/requests/consent/accept?consent_challenge="+challenge,
		bytes.NewReader(acceptBody))
	if err != nil {
		log.Printf("Error creating accept request: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	acceptReq.Header.Set("Content-Type", "application/json")

	acceptResp, err := h.oryClient.client.Do(acceptReq)
	if err != nil {
		log.Printf("Error accepting consent: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	defer acceptResp.Body.Close()

	if acceptResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(acceptResp.Body)
		log.Printf("Error accepting consent: %s", string(body))
		http.Error(w, "Error completing consent", http.StatusInternalServerError)
		return
	}

	var acceptResult struct {
		RedirectTo string `json:"redirect_to"`
	}
	if err := json.NewDecoder(acceptResp.Body).Decode(&acceptResult); err != nil {
		log.Printf("Error decoding accept response: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	log.Printf("Consent accepted, redirecting to: %s", acceptResult.RedirectTo)
	http.Redirect(w, r, acceptResult.RedirectTo, http.StatusFound)
}

// HandleError handles OAuth error redirects
func (h *LoginConsentHandler) HandleError(w http.ResponseWriter, r *http.Request) {
	errorCode := r.URL.Query().Get("error")
	errorDesc := r.URL.Query().Get("error_description")

	if errorDesc == "" {
		errorDesc = "An OAuth error occurred"
	}

	// URL decode the error description
	decodedDesc, err := url.QueryUnescape(errorDesc)
	if err == nil {
		errorDesc = decodedDesc
	}

	log.Printf("OAuth error: %s - %s", errorCode, errorDesc)

	// Return a simple HTML page
	tmpl := `<!DOCTYPE html>
<html>
<head>
    <title>OAuth Error</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 600px; margin: 50px auto; padding: 20px; }
        .error { background: #fee; border: 1px solid #fcc; padding: 20px; border-radius: 5px; }
        h1 { color: #c00; }
    </style>
</head>
<body>
    <div class="error">
        <h1>OAuth Error</h1>
        <p><strong>Error:</strong> {{.ErrorCode}}</p>
        <p><strong>Description:</strong> {{.ErrorDescription}}</p>
        <p><a href="/">Return to home</a></p>
    </div>
</body>
</html>`

	t := template.Must(template.New("error").Parse(tmpl))
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusBadRequest)
	t.Execute(w, map[string]string{
		"ErrorCode":        errorCode,
		"ErrorDescription": errorDesc,
	})
}
