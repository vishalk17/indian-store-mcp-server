# OAuth2 + Social Login Deployment Guide

## Architecture

```
ChatGPT/Claude
    │
    ├─> 1. Dynamic Client Registration: POST /oauth2/register (Hydra)
    │
    └─> 2. Authorization: GET /oauth2/auth?client_id=...&redirect_uri=... (Hydra)
            │
            └─> 3. Hydra redirects to /auth/login (Kratos UI)
                    │
                    └─> 4. User clicks "Sign in with GitHub/Microsoft/Google"
                            │
                            └─> 5. OAuth flow with social provider
                                    │
                                    └─> 6. Kratos receives user email
                                            │
                                            ├─> 7a. Check Postgres: SELECT * FROM identities WHERE email=?
                                            │       │
                                            │       ├─> EXISTS → Continue
                                            │       └─> NOT EXISTS → REJECT (registration disabled)
                                            │
                                            └─> 8. Return to Hydra consent
                                                    │
                                                    └─> 9. Issue JWT access token
                                                            │
                                                            └─> 10. ChatGPT calls /mcp with Bearer token
```

## Components

### 1. **Postgres** (bitnami/postgresql)
**Why:** Stores Kratos identities and Hydra OAuth2 clients/sessions
- Creates 2 databases: `kratos` and `hydra`

### 2. **Ory Kratos** (ory/kratos)
**Why:** Identity management + social login orchestration
- Handles GitHub/Microsoft/Google OAuth
- Stores user identities in Postgres
- **Rejects new users** (registration disabled)

### 3. **Kratos SelfService UI** (ory/kratos-selfservice-ui-node)
**Why:** Login interface showing "Sign in with..." buttons
- User-facing UI at `/auth/*`
- Displays social login options

### 4. **Ory Hydra** (ory/hydra)
**Why:** OAuth2/OIDC authorization server
- Issues JWT access tokens
- Dynamic Client Registration (RFC 7591)
- Token endpoint for ChatGPT/Claude

### 5. **MCP Server** (existing)
**Why:** Resource server protected by OAuth2
- Validates JWT tokens
- Serves tools to authorized clients

## Deployment Steps

### 1. Apply Secrets
```bash
kubectl apply -f k8s/secrets.yaml
```

### 2. Deploy Postgres
```bash
helm repo add bitnami https://charts.bitnami.com/bitnami
helm install postgres bitnami/postgresql \
  --set auth.username=ory \
  --set auth.password=changeme \
  --set auth.database=kratos \
  --set primary.initdb.scripts.init\.sql="CREATE DATABASE hydra\;"
```

### 3. Deploy Ory Kratos
```bash
helm repo add ory https://k8s.ory.sh/helm/charts
helm install kratos ory/kratos -f k8s/kratos-values.yaml
```

### 4. Deploy Kratos SelfService UI
```bash
helm install kratos-ui ory/kratos-selfservice-ui-node -f k8s/kratos-ui-values.yaml
```

### 5. Deploy Ory Hydra
```bash
helm install hydra ory/hydra -f k8s/hydra-values.yaml
```

### 6. Apply Gateway + HTTPRoutes
```bash
kubectl apply -f k8s/gateway.yaml
kubectl apply -f k8s/deployement.yaml
```

### 7. Verify
```bash
kubectl get pods
kubectl get httproutes
curl https://vishalk17.cloudwithme.dev/.well-known/openid-configuration
```

## User Allow/Deny Logic

**Rejection happens in Kratos:**
- `registration.enabled: false` in Kratos config
- When user completes social login, Kratos checks if identity exists
- If email NOT in `identities` table → **login fails**
- If email EXISTS → proceeds to Hydra consent

**To allow a user:**
```bash
# Insert via Kratos admin API
curl -X POST http://kratos-admin:4434/admin/identities \
  -H "Content-Type: application/json" \
  -d '{
    "schema_id": "default",
    "traits": {
      "email": "user@example.com"
    }
  }'
```

## Dynamic Client Registration

**ChatGPT/Claude registers like this:**

```bash
curl -X POST https://vishalk17.cloudwithme.dev/oauth2/clients \
  -H "Content-Type: application/json" \
  -d '{
    "client_name": "ChatGPT MCP Client",
    "redirect_uris": ["https://chatgpt.com/oauth/callback"],
    "grant_types": ["authorization_code", "refresh_token"],
    "response_types": ["code"],
    "scope": "openid email profile",
    "token_endpoint_auth_method": "none"
  }'
```

**Response:**
```json
{
  "client_id": "generated-uuid",
  "client_secret": null,
  "redirect_uris": ["https://chatgpt.com/oauth/callback"],
  "grant_types": ["authorization_code", "refresh_token"]
}
```

## OAuth2 Flow Example

```bash
# 1. Authorize (triggers login)
https://vishalk17.cloudwithme.dev/oauth2/auth?
  client_id=generated-uuid&
  redirect_uri=https://chatgpt.com/oauth/callback&
  response_type=code&
  scope=openid email&
  code_challenge=...&
  code_challenge_method=S256

# 2. User logs in via social provider

# 3. Exchange code for token
curl -X POST https://vishalk17.cloudwithme.dev/oauth2/token \
  -d "grant_type=authorization_code" \
  -d "code=..." \
  -d "redirect_uri=https://chatgpt.com/oauth/callback" \
  -d "client_id=generated-uuid" \
  -d "code_verifier=..."
```

## MCP Server JWT Validation

Update `main.go` with middleware:

```go
import (
    "github.com/golang-jwt/jwt/v5"
    "strings"
)

func jwtMiddleware(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        auth := r.Header.Get("Authorization")
        if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        
        tokenString := strings.TrimPrefix(auth, "Bearer ")
        
        // Parse JWT (public key from Hydra JWKS endpoint)
        token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
            // Fetch JWKS from https://vishalk17.cloudwithme.dev/.well-known/jwks.json
            // Validate signing key
            return publicKey, nil
        })
        
        if err != nil || !token.Valid {
            http.Error(w, "Invalid token", http.StatusUnauthorized)
            return
        }
        
        claims := token.Claims.(jwt.MapClaims)
        if claims["iss"] != "https://vishalk17.cloudwithme.dev/oauth2" {
            http.Error(w, "Invalid issuer", http.StatusUnauthorized)
            return
        }
        
        next(w, r)
    }
}

// In main():
http.HandleFunc("/mcp", jwtMiddleware(server.handleMCPRequest))
```

## Troubleshooting

```bash
# Check Kratos identities
kubectl exec -it deployment/kratos -- kratos list identities --endpoint http://localhost:4434

# Check Hydra clients
kubectl exec -it deployment/hydra -- hydra list clients --endpoint http://localhost:4445

# View logs
kubectl logs -f deployment/kratos
kubectl logs -f deployment/hydra
kubectl logs -f deployment/kratos-selfservice-ui-node
```

## Security Notes

1. **Change secrets in `k8s/secrets.yaml`** - use `openssl rand -hex 16`
2. **Add Google OAuth credentials** if needed
3. **TLS certificate** must be valid for `vishalk17.cloudwithme.dev`
4. **Postgres password** should be strong in production
5. **CORS** is wide open (`*`) - restrict to known origins

## Why Both Kratos + Hydra?

**Kratos:**
- Identity storage (who the user is)
- Social login integration
- User database (Postgres)

**Hydra:**
- OAuth2 token issuance (what the user can access)
- Dynamic Client Registration
- JWT signing

**They work together:**
Hydra delegates authentication to Kratos, then issues tokens after Kratos confirms user identity exists.
