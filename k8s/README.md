# Kubernetes Manifests

This directory uses Kustomize for managing Kubernetes resources across environments.

## Structure

```
k8s/
├── base/                           # Shared resources for all environments
│   ├── kustomization.yaml         # Base kustomization manifest
│   ├── namespace.yaml             # gshub namespace
│   ├── secrets.yaml               # Secrets (base64 encoded)
│   └── gshub/
│       ├── api.yaml               # API deployment
│       ├── cloudflared.yaml       # Cloudflare tunnel
│       ├── game-catalog.yaml      # Game definitions
│       ├── postgresql.yaml        # PostgreSQL database
│       └── rbac.yaml              # Service accounts & roles
│
├── overlays/
│   ├── dev/                       # Development-specific (k3d)
│   │   ├── kustomization.yaml     # Dev kustomization
│   │   ├── api-image-patch.yaml   # Patches API to use local image
│   │   └── agones-install.sh      # Agones install with dev settings
│   │
│   └── prod/                      # Production-specific (k3s)
│       ├── kustomization.yaml     # Prod kustomization
│       ├── api-image-patch.yaml   # Patches API to use Docker Hub image
│       └── agones-install.sh      # Agones install with prod settings
│
└── README.md                      # This file
```

## Usage

### Development Deployment (k3d)

```bash
# Step 1: Install Agones
bash k8s/overlays/dev/agones-install.sh

# Step 2: Deploy GSHUB platform
kubectl apply -k k8s/overlays/dev

# Step 3: Verify deployment
kubectl get all -n gshub
```

### Production Deployment (k3s)

```bash
# Step 1: Install Agones
bash k8s/overlays/prod/agones-install.sh

# Step 2: Deploy GSHUB platform
kubectl apply -k k8s/overlays/prod

# Step 3: Verify deployment
kubectl get all -n gshub
```

## Key Differences Between Environments

| Resource | Development | Production |
|----------|-------------|------------|
| **API Image** | `gshub-api` (local build) | `dasior/gshub-api:latest` |
| **Agones Ports** | 7000-7050 (TCP/UDP) | 7000-8000 (TCP/UDP) |
| **Agones Node Selector** | None (single node) | Master node only |
| **Environment Label** | `environment: development` | `environment: production` |
| **Common Labels** | `app.kubernetes.io/managed-by: kustomize` | Same |
|  | `app.kubernetes.io/part-of: gshub` | Same |

## Making Changes

### Update Shared Resources

Edit files in `base/` for changes affecting all environments:

```bash
# Example: Update PostgreSQL storage size
vim k8s/base/gshub/postgresql.yaml

# Apply to dev
kubectl apply -k k8s/overlays/dev

# Apply to prod
kubectl apply -k k8s/overlays/prod
```

### Update Environment-Specific Settings

Edit files in `overlays/dev/` or `overlays/prod/`:

```bash
# Example: Change dev API image
vim k8s/overlays/dev/api-image-patch.yaml

# Apply changes
kubectl apply -k k8s/overlays/dev
```

### Preview Changes Before Applying

```bash
# Render manifests without applying
kubectl kustomize k8s/overlays/dev

# Show diff of what would change
kubectl diff -k k8s/overlays/dev

# Dry-run validation
kubectl apply -k k8s/overlays/dev --dry-run=client
```

## Agones Configuration

Agones is installed via Helm with environment-specific settings defined in the install scripts:

**Development (k3d):**
- Port range: 7000-7050
- No node selector (single node cluster)
- Watches `gshub` namespace for GameServers

**Production (k3s):**
- Port range: 7000-8000
- Controller scheduled on master node only
- Tolerates master node taints
- Watches `gshub` namespace for GameServers

To update Agones configuration, edit the respective `agones-install.sh` script and re-run it.

## Troubleshooting

### Validation Errors

If you get validation errors when applying:

```bash
# Check kustomization syntax
kubectl kustomize k8s/overlays/dev

# Validate individual resources
kubectl apply -k k8s/overlays/dev --dry-run=server
```

### Resource Not Found

If resources aren't being created:

```bash
# Verify all resources are listed in base/kustomization.yaml
cat k8s/base/kustomization.yaml

# Check overlay is referencing base correctly
cat k8s/overlays/dev/kustomization.yaml
```

### Image Pull Errors (Dev)

If the dev API pod fails to start with image pull errors:

```bash
# Verify local image exists
docker images | grep gshub-api

# If using k3d, import the image
k3d image import gshub-api -c gshub-dev
```

### Wrong Image in Production

If prod is using the wrong image:

```bash
# Verify prod patch is correct
cat k8s/overlays/prod/api-image-patch.yaml

# Check what image is rendered
kubectl kustomize k8s/overlays/prod | grep "image:" | grep api

# Check running pod
kubectl get deployment api -n gshub -o jsonpath='{.spec.template.spec.containers[0].image}'
```

## Common Commands

```bash
# View all resources in gshub namespace
kubectl get all -n gshub

# Watch resources being created
kubectl get all -n gshub -w

# View rendered manifests
kubectl kustomize k8s/overlays/dev

# Apply changes
kubectl apply -k k8s/overlays/dev

# Delete all resources
kubectl delete -k k8s/overlays/dev

# Check pod logs
kubectl logs -n gshub -l app=api --tail=50 -f

# Port forward API
kubectl port-forward -n gshub svc/api 8080:8080

# Shell into API pod
kubectl exec -it deployment/api -n gshub -- /bin/sh
```

## Adding New Environments

To add a new environment (e.g., staging):

```bash
# Create overlay directory
mkdir -p k8s/overlays/staging

# Create kustomization.yaml
cat > k8s/overlays/staging/kustomization.yaml <<EOF
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

bases:
  - ../../base

commonLabels:
  environment: staging

patchesStrategicMerge:
  - api-image-patch.yaml
EOF

# Create patches as needed
# ...

# Deploy
kubectl apply -k k8s/overlays/staging
```

## References

- [Kustomize Documentation](https://kustomize.io/)
- [Kubernetes Labels and Selectors](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/)
- [Strategic Merge Patches](https://kubectl.docs.kubernetes.io/references/kustomize/kustomization/patchesstrategicmerge/)
- [Agones Documentation](https://agones.dev/site/docs/)
