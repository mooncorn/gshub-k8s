#!/bin/bash
# GSHUB Development Environment Setup Script
# This script initializes a local k3d cluster for development

# Strict error handling
set -e          # Exit on any error
set -u          # Exit on undefined variable
set -o pipefail # Exit on pipe failures

# Script configuration
CLUSTER_NAME="gshub"
K3D_PORT_RANGE="7000-7050"
REQUIRED_TOOLS=("k3d" "helm" "skaffold" "kubectl")

# Logging functions
function log_info() {
    echo "[INFO] $1"
}

function log_success() {
    echo "[SUCCESS] $1"
}

function log_error() {
    echo "[ERROR] $1" >&2
}

function log_warning() {
    echo "[WARNING] $1"
}

# Check if all required tools are installed
function check_prerequisites() {
    log_info "Checking prerequisites..."

    local missing_tools=()

    for tool in "${REQUIRED_TOOLS[@]}"; do
        if ! command -v "$tool" &> /dev/null; then
            missing_tools+=("$tool")
        else
            local version=""
            case "$tool" in
                k3d)
                    version=$(k3d version | head -n1)
                    ;;
                helm)
                    version=$(helm version --short 2>/dev/null || echo "version unknown")
                    ;;
                skaffold)
                    version=$(skaffold version)
                    ;;
                kubectl)
                    version=$(kubectl version --client --short 2>/dev/null || kubectl version --client 2>/dev/null | head -n1)
                    ;;
            esac
            log_info "  $tool found ($version)"
        fi
    done

    if [ ${#missing_tools[@]} -gt 0 ]; then
        log_error "Missing required tools: ${missing_tools[*]}"
        log_error "Please install the missing tools before running this script."
        echo ""
        log_info "Installation instructions:"
        log_info "  k3d:      curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash"
        log_info "  helm:     curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash"
        log_info "  skaffold: curl -Lo skaffold https://storage.googleapis.com/skaffold/releases/latest/skaffold-linux-amd64"
        log_info "  kubectl:  curl -LO https://dl.k8s.io/release/\$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
        return 1
    fi

    log_success "All prerequisites are installed"
    return 0
}

# Check if cluster exists
function cluster_exists() {
    k3d cluster list 2>/dev/null | grep -q "^${CLUSTER_NAME}"
}

# Prompt user if they want to delete existing cluster
function prompt_cluster_reset() {
    if ! cluster_exists; then
        log_info "Cluster '${CLUSTER_NAME}' does not exist, will create new one"
        return 0
    fi

    log_warning "Cluster '${CLUSTER_NAME}' already exists"
    echo ""
    echo "This script will DELETE the existing cluster and create a fresh one."
    echo "All data, deployments, and configurations will be lost."
    echo ""
    read -p "Do you want to delete and recreate the cluster? (yes/no) [no]: " -r response

    # Default to "no" if empty
    response=${response:-no}

    # Normalize to lowercase
    response=$(echo "$response" | tr '[:upper:]' '[:lower:]')

    case "$response" in
        yes|y)
            log_info "User confirmed cluster deletion"
            return 0
            ;;
        no|n|*)
            log_info "User cancelled cluster deletion"
            log_info "Exiting without making changes"
            return 1
            ;;
    esac
}

# Delete existing cluster
function delete_cluster() {
    log_info "Deleting existing cluster '${CLUSTER_NAME}'..."
    k3d cluster delete "$CLUSTER_NAME"
    log_success "Cluster deleted"
}

# Create new k3d cluster
function create_cluster() {
    log_info "Creating k3d cluster '${CLUSTER_NAME}'..."
    log_info "  Ports: ${K3D_PORT_RANGE}:${K3D_PORT_RANGE}"
    log_info "  Disabled: traefik, servicelb"

    k3d cluster create "$CLUSTER_NAME" \
        --api-port 6443 \
        --servers 1 \
        --agents 0 \
        --port "${K3D_PORT_RANGE}:${K3D_PORT_RANGE}@server:0" \
        --k3s-arg "--disable=traefik@server:0" \
        --k3s-arg "--disable=servicelb@server:0"

    log_success "Cluster created successfully"
}

# Label node for development environment
function label_node() {
    log_info "Labeling node for development environment..."

    local node_name
    node_name=$(kubectl get nodes -o jsonpath='{.items[0].metadata.name}')

    if [ -z "$node_name" ]; then
        log_error "Failed to get node name"
        return 1
    fi

    log_info "  Node: $node_name"

    kubectl label node "$node_name" \
        workload-type=control-plane \
        --overwrite

    kubectl label node "$node_name" \
        node-role.kubernetes.io/gameserver=true \
        --overwrite

    kubectl label node "$node_name" \
        platform.io/public-ip=localhost \
        --overwrite

    log_success "Node labeled successfully"
}

# Setup Helm repositories
function setup_helm_repo() {
    log_info "Setting up Helm repositories..."

    helm repo add agones https://agones.dev/chart/stable 2>/dev/null || true
    helm repo update

    log_success "Helm repositories configured"
}

# Create gshub namespace
function create_namespace() {
    log_info "Creating gshub namespace..."

    kubectl create namespace gshub --dry-run=client -o yaml | kubectl apply -f -
}

# Install Agones
function install_agones() {
    log_info "Installing Agones..."

    if [ ! -f "k8s/overlays/dev/agones-install.sh" ]; then
        log_error "Agones install script not found at: k8s/overlays/dev/agones-install.sh"
        return 1
    fi

    bash k8s/overlays/dev/agones-install.sh

    log_success "Agones installation complete"
}

# Start Skaffold development workflow
function start_skaffold() {
    log_info "Starting Skaffold development workflow..."
    log_info "This will build, deploy, and watch for changes"
    log_info "Press Ctrl+C to stop"
    echo ""

    cd api
    skaffold dev
}

# Main function
function main() {
    # Step 1: Check prerequisites
    if ! check_prerequisites; then
        exit 1
    fi

    # Step 2: Prompt user if cluster exists
    if ! prompt_cluster_reset; then
        exit 2  # User cancelled
    fi

    # Step 3: Delete cluster if it exists
    if cluster_exists; then
        delete_cluster
  
    fi

    # Step 4: Create new cluster
    create_cluster

    # Step 5: Label node
    label_node

    # Step 6: Setup Helm repositories
    setup_helm_repo

    # Step 7: Create namespace
    create_namespace

    # Step 8: Install Agones
    install_agones

    # Step 9: Start Skaffold
    log_success "Setup Complete"
    log_info "You can now run:"
    log_info " - cd api"
    log_info " - skaffold dev"
}

# Execute main function
main
