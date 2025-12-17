# Installation Guide

Complete step-by-step guide to deploy the Indian Store MCP Server with Ory Hydra OAuth on Kubernetes.

## 📋 Prerequisites

### Required

- **Kubernetes cluster** (microk8s, k3s, GKE, EKS, AKS, or any K8s distribution)
- **kubectl** configured and connected to your cluster
- **Helm 3** installed
- **Domain name** with DNS pointing to your cluster
- **TLS certificate** for your domain
- **Gateway API** installed in your cluster

### Optional

- **Docker** (for building custom images)
- **Python 3** with bcrypt (for generating password hashes)

---

## 🚀 Quick Installation

```bash
# Clone or navigate to the repository
cd /home/vishalk17/ory/store

# 1. Deploy PostgreSQL
kubectl apply -f k8s/hydra/postgres-sts.yaml

# 2. Install Ory Hydra
export SYSTEM_SECRET=$(openssl rand -base64 32)
export COOKIE_SECRET=$(openssl rand -base64 32)

helm repo add ory https://k8s.ory.sh/helm/charts
helm repo update

# Update k8s/hydra/ory-hydra-values.yaml with your domain
# Then install:
helm install ory-hydra ory/hydra \
  --namespace default \
  --values k8s/hydra/ory-hydra-values.yaml

# 3. Deploy MCP Server
# Update k8s/configmap.yaml with your domain and database URL
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/deployement.yaml

# 4. Deploy Gateway
# Update k8s/gateway.yaml with your domain and TLS certificate
kubectl apply -f k8s/gateway.yaml

# 5. Create first user
python3 -c "import bcrypt; print(bcrypt.hashpw(b'admin123', bcrypt.gensalt(rounds=10)).decode())"
# Copy the hash, then:
kubectl exec -it deployment/postgres -- psql -U ory_hydra -d ory_hydra -c \
  "INSERT INTO users (email, password_hash, name) VALUES ('admin@example.com', '\$2a\$10\$HASH_HERE', 'Admin');"

# 6. Verify
kubectl get pods
kubectl get httproute
curl https://YOUR_DOMAIN/health
```

---

## 📖 Detailed Installation Steps

### Step 1: Prepare Your Environment

#### 1.1 Verify Kubernetes Cluster

```bash
kubectl cluster-info
kubectl get nodes
```

**Expected output**: Your cluster should be running and nodes should be Ready.

#### 1.2 Install Gateway API (if not already installed)

```bash
# For most Kubernetes distributions:
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.0.0/standard-install.yaml

# For microk8s:
microk8s enable gateway
```

#### 1.3 Verify Helm Installation

```bash
helm version
```

**Expected**: Helm v3.x or later

---

### Step 2: Deploy PostgreSQL

PostgreSQL is used by both Ory Hydra (OAuth data) and the MCP Server (users).

#### 2.1 Deploy PostgreSQL

```bash
kubectl apply -f k8s/hydra/postgres-sts.yaml
```

#### 2.2 Verify PostgreSQL is Running

```bash
kubectl get pods | grep postgres
# Should show: postgres-xxx    1/1     Running

kubectl logs deployment/postgres
# Should show: database system is ready to accept connections
```

#### 2.3 Test PostgreSQL Connection

```bash
kubectl exec -it deployment/postgres -- psql -U ory_hydra -d ory_hydra -c "SELECT version();"
```

**Expected**: PostgreSQL version information

**Credentials**:
- Database: `ory_hydra`
- User: `ory_hydra`
- Password: `ory_password_123`

**⚠️ Production Note**: Change these credentials! Update:
- `k8s/hydra/postgres-sts.yaml` - PostgreSQL env vars
- `k8s/hydra/ory-hydra-values.yaml` - Hydra DSN
- `k8s/configmap.yaml` - MCP Server DATABASE_URL

---

### Step 3: Deploy Ory Hydra

#### 3.1 Generate Secrets

```bash
export SYSTEM_SECRET=$(openssl rand -base64 32)
export COOKIE_SECRET=$(openssl rand -base64 32)

echo "SYSTEM_SECRET: $SYSTEM_SECRET"
echo "COOKIE_SECRET: $COOKIE_SECRET"
```

**Save these secrets!** You'll need them for the Helm values.

#### 3.2 Configure Ory Hydra Values

Edit `k8s/hydra/ory-hydra-values.yaml`:

