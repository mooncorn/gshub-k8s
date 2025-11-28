# gshub-k8s

### Getting Started

#### Prerequisites 
- Docker
- K3d
- Stripe CLI

#### Setup
##### Create cluster
```
k3d cluster create gshub
```

##### Install dependencies
- Agones
```
kubectl create namespace agones-system
kubectl apply --server-side -f https://raw.githubusercontent.com/googleforgames/agones/release-1.53.0/install/yaml/install.yaml
```

- Skaffold
```
# For Linux x86_64 (amd64)
curl -Lo skaffold https://storage.googleapis.com/skaffold/releases/latest/skaffold-linux-amd64 && \
sudo install skaffold /usr/local/bin/

# For macOS on ARMv8 (arm64)
curl -Lo skaffold https://storage.googleapis.com/skaffold/releases/latest/skaffold-darwin-arm64 && \
sudo install skaffold /usr/local/bin/
```

- Apply k3s configurations
```
kubectl apply -f ./k8s/namespaces.yaml
kubectl apply -f ./k8s/secrets.yaml
kubectl apply -f ./k8s/gshub/game-catalog.yaml
kubectl apply -f ./k8s/gshub/rbac.yaml
kubectl apply -f ./k8s/gshub/postgresql.yaml
```

#### Setup Stripe products and webhook
```
stripe listen --forward-to localhost:3000/webhook
```

### Get Mailersend API key

#### Set environment variables

Refer to:
 - /k8s/secrets.yaml
 - /k8s/gshub/api.yaml

#### Run the app
```
cd api
skaffold dev
```

### Notes
- To access the database from outside the cluster, forward the port:
```
kubectl port-forward -n gshub svc/postgresql 5432:5432
```