# K3s Game Server Hosting Infrastructure

## Architecture Overview

```
                         Internet
                             │
               ┌─────────────┴─────────────┐
               │                           │
               ▼                           ▼
       ┌───────────────┐          ┌────────────────┐
       │  Cloudflare   │          │  Cloudflare    │
       │  Tunnel       │          │  DNS           │
       │               │          │                │
       │ panel.example │          │ *.play.example │
       │ api.example   │          │                │
       └───────────────┘          └────────────────┘
               │                           │
               ▼                           │
       ┌───────────────┐                   │
       │ Control Plane │                   │
       │ 10.0.0.1      │                   │
       │               │                   │
       │ ┌───────────┐ │                   │
       │ │ K3s Server│ │                   │
       │ │ Go API   ─┼─┼── manages DNS ────┘
       │ │ Frontend  │ │
       │ │ PostgreSQL│ │
       │ │cloudflared│ │
       │ └───────────┘ │
       └───────────────┘
               │
     ┌─────────┼─────────┐
     ▼         ▼         ▼
 ┌───────┐ ┌───────┐ ┌───────┐
 │Worker1│ │Worker2│ │Worker3│
 │45.x.10│ │45.x.11│ │45.x.12│
 │       │ │       │ │       │
 │ games │ │ games │ │ games │
 └───────┘ └───────┘ └───────┘
```

---

## Node Specifications

### Control Plane (1 node)

|Spec|Value|
|---|---|
|CPU|4 vCPU|
|RAM|8 GB|
|Storage|100 GB SSD|
|Network|Private IP only|

### Workers (3+ nodes)

|Spec|Value|
|---|---|
|CPU|8-16 vCPU|
|RAM|32-64 GB|
|Storage|200 GB NVMe|
|Network|Public IP + Private IP|

---

## K3s Installation

### Control Plane

```bash
curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="server \
  --disable traefik \
  --disable servicelb \
  --node-taint CriticalAddonsOnly=true:NoExecute \
  --tls-san 10.0.0.1" sh -
```

### Workers

```bash
# Get token from control plane
cat /var/lib/rancher/k3s/server/node-token

# On each worker
curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="agent" \
  K3S_URL="https://10.0.0.1:6443" \
  K3S_TOKEN="<token>" sh -
```

### Label Workers

```bash
kubectl label node worker-01 \
  node-role.kubernetes.io/gameserver=true \
  platform.io/public-ip=45.x.x.10

kubectl label node worker-02 \
  node-role.kubernetes.io/gameserver=true \
  platform.io/public-ip=45.x.x.11

kubectl label node worker-03 \
  node-role.kubernetes.io/gameserver=true \
  platform.io/public-ip=45.x.x.12
```

---

## Agones Installation

```bash
helm repo add agones https://agones.dev/chart/stable
helm repo update

helm install agones agones/agones \
  --namespace agones-system \
  --create-namespace \
  --set gameservers.namespaces="{gameservers}" \
  --set agones.ping.http.enabled=false \
  --set agones.ping.udp.enabled=false
```

---

## Namespaces

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: gameservers
---
apiVersion: v1
kind: Namespace
metadata:
  name: platform
```

---

## Platform Services

### Secrets

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: platform-secrets
  namespace: platform
type: Opaque
stringData:
  db-password: "<generate-secure-password>"
  jwt-secret: "<generate-secure-secret>"
  cloudflare-token: "<cloudflare-api-token>"
  cloudflare-tunnel-token: "<tunnel-token>"
  stripe-secret: "<stripe-secret-key>"
  stripe-webhook-secret: "<stripe-webhook-secret>"
```

### PostgreSQL

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: postgresql
  namespace: platform
spec:
  serviceName: postgresql
  replicas: 1
  selector:
    matchLabels:
      app: postgresql
  template:
    metadata:
      labels:
        app: postgresql
    spec:
      nodeSelector:
        node-role.kubernetes.io/control-plane: "true"
      tolerations:
      - key: "CriticalAddonsOnly"
        operator: "Exists"
        effect: "NoExecute"
      containers:
      - name: postgresql
        image: postgres:16-alpine
        ports:
        - containerPort: 5432
        env:
        - name: POSTGRES_DB
          value: "platform"
        - name: POSTGRES_USER
          value: "platform"
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: platform-secrets
              key: db-password
        volumeMounts:
        - name: data
          mountPath: /var/lib/postgresql/data
        resources:
          requests:
            cpu: "500m"
            memory: "512Mi"
          limits:
            cpu: "2"
            memory: "2Gi"
  volumeClaimTemplates:
  - metadata:
      name: data
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 20Gi
---
apiVersion: v1
kind: Service
metadata:
  name: postgresql
  namespace: platform