```yaml
hydra:
  config:
    # IMPORTANT: Update with your actual domain
    dsn: postgres://ory_hydra:ory_password_123@postgres.default.svc.cluster.local:5432/ory_hydra?sslmode=disable
    
    urls:
      self:
        issuer: https://YOUR_DOMAIN/ory  # ← Change this
      login: https://YOUR_DOMAIN/login   # ← Change this
      consent: https://YOUR_DOMAIN/consent  # ← Change this
      error: https://YOUR_DOMAIN/oauth2/fallbacks/error  # ← Change this
    
    secrets:
      system:
        - YOUR_SYSTEM_SECRET_HERE  # ← Paste from above
      cookie:
        - YOUR_COOKIE_SECRET_HERE  # ← Paste from above
    
    oauth2:
      expose_internal_errors: true  # Set to false in production
    
    serve:
      public:
        port: 4444
      admin:
        port: 4445

  automigration:
    enabled: true
    
service:
  public:
    enabled: true
    type: ClusterIP
    port: 4444
  admin:
    enabled: true
    type: ClusterIP
    port: 4445

maester:
  enabled: true
```

#### 3.3 Add Helm Repository

```bash
helm repo add ory https://k8s.ory.sh/helm/charts
helm repo update
```

#### 3.4 Install Ory Hydra

```bash
helm install ory-hydra ory/hydra \
  --namespace default \
  --values k8s/hydra/ory-hydra-values.yaml
```

#### 3.5 Verify Ory Hydra Installation

```bash
# Check pods
kubectl get pods -l app.kubernetes.io/name=hydra

# Check services
kubectl get svc | grep ory-hydra

# Check logs
kubectl logs -l app.kubernetes.io/name=hydra --tail=50
```

**Expected pods**:
- `ory-hydra-xxx` - Running (1/1)
- `ory-hydra-automigrate-xxx` - Completed (0/1)
- `ory-hydra-hydra-maester-xxx` - Running (1/1)

**Expected services**:
- `ory-hydra-public` - ClusterIP port 4444
- `ory-hydra-admin` - ClusterIP port 4445

#### 3.6 Test Ory Hydra

```bash
# Port forward (temporary test)
kubectl port-forward svc/ory-hydra-public 4444:4444 &

# Test health endpoint
curl http://localhost:4444/health/ready

# Stop port forward
killall kubectl
```

---

### Step 4: Configure MCP Server

#### 4.1 Update ConfigMap

Edit `k8s/configmap.yaml`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: indian-store-config
  namespace: default
data:
  PORT: "8080"
  
  # IMPORTANT: Update with your domain
  ORY_URL: "https://YOUR_DOMAIN/ory"
  ORY_CALLBACK_URL: "https://YOUR_DOMAIN/oauth/callback"
  
  # Internal Kubernetes URLs (usually don't need to change)
  ORY_INTERNAL_URL: "http://ory-hydra-public.default.svc.cluster.local:4444"
  ORY_ADMIN_URL: "http://ory-hydra-admin.default.svc.cluster.local:4445"
  
  # Database connection (update if you changed PostgreSQL credentials)
  DATABASE_URL: "postgres://ory_hydra:ory_password_123@postgres.default.svc.cluster.local:5432/ory_hydra?sslmode=disable"
---
apiVersion: v1
kind: Secret
metadata:
  name: indian-store-secrets
  namespace: default
type: Opaque
stringData:
  # JWT Secret (optional - for internal use)
  JWT_SECRET: "GENERATE_WITH_openssl_rand_base64_32"
```

#### 4.2 Apply Configuration

```bash
kubectl apply -f k8s/configmap.yaml
```

---

### Step 5: Deploy MCP Server

#### 5.1 Apply Deployment

```bash
kubectl apply -f k8s/deployement.yaml
```

#### 5.2 Verify Deployment

```bash
# Check pods
kubectl get pods -l app=mcp-service-indian-store

# Check service
kubectl get svc mcp-service-indian-store

# Check logs
kubectl logs -l app=mcp-service-indian-store --tail=50
```

**Expected in logs**:
```
Configuration loaded successfully
Ory client initialized with URL: https://YOUR_DOMAIN/ory
User store initialized with database connection
Indian Store MCP Server with Ory OAuth starting on 0.0.0.0:8080
```

**Default user created**: `admin@indian-store.com` / `admin123`

---

### Step 6: Configure Gateway

#### 6.1 Prepare TLS Certificate

You need a TLS certificate for your domain. Options:

**Option A: Let's Encrypt (recommended)**
```bash
# Install cert-manager
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml

