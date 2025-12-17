# Indian Store MCP Server with Ory Hydra OAuth

A production-ready Model Context Protocol (MCP) server with external OAuth 2.0 authentication powered by Ory Hydra. This implementation separates authentication concerns: **your MCP server handles user authentication**, while **Ory Hydra handles OAuth token management**.

## 📖 Table of Contents

- [Architecture Overview](#architecture-overview)
- [What We Implemented](#what-we-implemented)
- [How It Works](#how-it-works)
- [Directory Structure](#directory-structure)
- [Components Deep Dive](#components-deep-dive)
- [Security Model](#security-model)
- [Installation](#installation)
- [User Management](#user-management)
- [Testing](#testing)
- [Troubleshooting](#troubleshooting)

---

## 🏗️ Architecture Overview

### High-Level Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                      MCP CLIENTS                                 │
│              (ChatGPT, Claude, Desktop Apps)                     │
└───────────────────────────┬──────────────────────────────────────┘
                            │
                            │ HTTPS
                            ▼
┌──────────────────────────────────────────────────────────────────┐
│                    GATEWAY API (Kubernetes)                      │
│  Routes:                                                         │
│  • /.well-known/* → MCP Server (OAuth Discovery)               │
│  • /oauth2/* → Ory Hydra (OAuth Endpoints)                     │
│  • /oauth/register → MCP Server (Client Registration)          │
│  • /login, /consent → MCP Server (Auth Handlers)               │
│  • /mcp → MCP Server (Protected API)                           │
└────────────────┬────────────────┬────────────────────────────────┘
                 │                │
                 │                │
     ┌───────────▼───────┐    ┌──▼─────────────────────────┐
     │   MCP SERVER      │    │    ORY HYDRA               │
     │   (Port 8080)     │◄───┤    (OAuth Provider)        │
     │                   │    │    Public: 4444            │
     │ Responsibilities: │    │    Admin: 4445             │
     │ • User Auth       │    │                            │
     │ • Login UI        │    │  Responsibilities:         │
     │ • Consent         │    │  • OAuth Protocol          │
     │ • Client Reg      │    │  • Token Issuance          │
     │ • MCP Protocol    │    │  • Token Validation        │
     │ • Token Validate  │    │  • Client Storage          │
     └──────┬────────────┘    └───────┬────────────────────┘
            │                         │
            │                         │
            ▼                         ▼
     ┌────────────────────────────────────────┐
     │         PostgreSQL Database            │
     │  • users (MCP Server)                  │
     │  • hydra_* tables (Ory Hydra)          │
     │    - clients                           │
     │    - access_tokens                     │
     │    - refresh_tokens                    │
     │    - authorization_codes               │
     └────────────────────────────────────────┘
```

### Key Principle

**YOUR MCP SERVER = Authentication Provider**  
**ORY HYDRA = OAuth Token Manager**

- You control WHO can login (user database)
- You verify passwords
- Ory trusts YOUR authentication decision
- Ory handles OAuth complexity

---

## 🎯 What We Implemented

### 1. User Authentication System (`internal/users/`)

**File**: `users.go`

**What it does**:
- Stores users in PostgreSQL
- Hashes passwords with bcrypt (cost 10)
- Authenticates users (email + password verification)
- Manages user CRUD operations

**Key Functions**:
```go
NewUserStore(databaseURL) → Connects to PostgreSQL, creates tables
AddUser(email, password, name) → Adds user with hashed password
Authenticate(email, password) → Verifies credentials
GetUser(email) → Retrieves user info
ListUsers() → Returns all users (no passwords)
DeleteUser(email) → Removes user
```

**Security**:
- Passwords never stored in plaintext
- bcrypt prevents rainbow table attacks
- SQL injection protection via parameterized queries

---

### 2. OAuth Integration Layer (`internal/oauth/`)

#### 2a. Ory HTTP Client (`ory_client.go`)

**What it does**: Communicates with Ory Hydra APIs

**Key Functions**:
```go
IntrospectToken(token) → Validates access token with Ory
GetAuthorizationURL(state) → Builds OAuth authorize URL
ExchangeCodeForToken(code) → Exchanges auth code for tokens
RefreshToken(refreshToken) → Gets new access token
GetUserInfo(accessToken) → Fetches user details
```

**Important**: Uses internal Kubernetes URLs for server-to-server calls:
- External: `https://domain.com/ory` (for browser redirects)
- Internal: `http://ory-hydra-public.default.svc.cluster.local:4444` (for token exchange)
- Admin: `http://ory-hydra-admin.default.svc.cluster.local:4445` (for introspection)

#### 2b. Dynamic Client Registration (`registration.go`)

**What it does**: Implements RFC 7591 - clients register themselves

**Flow**:
```
1. ChatGPT calls: POST /oauth/register
2. MCP Server validates request
3. MCP Server forwards to Ory Admin API: POST /admin/clients
4. Ory creates client in PostgreSQL
5. Returns client_id + client_secret to ChatGPT
```

**Why needed**: ChatGPT/Claude don't have pre-configured credentials

#### 2c. Login & Consent Handlers (`login_consent.go`)

**What it does**: Handles Ory's login and consent redirects

**Login Flow**:
```
1. Ory redirects to: /login?login_challenge=xyz
2. Check if user has session cookie
   ├─> YES → Auto-approve
   └─> NO → Show login form
3. User submits email + password
4. Call userStore.Authenticate(email, password)
5. If valid:
   └─> Call Ory Admin API: PUT /admin/oauth2/auth/requests/login/accept
       Body: {"subject": "user@example.com"}
6. Ory trusts us: "This user is legit"
7. Redirect to consent
```

**Consent Flow**:
```
1. Ory redirects to: /consent?consent_challenge=abc
2. Get user info from subject
3. Auto-approve consent
4. Call Ory Admin API: PUT /admin/oauth2/auth/requests/consent/accept
   Body: {
     "grant_scope": ["openid", "email"],
     "session": {"id_token": {"email": "...", "name": "..."}}
   }
5. Ory issues authorization code
6. Redirect back to client with code
```

**Session Management**:
- 24-hour session cookies
- HttpOnly, Secure, SameSite=Lax
- Random 64-character session IDs
- In-memory storage (can be moved to Redis)

#### 2d. OAuth Handlers (`handlers.go`)

**What it does**: Helper functions for OAuth flows (not heavily used in current architecture)

---

### 3. Authentication Middleware (`internal/middleware/auth.go`)

**What it does**: Protects `/mcp` endpoint

**Flow**:
```
1. Extract Bearer token from Authorization header
2. Call oryClient.IntrospectToken(token)
3. Ory Admin API: POST /admin/oauth2/introspect
4. Ory checks PostgreSQL: Is token valid?
5. If active=true → Allow request
6. If active=false → Return 401 Unauthorized
```

**Applied to**: `/mcp` endpoint (every MCP protocol request)

---

### 4. Configuration Management (`internal/config/config.go`)

**What it does**: Loads configuration from environment variables

**Key Variables**:
```go
ORY_URL              // External URL (browser redirects)
ORY_INTERNAL_URL     // Internal URL (token exchange)
ORY_ADMIN_URL        // Admin API (introspection)
DATABASE_URL         // PostgreSQL connection string
PORT                 // Server port (8080)
```

**Validation**: Fails fast if required vars missing

---

### 5. Main Server (`main.go`)

**What it does**: Wires everything together

**Routes**:
```go
// OAuth Discovery
GET /.well-known/oauth-authorization-server → OAuth discovery metadata

// OAuth Flows
POST /oauth/register → Dynamic client registration
GET  /login → Login form (or auto-approve if session exists)
POST /login → Process login credentials
GET  /consent → Consent screen (auto-approve)
GET  /oauth/authorize → Redirect to /oauth2/auth (compatibility)

// MCP Protocol
POST /mcp → Protected MCP endpoint (requires Bearer token)

// Health
GET /health → Health check
```

**Initialization Order**:
1. Load config
2. Initialize Ory client
3. Connect to PostgreSQL (user store)
4. Create handlers (registration, login/consent, auth middleware)
5. Register routes
6. Start HTTP server

---

## 🔄 How It Works: Complete OAuth Flow

### Phase 1: Client Discovery

```
ChatGPT → GET /.well-known/oauth-authorization-server
MCP Server → Returns:
{
  "authorization_endpoint": "https://domain.com/oauth2/auth",
  "token_endpoint": "https://domain.com/oauth2/token",
  "registration_endpoint": "https://domain.com/oauth/register",
  ...
}
```

### Phase 2: Dynamic Client Registration

```
ChatGPT → POST /oauth/register
{
  "client_name": "ChatGPT",
  "redirect_uris": ["https://chatgpt.com/callback"],
  "grant_types": ["authorization_code", "refresh_token"],
  "response_types": ["code"],
  "scope": "openid offline_access email profile"
}

MCP Server → Ory Admin API: POST /admin/clients
Ory → Creates client in PostgreSQL
Ory → Returns client_id + client_secret
MCP Server → Returns to ChatGPT
```

**Storage**: Client stored in PostgreSQL `hydra_client` table

### Phase 3: Authorization Request

```
ChatGPT → Browser opens:
  https://domain.com/oauth2/auth?
    client_id=abc123&
    redirect_uri=https://chatgpt.com/callback&
    response_type=code&
    scope=openid+email&
    state=xyz

Ory Hydra → Checks: Is user authenticated?
Ory → NO → Redirects to: /login?login_challenge=challenge_token
```

### Phase 4: User Authentication (YOUR CONTROL)

```
Browser → GET /login?login_challenge=challenge_token

MCP Server:
  1. Check session cookie
     ├─> Exists & Valid → Skip to step 5
     └─> No session → Continue
  
  2. Show login form HTML
  
  3. User enters:
     Email: john@company.com
     Password: mypassword123
  
  4. POST /login (form submission)
  
  5. userStore.Authenticate("john@company.com", "mypassword123")
     ├─> Query PostgreSQL: SELECT * FROM users WHERE email = ?
     ├─> User found?
     │   ├─> NO → Return "Invalid credentials" ❌
     │   └─> YES → Continue
     │
     └─> bcrypt.CompareHashAndPassword(stored_hash, entered_password)
         ├─> Match?
         │   ├─> NO → Return "Invalid credentials" ❌
         │   └─> YES → User authenticated ✅
  
  6. Create session (24h cookie)
  
  7. Tell Ory user is authenticated:
     PUT /admin/oauth2/auth/requests/login/accept?login_challenge=challenge_token
     Body: {
       "subject": "john@company.com",
       "remember": true,
       "remember_for": 86400
     }
  
  8. Ory trusts us: "OK, this user is legit"
  
  9. Ory responds: {"redirect_to": "/consent?consent_challenge=consent_token"}
  
  10. Redirect browser to consent URL
```

**Key Point**: Ory NEVER sees the password. You verified it.

### Phase 5: Consent

```
Browser → GET /consent?consent_challenge=consent_token

MCP Server:
  1. Call Ory: GET /admin/oauth2/auth/requests/consent?consent_challenge=consent_token
  
  2. Ory returns:
     {
       "subject": "john@company.com",
       "requested_scope": ["openid", "email", "profile"],
       "client": {"client_id": "abc123"}
     }
  
  3. Get user from database: userStore.GetUser("john@company.com")
  
  4. Auto-approve consent:
     PUT /admin/oauth2/auth/requests/consent/accept?consent_challenge=consent_token
     Body: {
       "grant_scope": ["openid", "email", "profile"],
       "remember": true,
       "remember_for": 86400,
       "session": {
         "id_token": {
           "email": "john@company.com",
           "name": "John Doe"
         }
       }
     }
  
  5. Ory issues authorization code
  
  6. Ory responds: {"redirect_to": "https://chatgpt.com/callback?code=AUTH_CODE"}
  
  7. Redirect browser back to ChatGPT
```

### Phase 6: Token Exchange

```
ChatGPT → POST /oauth2/token (directly to Ory)
Body:
  grant_type=authorization_code&
  code=AUTH_CODE&
  redirect_uri=https://chatgpt.com/callback&
  client_id=abc123&
  client_secret=secret123

Ory → Validates:
  ✓ Authorization code valid?
  ✓ Client credentials correct?
  ✓ Redirect URI matches?

Ory → Returns:
{
  "access_token": "eyJhbGciOiJSUzI1NiIs...",
  "refresh_token": "eyJhbGciOiJSUzI1NiIs...",
  "token_type": "Bearer",
  "expires_in": 3600,
  "scope": "openid email profile"
}
```

**Storage**: Tokens stored in PostgreSQL `hydra_access` and `hydra_refresh` tables

### Phase 7: API Access

```
ChatGPT → POST /mcp
Header: Authorization: Bearer eyJhbGciOiJSUzI1NiIs...
Body: {"jsonrpc": "2.0", "method": "tools/list", "id": 1}

MCP Server (middleware/auth.go):
  1. Extract Bearer token
  
  2. Call oryClient.IntrospectToken(token)
     POST /admin/oauth2/introspect (Ory Admin API)
     Body: {"token": "eyJhbGciOiJSUzI1NiIs..."}
  
  3. Ory checks PostgreSQL:
     ✓ Token exists?
     ✓ Token not expired?
     ✓ Token not revoked?
  
  4. Ory returns:
     {
       "active": true,
       "sub": "john@company.com",
       "scope": "openid email profile",
       "exp": 1734567890
     }
  
  5. If active=true → Process MCP request
  6. If active=false → Return 401 Unauthorized

MCP Server (main.go):
  7. Process JSON-RPC request
     {
       "jsonrpc": "2.0",
       "id": 1,
       "result": {
         "tools": [
           {
             "name": "list_indian_stores",
             "description": "...",
             "inputSchema": {...}
           }
         ]
       }
     }
```

---

## 📁 Directory Structure

```
indian-store-mcp-server/
├── main.go                          # Main server (routes, initialization)
├── internal/
│   ├── config/
│   │   └── config.go                # Configuration loader
│   ├── users/
│   │   └── users.go                 # User management (PostgreSQL)
│   ├── oauth/
│   │   ├── ory_client.go           # Ory HTTP client
│   │   ├── registration.go         # Dynamic client registration
│   │   ├── login_consent.go        # Login/consent handlers
│   │   └── handlers.go             # OAuth helper functions
│   └── middleware/
│       └── auth.go                  # Token validation middleware
├── k8s/
│   ├── deployement.yaml             # MCP server deployment
│   ├── configmap.yaml               # Configuration (URLs, database)
│   ├── gateway.yaml                 # Gateway API routing
│   └── hydra/
│       ├── postgres-sts.yaml        # PostgreSQL deployment
│       └── ory-hydra-values.yaml    # Helm values for Ory Hydra
├── Dockerfile
├── go.mod
├── go.sum
├── README.md                        # This file
├── INSTALLATION.md                  # Deployment guide
├── AUTHENTICATION.md                # Security deep dive
└── USER_MANAGEMENT.md              # User operations guide
```

---

## 🔐 Security Model

### Authentication Layers

1. **User Authentication** (Your MCP Server)
   - Email/password verification
   - bcrypt password hashing (cost 10)
   - PostgreSQL user storage
   - Session management (24h cookies)

2. **OAuth Token Validation** (Ory Hydra)
   - Access token introspection
   - Token expiration checking
   - Token revocation support
   - Refresh token rotation

### Trust Boundaries

```
┌─────────────────────────┐
│  Your MCP Server        │
│  (Trusted)              │
│  • Verifies passwords   │
│  • Creates sessions     │
│  • Tells Ory who's OK   │
└───────────┬─────────────┘
            │ Admin API (trusted)
            ▼
┌─────────────────────────┐
│  Ory Hydra              │
│  (Trusts your auth)     │
│  • Issues tokens        │
│  • Validates tokens     │
└─────────────────────────┘
```

### What Cannot Be Bypassed

- ❌ Can't create users without kubectl/database access
- ❌ Can't login without correct password
- ❌ Can't forge OAuth tokens
- ❌ Can't access MCP without valid token
- ❌ Can't use expired tokens

---

## 🚀 Quick Start

See **[INSTALLATION.md](./INSTALLATION.md)** for complete deployment guide.

**TL;DR**:
```bash
# 1. Deploy PostgreSQL
kubectl apply -f k8s/hydra/postgres-sts.yaml

# 2. Deploy Ory Hydra
helm install ory-hydra ory/hydra -f k8s/hydra/ory-hydra-values.yaml

# 3. Deploy MCP Server
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/deployement.yaml
kubectl apply -f k8s/gateway.yaml

# 4. Create a user
kubectl exec -it deployment/postgres -- psql -U ory_hydra -d ory_hydra -c \
  "INSERT INTO users (email, password_hash, name) 
   VALUES ('admin@example.com', '<bcrypt_hash>', 'Admin');"
```

---

## 👤 User Management

See **[USER_MANAGEMENT.md](./USER_MANAGEMENT.md)** for complete guide.

**Create user**:
```bash
# Generate bcrypt hash
python3 -c "import bcrypt; print(bcrypt.hashpw(b'password', bcrypt.gensalt(rounds=10)).decode())"

# Insert into database
kubectl exec -it deployment/postgres -- psql -U ory_hydra -d ory_hydra -c \
  "INSERT INTO users (email, password_hash, name) VALUES ('user@example.com', '\$2a\$10\$...', 'User');"
```

**List users**:
```bash
kubectl exec -it deployment/postgres -- psql -U ory_hydra -d ory_hydra -c \
  "SELECT email, name, created_at FROM users;"
```

---

## 🧪 Testing

### Test OAuth Discovery
```bash
curl https://your-domain.com/.well-known/oauth-authorization-server
```

### Test Health
```bash
curl https://your-domain.com/health
```

### Test with ChatGPT
1. Go to ChatGPT Settings → Integrations
2. Add MCP Server: `https://your-domain.com`
3. Login when prompted
4. Should show "Connected"

---

## 🐛 Troubleshooting

### Check MCP Server Logs
```bash
kubectl logs -l app=mcp-service-indian-store --tail=100
```

### Check Ory Hydra Logs
```bash
kubectl logs -l app.kubernetes.io/name=hydra --tail=100
```

### Common Issues

**401 Unauthorized on /mcp**:
- Check token is valid: Token might be expired
- Verify Ory Admin URL is internal: `http://ory-hydra-admin.default.svc.cluster.local:4445`

**Login page not showing**:
- Check Gateway routes: `kubectl get httproute`
- Verify MCP server is running: `kubectl get pods`

**Users persisting after pod restart**:
- ✅ Users are in PostgreSQL (persistent)
- ✅ Check database: `kubectl exec -it deployment/postgres -- psql -U ory_hydra -d ory_hydra -c "SELECT * FROM users;"`

---

## 📚 Additional Documentation

- **[INSTALLATION.md](./INSTALLATION.md)** - Complete deployment guide
- **[AUTHENTICATION.md](./AUTHENTICATION.md)** - Security model and authentication flow
- **[USER_MANAGEMENT.md](./USER_MANAGEMENT.md)** - User operations

---

## 🤝 Contributing

This is a reference implementation. Feel free to adapt for your needs:
- Replace in-memory sessions with Redis
- Add 2FA/MFA support
- Implement user registration UI (if needed)
- Add RBAC/permissions
- Integrate with LDAP/AD

---

## 📄 License

MIT License

---

## 🔑 Key Takeaways

1. **Your MCP server owns user authentication** - You control who can login
2. **Ory Hydra owns OAuth complexity** - Token management, refresh, revocation
3. **Trust via Admin API** - Ory trusts your authentication decisions
4. **Separation of concerns** - Auth logic separate from token management
5. **Production-ready** - External OAuth provider, persistent storage, scalable

**The beauty**: You get enterprise OAuth without building an OAuth server!
