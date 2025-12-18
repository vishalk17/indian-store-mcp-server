# Dynamic Client Registration - Workaround

## Issue
Hydra doesn't advertise the `registration_endpoint` in the OIDC discovery document (`.well-known/openid-configuration`), even though dynamic client registration (RFC 7591) is enabled and working.

## Verification
Dynamic registration IS working:
```bash
curl -k https://vishalk17.cloudwithme.dev/oauth2/register \
  -X POST \
  -H "Content-Type: application/json" \
  -d '{
    "client_name": "Test Client",
    "redirect_uris": ["https://example.com/callback"],
    "grant_types": ["authorization_code", "refresh_token"],
    "response_types": ["code"]
  }'
```

**Response:** Client created successfully with `client_id` and `client_secret`.

## Workaround for ChatGPT/Claude

When configuring the OAuth connector in ChatGPT/Claude, **manually provide the registration endpoint**:

```
Registration Endpoint: https://vishalk17.cloudwithme.dev/oauth2/register
Authorization Endpoint: https://vishalk17.cloudwithme.dev/oauth2/oauth2/auth
Token Endpoint: https://vishalk17.cloudwithme.dev/oauth2/oauth2/token
Issuer: https://vishalk17.cloudwithme.dev/oauth2
```

## Alternative: Pre-register Client

Instead of dynamic registration, pre-create the OAuth client:

### Via Hydra Admin API
```bash
kubectl port-forward svc/hydra-admin 4445:4445

curl -X POST http://localhost:4445/admin/clients \
  -H "Content-Type: application/json" \
  -d '{
    "client_id": "chatgpt-mcp-client",
    "client_name": "ChatGPT MCP Client",
    "client_secret": "your-secret-here",
    "redirect_uris": [
      "https://chatgpt.com/aip/oauth/callback"
    ],
    "grant_types": ["authorization_code", "refresh_token"],
    "response_types": ["code"],
    "scope": "openid email profile",
    "token_endpoint_auth_method": "client_secret_post"
  }'
```

### Via kubectl exec
```bash
kubectl exec -it deployment/hydra -- \
  hydra create client \
    --endpoint http://localhost:4445 \
    --id chatgpt-mcp-client \
    --name "ChatGPT MCP Client" \
    --grant-type authorization_code,refresh_token \
    --response-type code \
    --scope openid,email,profile \
    --redirect-uri "https://chatgpt.com/aip/oauth/callback"
```

Then provide these to ChatGPT/Claude:
- **Client ID**: `chatgpt-mcp-client`
- **Client Secret**: (from response)
- **Authorization URL**: `https://vishalk17.cloudwithme.dev/oauth2/oauth2/auth`
- **Token URL**: `https://vishalk17.cloudwithme.dev/oauth2/oauth2/token`
- **Scopes**: `openid email profile`

## Why This Happens

Ory Hydra's dynamic client registration is implemented but doesn't automatically add the `registration_endpoint` field to the OIDC discovery document. This is a known limitation in certain Hydra versions.

The endpoint exists and works, it's just not advertised in the discovery document.

## Tested & Working

```bash
# 1. Register client dynamically
CLIENT_RESPONSE=$(curl -sk https://vishalk17.cloudwithme.dev/oauth2/register \
  -X POST \
  -H "Content-Type: application/json" \
  -d '{
    "client_name": "Test",
    "redirect_uris": ["https://example.com/callback"],
    "grant_types": ["authorization_code"],
    "response_types": ["code"]
  }')

# Extract client_id
CLIENT_ID=$(echo $CLIENT_RESPONSE | jq -r '.client_id')

echo "Client registered: $CLIENT_ID"

# 2. Use in OAuth flow
echo "Authorization URL:"
echo "https://vishalk17.cloudwithme.dev/oauth2/oauth2/auth?client_id=$CLIENT_ID&response_type=code&redirect_uri=https://example.com/callback&scope=openid+email"
```

This flow works end-to-end.
