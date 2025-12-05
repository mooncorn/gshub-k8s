#!/bin/bash
# Agones installation script for Production Environment (k3s)

set -e

echo "Installing Agones for Production Environment (k3s)"
echo "=================================================="

# Add Agones Helm repository if not already added
helm repo add agones https://agones.dev/chart/stable 2>/dev/null || true
helm repo update

# Install or upgrade Agones with production settings
# All components need tolerations to run on master/control-plane nodes
helm upgrade --install agones agones/agones \
    --namespace agones-system \
    --create-namespace \
    --set agones.controller.portRanges[0].minPort=25501 \
    --set agones.controller.portRanges[0].maxPort=25999 \
    --set agones.controller.portRanges[0].protocol=TCP \
    --set agones.controller.portRanges[1].minPort=25501 \
    --set agones.controller.portRanges[1].maxPort=25999 \
    --set agones.controller.portRanges[1].protocol=UDP \
    --set agones.controller.tolerations[0].key="node-role.kubernetes.io/master" \
    --set agones.controller.tolerations[0].operator=Equal \
    --set-string agones.controller.tolerations[0].value="true" \
    --set agones.controller.tolerations[0].effect=NoSchedule \
    --set agones.allocator.tolerations[0].key="node-role.kubernetes.io/master" \
    --set agones.allocator.tolerations[0].operator=Equal \
    --set-string agones.allocator.tolerations[0].value="true" \
    --set agones.allocator.tolerations[0].effect=NoSchedule \
    --set agones.ping.tolerations[0].key="node-role.kubernetes.io/master" \
    --set agones.ping.tolerations[0].operator=Equal \
    --set-string agones.ping.tolerations[0].value="true" \
    --set agones.ping.tolerations[0].effect=NoSchedule \
    --set agones.extensions.tolerations[0].key="node-role.kubernetes.io/master" \
    --set agones.extensions.tolerations[0].operator=Equal \
    --set-string agones.extensions.tolerations[0].value="true" \
    --set agones.extensions.tolerations[0].effect=NoSchedule \
    --set gameservers.namespaces[0]=gshub \
    --wait \
    --timeout 5m

echo ""
echo "Agones installed successfully!"
echo "Port range: 25501-25999 (TCP/UDP)"
echo "Watching namespace: gshub"
echo "Controller scheduled on: master nodes only"
