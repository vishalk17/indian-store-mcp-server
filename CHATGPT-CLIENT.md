# ChatGPT OAuth Client - Pre-registered

## Client Credentials

```json
{
  "client_id": "25cb23d0-a470-46f7-b64a-fed0c5162755",
  "client_name": "ChatGPT MCP Client",
  "token_endpoint_auth_method": "none"
}
```

**Note:** No client secret (public client with PKCE)

## Configuration for ChatGPT

When setting up the OAuth connector in ChatGPT, use these values:

### Basic Info
- **Client ID**: `25cb23d0-a470-46f7-b64a-fed0c5162755`
- **Client Secret**: (leave empty - public client)
- **Authorization Code Flow**: YES
- **PKCE**: YES (required for public clients)

### URLs
- **Authorization URL**: `https://vishalk17.cloudwithme.dev/oauth2/oauth2/auth`
- **Token URL**: `https://vishalk17.cloudwithme.dev/oauth2/oauth2/token`
- **Issuer**: `https://vishalk17.cloudwithme.dev/oauth2`

### Scopes
```
openid email profile
```

### Redirect URI (already configured)
```
https://chatgpt.com/aip/oauth/callback
```

## Flow

1. ChatGPT initiates OAuth with above client_id
2. User redirected to: `https://vishalk17.cloudwithme.dev/auth/login`
3. User sees "Sign in with GitHub / Microsoft"
4. After social login, Kratos checks if user exists in Postgres
5. If exists → consent → Hydra issues JWT token
6. ChatGPT receives token and calls MCP server

## Add Allowed User

Before testing, add a user to Kratos:

```bash
kubectl port-forward svc/kratos-admin 4434:80 &
sleep 2

curl -X POST http://localhost:4434/admin/identities \
  -H "Content-Type: application/json" \
  -d '{
    "schema_id": "default",
    "traits": {
      "email": "your-github-or-microsoft-email@example.com"
    }
  }'

pkill -f "port-forward svc/kratos-admin"
```

**Important:** Use the SAME email that your GitHub/Microsoft account returns!

## Test MCP Endpoint

After getting a token from ChatGPT:

```bash
curl -k https://vishalk17.cloudwithme.dev/mcp \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/list"
  }'
```

## Why Pre-registration?

Ory Hydra v2.x does NOT advertise the `registration_endpoint` in the OIDC discovery document, even when dynamic client registration is enabled. This is intentional behavior.

Pre-registration via Admin API is the recommended approach.