spec:
  selector:
    app: postgresql
  ports:
  - port: 5432
```

### Go API

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
  namespace: platform
spec:
  replicas: 1
  selector:
    matchLabels:
      app: api
  template:
    metadata:
      labels:
        app: api
    spec:
      nodeSelector:
        node-role.kubernetes.io/control-plane: "true"
      tolerations:
      - key: "CriticalAddonsOnly"
        operator: "Exists"
        effect: "NoExecute"
      serviceAccountName: platform-api
      containers:
      - name: api
        image: your-registry/platform-api:latest
        ports:
        - containerPort: 8080
        env:
        - name: DATABASE_URL
          value: "postgres://platform:$(DB_PASSWORD)@postgresql.platform.svc:5432/platform"
        - name: DB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: platform-secrets
              key: db-password
        - name: JWT_SECRET
          valueFrom:
            secretKeyRef:
              name: platform-secrets
              key: jwt-secret
        - name: CLOUDFLARE_API_TOKEN
          valueFrom:
            secretKeyRef:
              name: platform-secrets
              key: cloudflare-token
        - name: CLOUDFLARE_ZONE_ID
          value: "<your-zone-id>"
        - name: DNS_BASE_DOMAIN
          value: "play.example.com"
        - name: STRIPE_SECRET_KEY
          valueFrom:
            secretKeyRef:
              name: platform-secrets
              key: stripe-secret
        - name: STRIPE_WEBHOOK_SECRET
          valueFrom:
            secretKeyRef:
              name: platform-secrets
              key: stripe-webhook-secret
        - name: RECONCILE_INTERVAL
          value: "30s"
        resources:
          requests:
            cpu: "250m"
            memory: "256Mi"
          limits:
            cpu: "1"
            memory: "512Mi"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 10
---
apiVersion: v1
kind: Service
metadata:
  name: api
  namespace: platform
spec:
  selector:
    app: api
  ports:
  - port: 8080
```

The API starts the reconciler on boot:

```go
func main() {
    // ... setup db, k8s client, etc.
    
    service := NewService(db, k8sClient, cloudflare)
    
    // Start reconciler in background
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    go service.StartReconciler(ctx)
    
    // Start HTTP server
    router := setupRoutes(service)
    router.Run(":8080")
}
```

### API Endpoints

```go
func setupRoutes(s *Service) *gin.Engine {
    r := gin.Default()
    
    // Public
    r.POST("/auth/register", s.Register)
    r.POST("/auth/login", s.Login)
    r.POST("/webhooks/stripe", s.HandleStripeWebhook)
    
    // Protected
    api := r.Group("/api", s.AuthMiddleware)
    {
        // Games catalog
        api.GET("/games", s.ListGames)
        api.GET("/games/:game/plans", s.ListPlans)
        
        // Servers
        api.GET("/servers", s.ListServers)
        api.POST("/servers", s.CreateServer)
        api.GET("/servers/:id", s.GetServer)
        api.DELETE("/servers/:id", s.DeleteServer)
        api.POST("/servers/:id/stop", s.StopServer)
        api.POST("/servers/:id/start", s.StartServer)
        api.POST("/servers/:id/restart", s.RestartServer)
        
        // Server console/logs
        api.GET("/servers/:id/logs", s.GetLogs)
        api.GET("/servers/:id/console", s.WebSocketConsole)
    }
    
    // Health checks
    r.GET("/health", s.Health)
    r.GET("/ready", s.Ready)
    
    return r
}
```

