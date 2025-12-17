# Authentication & Authorization Guide

## How Only Legit Users Can Access the MCP Server

### 🔐 Security Model Overview

```
┌─────────────────────────────────────────────────────────────┐
│  ADMINISTRATOR (You)                                        │
│  ─────────────────────                                      │
│  • Has kubectl access to Kubernetes cluster                │
│  • Can exec into PostgreSQL pod                            │
│  • ONLY person who can create users                        │
└────────────────────────┬────────────────────────────────────┘
                         │
                         │ Creates users directly in database
                         ▼
┌─────────────────────────────────────────────────────────────┐
│  PostgreSQL Database                                        │
│  ───────────────────                                        │
│  users table:                                               │
│  • email (unique)                                           │
│  • password_hash (bcrypt)                                   │
│  • name                                                     │
│  • created_at                                               │
└────────────────────────┬────────────────────────────────────┘
                         │
                         │ MCP Server queries during login
                         ▼
┌─────────────────────────────────────────────────────────────┐
│  END USER (Employee/Authorized Person)                     │
│  ────────────────────────────────────                       │
│  • Can ONLY login if YOU created their account             │
│  • Must know their email + password                        │
│  • Cannot create their own account                         │
│  • Cannot access without valid credentials                 │
└─────────────────────────────────────────────────────────────┘
```

---

## Step-by-Step: How Users Are Verified

### Phase 1: User Creation (Administrator Only)

**Who can do this?** Only administrators with kubectl access

**How to create a user:**

```bash
# Step 1: Generate bcrypt hash for password
python3 -c "import bcrypt; print(bcrypt.hashpw(b'user_password', bcrypt.gensalt(rounds=10)).decode())"
# Output: $2a$10$abc123xyz...

# Step 2: Insert user into database
kubectl exec -it deployment/postgres -- psql -U ory_hydra -d ory_hydra -c \
  "INSERT INTO users (email, password_hash, name) 
   VALUES ('john@company.com', '\$2a\$10\$abc123xyz...', 'John Doe');"

# Step 3: Verify user was created
kubectl exec -it deployment/postgres -- psql -U ory_hydra -d ory_hydra -c \
  "SELECT email, name, created_at FROM users WHERE email = 'john@company.com';"
```

**Result:** User `john@company.com` can now login. Nobody else can.

---

### Phase 2: User Login (Authentication)

**When:** User tries to connect ChatGPT/Claude to your MCP server

**Flow:**

```
1. ChatGPT starts OAuth flow
   └─> Redirects browser to: https://your-domain.com/login?login_challenge=xyz

2. User sees login form
   ┌─────────────────────────────┐
   │  🏪 Indian Store MCP        │
   │                             │
   │  Email: john@company.com    │
   │  Password: ************     │
   │                             │
   │  [ Sign In ]                │
   └─────────────────────────────┘

3. User submits credentials
   └─> POST /login with email + password

4. MCP Server validates (internal/users/users.go:102-120)
   ├─> Query PostgreSQL: SELECT * FROM users WHERE email = 'john@company.com'
   ├─> User found? 
   │   ├─> NO → Return "Invalid credentials" ❌ BLOCKED
   │   └─> YES → Continue to password check
   │
   └─> Compare password with bcrypt hash
       ├─> bcrypt.CompareHashAndPassword(stored_hash, entered_password)
       ├─> Match?
       │   ├─> NO → Return "Invalid credentials" ❌ BLOCKED
       │   └─> YES → User authenticated ✅ PROCEED

5. MCP Server tells Ory Hydra (internal/oauth/login_consent.go:276-320)
   └─> PUT /admin/oauth2/auth/requests/login/accept
       Body: {"subject": "john@company.com"}
   └─> Ory trusts your decision: "This user is legit"

6. OAuth flow continues
   └─> Consent screen (auto-approved)
   └─> Ory issues access token
   └─> ChatGPT receives token

7. ChatGPT can now call MCP endpoint
   └─> POST /mcp with Bearer token
```

---

### Phase 3: API Access (Authorization)

**Every MCP API call is protected:**

```
1. ChatGPT calls: POST /mcp
   Header: Authorization: Bearer abc123token

2. MCP Server validates token (internal/middleware/auth.go:22-60)
   ├─> Extract Bearer token from header
   ├─> Call Ory Admin API: POST /admin/oauth2/introspect
   │   Body: {"token": "abc123token"}
   │
   └─> Ory responds:
       {
         "active": true,
         "sub": "john@company.com",
         "exp": 1234567890
       }

3. Token valid?
   ├─> NO → Return 401 Unauthorized ❌ BLOCKED
   └─> YES → Process MCP request ✅

4. MCP Server processes request
   └─> Returns tools/list or tools/call result
```

---

## 🛡️ Security Checkpoints

### Checkpoint 1: User Exists in Database
**Location:** Database query in `Authenticate()`  
**Check:** `SELECT * FROM users WHERE email = ?`  
**Blocks:** Non-existent users

### Checkpoint 2: Password Matches
**Location:** bcrypt comparison in `Authenticate()`  
**Check:** `bcrypt.CompareHashAndPassword()`  
**Blocks:** Wrong passwords

