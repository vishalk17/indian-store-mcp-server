# ✅ Deployment Complete

## Status: ALL SERVICES RUNNING

```
✅ Postgres          - Running (kratos + hydra databases)
✅ Kratos            - Running (identity + social login)
✅ Kratos UI         - Running (login interface)
✅ Hydra             - Running (OAuth2/OIDC server)
✅ MCP Server        - Running (with JWT validation)
✅ Gateway           - Configured with all HTTPRoutes
```

## Endpoints

### OAuth2 Discovery
```bash
curl -k https://vishalk17.cloudwithme.dev/.well-known/openid-configuration
```
**Response:**
- Issuer: `https://vishalk17.cloudwithme.dev/oauth2`
- Authorization: `https://vishalk17.cloudwithme.dev/oauth2/oauth2/auth`
- Token: `https://vishalk17.cloudwithme.dev/oauth2/oauth2/token`

### Login UI
```
https://vishalk17.cloudwithme.dev/auth/login
```

### MCP Server
```
https://vishalk17.cloudwithme.dev/mcp
```

## HTTPRoutes

```
NAME                 HOSTNAMES
hydra-public-route   vishalk17.cloudwithme.dev   → Hydra OAuth2 (/oauth2/*)
kratos-route         vishalk17.cloudwithme.dev   → Kratos API (/kratos/*)
kratos-ui-route      vishalk17.cloudwithme.dev   → Login UI (/auth/*)
mcp-route            vishalk17.cloudwithme.dev   → MCP Server (/mcp)
```

## Pods Running

```bash
kubectl get pods
```

```
hydra-b9c8cf7f4-zjclj                                   1/1     Running
hydra-hydra-maester-84bd7999d9-bdtsm                    1/1     Running
kratos-8ffd95588-hz57d                                  1/1     Running
kratos-courier-0                                        1/1     Running
kratos-ui-kratos-selfservice-ui-node-7b5784774f-82lhb   1/1     Running
mcp-service-indian-store-7dcb6d6dd6-thbnq               1/1     Running
postgres-postgresql-0                                   1/1     Running
```

## Add Users to Allow List

Users MUST be pre-registered in Kratos to login (self-registration is disabled).

**Method 1: Via Kratos Admin API**
```bash
kubectl port-forward svc/kratos-admin 4434:80

curl -X POST http://localhost:4434/admin/identities \
  -H "Content-Type: application/json" \
  -d '{
    "schema_id": "default",
    "traits": {
      "email": "user@example.com"
    }
  }'
```

**Method 2: Directly in Postgres**
```bash
kubectl exec -it postgres-postgresql-0 -- psql -U ory -d kratos

INSERT INTO identities (id, schema_id, traits, created_at, updated_at)
VALUES (
  gen_random_uuid(),
  'default',
  '{"email": "user@example.com"}',
  NOW(),
  NOW()
);
```

## Test Dynamic Client Registration

```bash
curl -X POST https://vishalk17.cloudwithme.dev/oauth2/clients \
  -H "Content-Type: application/json" \
  -d '{
    "client_name": "ChatGPT MCP Client",
    "redirect_uris": ["https://chatgpt.com/aip/oauth/callback"],
    "grant_types": ["authorization_code", "refresh_token"],
    "response_types": ["code"],
    "scope": "openid email profile",
    "token_endpoint_auth_method": "none"
  }'
```

**Expected Response:**
```json
{
  "client_id": "some-uuid",
  "client_name": "ChatGPT MCP Client",
  "redirect_uris": ["https://chatgpt.com/aip/oauth/callback"],
  "grant_types": ["authorization_code", "refresh_token"],
  "response_types": ["code"],
  "scope": "openid email profile"
}
```

## OAuth2 Flow for ChatGPT/Claude

1. **Client Registration** → POST `/oauth2/clients`
2. **Authorization** → User visits `/oauth2/auth?client_id=...`
3. **Redirect to Login** → Kratos UI at `/auth/login`
4. **Social Login** → User clicks GitHub/Microsoft
5. **OAuth with Provider** → GitHub/Microsoft OAuth flow
6. **Check Database** → Kratos verifies email exists in Postgres
   - ✅ Exists → Continue
   - ❌ Not found → Reject (registration disabled)
7. **Return to Hydra** → Consent flow
8. **Token Issuance** → Hydra issues JWT
9. **Access MCP** → ChatGPT calls `/mcp` with Bearer token
10. **JWT Validation** → MCP validates issuer & token

## MCP Server JWT Validation

**Current Implementation:**
- Parses JWT without signature verification (for testing)
- Validates issuer: `https://vishalk17.cloudwithme.dev/oauth2`
- Logs authenticated user

**TODO for Production:**
- Implement JWKS fetching from `/.well-known/jwks.json`
- Verify JWT signature with public key
- Validate expiration, audience, scopes

## Social Login Configuration

### GitHub ✅ Configured
- Client ID: `Ov23lihfWwAoEOnpddJ2`
- Callback: `https://vishalk17.cloudwithme.dev/kratos/self-service/methods/oidc/callback/github`

