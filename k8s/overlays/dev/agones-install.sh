#!/bin/bash
# Agones installation script for Development Environment (k3d)

set -e

echo "Installing Agones for Development Environment (k3d)"
echo "=================================================="

# Add Agones Helm repository if not already added
helm repo add agones https://agones.dev/chart/stable 2>/dev/null || true
helm repo update

# Install or upgrade Agones with development settings
helm upgrade --install agones agones/agones \
  --namespace agones-system \
  --create-namespace \
  --set agones.controller.portRanges[0].minPort=7000 \
  --set agones.controller.portRanges[0].maxPort=7050 \
  --set agones.controller.portRanges[0].protocol=TCP \
  --set agones.controller.portRanges[1].minPort=7000 \
  --set agones.controller.portRanges[1].maxPort=7050 \
  --set agones.controller.portRanges[1].protocol=UDP \
  --set gameservers.namespaces[0]=gshub \
  --wait \
  --timeout 5m

echo ""
echo "Agones installed successfully!"
echo "Port range: 7000-7050 (TCP/UDP)"
echo "Watching namespace: gshub"
echo ""
echo "Note: No node selector configured (suitable for single-node k3d)"