### Checkpoint 3: Valid Session
**Location:** Session cookie check in `HandleLogin()`  
**Check:** Session exists and not expired (24h)  
**Blocks:** Expired sessions

### Checkpoint 4: Valid OAuth Token
**Location:** Token introspection in `RequireAuth()`  
**Check:** Ory validates token is active  
**Blocks:** Invalid/expired tokens

---

## ❌ What CANNOT Happen

- ❌ Users cannot self-register (no public signup API)
- ❌ Users cannot create accounts via API calls
- ❌ Users cannot login without being in database
- ❌ Users cannot bypass password check
- ❌ Users cannot access MCP without OAuth token
- ❌ Users cannot forge OAuth tokens (validated by Ory)
- ❌ Random people cannot access the system

---

## ✅ What CAN Happen

- ✅ Administrator creates users via kubectl
- ✅ Authorized users login with email/password
- ✅ Authenticated users get OAuth tokens
- ✅ Token holders can call MCP endpoints
- ✅ Sessions expire after 24 hours
- ✅ Passwords are securely hashed (bcrypt cost 10)
- ✅ All authentication is logged

---

## 📊 User Lifecycle

```
┌─────────────────────┐
│ ADMINISTRATOR       │
│ Creates user in DB  │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│ USER IN DATABASE    │
│ Waiting to login    │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│ USER LOGS IN        │
│ Enters credentials  │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│ MCP VERIFIES        │
│ Email + Password    │
└──────────┬──────────┘
           │
           ├─> ❌ Invalid → Login Failed
           │
           └─> ✅ Valid
               │
               ▼
┌─────────────────────┐
│ ORY ISSUES TOKEN    │
│ Based on MCP trust  │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│ USER ACCESSES MCP   │
│ With valid token    │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│ TOKEN VALIDATED     │
│ On every API call   │
└─────────────────────┘
```

---

## 🔍 Audit & Monitoring

**What gets logged:**

```bash
# Check authentication logs
kubectl logs -l app=mcp-service-indian-store | grep -i "auth"

# Successful login
"User john@company.com authenticated successfully"
"Authenticated user: john@company.com (john@company.com)"

# Failed login
"Authentication failed for john@company.com: invalid credentials"

# Token validation
"Token introspection failed: invalid token"
"Token is not active"
```

---

## 🚨 Security Incidents

### Scenario 1: Someone tries random email/password
```
Login attempt: hacker@evil.com / password123
└─> Database query: SELECT * FROM users WHERE email = 'hacker@evil.com'
└─> Result: No rows found
└─> Response: "Invalid credentials"
└─> Logged: "Authentication failed for hacker@evil.com"
└─> BLOCKED ❌
```

### Scenario 2: Someone tries correct email, wrong password
```
Login attempt: john@company.com / wrongpass
└─> Database query: User found ✓
└─> bcrypt.CompareHashAndPassword()
└─> Result: Hash mismatch
└─> Response: "Invalid credentials"
└─> Logged: "Authentication failed for john@company.com"
└─> BLOCKED ❌
```

### Scenario 3: Someone tries to call API without token
```
API call: POST /mcp (no Authorization header)
└─> Middleware check: No Bearer token
└─> Response: 401 Unauthorized "Missing token"
└─> Logged: "Missing Authorization header"
└─> BLOCKED ❌
```

### Scenario 4: Someone tries to use expired token
```
API call: POST /mcp with old token
└─> Token introspection: Ory checks token
└─> Ory response: {"active": false}
└─> Response: 401 Unauthorized "Token expired"
└─> Logged: "Token is not active"
└─> BLOCKED ❌
```

---

## 📝 Best Practices

### For Administrators

1. **Create users carefully**
   - Only create accounts for authorized personnel
   - Use strong passwords (min 12 characters)
   - Include special characters, numbers, uppercase

2. **Rotate passwords regularly**
   - Update password_hash in database every 90 days
   - Generate new bcrypt hash for new password

3. **Monitor access logs**
   - Check for failed login attempts
   - Look for suspicious patterns

4. **Remove users when they leave**
   ```bash
   kubectl exec -it deployment/postgres -- psql -U ory_hydra -d ory_hydra -c \
     "DELETE FROM users WHERE email = 'ex-employee@company.com';"
   ```

### For End Users

1. **Keep credentials secure**
   - Don't share email/password
   - Use password manager

2. **Report suspicious activity**
   - Unknown login locations
   - Unexpected access requests

3. **Log out when done**
   - Sessions expire after 24 hours
   - But manual logout is good practice

---

## 🔑 Summary

**Only legit users can access because:**

1. ✅ Users must be created by administrator (you)
2. ✅ Users must exist in PostgreSQL database
3. ✅ Users must provide correct email + password
4. ✅ Passwords are verified with bcrypt hashing
5. ✅ MCP server controls who Ory trusts
6. ✅ Every API call requires valid OAuth token
7. ✅ Tokens are validated by Ory on every request
8. ✅ No public user creation API exists

**The trust chain:**
```
You trust → Database (users you created)
MCP Server verifies → Password against database
MCP Server tells Ory → "This user is legit"
Ory trusts MCP Server → Issues OAuth token
Token validates → Every API call
```

**Bottom line:** If you didn't create the user in the database, they CANNOT access the system. Period.