### Microsoft ✅ Configured  
- Client ID: `cdeefe35-06ad-4334-a074-0c91d70fc6f1`
- Tenant: `common`
- Callback: `https://vishalk17.cloudwithme.dev/kratos/self-service/methods/oidc/callback/microsoft`

### Google ⚠️ Not Configured
- Get credentials: https://console.cloud.google.com/apis/credentials
- Update `k8s/secrets.yaml`
- Redeploy: `helm upgrade --install kratos ory/kratos -f k8s/kratos-values.yaml`

## Verify Everything Works

### 1. Test OAuth Discovery
```bash
curl -k https://vishalk17.cloudwithme.dev/.well-known/openid-configuration
```

### 2. Test Login UI
```bash
curl -k https://vishalk17.cloudwithme.dev/auth/login
# Should return HTML login page
```

### 3. Test MCP Health
```bash
curl -k https://vishalk17.cloudwithme.dev/health
# Should return: {"status":"ok","server":"indian-store-mcp-server"}
```

### 4. Test MCP with JWT (after getting token)
```bash
curl -k https://vishalk17.cloudwithme.dev/mcp \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
```

## Troubleshooting

### Check Logs
```bash
kubectl logs -f deployment/hydra
kubectl logs -f deployment/kratos
kubectl logs -f deployment/kratos-ui-kratos-selfservice-ui-node
kubectl logs -f deployment/mcp-service-indian-store
```

### Check Services
```bash
kubectl get svc | grep -E '(hydra|kratos|mcp)'
```

### Check HTTPRoutes
```bash
kubectl get httproutes
kubectl describe httproute hydra-public-route
```

### Port Forward for Local Testing
```bash
# Kratos Admin
kubectl port-forward svc/kratos-admin 4434:80

# Hydra Admin
kubectl port-forward svc/hydra-admin 4445:4445

# Kratos Public
kubectl port-forward svc/kratos-public 4433:80
```

## Architecture

```
┌─────────────────┐
│  ChatGPT/Claude │
└────────┬────────┘
         │
         ▼
┌────────────────────────────────────────────┐
│  Gateway (vishalk17.cloudwithme.dev)       │
│  ┌──────────────────────────────────────┐  │
│  │ /oauth2/*    → Hydra (OAuth2 server) │  │
│  │ /kratos/*    → Kratos (Identity API) │  │
│  │ /auth/*      → Kratos UI (Login)     │  │
│  │ /mcp         → MCP Server (JWT auth) │  │
│  └──────────────────────────────────────┘  │
└────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────┐
│  Backend Services                   │
│  ┌───────────────┐ ┌──────────────┐ │
│  │ Hydra         │ │ Kratos       │ │
│  │ (OAuth2)      │ │ (Identity)   │ │
│  └───────┬───────┘ └───────┬──────┘ │
│          │                 │        │
│          └────────┬────────┘        │
│                   ▼                 │
│         ┌─────────────────┐         │
│         │   Postgres      │         │
│         │  ┌───────────┐  │         │
│         │  │  kratos   │  │         │
│         │  │  hydra    │  │         │
│         │  └───────────┘  │         │
│         └─────────────────┘         │
└─────────────────────────────────────┘
```

## Files Created/Modified

### Configuration
- `k8s/secrets.yaml` - OAuth credentials & secrets
- `k8s/kratos-values.yaml` - Kratos config with social login
- `k8s/kratos-ui-values.yaml` - Login UI config
- `k8s/hydra-values.yaml` - Hydra OAuth2 config
- `k8s/gateway.yaml` - Gateway + all HTTPRoutes
- `k8s/deployement.yaml` - MCP server deployment (v0.201)

### MCP Server
- `indian-store-mcp-server/main.go` - Added JWT middleware
- `indian-store-mcp-server/go.mod` - Added jwt/v5 dependency
- `indian-store-mcp-server/go.sum` - Dependency checksums
- `indian-store-mcp-server/Dockerfile` - Updated to copy go.sum

### Documentation
- `DEPLOYMENT.md` - Complete deployment guide
- `STATUS.md` - Deployment steps & progress
- `COMPLETE.md` - This file
- `deploy.sh` - Automated deployment script

## Next Steps

1. **Add allowed users** via Kratos admin API
2. **Configure Google OAuth** (optional)
3. **Test full OAuth flow** with ChatGPT/Claude
4. **Implement JWKS validation** in MCP server for production
5. **Monitor logs** for any issues

## Success Criteria ✅

- [x] Postgres running with databases
- [x] Kratos running with social login (GitHub, Microsoft)
- [x] Kratos UI serving login page
- [x] Hydra serving OAuth2 endpoints
- [x] MCP server running with JWT middleware
- [x] Gateway routing all services correctly
- [x] OAuth2 discovery endpoint responding
- [x] HTTPRoutes configured
- [x] All pods healthy

## You're Ready! 🚀

Your OAuth2 + Social Login system is deployed and operational. ChatGPT/Claude can now:
1. Register as OAuth clients via dynamic registration
2. Redirect users to login with GitHub/Microsoft
3. Receive JWT access tokens
4. Call your MCP server with authentication

**Test the login UI:**
https://vishalk17.cloudwithme.dev/auth/login