# Create ClusterIssuer
kubectl apply -f - <<EOF
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: YOUR_EMAIL@example.com
    privateKeySecretRef:
      name: letsencrypt-prod
    solvers:
    - http01:
        ingress:
          class: nginx
EOF

# Certificate will be auto-created by Gateway
```

**Option B: Existing Certificate**
```bash
kubectl create secret tls YOUR_DOMAIN-tls \
  --cert=/path/to/cert.pem \
  --key=/path/to/key.pem
```

#### 6.2 Update Gateway Configuration

Edit `k8s/gateway.yaml`:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: mcp-service-indian-store-gateway
  namespace: default
spec:
  gatewayClassName: YOUR_GATEWAY_CLASS  # e.g., cilium, istio, nginx
  listeners:
    - name: https
      protocol: HTTPS
      port: 443
      hostname: "YOUR_DOMAIN"  # ← Change this
      tls:
        mode: Terminate
        certificateRefs:
          - kind: Secret
            name: YOUR_DOMAIN-tls  # ← Your TLS secret name
```

#### 6.3 Apply Gateway Configuration

```bash
kubectl apply -f k8s/gateway.yaml
```

#### 6.4 Verify Gateway

```bash
kubectl get gateway
kubectl get httproute

kubectl describe gateway mcp-service-indian-store-gateway
```

**Expected**: Gateway should be "Programmed" and "Accepted"

---

### Step 7: Create First User

#### 7.1 Generate Password Hash

```bash
python3 -c "import bcrypt; print(bcrypt.hashpw(b'YOUR_PASSWORD', bcrypt.gensalt(rounds=10)).decode())"
```

**Example output**: `$2a$10$abcdefghijklmnopqrstuvwxyz...`

#### 7.2 Insert User into Database

```bash
kubectl exec -it deployment/postgres -- psql -U ory_hydra -d ory_hydra -c \
  "INSERT INTO users (email, password_hash, name) 
   VALUES ('admin@example.com', '\$2a\$10\$PASTE_HASH_HERE', 'Administrator');"
```

**⚠️ Important**: Escape the dollar signs in the hash with backslashes: `\$2a\$10\$...`

#### 7.3 Verify User Creation

```bash
kubectl exec -it deployment/postgres -- psql -U ory_hydra -d ory_hydra -c \
  "SELECT email, name, created_at FROM users;"
```

---

### Step 8: Verify Installation

#### 8.1 Check All Components

```bash
kubectl get pods -n default
```

**Expected**:
```
NAME                                       READY   STATUS
mcp-service-indian-store-xxx               1/1     Running
ory-hydra-xxx                              1/1     Running
ory-hydra-hydra-maester-xxx                1/1     Running
postgres-xxx                               1/1     Running
```

#### 8.2 Test Health Endpoint

```bash
curl https://YOUR_DOMAIN/health
```

**Expected**: `{"status":"ok","server":"indian-store-mcp-server"}`

#### 8.3 Test OAuth Discovery

```bash
curl https://YOUR_DOMAIN/.well-known/oauth-authorization-server | jq
```

**Expected**: JSON with OAuth endpoints

#### 8.4 Test Login Page (Manual)

Open in browser:
```
https://YOUR_DOMAIN/login?login_challenge=test
```

**Expected**: Should show login form (will show "Missing login_challenge" error, but page should render)

---

## 🧪 Testing with MCP Clients

### Test with ChatGPT

1. Open ChatGPT (web or desktop)
2. Go to Settings → Integrations
3. Click "Add MCP Server"
4. Enter URL: `https://YOUR_DOMAIN`
5. Browser redirects to login page
6. Enter credentials (the user you created)
7. Click "Sign In"
8. Should see "Connected" in ChatGPT

### Test with Claude Desktop

1. Open Claude Desktop
2. Go to Settings → Developer
3. Add MCP Server: `https://YOUR_DOMAIN`
4. Follow login flow (same as ChatGPT)

---

## 🔧 Post-Installation Configuration

### Change Default Admin Password

```bash
# Generate new hash
python3 -c "import bcrypt; print(bcrypt.hashpw(b'NEW_PASSWORD', bcrypt.gensalt(rounds=10)).decode())"

# Update in database
kubectl exec -it deployment/postgres -- psql -U ory_hydra -d ory_hydra -c \
  "UPDATE users SET password_hash = '\$2a\$10\$NEW_HASH' WHERE email = 'admin@indian-store.com';"
```

### Add More Users


