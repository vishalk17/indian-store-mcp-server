#!/bin/bash
set -e

echo "=== OAuth2 + Social Login Deployment ==="
echo ""

# 1. Apply secrets
echo "1. Applying secrets..."
kubectl apply -f k8s/secrets.yaml

# 2. Deploy Postgres
echo "2. Deploying Postgres..."
helm repo add bitnami https://charts.bitnami.com/bitnami 2>/dev/null || true
helm upgrade --install postgres bitnami/postgresql \
  --set auth.username=ory \
  --set auth.password=changeme \
  --set auth.database=kratos \
  --set primary.initdb.scripts.init\\.sql="CREATE DATABASE hydra\\;"

echo "Waiting for Postgres..."
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=postgresql --timeout=120s

# 3. Deploy Ory Kratos
echo "3. Deploying Ory Kratos..."
helm repo add ory https://k8s.ory.sh/helm/charts 2>/dev/null || true
helm upgrade --install kratos ory/kratos -f k8s/kratos-values.yaml

echo "Waiting for Kratos..."
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=kratos --timeout=120s

# 4. Deploy Kratos UI
echo "4. Deploying Kratos SelfService UI..."
helm upgrade --install kratos-ui ory/kratos-selfservice-ui-node -f k8s/kratos-ui-values.yaml

# 5. Deploy Hydra
echo "5. Deploying Ory Hydra..."
helm upgrade --install hydra ory/hydra -f k8s/hydra-values.yaml

echo "Waiting for Hydra..."
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=hydra --timeout=120s

# 6. Apply Gateway + HTTPRoutes
echo "6. Applying Gateway + HTTPRoutes..."
kubectl apply -f k8s/gateway.yaml

# 7. Deploy MCP server
echo "7. Deploying MCP server..."
kubectl apply -f k8s/deployement.yaml

echo ""
echo "=== Deployment Complete ==="
echo ""
echo "Verify with:"
echo "  kubectl get pods"
echo "  kubectl get httproutes"
echo "  curl https://vishalk17.cloudwithme.dev/.well-known/openid-configuration"
echo ""
echo "Add a user:"
echo "  kubectl exec -it deployment/kratos -- kratos create identity \\"
echo "    --endpoint http://localhost:4434 \\"
echo "    --format json-pretty \\"
echo "    user@example.com \\"
echo "    --schema-id default \\"
echo "    --trait email=user@example.com"
