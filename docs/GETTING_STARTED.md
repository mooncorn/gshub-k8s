# Getting Started with GSHUB Game Server Hosting Platform

This guide walks you through setting up the GSHUB platform in both development (k3d) and production (k3s) environments.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Development Setup (k3d)](#development-setup-k3d)
- [Production Setup (k3s)](#production-setup-k3s)
- [Verification](#verification)
- [Troubleshooting](#troubleshooting)

---

## Prerequisites

### All Environments

- **kubectl**: Kubernetes command-line tool
  ```bash
  # Linux
  curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
  sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl

  # Verify
  kubectl version --client
  ```

- **Helm**: Package manager for Kubernetes
  ```bash
  curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

  # Verify
  helm version
  ```

### Development Only

- **Docker**: Container runtime for k3d
  ```bash
  # Install Docker (Ubuntu/Debian)
  curl -fsSL https://get.docker.com | sh
  sudo usermod -aG docker $USER
  # Log out and back in for group changes to take effect

  # Verify
  docker version
  ```

- **k3d**: Lightweight Kubernetes in Docker
  ```bash
  curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash

  # Verify
  k3d version
  ```

### Production Only

- **Multiple Servers**:
  - 1x Master node (4 vCPU, 8 GB RAM, 100 GB SSD)
  - 1+ Agent nodes (8-16 vCPU, 32-64 GB RAM, 200 GB NVMe, **public IPs required**)

- **Operating System**: Ubuntu 22.04 LTS or similar (all nodes)

- **Network Requirements**:
  - Master node: Public IP OR accessible via VPN/private network
  - Agent nodes: **Public IPs required** (for player connections to game servers)
  - Port 6443 access from agents to master (for Kubernetes API)

---

## Development Setup (k3d)

Perfect for local development and testing on a single machine.

### Step 1: Create k3d Cluster

```bash
# Create cluster with port range exposed for game servers
k3d cluster create gshub-dev \
  --api-port 6443 \
  --servers 1 \
  --agents 0 \
  --port "7000-7050:7000-7050@server:0" \
  --k3s-arg "--disable=traefik@server:0" \
  --k3s-arg "--disable=servicelb@server:0"

# Verify cluster is running
kubectl cluster-info
kubectl get nodes
```

**What this does**:
- Creates a single-node k3s cluster in Docker
- Exposes ports 7000-7050 from the container to localhost (for game server connections)
- Disables Traefik (using Cloudflare Tunnel) and ServiceLB (not needed)

### Step 2: Label the k3d Node

In k3d, the single node serves both as control plane and game compute:

```bash
# Get the node name
NODE_NAME=$(kubectl get nodes -o jsonpath='{.items[0].metadata.name}')

# Label node for control plane workload (serves both purposes in k3d)
kubectl label node $NODE_NAME workload-type=control-plane
kubectl label node $NODE_NAME node-role.kubernetes.io/gameserver=true
kubectl label node $NODE_NAME platform.io/public-ip=localhost

# Verify labels
kubectl get nodes --show-labels | grep -E "workload-type|gameserver|public-ip"
```

**Why these labels**:
- `workload-type=control-plane` - Allows API, PostgreSQL, Cloudflare, AND GameServers to schedule
- `node-role.kubernetes.io/gameserver=true` - Required by GameServer node affinity
- `platform.io/public-ip=localhost` - Makes players connect to `localhost` in development

### Step 3: Install Agones (Development)

```bash
# Add Agones Helm repository
helm repo add agones https://agones.dev/chart/stable
helm repo update

# Install Agones with development configuration
bash k8s/overlays/dev/agones-install.sh

# Wait for Agones to be ready
kubectl wait --for=condition=available deployment/agones-controller \
  -n agones-system --timeout=300s

# Verify installation
kubectl get pods -n agones-system
```

**Expected output**:
```
NAME                                 READY   STATUS    RESTARTS   AGE
agones-controller-xxx                1/1     Running   0          30s
agones-allocator-xxx                 1/1     Running   0          30s
agones-extensions-xxx                1/1     Running   0          30s
```

### Step 4: Deploy GSHUB Platform

```bash
# Deploy all resources with Kustomize
kubectl apply -k k8s/overlays/dev

# Wait for PostgreSQL to be ready
kubectl wait --for=condition=ready pod -l app=postgresql \
  -n gshub --timeout=180s

# Wait for API to be ready
kubectl wait --for=condition=ready pod -l app=api \
  -n gshub --timeout=120s

# Verify all pods are running
kubectl get pods -n gshub
```

### Step 5: Development Workflow

**Using Skaffold (recommended for development)**:

```bash
# Navigate to API directory
cd api

# Start development with hot-reload
skaffold dev

# This will:
# - Build the API container
# - Deploy to k3d cluster
# - Watch for code changes and auto-reload
# - Port-forward API to localhost:8080
```

**Access the API**:
```bash
# API is available at
http://localhost:8080

# Test health endpoint
curl http://localhost:8080/health
```

**Connecting to game servers**:
- Game servers will be accessible at `localhost:7000-7050`
- Example: Minecraft server at `localhost:7042`

### Resetting Development Environment

```bash
# Delete cluster
k3d cluster delete gshub-dev

# Recreate from scratch
k3d cluster create gshub-dev \
  --api-port 6443 \
  --servers 1 \
  --agents 0 \
  --port "7000-7050:7000-7050@server:0" \
  --k3s-arg "--disable=traefik@server:0" \
  --k3s-arg "--disable=servicelb@server:0"

# Label nodes, install Agones, and redeploy
NODE_NAME=$(kubectl get nodes -o jsonpath='{.items[0].metadata.name}')
kubectl label node $NODE_NAME workload-type=control-plane
kubectl label node $NODE_NAME node-role.kubernetes.io/gameserver=true
kubectl label node $NODE_NAME platform.io/public-ip=localhost

bash k8s/overlays/dev/agones-install.sh
kubectl apply -k k8s/overlays/dev
```

---

## Production Setup (k3s)

For production deployment with dedicated master and agent nodes.

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                     Master Node (Private)                   │
│  ┌──────────┐  ┌────────────┐  ┌─────────────────┐         │
│  │   API    │  │ PostgreSQL │  │ Cloudflare      │         │
│  │ Service  │  │  Database  │  │ Tunnel          │         │
│  └──────────┘  └────────────┘  └─────────────────┘         │
│                                                             │
│  ┌─────────────────────────────────────────────┐           │
│  │      Agones Controller (Manages GameServers)│           │
│  └─────────────────────────────────────────────┘           │
└─────────────────────────────────────────────────────────────┘
                          │
                          │ K8s API (6443)
                          │
          ┌───────────────┴───────────────┐
          │                               │
┌─────────▼───────────┐         ┌────────▼────────────┐
│  Agent Node 1       │         │  Agent Node 2       │
│  (Public IP)        │         │  (Public IP)        │
│                     │         │                     │
│  ┌──────────────┐   │         │  ┌──────────────┐   │
│  │ GameServer 1 │   │         │  │ GameServer 3 │   │
│  │ Port: 7042   │   │         │  │ Port: 7156   │   │
│  └──────────────┘   │         │  └──────────────┘   │
│  ┌──────────────┐   │         │  ┌──────────────┐   │
│  │ GameServer 2 │   │         │  │ GameServer 4 │   │
│  │ Port: 7089   │   │         │  │ Port: 7231   │   │
│  └──────────────┘   │         │  └──────────────┘   │
└─────────────────────┘         └─────────────────────┘
         │                               │
         │ Players connect directly      │
         └───────────────────────────────┘
        (PublicIP:Port, e.g., 45.x.x.10:7042)
```

### Step 1: Prepare Servers

**On all nodes**:

```bash
# Update system
sudo apt-get update && sudo apt-get upgrade -y

# Install required packages
sudo apt-get install -y curl wget

# Disable swap (required by Kubernetes)
sudo swapoff -a
sudo sed -i '/ swap / s/^/#/' /etc/fstab

# Enable kernel modules
cat <<EOF | sudo tee /etc/modules-load.d/k8s.conf
overlay
br_netfilter
EOF

sudo modprobe overlay
sudo modprobe br_netfilter

# Configure sysctl
cat <<EOF | sudo tee /etc/sysctl.d/k8s.conf
net.bridge.bridge-nf-call-iptables  = 1
net.bridge.bridge-nf-call-ip6tables = 1
net.ipv4.ip_forward                 = 1
EOF

sudo sysctl --system
```

### Step 2: Install k3s on Master Node

**On the master node**:

```bash
# Get the master node's public IP (or VPN IP if using VPN)
MASTER_IP=$(curl -4 -s ifconfig.me)
echo "Master IP: $MASTER_IP"

# Install k3s as server with specific configuration
curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="server \
  --disable traefik \
  --disable servicelb \
  --node-taint node-role.kubernetes.io/master=true:NoSchedule \
  --tls-san $MASTER_IP \
  --advertise-address $MASTER_IP" sh -

# Wait for k3s to be ready
sudo systemctl status k3s

# Make kubectl accessible for current user
mkdir -p ~/.kube
sudo cp /etc/rancher/k3s/k3s.yaml ~/.kube/config
sudo chown $(id -u):$(id -g) ~/.kube/config

# Set KUBECONFIG environment variable
echo 'export KUBECONFIG=~/.kube/config' >> ~/.bashrc
source ~/.bashrc

# Verify master is ready
kubectl get nodes
```

**Expected output**:
```
NAME            STATUS   ROLES                  AGE   VERSION
master-node     Ready    control-plane,master   30s   v1.28.x+k3s1
```

### Step 3: Configure Firewall on Master

```bash
# Allow SSH (if not already allowed)
sudo ufw allow ssh

# Allow k3s API from specific agent IPs
sudo ufw allow from <agent-1-public-ip> to any port 6443 comment 'k3s API - agent-01'
sudo ufw allow from <agent-2-public-ip> to any port 6443 comment 'k3s API - agent-02'

# Enable firewall
sudo ufw --force enable

# Verify
sudo ufw status numbered
```

### Step 4: Label Master Node

```bash
# Get master hostname
MASTER_HOSTNAME=$(hostname)

# Label master node
kubectl label node $MASTER_HOSTNAME \
  node-role.kubernetes.io/master=true \
  workload-type=control-plane

# Verify labels
kubectl get nodes --show-labels | grep control-plane
```

### Step 5: Get Join Token for Agent Nodes

**On the master node**:

```bash
# Save the join token
sudo cat /var/lib/rancher/k3s/server/node-token

# Save this token - you'll need it for agent nodes!
```

### Step 6: Install k3s on Agent Nodes

**On each agent node**:

```bash
# Set master IP and token (replace with your values)
MASTER_IP="<master-public-ip-or-vpn-ip>"
JOIN_TOKEN="<token-from-master>"

# Install k3s as agent
curl -sfL https://get.k3s.io | \
  K3S_URL="https://${MASTER_IP}:6443" \
  K3S_TOKEN="${JOIN_TOKEN}" \
  INSTALL_K3S_EXEC="agent" sh -

# Verify agent is running
sudo systemctl status k3s-agent
```

### Step 7: Label Agent Nodes

**On the master node** (from where you have kubectl access):

```bash
# Wait for agent nodes to appear
kubectl get nodes -w
# Press Ctrl+C once all agents are visible

# For each agent node, label it with its PUBLIC IP (for player connections)
# Replace values with your actual hostnames and public IPs

# Agent 1
kubectl label node agent-01 \
  node-role.kubernetes.io/gameserver=true \
  workload-type=game-compute \
  platform.io/public-ip=45.x.x.10

# Agent 2
kubectl label node agent-02 \
  node-role.kubernetes.io/gameserver=true \
  workload-type=game-compute \
  platform.io/public-ip=45.y.y.20

# Verify all labels
kubectl get nodes --show-labels | grep -E "master|gameserver"
```

**Critical**: The `platform.io/public-ip` label **MUST** be the agent's **public IP** because this is what players use to connect to game servers!

### Step 8: Configure Firewall on Agent Nodes

**On each agent node**:

```bash
# Allow game server port range (7000-8000)
sudo ufw allow 7000:8000/tcp
sudo ufw allow 7000:8000/udp

# Allow k3s communication from master
sudo ufw allow from <master-ip> to any port 6443

# Enable firewall
sudo ufw --force enable

# Verify rules
sudo ufw status numbered
```

### Step 9: Install Agones (Production)

**On the master node** (or wherever you have kubectl access):

```bash
# Add Agones Helm repository
helm repo add agones https://agones.dev/chart/stable
helm repo update

# Install Agones with production configuration
bash k8s/overlays/prod/agones-install.sh

# Wait for Agones to be ready
kubectl wait --for=condition=available deployment/agones-controller \
  -n agones-system --timeout=300s

# Verify installation
kubectl get pods -n agones-system -o wide
```

**Verify Agones controller is on master**:
```
NAME                                 READY   STATUS    NODE
agones-controller-xxx                1/1     Running   master-node
agones-allocator-xxx                 1/1     Running   master-node
agones-extensions-xxx                1/1     Running   master-node
```

### Step 10: Configure Secrets

**Edit the secrets file**:

```bash
# Edit secrets with your values
nano k8s/base/secrets.yaml
```

**Update the following values** (base64 encoded):

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: gshub-secrets
  namespace: gshub
type: Opaque
data:
  db-password: <base64-encoded-password>
  jwt-secret: <base64-encoded-secret>
  mailersend-api-key: <base64-encoded-key>
  stripe-secret-key: <base64-encoded-key>
  stripe-webhook-secret: <base64-encoded-secret>
  tunnel-token: <base64-encoded-token>
```

**To encode values**:
```bash
echo -n "your-secret-value" | base64
```

### Step 11: Deploy GSHUB Platform

```bash
# Deploy all resources with Kustomize
kubectl apply -k k8s/overlays/prod

# Wait for PostgreSQL to be ready
kubectl wait --for=condition=ready pod -l app=postgresql \
  -n gshub --timeout=180s

# Wait for API to be ready
kubectl wait --for=condition=ready pod -l app=api \
  -n gshub --timeout=120s

# Verify all pods
kubectl get pods -n gshub -o wide
```

**Expected output**:
```
NAME                READY   STATUS    RESTARTS   AGE   NODE
api-xxx             1/1     Running   0          2m    master-node
postgresql-0        1/1     Running   0          3m    master-node
cloudflared-xxx     1/1     Running   0          1m    master-node
```

---

## Verification

### Test API Health

**Development (k3d)**:
```bash
curl http://localhost:8080/health
```

**Production (k3s)**:
```bash
# Port forward to access API locally
kubectl port-forward -n gshub svc/api 8080:8080 &

# Test health endpoint
curl http://localhost:8080/health
```

**Expected response**:
```json
{"status":"ok"}
```

### Create a Test Game Server

```bash
# Create a test Minecraft server (requires authentication)
curl -X POST http://localhost:8080/api/v1/servers \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <your-jwt-token>" \
  -d '{
    "display_name": "Test Server",
    "subdomain": "test-mc",
    "game": "minecraft",
    "plan": "small"
  }'
```

### Watch GameServer Creation

```bash
# Watch GameServers being created
kubectl get gameservers -n gshub -w

# Expected progression:
# NAME          STATE       ADDRESS      PORT   NODE
# server-xxx    Creating
# server-xxx    Starting    10.42.1.5           agent-01
# server-xxx    Ready       10.42.1.5    7042   agent-01
```

### Test Player Connection

**Development (k3d)**:
```bash
# Connect to: localhost:<port>
nc -zv localhost 7042
```

**Production (k3s)**:
```bash
# Connect to agent's public IP
nc -zv 45.x.x.10 7042
```

---

## Troubleshooting

### Pods Not Scheduling ("didn't match Pod's node affinity/selector")

**Cause**: Node is missing required labels.

**Solution**:
```bash
# Get node name
NODE_NAME=$(kubectl get nodes -o jsonpath='{.items[0].metadata.name}')

# Add required labels
kubectl label node $NODE_NAME workload-type=control-plane --overwrite
kubectl label node $NODE_NAME node-role.kubernetes.io/gameserver=true --overwrite
kubectl label node $NODE_NAME platform.io/public-ip=localhost --overwrite
```

### GameServer Stuck in "Creating" State

**Check GameServer events**:
```bash
kubectl describe gameserver <name> -n gshub
```

**Common issues**:
- No agent nodes available with correct labels
- Port range exhausted on all agents
- Image pull failure

### API Cannot Connect to Database

**Check PostgreSQL status**:
```bash
kubectl get pods -n gshub -l app=postgresql
kubectl logs -n gshub -l app=postgresql --tail=50
```

**Check API logs**:
```bash
kubectl logs -n gshub -l app=api --tail=50 | grep -i "database\|postgres"
```

### Player Cannot Connect to Game Server

**Diagnostic steps**:

```bash
# 1. Check GameServer is Ready
kubectl get gameserver <name> -n gshub

# 2. Get node and port info
kubectl get gameserver <name> -n gshub -o yaml | grep -A5 status

# 3. Check node public IP label
NODE_NAME=$(kubectl get gameserver <name> -n gshub -o jsonpath='{.status.nodeName}')
kubectl get node $NODE_NAME -o jsonpath='{.metadata.labels.platform\.io/public-ip}'

# 4. Test connectivity
PUBLIC_IP=$(kubectl get node $NODE_NAME -o jsonpath='{.metadata.labels.platform\.io/public-ip}')
HOST_PORT=$(kubectl get gameserver <name> -n gshub -o jsonpath='{.status.ports[0].port}')
nc -zv $PUBLIC_IP $HOST_PORT
```

**Common fixes**:
```bash
# Firewall issue on agent node
ssh <agent-node>
sudo ufw allow 7000:8000/tcp
sudo ufw allow 7000:8000/udp

# Wrong public IP label
kubectl label node <agent-node> platform.io/public-ip=<correct-ip> --overwrite
```

### Agones Controller Issues

```bash
# Check Agones pods
kubectl get pods -n agones-system
kubectl logs -n agones-system -l app=agones -c agones-controller

# Verify CRDs
kubectl get crds | grep agones
```

---

## Quick Reference

### Common Commands

```bash
# Get all resources in gshub namespace
kubectl get all -n gshub

# View logs for API
kubectl logs -n gshub -l app=api --tail=100 -f

# View all GameServers
kubectl get gameservers -n gshub

# Restart API deployment
kubectl rollout restart deployment/api -n gshub

# Port forward API locally
kubectl port-forward -n gshub svc/api 8080:8080

# Shell into API pod
kubectl exec -it deployment/api -n gshub -- /bin/sh
```

### Environment-Specific Commands

**k3d Development**:
```bash
k3d cluster stop gshub-dev     # Stop cluster
k3d cluster start gshub-dev    # Start cluster
k3d cluster delete gshub-dev   # Delete cluster
k3d cluster list               # List clusters
```

**k3s Production**:
```bash
# On master
sudo systemctl status k3s
sudo systemctl restart k3s
sudo journalctl -u k3s -f

# On agent
sudo systemctl status k3s-agent
sudo systemctl restart k3s-agent
sudo journalctl -u k3s-agent -f
```

### Deployment Commands Summary

| Environment | Agones Install | Deploy Platform |
|-------------|----------------|-----------------|
| Development | `bash k8s/overlays/dev/agones-install.sh` | `kubectl apply -k k8s/overlays/dev` |
| Production | `bash k8s/overlays/prod/agones-install.sh` | `kubectl apply -k k8s/overlays/prod` |

---

## Next Steps

After successful setup:

1. **Configure Stripe**: Set up Stripe webhooks and products for billing
2. **Configure Email**: Set up MailerSend for email notifications
3. **Set up Cloudflare Tunnel**: Configure tunnel for external API access
4. **Monitor Resources**: Set up monitoring with Prometheus/Grafana
5. **Backup Database**: Configure regular PostgreSQL backups
6. **Add More Agents**: Scale horizontally by adding more agent nodes

For more details on the Kubernetes structure, see [k8s/README.md](../k8s/README.md).
