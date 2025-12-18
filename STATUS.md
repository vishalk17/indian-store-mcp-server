# Deployment Status

## ✅ Completed
1. **Secrets** - Applied with real OAuth credentials (GitHub, Microsoft)
2. **MCP Server** - Updated with JWT middleware (v0.201)
3. **Postgres** - Running with kratos & hydra databases
4. **Kratos** - Running with social login configured

## ⏳ Remaining Steps

### 1. Deploy Hydra
```bash
cd /home/vishalk17/ory/store
helm upgrade --install hydra ory/hydra -f k8s/hydra-values.yaml
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=hydra --timeout=180s
```

### 2. Deploy Kratos UI
```bash
helm upgrade --install kratos-ui ory/kratos-selfservice-ui-node -f k8s/kratos-ui-values.yaml
```

### 3. Apply Gateway + HTTPRoutes
```bash
kubectl apply -f k8s/gateway.yaml
```

### 4. Deploy Updated MCP Server
```bash
kubectl apply -f k8s/deployement.yaml
kubectl rollout restart deployment/mcp-service-indian-store
```

### 5. Verify Deployment
```bash
kubectl get pods
kubectl get httproutes
curl -k https://vishalk17.cloudwithme.dev/.well-known/openid-configuration
```

### 6. Add Test User to Kratos
```bash
kubectl exec -it deployment/kratos -- sh
kratos create identity \
  --endpoint http://localhost:4434 \
  --format json-pretty \
  test@example.com \
  --schema-id default \
  --trait email=test@example.com
```

### 7. Test Dynamic Client Registration
```bash
curl -X POST https://vishalk17.cloudwithme.dev/oauth2/clients \
  -H "Content-Type: application/json" \
  -d '{
    "client_name": "Test MCP Client",
    "redirect_uris": ["https://example.com/callback"],
    "grant_types": ["authorization_code", "refresh_token"],
    "response_types": ["code"],
    "scope": "openid email profile",
    "token_endpoint_auth_method": "none"
  }'
```

## MCP Server Changes

**Added JWT middleware:**
- Validates `Authorization: Bearer <token>` header
- Checks issuer: `https://vishalk17.cloudwithme.dev/oauth2`
- Logs authenticated requests
- Currently uses ParseUnverified (TODO: implement JWKS validation)

**Files modified:**
- `indian-store-mcp-server/main.go` - Added jwtMiddleware()
- `indian-store-mcp-server/go.mod` - Added jwt/v5 dependency
- `indian-store-mcp-server/Dockerfile` - Copy go.sum
- `k8s/deployement.yaml` - Updated image to 0.201

## Configuration Notes

### GitHub OAuth Callback
- URL: `https://vishalk17.cloudwithme.dev/kratos/self-service/methods/oidc/callback/github`
- Already configured with provided credentials

### Microsoft OAuth Callback  
- URL: `https://vishalk17.cloudwithme.dev/kratos/self-service/methods/oidc/callback/microsoft`
- Tenant: `common` (multi-tenant)
- Already configured with provided credentials

### Google OAuth (Not configured)
- URL: `https://vishalk17.cloudwithme.dev/kratos/self-service/methods/oidc/callback/google`
- TODO: Get credentials from https://console.cloud.google.com/apis/credentials
- Update `k8s/secrets.yaml` and redeploy Kratos

## Troubleshooting

### Check Logs
```bash
kubectl logs -f deployment/kratos
kubectl logs -f deployment/hydra  
kubectl logs -f deployment/kratos-selfservice-ui-node
kubectl logs -f deployment/mcp-service-indian-store
```

### Test MCP Without OAuth (for debugging)
```bash
# Temporarily remove middleware
kubectl exec -it deployment/mcp-service-indian-store -- sh
# Then curl localhost:8080/mcp
```

### Verify Postgres Databases
```bash
kubectl exec -it postgres-postgresql-0 -- psql -U ory -d kratos -c '\dt'
kubectl exec -it postgres-postgresql-0 -- psql -U ory -d hydra -c '\dt'
```

## Architecture Flow

```
ChatGPT/Claude
    ↓
POST /oauth2/register (Hydra) → Register client
    ↓
GET /oauth2/auth (Hydra) → Redirect to Kratos UI
    ↓
https://vishalk17.cloudwithme.dev/auth/login
    ↓
User clicks "Sign in with GitHub/Microsoft"
    ↓
OAuth flow with social provider
    ↓
Kratos receives email → Check Postgres identities table
    ↓
User exists? → YES → Return to Hydra consent
              → NO → Reject (registration disabled)
    ↓
Hydra issues JWT access token
    ↓
POST /mcp with Authorization: Bearer <token>
    ↓
JWT middleware validates → MCP processes request
```

## All Deployments Completed, Run:
```bash
./deploy.sh
```
Or continue manually from step 1 above.
