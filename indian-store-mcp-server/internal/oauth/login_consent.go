package oauth

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"indian-store-mcp-server/internal/users"
)

// Session represents a user session
type Session struct {
	Email     string
	CreatedAt time.Time
}

// LoginConsentHandler handles login and consent flows
type LoginConsentHandler struct {
	oryClient  *OryClient
	userStore  *users.UserStore
	sessions   map[string]*Session
	sessionMu  sync.RWMutex
}

func NewLoginConsentHandler(oryClient *OryClient, userStore *users.UserStore) *LoginConsentHandler {
	return &LoginConsentHandler{
		oryClient: oryClient,
		userStore: userStore,
		sessions:  make(map[string]*Session),
	}
}

// generateSessionID creates a random session ID
func (h *LoginConsentHandler) generateSessionID() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// getSession retrieves a session by cookie
func (h *LoginConsentHandler) getSession(r *http.Request) (*Session, bool) {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		return nil, false
	}

	h.sessionMu.RLock()
	defer h.sessionMu.RUnlock()

	session, exists := h.sessions[cookie.Value]
	if !exists {
		return nil, false
	}

	// Check if session is expired (24 hours)
	if time.Since(session.CreatedAt) > 24*time.Hour {
		return nil, false
	}

	return session, true
}

// createSession creates a new session
func (h *LoginConsentHandler) createSession(w http.ResponseWriter, email string) string {
	sessionID := h.generateSessionID()

	h.sessionMu.Lock()
	h.sessions[sessionID] = &Session{
		Email:     email,
		CreatedAt: time.Now(),
	}
	h.sessionMu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400, // 24 hours
	})

	return sessionID
}

// HandleLogin handles the login page
func (h *LoginConsentHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	challenge := r.URL.Query().Get("login_challenge")
	if challenge == "" {
		http.Error(w, "Missing login_challenge", http.StatusBadRequest)
		return
	}

	log.Printf("Login challenge received: %s", challenge)

	// Check if user already has a session
	if session, exists := h.getSession(r); exists {
		// User is already logged in, accept the login automatically
		log.Printf("User %s already logged in, auto-accepting", session.Email)
		h.acceptLogin(w, r, challenge, session.Email)
		return
	}

	// Handle POST request (login form submission)
	if r.Method == "POST" {
		r.ParseForm()
		email := r.FormValue("email")
		password := r.FormValue("password")

		// Authenticate user
		user, err := h.userStore.Authenticate(email, password)
		if err != nil {
			log.Printf("Authentication failed for %s: %v", email, err)
			h.showLoginForm(w, challenge, "Invalid email or password")
			return
		}

		log.Printf("User %s authenticated successfully", user.Email)

		// Create session
		h.createSession(w, user.Email)

		// Accept the login
		h.acceptLogin(w, r, challenge, user.Email)
		return
	}

	// Show login form
	h.showLoginForm(w, challenge, "")
}

// showLoginForm displays the login form
func (h *LoginConsentHandler) showLoginForm(w http.ResponseWriter, challenge, errorMsg string) {
	tmpl := `<!DOCTYPE html>
<html>
<head>
    <title>Indian Store MCP - Login</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            margin: 0;
        }
        .login-container {
            background: white;
            padding: 40px;
            border-radius: 10px;
            box-shadow: 0 10px 40px rgba(0,0,0,0.2);
            width: 100%;
            max-width: 400px;
        }
        h1 {
            color: #333;
            margin: 0 0 10px 0;
            font-size: 24px;
        }
        .subtitle {
            color: #666;
            margin: 0 0 30px 0;
            font-size: 14px;
        }
        .form-group {
            margin-bottom: 20px;
        }
        label {
            display: block;
            margin-bottom: 5px;
            color: #555;
            font-size: 14px;
            font-weight: 500;
        }
        input[type="email"],
        input[type="password"] {
            width: 100%;
            padding: 12px;
            border: 2px solid #e0e0e0;
            border-radius: 5px;
            font-size: 14px;
            box-sizing: border-box;
            transition: border-color 0.3s;
        }
        input[type="email"]:focus,
        input[type="password"]:focus {
            outline: none;
            border-color: #667eea;
        }
        button {
            width: 100%;
            padding: 12px;
            background: #667eea;
            color: white;
            border: none;
            border-radius: 5px;
            font-size: 16px;
            font-weight: 600;
            cursor: pointer;
            transition: background 0.3s;
        }
        button:hover {
            background: #5568d3;
        }
        .error {
            background: #fee;
            border: 1px solid #fcc;
            color: #c00;
            padding: 12px;
            border-radius: 5px;
            margin-bottom: 20px;
            font-size: 14px;
        }
        .info {
            background: #e3f2fd;
            border: 1px solid #90caf9;
            color: #1976d2;
            padding: 12px;
            border-radius: 5px;
            margin-top: 20px;
            font-size: 12px;
        }
    </style>
</head>
<body>
    <div class="login-container">
        <h1>Indian Store MCP</h1>
        <p class="subtitle">Sign in to authorize access</p>
        
        {{if .Error}}
        <div class="error">{{.Error}}</div>
        {{end}}
        
        <form method="POST">
            <div class="form-group">
                <label for="email">Email</label>
                <input type="email" id="email" name="email" required autofocus>
            </div>
            
            <div class="form-group">
                <label for="password">Password</label>
                <input type="password" id="password" name="password" required>
            </div>
            
            <button type="submit">Sign In</button>
        </form>
        
        <div class="info">
            <strong>Demo credentials:</strong><br>
            Email: admin@indian-store.com<br>
            Password: admin123
        </div>
    </div>
</body>
</html>`

	t := template.Must(template.New("login").Parse(tmpl))
	w.Header().Set("Content-Type", "text/html")
	t.Execute(w, map[string]string{
		"Challenge": challenge,
		"Error":     errorMsg,
	})
}

// acceptLogin accepts the login with Hydra
func (h *LoginConsentHandler) acceptLogin(w http.ResponseWriter, r *http.Request, challenge, userEmail string) {
	acceptData := map[string]interface{}{
		"subject":      userEmail,
		"remember":     true,
		"remember_for": 86400, // 24 hours
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

	log.Printf("Login accepted for %s, redirecting to: %s", userEmail, acceptResult.RedirectTo)
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

	// Get user info from session
	user, exists := h.userStore.GetUser(consentInfo.Subject)
	if !exists {
		log.Printf("User not found: %s", consentInfo.Subject)
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}

	// Auto-accept the consent with user information
	acceptData := map[string]interface{}{
		"grant_scope": consentInfo.RequestedScope,
		"grant_access_token_audience": []string{},
		"remember": true,
		"remember_for": 86400, // 24 hours
		"session": map[string]interface{}{
			"id_token": map[string]interface{}{
				"email": user.Email,
				"name":  user.Name,
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
