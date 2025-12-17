# Indian Store MCP Server - Kubernetes Deployment

Complete Kubernetes setup for Indian Store MCP Server with Ory OAuth integration.

## Directory Structure

```
k8s/
├── hydra/                    # Ory Hydra OAuth server
│   ├── postgres-sts.yaml     # PostgreSQL database
│   ├── ory-hydra-values.yaml # Helm values for Hydra
│   └── README.md             # Hydra deployment guide
├── configmap.yaml            # MCP server config and secrets
├── deployement.yaml          # MCP server deployment
├── gateway.yaml              # Gateway API configuration
└── README.md                 # This file
```

## Quick Deploy

```bash
# 1. Deploy Ory Hydra (PostgreSQL + Hydra)
cd hydra/
kubectl apply -f postgres-sts.yaml
kubectl wait --for=condition=ready pod -l app=postgres --timeout=60s
helm install ory-hydra ory/hydra -f ory-hydra-values.yaml -n default
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=hydra --timeout=120s

# 2. Deploy MCP Server
cd ..
kubectl apply -f configmap.yaml
kubectl apply -f deployement.yaml

# 3. Deploy Gateway
kubectl apply -f gateway.yaml
```

## Verify Deployment

```bash
# Check all pods are running
kubectl get pods -n default

# Check services
kubectl get svc -n default

# Test MCP server health
curl https://vishalk17.cloudwithme.dev/health

# Test Ory Hydra
curl https://vishalk17.cloudwithme.dev/ory/.well-known/openid-configuration

# Test OAuth discovery
curl https://vishalk17.cloudwithme.dev/.well-known/oauth-authorization-server
```

## Architecture

```
Internet → Gateway (vishalk17.cloudwithme.dev)
           ├── /ory/* → Ory Hydra (OAuth server)
           └── /*     → MCP Server
                        └── talks to Ory Hydra Admin API internally
```

## Configuration Files

### configmap.yaml
Contains:
- **ConfigMap**: Non-sensitive config (URLs, ports)
- **Secret**: Sensitive data (client ID, client secret, JWT secret)

### deployement.yaml
- MCP server Deployment
- Service exposing port 8080

### gateway.yaml
- GatewayClass and Gateway (HTTP/HTTPS)
- HTTPRoute for `/ory/*` → Ory Hydra
- HTTPRoute for `/*` → MCP Server

## Updating Configuration

### Update MCP Server Config
```bash
# Edit configmap.yaml
vim configmap.yaml

# Apply changes
kubectl apply -f configmap.yaml

# Restart MCP server
kubectl rollout restart deployment/mcp-service-indian-store
```

### Update Ory Hydra
```bash
# Edit hydra/ory-hydra-values.yaml
vim hydra/ory-hydra-values.yaml

# Upgrade Helm release
helm upgrade ory-hydra ory/hydra -f hydra/ory-hydra-values.yaml -n default
```

## OAuth Clients

**No manual setup needed!** MCP clients (ChatGPT, Claude) automatically register themselves via `/oauth/register` when they first connect.

To view registered clients:
```bash
kubectl exec deployment/ory-hydra -- hydra list clients --endpoint http://localhost:4445
```

## Endpoints

- `https://vishalk17.cloudwithme.dev/` - MCP server root
- `https://vishalk17.cloudwithme.dev/health` - Health check
- `https://vishalk17.cloudwithme.dev/mcp` - MCP JSON-RPC endpoint
- `https://vishalk17.cloudwithme.dev/oauth/*` - OAuth endpoints
- `https://vishalk17.cloudwithme.dev/.well-known/oauth-authorization-server` - OAuth discovery
- `https://vishalk17.cloudwithme.dev/ory/*` - Ory Hydra public API

## Troubleshooting

### View Logs
```bash
# MCP server
kubectl logs -f deployment/mcp-service-indian-store

# Ory Hydra
kubectl logs -f deployment/ory-hydra

# PostgreSQL
kubectl logs -f deployment/postgres
```

### Check Gateway Status
```bash
kubectl describe gateway mcp-service-indian-store-gateway
kubectl describe httproute ory-hydra-route
kubectl describe httproute mcp-service-indian-store-route
```

### Port Forward for Local Testing
```bash
# MCP server
kubectl port-forward svc/mcp-service-indian-store 8080:80

# Ory Hydra admin
kubectl port-forward svc/ory-hydra-admin 4445:4445
```

## Cleanup

```bash
# Remove everything
kubectl delete -f gateway.yaml
kubectl delete -f deployement.yaml
kubectl delete -f configmap.yaml
helm uninstall ory-hydra -n default
kubectl delete -f hydra/postgres-sts.yaml
```

## Notes

- PostgreSQL uses `emptyDir` volume (data lost on pod restart) - for production, use PersistentVolumeClaim
- TLS certificate `vishalk17.cloudwithme.dev-tls` must exist before deploying Gateway
- All secrets should be rotated regularly in production
- Consider using external secret management (Vault, AWS Secrets Manager, etc.) for production