### API RBAC

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: platform-api
  namespace: platform
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: platform-api
rules:
- apiGroups: ["agones.dev"]
  resources: ["gameservers"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: [""]
  resources: ["pods", "pods/log", "pods/exec"]
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources: ["persistentvolumeclaims"]
  verbs: ["get", "list", "watch", "create", "delete"]
- apiGroups: [""]
  resources: ["nodes"]
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: platform-api
subjects:
- kind: ServiceAccount
  name: platform-api
  namespace: platform
roleRef:
  kind: ClusterRole
  name: platform-api
  apiGroup: rbac.authorization.k8s.io
```

### React Frontend

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: frontend
  namespace: platform
spec:
  replicas: 1
  selector:
    matchLabels:
      app: frontend
  template:
    metadata:
      labels:
        app: frontend
    spec:
      nodeSelector:
        node-role.kubernetes.io/control-plane: "true"
      tolerations:
      - key: "CriticalAddonsOnly"
        operator: "Exists"
        effect: "NoExecute"
      containers:
      - name: frontend
        image: your-registry/platform-frontend:latest
        ports:
        - containerPort: 3000
        env:
        - name: NEXT_PUBLIC_API_URL
          value: "https://api.example.com"
        - name: NEXT_PUBLIC_STRIPE_PUBLIC_KEY
          value: "<stripe-public-key>"
        resources:
          requests:
            cpu: "100m"
            memory: "128Mi"
          limits:
            cpu: "500m"
            memory: "256Mi"
---
apiVersion: v1
kind: Service
metadata:
  name: frontend
  namespace: platform
spec:
  selector:
    app: frontend
  ports:
  - port: 3000
```

### Cloudflare Tunnel

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cloudflared
  namespace: platform
spec:
  replicas: 1
  selector:
    matchLabels:
      app: cloudflared
  template:
    metadata:
      labels:
        app: cloudflared
    spec:
      nodeSelector:
        node-role.kubernetes.io/control-plane: "true"
      tolerations:
      - key: "CriticalAddonsOnly"
        operator: "Exists"
        effect: "NoExecute"
      containers:
      - name: cloudflared
        image: cloudflare/cloudflared:latest
        args:
        - tunnel
        - --no-autoupdate
        - run
        - --token
        - $(TUNNEL_TOKEN)
        env:
        - name: TUNNEL_TOKEN
          valueFrom:
            secretKeyRef:
              name: platform-secrets
              key: cloudflare-tunnel-token
        resources:
          requests:
            cpu: "50m"
            memory: "64Mi"
          limits:
            cpu: "200m"
            memory: "128Mi"
```

Configure in Cloudflare Zero Trust dashboard:

|Hostname|Service|
|---|---|
|panel.example.com|http://frontend.platform.svc:3000|
|api.example.com|http://api.platform.svc:8080|

---

## Game Catalog

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: game-catalog
  namespace: platform
data:
  games.yaml: |
    games:
      minecraft-java:
        name: "Minecraft: Java Edition"
        image: "itzg/minecraft-server:latest"
        port: 25565
        protocol: TCP
        env:
          EULA: "TRUE"
          TYPE: "PAPER"
        plans:
          starter:
            name: "Starter"
            players: "2-5"
            cpu: "2"
            memory: "2Gi"
            storage: "10Gi"
            price: 300
          standard:
            name: "Standard"
            players: "5-15"
            cpu: "3"
            memory: "4Gi"
            storage: "25Gi"
            price: 600
          performance:
            name: "Performance"
            players: "15-30"
            cpu: "4"
            memory: "8Gi"
            storage: "50Gi"
            price: 1200

      minecraft-bedrock:
        name: "Minecraft: Bedrock Edition"
        image: "itzg/minecraft-bedrock-server:latest"
        port: 19132
        protocol: UDP
        env:
          EULA: "TRUE"
        plans:
          starter:
            name: "Starter"
            players: "2-5"
            cpu: "1"
            memory: "1Gi"
            storage: "5Gi"
            price: 200
          standard:
            name: "Standard"
            players: "5-10"
            cpu: "2"
            memory: "2Gi"
            storage: "10Gi"
            price: 400

      valheim:
        name: "Valheim"
        image: "lloesche/valheim-server:latest"
        port: 2456
        protocol: UDP
        additionalPorts:
          - name: "query"
            port: 2457
            protocol: UDP
        env:
          SERVER_PUBLIC: "false"
        plans:
          starter:
            name: "Starter"
            players: "2-4"
            cpu: "2"
            memory: "4Gi"
            storage: "10Gi"
            price: 500
          standard:
            name: "Standard"
            players: "4-8"
            cpu: "3"
            memory: "6Gi"
            storage: "20Gi"
            price: 800

      rust:
        name: "Rust"
        image: "didstopia/rust-server:latest"
        port: 28015
        protocol: UDP
        additionalPorts:
          - name: "rcon"
            port: 28016
            protocol: TCP
        plans:
          starter:
            name: "Starter"
            players: "10-25"
            cpu: "3"
            memory: "6Gi"
            storage: "20Gi"
            price: 800
          standard:
            name: "Standard"
            players: "25-50"
            cpu: "4"
            memory: "10Gi"
            storage: "40Gi"
            price: 1500

      ark:
        name: "ARK: Survival Evolved"
        image: "hermsi/ark-server:latest"
        port: 7777
        protocol: UDP
        additionalPorts:
          - name: "query"
            port: 27015
            protocol: UDP
          - name: "rcon"
            port: 27020
            protocol: TCP
        plans:
          starter:
            name: "Starter"
            players: "5-10"
            cpu: "4"
            memory: "8Gi"
            storage: "30Gi"
            price: 1200
          standard:
            name: "Standard"
            players: "10-25"
            cpu: "6"
            memory: "12Gi"
            storage: "60Gi"
            price: 2000
```

---

## Data Consistency

The API uses a DB-first approach with a background reconciliation loop to ensure consistency between PostgreSQL and K8s.

### Strategy

1. **DB is source of truth** — always write to DB first
2. **Pending status** — servers start as "pending" until K8s confirms
3. **Reconciler** — background loop syncs state every 30 seconds

### Server States

```
pending  → K8s resources being created
running  → GameServer is Ready
stopped  → User stopped server (GameServer deleted, PVC kept)
failed   → Creation failed (reconciler will retry)
deleted  → Marked for deletion (reconciler cleans up)
```

### Reconciliation Loop

```go
func (s *Service) StartReconciler(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    for {
        select {
        case <-ticker.C:
            s.reconcile(ctx)
        case <-ctx.Done():
            return
        }
    }
}

func (s *Service) reconcile(ctx context.Context) {
    // Get all servers from DB
    dbServers, _ := s.db.GetAllServers(ctx)
    
    // Get all GameServers from K8s
    k8sList, _ := s.k8s.ListGameServers(ctx, "gameservers")
    k8sMap := make(map[string]*agonesv1.GameServer)
    for i := range k8sList {
        k8sMap[k8sList[i].Name] = &k8sList[i]
    }
    
    now := time.Now()
    
    for _, server := range dbServers {
        gs, existsInK8s := k8sMap[server.ID]
        
        switch server.Status {
        case "pending", "failed":
            // Retry creation
            if !existsInK8s {
                if err := s.createK8sResources(ctx, server); err != nil {
                    s.db.UpdateStatus(ctx, server.ID, "failed", err.Error())
                } else {
                    s.db.UpdateStatus(ctx, server.ID, "running", "")
                }
            }
            
        case "running":
            if !existsInK8s {
                // K8s lost it, recreate
                s.createK8sResources(ctx, server)
            } else if gs.Status.State == "Ready" {
                // Sync port/IP (handles node migration)
                s.syncServerInfo(ctx, server, gs)
            }
            
        case "stopped":
            // Delete GameServer but keep PVC
            if existsInK8s {
                s.k8s.DeleteGameServer(ctx, server.ID)
            }
            
        case "expired":
            // Delete GameServer but keep PVC (grace period)
            if existsInK8s {
                s.k8s.DeleteGameServer(ctx, server.ID)
            }
            // Check if grace period is over
            if server.DeleteAfter != nil && now.After(*server.DeleteAfter) {
                s.db.UpdateStatus(ctx, server.ID, "deleted", "grace period ended")
            }
            
        case "deleted":
            // Full cleanup
            if existsInK8s {
                s.k8s.DeleteGameServer(ctx, server.ID)
            }
            // Delete PVC
            if err := s.k8s.DeletePVC(ctx, server.ID+"-data"); err == nil || isNotFound(err) {
                // Delete DNS record
                s.cloudflare.DeleteRecord(server.DNSRecord)
                // Hard delete from DB
                s.db.HardDelete(ctx, server.ID)
            }
        }
        
        // Remove from map (to find orphans)
        delete(k8sMap, server.ID)
    }
    
    // Clean up orphans (in K8s but not in DB)
    for name := range k8sMap {
        s.k8s.DeleteGameServer(ctx, name)
    }
}

func (s *Service) syncServerInfo(ctx context.Context, server *Server, gs *agonesv1.GameServer) {
    node, _ := s.k8s.GetNode(ctx, gs.Status.NodeName)
    nodeIP := node.Labels["platform.io/public-ip"]
    port := gs.Status.Ports[0].Port
    
    // Check if anything changed (node migration)
    if server.NodeIP != nodeIP || server.Port != int(port) {
        // Update DNS
        s.cloudflare.UpdateARecord(server.DNSRecord, nodeIP)
        
        // Update DB
        s.db.UpdateServerInfo(ctx, server.ID, nodeIP, int(port))
    }
}
```

### What This Handles

|Scenario|Resolution|
|---|---|
|K8s create fails|DB status = "failed", reconciler retries|
|DB create fails|Nothing created, clean error to user|
|GameServer crashes|Agones restarts it automatically|
|Node dies|Agones reschedules, reconciler updates DNS|
|Orphan GameServer|Reconciler deletes it|
|Missing GameServer|Reconciler recreates it|

---

## Server Lifecycle & Deletion

### Server States

```
pending   → Creating K8s resources
running   → GameServer is Ready
stopped   → User manually stopped (GameServer deleted, PVC kept)
expired   → Subscription ended, grace period active
deleted   → Grace period over, full cleanup pending
```

### Lifecycle Flow

```
                    User creates server
                            │
                            ▼
                    ┌───────────────┐
                    │    pending    │
                    └───────────────┘
                            │
                            ▼
                    ┌───────────────┐
        ┌──────────│    running    │──────────┐
        │          └───────────────┘          │
        │                   │                 │
   User stops         Subscription       User deletes
        │               expires               │
        ▼                   ▼                 │
┌───────────────┐   ┌───────────────┐         │
│    stopped    │   │    expired    │         │
└───────────────┘   └───────────────┘         │
        │                   │                 │
   User starts         7 day grace           │
        │               period               │
        │                   │                 │
        ▼                   ▼                 ▼
┌───────────────┐   ┌───────────────┐   ┌───────────────┐
│    pending    │   │    deleted    │◄──│    deleted    │
│  (restart)    │   │               │   │  (immediate)  │
└───────────────┘   └───────────────┘   └───────────────┘
                            │
                    Reconciler cleans up
                            │
                            ▼
                    ┌───────────────┐
                    │  Hard delete  │
                    │  PVC + DNS +  │
                    │   DB row      │
                    └───────────────┘
```

### Grace Periods

```go
const (
    GracePeriodExpired = 7 * 24 * time.Hour   // 7 days after subscription expires
)
```

### Stripe Webhook Handler

```go
func (s *Service) HandleStripeWebhook(c *gin.Context) {
    payload, _ := io.ReadAll(c.Request.Body)
    event, err := webhook.ConstructEvent(payload, 
        c.GetHeader("Stripe-Signature"), 
        s.stripeWebhookSecret)
    if err != nil {
        c.Status(400)
        return
    }
    
    switch event.Type {
    case "customer.subscription.deleted",
         "invoice.payment_failed":
        var sub stripe.Subscription
        json.Unmarshal(event.Data.Raw, &sub)
        serverID := sub.Metadata["server_id"]
        
        // Mark expired with 7-day grace period
        s.db.ExpireServer(ctx, serverID, time.Now().Add(GracePeriodExpired))
        
    case "invoice.payment_succeeded":
        var inv stripe.Invoice
        json.Unmarshal(event.Data.Raw, &inv)
        serverID := inv.Subscription.Metadata["server_id"]
        
        // Reactivate if within grace period
        s.db.ReactivateServer(ctx, serverID)
    }
    
    c.Status(200)
}
```

```go
func (db *DB) ExpireServer(ctx context.Context, id string, deleteAfter time.Time) error {
    _, err := db.Exec(ctx, `
        UPDATE servers 
        SET status = 'expired',
            expired_at = NOW(),
            delete_after = $2,
            updated_at = NOW()
        WHERE id = $1 AND status = 'running'
    `, id, deleteAfter)
    return err
}

func (db *DB) ReactivateServer(ctx context.Context, id string) error {
    _, err := db.Exec(ctx, `
        UPDATE servers 
        SET status = 'pending',
            expired_at = NULL,
            delete_after = NULL,
            updated_at = NOW()
        WHERE id = $1 AND status = 'expired'
    `, id)
    return err
}
```

### User Actions

```go
// Stop server (keeps data, stops billing if on usage-based plan)
func (s *Service) StopServer(ctx context.Context, userID, serverID string) error {
    server, err := s.db.GetServer(ctx, serverID)
    if err != nil || server.UserID != userID {
        return ErrNotFound
    }
    if server.Status != "running" {
        return ErrInvalidState
    }
    
    _, err = s.db.Exec(ctx, `
        UPDATE servers 
        SET status = 'stopped', 
            stopped_at = NOW(),
            updated_at = NOW()
        WHERE id = $1
    `, serverID)
    return err
    // Reconciler deletes GameServer, keeps PVC
}

// Start stopped server
func (s *Service) StartServer(ctx context.Context, userID, serverID string) error {
    server, err := s.db.GetServer(ctx, serverID)
    if err != nil || server.UserID != userID {
        return ErrNotFound
    }
    if server.Status != "stopped" {
        return ErrInvalidState
    }
    
    _, err = s.db.Exec(ctx, `
        UPDATE servers 
        SET status = 'pending',
            stopped_at = NULL,
            updated_at = NOW()
        WHERE id = $1
    `, serverID)
    return err
    // Reconciler creates GameServer with existing PVC
}

// Delete server permanently (no grace period)
func (s *Service) DeleteServer(ctx context.Context, userID, serverID string) error {
    server, err := s.db.GetServer(ctx, serverID)
    if err != nil || server.UserID != userID {
        return ErrNotFound
    }
    
    // Cancel Stripe subscription
    if server.StripeSubscriptionID != "" {
        s.stripe.Subscriptions.Cancel(server.StripeSubscriptionID, nil)
    }
    
    _, err = s.db.Exec(ctx, `
        UPDATE servers 
        SET status = 'deleted',
            delete_after = NOW(),
            updated_at = NOW()
        WHERE id = $1
    `, serverID)
    return err
    // Reconciler cleans up everything
}
```

### What Gets Deleted When

|State|GameServer|PVC (Data)|DNS|DB Row|
|---|---|---|---|---|
|stopped|❌ Deleted|✅ Kept|✅ Kept|✅ Kept|
|expired|❌ Deleted|✅ Kept|✅ Kept|✅ Kept|
|deleted|❌ Deleted|❌ Deleted|❌ Deleted|❌ Deleted|

---

## API Flow

The Go API handles everything:

```
1. User creates server via API
2. API creates Agones GameServer + PVC
3. API watches GameServer until Ready
4. API reads node IP (from label) + assigned port
5. API creates Cloudflare DNS record
6. API stores server info in PostgreSQL
7. API returns connection string to user
```

### Example: Create Server (DB-First)

```go
func (s *Service) CreateServer(ctx context.Context, userID, game, plan, name string) (*Server, error) {
    // 1. Generate unique server ID
    serverID := fmt.Sprintf("%s-%s-%s", userID[:8], game[:2], randomString(6))
    dnsRecord := fmt.Sprintf("%s.play.example.com", serverID)
    
    // 2. Create in DB first with pending status
    server := &Server{
        ID:        serverID,
        UserID:    userID,
        Name:      name,
        Game:      game,
        Plan:      plan,
        Status:    "pending",
        DNSRecord: dnsRecord,
    }
    if err := s.db.CreateServer(ctx, server); err != nil {
        return nil, err  // Nothing to clean up
    }
    
    // 3. Create K8s resources (PVC + GameServer)
    if err := s.createK8sResources(ctx, server); err != nil {
        // Mark as failed - reconciler will retry
        s.db.UpdateStatus(ctx, serverID, "failed", err.Error())
        return nil, err
    }
    
    // 4. Wait for Ready state (with timeout)
    gs, err := s.waitForReady(ctx, serverID, 2*time.Minute)
    if err != nil {
        s.db.UpdateStatus(ctx, serverID, "failed", "timeout waiting for ready")
        return nil, err
    }
    
    // 5. Get node public IP and port
    node, _ := s.k8s.GetNode(ctx, gs.Status.NodeName)
    nodeIP := node.Labels["platform.io/public-ip"]
    port := gs.Status.Ports[0].Port
    
    // 6. Create DNS record
    if err := s.cloudflare.CreateARecord(dnsRecord, nodeIP); err != nil {
        s.db.UpdateStatus(ctx, serverID, "failed", "dns creation failed")
        return nil, err
    }
    
    // 7. Update DB with final info
    server.Status = "running"
    server.NodeIP = nodeIP
    server.Port = int(port)
    if err := s.db.UpdateServer(ctx, server); err != nil {
        return nil, err
    }
    
    return server, nil
}

func (s *Service) createK8sResources(ctx context.Context, server *Server) error {
    gameConfig := s.catalog.GetGame(server.Game)
    planConfig := gameConfig.Plans[server.Plan]
    
    // Create PVC
    pvc := &corev1.PersistentVolumeClaim{
        ObjectMeta: metav1.ObjectMeta{
            Name:      server.ID + "-data",
            Namespace: "gameservers",
            Labels: map[string]string{
                "platform.io/server-id": server.ID,
            },
        },
        Spec: corev1.PersistentVolumeClaimSpec{
            AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
            Resources: corev1.VolumeResourceRequirements{
                Requests: corev1.ResourceList{
                    corev1.ResourceStorage: resource.MustParse(planConfig.Storage),
                },
            },
        },
    }
    if err := s.k8s.Create(ctx, pvc); err != nil {
        return err
    }
    
    // Create GameServer
    gs := &agonesv1.GameServer{
        ObjectMeta: metav1.ObjectMeta{
            Name:      server.ID,
            Namespace: "gameservers",
            Labels: map[string]string{
                "platform.io/user-id":   server.UserID,
                "platform.io/server-id": server.ID,
                "platform.io/game":      server.Game,
                "platform.io/plan":      server.Plan,
            },
        },
        Spec: agonesv1.GameServerSpec{
            Ports: []agonesv1.GameServerPort{{
                Name:          "game",
                PortPolicy:    agonesv1.Dynamic,
                ContainerPort: int32(gameConfig.Port),
                Protocol:      corev1.Protocol(gameConfig.Protocol),
            }},
            Template: corev1.PodTemplateSpec{
                Spec: corev1.PodSpec{
                    Containers: []corev1.Container{{
                        Name:      server.Game,
                        Image:     gameConfig.Image,
                        Env:       buildEnv(gameConfig.Env),
                        Resources: buildResources(planConfig),
                        VolumeMounts: []corev1.VolumeMount{{
                            Name:      "data",
                            MountPath: "/data",
                        }},
                    }},
                    Volumes: []corev1.Volume{{
                        Name: "data",
                        VolumeSource: corev1.VolumeSource{
                            PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
                                ClaimName: server.ID + "-data",
                            },
                        },
                    }},
                },
            },
        },
    }
    
    return s.k8s.Create(ctx, gs)
}
```

---

## Network Policies

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: gameserver-policy
  namespace: gameservers
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  - Egress
  ingress:
  # Allow game traffic from internet
  - ports:
    - port: 25000
      endPort: 29999
      protocol: TCP
    - port: 25000
      endPort: 29999
      protocol: UDP
  egress:
  # Allow DNS
  - to:
    - namespaceSelector:
        matchLabels:
          kubernetes.io/metadata.name: kube-system
    ports:
    - port: 53
      protocol: UDP
  # Allow internet (game updates, auth)
  - to:
    - ipBlock:
        cidr: 0.0.0.0/0
        except:
        - 10.0.0.0/8
        - 172.16.0.0/12
        - 192.168.0.0/16
```

---

## Database Schema

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    stripe_customer_id VARCHAR(255),
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE servers (
    id VARCHAR(50) PRIMARY KEY,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    game VARCHAR(50) NOT NULL,
    plan VARCHAR(50) NOT NULL,
    status VARCHAR(20) DEFAULT 'pending',
    status_message TEXT,
    dns_record VARCHAR(255),
    node_ip VARCHAR(45),
    port INTEGER,
    stripe_subscription_id VARCHAR(255),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    stopped_at TIMESTAMP,
    expired_at TIMESTAMP,
    delete_after TIMESTAMP
);

CREATE TABLE subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id VARCHAR(50) REFERENCES servers(id) ON DELETE CASCADE,
    stripe_subscription_id VARCHAR(255) UNIQUE,
    status VARCHAR(20) DEFAULT 'active',
    current_period_end TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_servers_user_id ON servers(user_id);
CREATE INDEX idx_servers_status ON servers(status);
CREATE INDEX idx_servers_delete_after ON servers(delete_after) 
    WHERE delete_after IS NOT NULL;
```

---

## Backups (Add Later)

When needed, add S3 backups:

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: backup
  namespace: platform
spec:
  schedule: "0 */6 * * *"
  jobTemplate:
    spec:
      template:
        spec:
          nodeSelector:
            node-role.kubernetes.io/control-plane: "true"
          tolerations:
          - key: "CriticalAddonsOnly"
            operator: "Exists"
            effect: "NoExecute"
          containers:
          - name: backup
            image: postgres:16-alpine
            command: ["/bin/sh", "-c"]
            args:
            - |
              pg_dump -h postgresql -U platform -d platform | \
              gzip | \
              aws s3 cp - s3://your-bucket/backups/db-$(date +%Y%m%d-%H%M%S).sql.gz
            env:
            - name: PGPASSWORD
              valueFrom:
                secretKeyRef:
                  name: platform-secrets
                  key: db-password
            - name: AWS_ACCESS_KEY_ID
              valueFrom:
                secretKeyRef:
                  name: aws-secrets
                  key: access-key
            - name: AWS_SECRET_ACCESS_KEY
              valueFrom:
                secretKeyRef:
                  name: aws-secrets
                  key: secret-key
          restartPolicy: OnFailure
```

---

## Monitoring (Add Later)

When needed:

```bash
# Simple metrics
kubectl top nodes
kubectl top pods -n gameservers

# Logs
kubectl logs -n gameservers <pod-name>

# Later: Full stack
helm install monitoring prometheus-community/kube-prometheus-stack
```

---

## Directory Structure

```
/platform
├── k8s/
│   ├── namespaces.yaml
│   ├── secrets.yaml
│   ├── platform/
│   │   ├── postgresql.yaml
│   │   ├── api.yaml
│   │   ├── frontend.yaml
│   │   ├── cloudflared.yaml
│   │   ├── rbac.yaml
│   │   └── game-catalog.yaml
│   └── gameservers/
│       └── network-policy.yaml
├── api/                    # Go API/Gin
│   ├── main.go
│   ├── handlers/
│   ├── services/
│   └── Dockerfile
└── frontend/               # React/Vite
    ├── src/
    ├── package.json
    └── Dockerfile
```

---

## Deployment Checklist

### Infrastructure

- [ ] Provision control plane (private IP)
- [ ] Provision 3+ workers (public IPs)
- [ ] Install K3s server
- [ ] Join workers
- [ ] Label workers with public IPs

### Platform

- [ ] Install Agones
- [ ] Create namespaces
- [ ] Create secrets
- [ ] Deploy PostgreSQL
- [ ] Run database migrations
- [ ] Deploy API
- [ ] Deploy Frontend
- [ ] Deploy Cloudflare Tunnel

### Cloudflare

- [ ] Create tunnel
- [ ] Configure panel.example.com → frontend
- [ ] Configure api.example.com → api
- [ ] Setup DNS zone for *.play.example.com

### Stripe

- [ ] Create products/prices for each plan
- [ ] Configure webhook endpoint (api.example.com/webhooks/stripe)

### Test

- [ ] User signup
- [ ] Create server
- [ ] Verify DNS record created
- [ ] Connect to game server
- [ ] Stop/start server
- [ ] Delete server

---

## Scaling

|Users|Servers|Workers|Notes|
|---|---|---|---|
|1-50|~100|3|Starting setup|
|50-200|~400|5-8|Add workers|
|200-500|~1000|10-15|Add workers|
|500+|2000+|20+|Consider multi-region|

To scale: just add more workers and label them.

```bash
# New worker
curl -sfL https://get.k3s.io | K3S_URL="https://10.0.0.1:6443" K3S_TOKEN="<token>" sh -

# Label it
kubectl label node worker-04 \
  node-role.kubernetes.io/gameserver=true \
  platform.io/public-ip=45.x.x.13
```