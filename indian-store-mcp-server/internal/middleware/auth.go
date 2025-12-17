package middleware

import (
	"log"
	"net/http"
	"strings"

	"indian-store-mcp-server/internal/oauth"
)

type AuthMiddleware struct {
	oryClient *oauth.OryClient
}

func NewAuthMiddleware(oryClient *oauth.OryClient) *AuthMiddleware {
	return &AuthMiddleware{
		oryClient: oryClient,
	}
}

// RequireAuth validates Ory token before allowing access
func (m *AuthMiddleware) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			log.Println("Missing Authorization header")
			http.Error(w, "Unauthorized: Missing token", http.StatusUnauthorized)
			return
		}

		// Extract Bearer token
		token := ""
		if len(authHeader) > 7 && strings.HasPrefix(authHeader, "Bearer ") {
			token = authHeader[7:]
		} else {
			log.Println("Invalid Authorization header format")
			http.Error(w, "Unauthorized: Invalid token format", http.StatusUnauthorized)
			return
		}

		// Validate token with Ory
		introResp, err := m.oryClient.IntrospectToken(token)
		if err != nil {
			log.Printf("Token introspection failed: %v", err)
			http.Error(w, "Unauthorized: Invalid token", http.StatusUnauthorized)
			return
		}

		if !introResp.Active {
			log.Println("Token is not active")
			http.Error(w, "Unauthorized: Token expired or invalid", http.StatusUnauthorized)
			return
		}

		log.Printf("Authenticated user: %s (%s)", introResp.Email, introResp.Sub)
		
		// Token is valid, proceed to handler
		next(w, r)
	}
}

// CORS middleware for handling cross-origin requests
func CORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "3600")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}
