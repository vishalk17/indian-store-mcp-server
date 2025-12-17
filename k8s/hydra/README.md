# Ory Hydra Setup

This directory contains everything needed to deploy Ory Hydra OAuth server.

## Files

- `postgres-sts.yaml` - PostgreSQL database for Hydra (⚠️ uses emptyDir, data lost on restart)
- `ory-hydra-values.yaml` - Helm values for Ory Hydra installation

## Prerequisites

1. Add Ory Helm repository:
   ```bash
   helm repo add ory https://k8s.ory.sh/helm/charts
   helm repo update
   ```

## Deployment

### 1. Deploy PostgreSQL
```bash
kubectl apply -f postgres-sts.yaml
```

Wait for it to be ready:
```bash
kubectl wait --for=condition=ready pod -l app=postgres --timeout=60s
```

### 2. Install Ory Hydra
```bash
helm install ory-hydra ory/hydra -f ory-hydra-values.yaml -n default
```

Wait for it to be ready:
```bash
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=hydra --timeout=120s
```

## Verify

Check Hydra is running:
```bash
kubectl get pods -l app.kubernetes.io/name=hydra
kubectl logs -l app.kubernetes.io/name=hydra
```

## Update

To update Hydra configuration:
```bash
helm upgrade ory-hydra ory/hydra -f ory-hydra-values.yaml -n default
```

## Cleanup

```bash
helm uninstall ory-hydra -n default
kubectl delete -f postgres-sts.yaml
```

## Configuration

All configuration is in `ory-hydra-values.yaml`. Key settings:

- **PostgreSQL DSN**: Must match credentials in `postgres-sts.yaml`
- **Issuer URL**: Your public domain
- **Secrets**: System and cookie secrets (generate with `openssl rand -base64 32`)
- **Login/Consent URLs**: Endpoints you need to implement for user authentication
