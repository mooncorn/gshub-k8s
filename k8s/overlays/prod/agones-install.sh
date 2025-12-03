#!/bin/bash
# Agones installation script for Production Environment (k3s)

set -e

echo "Installing Agones for Production Environment (k3s)"
echo "=================================================="

# Add Agones Helm repository if not already added
helm repo add agones https://agones.dev/chart/stable 2>/dev/null || true
helm repo update

# Install or upgrade Agones with production settings
helm upgrade --install agones agones/agones \
  --namespace agones-system \
  --create-namespace \
  --set agones.controller.portRanges[0].minPort=7000 \
  --set agones.controller.portRanges[0].maxPort=8000 \
  --set agones.controller.portRanges[0].protocol=TCP \
  --set agones.controller.portRanges[1].minPort=7000 \
  --set agones.controller.portRanges[1].maxPort=8000 \
  --set agones.controller.portRanges[1].protocol=UDP \
  --set controller.nodeSelector."node-role\.kubernetes\.io/master"="true" \
  --set controller.tolerations[0].key="node-role.kubernetes.io/master" \
  --set controller.tolerations[0].operator=Equal \
  --set controller.tolerations[0].value="true" \
  --set controller.tolerations[0].effect=NoSchedule \
  --set gameservers.namespaces[0]=gshub \
  --wait \
  --timeout 5m

echo ""
echo "Agones installed successfully!"
echo "Port range: 7000-8000 (TCP/UDP)"
echo "Watching namespace: gshub"
echo "Controller scheduled on: master nodes only"