```bash
# Generate hash
python3 -c "import bcrypt; print(bcrypt.hashpw(b'user_password', bcrypt.gensalt(rounds=10)).decode())"

# Insert user
kubectl exec -it deployment/postgres -- psql -U ory_hydra -d ory_hydra -c \
  "INSERT INTO users (email, password_hash, name) VALUES ('user@example.com', '\$2a\$10\$HASH', 'User Name');"
```

### Enable Persistent Storage (Production)

**Current setup uses `emptyDir`** - data lost on pod restart.

**For production**, update `k8s/hydra/postgres-sts.yaml`:

```yaml
volumes:
  - name: postgres-storage
    persistentVolumeClaim:
      claimName: postgres-pvc
```

Create PVC:
```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: postgres-pvc
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
EOF
```

---

## 🚨 Troubleshooting

### Pod Not Starting

```bash
# Check pod status
kubectl get pods

# Check pod events
kubectl describe pod POD_NAME

# Check logs
kubectl logs POD_NAME
```

### Database Connection Issues

```bash
# Test PostgreSQL connection
kubectl exec -it deployment/postgres -- psql -U ory_hydra -d ory_hydra -c "SELECT 1;"

# Check if database exists
kubectl exec -it deployment/postgres -- psql -U ory_hydra -c "\l"
```

### Ory Hydra Not Working

```bash
# Check Ory logs
kubectl logs -l app.kubernetes.io/name=hydra --tail=100

# Check if migration completed
kubectl logs job/ory-hydra-automigrate

# Port forward and test locally
kubectl port-forward svc/ory-hydra-public 4444:4444
curl http://localhost:4444/health/ready
```

### Gateway Not Working

```bash
# Check gateway status
kubectl describe gateway mcp-service-indian-store-gateway

# Check HTTPRoute
kubectl describe httproute

# Check TLS certificate
kubectl get secret YOUR_DOMAIN-tls
kubectl describe secret YOUR_DOMAIN-tls
```

### 404 Errors

- **Problem**: Routes not configured correctly
- **Solution**: Verify HTTPRoute is created and Gateway is referenced correctly

### 502 Bad Gateway

- **Problem**: Backend service not responding
- **Solution**: Check if pods are running and services exist

---

## 🔄 Updating

### Update MCP Server

```bash
# Build new image
docker build -t ghcr.io/YOUR_USERNAME/indian-store-mcp-server:VERSION .
docker push ghcr.io/YOUR_USERNAME/indian-store-mcp-server:VERSION

# Update deployment
kubectl set image deployment/mcp-service-indian-store \
  indian-store-mcp=ghcr.io/YOUR_USERNAME/indian-store-mcp-server:VERSION

# Or edit deployment.yaml and apply
kubectl apply -f k8s/deployement.yaml
```

### Update Ory Hydra

```bash
helm upgrade ory-hydra ory/hydra \
  --namespace default \
  --values k8s/hydra/ory-hydra-values.yaml
```

### Update Configuration

```bash
# Edit configmap.yaml
kubectl apply -f k8s/configmap.yaml

# Restart pods to pick up new config
kubectl rollout restart deployment/mcp-service-indian-store
```

---

## 🗑️ Uninstalling

```bash
# Remove MCP Server
kubectl delete -f k8s/gateway.yaml
kubectl delete -f k8s/deployement.yaml
kubectl delete -f k8s/configmap.yaml

# Remove Ory Hydra
helm uninstall ory-hydra -n default

# Remove PostgreSQL
kubectl delete -f k8s/hydra/postgres-sts.yaml

# Remove PVC (if created)
kubectl delete pvc postgres-pvc
```

---

## 📚 Next Steps

- Read [AUTHENTICATION.md](./AUTHENTICATION.md) to understand the security model
- Check [README.md](./README.md) for architecture details

---

## ✅ Installation Checklist

- [ ] Kubernetes cluster running
- [ ] kubectl configured
- [ ] Helm 3 installed
- [ ] Gateway API installed
- [ ] Domain DNS configured
- [ ] TLS certificate ready
- [ ] PostgreSQL deployed and running
- [ ] Ory Hydra deployed and healthy
- [ ] MCP Server deployed and running
- [ ] Gateway configured with TLS
- [ ] First user created
- [ ] Health endpoint responding
- [ ] OAuth discovery working
- [ ] Login page accessible
- [ ] Tested with ChatGPT/Claude

**Congratulations! Your MCP server with Ory OAuth is now running! 🎉**
