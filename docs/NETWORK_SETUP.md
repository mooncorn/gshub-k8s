# Network Setup Guide

This guide covers different networking approaches for connecting agent nodes to the master node in production.

## Overview

Agent nodes need to communicate with the master node's Kubernetes API server (port 6443) to join the cluster. You have several options depending on your infrastructure and security requirements.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        Internet                              │
└──────────────▲──────────────────────────────────────────────┘
               │
               │ Cloudflare Tunnel (for API/Web traffic only)
               │
┌──────────────┴──────────────────────────────────────────────┐
│                     Master Node                              │
│  ┌─────────────────────────────────────────────────────────┐│
│  │  Public IP: 203.0.113.10                                ││
│  │  Private/VPN IP: 10.0.0.1                               ││
│  │                                                          ││
│  │  Port 6443: Kubernetes API (for agents)                 ││
│  │  Port 8080: HTTP API (via Cloudflare Tunnel)            ││
│  └─────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────┘
               │
               │ K3s API Connection (port 6443)
               │ via Public IP + Firewall OR VPN
               │
    ┌──────────┴──────────┐
    │                     │
┌───▼─────────────────┐ ┌─▼───────────────────┐
│  Agent Node 1       │ │  Agent Node 2       │
│  Public: 45.x.x.10  │ │  Public: 45.y.y.20  │
│  Private: 10.0.0.11 │ │  Private: 10.0.0.12 │
│                     │ │                     │
│  GameServers run    │ │  GameServers run    │
│  here               │ │  here               │
└─────────────────────┘ └─────────────────────┘
         │                        │
         │ Players connect here   │
         └────────────────────────┘
            (Public IPs + Ports 7000-8000)
```

## Option 1: Public IP with Firewall (Simplest)

Use this if your master node has a public IP and you want a simple setup.

### Pros
- Simple to set up
- No additional software required
- Works out of the box

### Cons
- Master's public IP is exposed (mitigated by firewall)
- Less secure than VPN approaches

### Setup

**On Master Node:**

1. **Get master's public IP:**
```bash
curl -4 ifconfig.me
# Example output: 203.0.113.10
```

2. **Configure firewall to allow agent connections:**
```bash
# Allow k3s API from specific agent IPs
sudo ufw allow from 45.x.x.10 to any port 6443 comment 'k3s API - agent-01'
sudo ufw allow from 45.y.y.20 to any port 6443 comment 'k3s API - agent-02'

# Verify rules
sudo ufw status numbered
```

3. **Install k3s with public IP as TLS SAN:**
```bash
PUBLIC_IP=$(curl -4 -s ifconfig.me)

curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="server \
  --disable traefik \
  --disable servicelb \
  --node-taint node-role.kubernetes.io/master=true:NoSchedule \
  --tls-san $PUBLIC_IP \
  --advertise-address $PUBLIC_IP" sh -
```

**On Agent Nodes:**

1. **Join cluster using master's public IP:**
```bash
MASTER_PUBLIC_IP="203.0.113.10"
JOIN_TOKEN="<token-from-master>"

curl -sfL https://get.k3s.io | \
  K3S_URL="https://${MASTER_PUBLIC_IP}:6443" \
  K3S_TOKEN="${JOIN_TOKEN}" \
  INSTALL_K3S_EXEC="agent" sh -
```

2. **Verify connection:**
```bash
sudo systemctl status k3s-agent
sudo journalctl -u k3s-agent -f
```

---

## Option 2: WireGuard VPN (Recommended for Security)

Use this for a secure private network between all nodes.

### Pros
- Most secure option
- Encrypted communication
- Master doesn't need public IP
- Can restrict k3s API to VPN network only

### Cons
- Requires additional setup
- Need to manage VPN configuration

### Setup

**Install WireGuard on All Nodes:**

```bash
# Ubuntu/Debian
sudo apt update
sudo apt install wireguard wireguard-tools

# Verify
wg --version
```

**On Master Node:**

1. **Generate keys:**
```bash
cd /etc/wireguard
sudo su
wg genkey | tee privatekey | wg pubkey > publickey
chmod 600 privatekey
```

2. **Create WireGuard config:**
```bash
cat <<EOF > /etc/wireguard/wg0.conf
[Interface]
PrivateKey = $(cat /etc/wireguard/privatekey)
Address = 10.0.0.1/24
ListenPort = 51820
PostUp = iptables -A FORWARD -i wg0 -j ACCEPT; iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
PostDown = iptables -D FORWARD -i wg0 -j ACCEPT; iptables -t nat -D POSTROUTING -o eth0 -j MASQUERADE

# Agent 1
[Peer]
PublicKey = <agent-1-public-key>
AllowedIPs = 10.0.0.11/32

# Agent 2
[Peer]
PublicKey = <agent-2-public-key>
AllowedIPs = 10.0.0.12/32
EOF
```

3. **Enable and start WireGuard:**
```bash
sudo systemctl enable wg-quick@wg0
sudo systemctl start wg-quick@wg0

# Verify
sudo wg show
```

4. **Allow WireGuard port:**
```bash
sudo ufw allow 51820/udp
```

5. **Get master's public key:**
```bash
sudo cat /etc/wireguard/publickey
# Save this for agent configuration
```

**On Each Agent Node:**

1. **Generate keys:**
```bash
cd /etc/wireguard
sudo su
wg genkey | tee privatekey | wg pubkey > publickey
chmod 600 privatekey
```

2. **Get agent's public key** (send this to master admin):
```bash
sudo cat /etc/wireguard/publickey
```

3. **Create WireGuard config:**
```bash
cat <<EOF > /etc/wireguard/wg0.conf
[Interface]
PrivateKey = $(cat /etc/wireguard/privatekey)
Address = 10.0.0.11/24  # Use .11 for agent-1, .12 for agent-2, etc.

[Peer]
PublicKey = <master-public-key>
Endpoint = <master-public-ip>:51820
AllowedIPs = 10.0.0.0/24
PersistentKeepalive = 25
EOF
```

4. **Enable and start WireGuard:**
```bash
sudo systemctl enable wg-quick@wg0
sudo systemctl start wg-quick@wg0

# Verify
sudo wg show
```

5. **Test VPN connectivity:**
```bash
ping -c 3 10.0.0.1  # Master's VPN IP
```

**Install k3s Using VPN IPs:**

**Master:**
```bash
curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="server \
  --disable traefik \
  --disable servicelb \
  --node-taint node-role.kubernetes.io/master=true:NoSchedule \
  --tls-san 10.0.0.1 \
  --advertise-address 10.0.0.1 \
  --node-ip 10.0.0.1" sh -
```

**Agents:**
```bash
MASTER_VPN_IP="10.0.0.1"
JOIN_TOKEN="<token-from-master>"

curl -sfL https://get.k3s.io | \
  K3S_URL="https://${MASTER_VPN_IP}:6443" \
  K3S_TOKEN="${JOIN_TOKEN}" \
  INSTALL_K3S_EXEC="agent --node-ip 10.0.0.11" sh -
  # Use .11 for agent-1, .12 for agent-2, etc.
```

---

## Option 3: Tailscale (Easiest VPN)

Tailscale provides a managed WireGuard VPN with zero configuration.

### Pros
- Extremely easy setup
- Managed solution (no key management)
- Works behind NAT/firewalls
- Free for personal use (up to 20 devices)

### Cons
- Relies on third-party service
- May have latency overhead

### Setup

**On All Nodes (Master + Agents):**

1. **Install Tailscale:**
```bash
curl -fsSL https://tailscale.com/install.sh | sh
```

2. **Authenticate:**
```bash
sudo tailscale up

# Follow the URL to authenticate
# This will assign each node a Tailscale IP (e.g., 100.x.x.x)
```

3. **Get Tailscale IP:**
```bash
tailscale ip -4
# Example: 100.64.0.1 (master)
#          100.64.0.11 (agent-1)
#          100.64.0.12 (agent-2)
```

**Install k3s Using Tailscale IPs:**

**Master:**
```bash
TAILSCALE_IP=$(tailscale ip -4)

curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="server \
  --disable traefik \
  --disable servicelb \
  --node-taint node-role.kubernetes.io/master=true:NoSchedule \
  --tls-san $TAILSCALE_IP \
  --advertise-address $TAILSCALE_IP \
  --node-ip $TAILSCALE_IP" sh -
```

**Agents:**
```bash
MASTER_TAILSCALE_IP="100.64.0.1"  # Master's Tailscale IP
AGENT_TAILSCALE_IP=$(tailscale ip -4)
JOIN_TOKEN="<token-from-master>"

curl -sfL https://get.k3s.io | \
  K3S_URL="https://${MASTER_TAILSCALE_IP}:6443" \
  K3S_TOKEN="${JOIN_TOKEN}" \
  INSTALL_K3S_EXEC="agent --node-ip $AGENT_TAILSCALE_IP" sh -
```

**Benefits:**
- Nodes can be anywhere (different data centers, cloud providers, even home networks)
- Automatic reconnection
- Built-in access control

---

## Option 4: Cloud Provider Private Network

If using a cloud provider (AWS, GCP, Azure, DigitalOcean, etc.), use their private networking.

### AWS VPC
```bash
# Use private IPs from VPC
# Master: 10.0.1.10
# Agents: 10.0.1.11, 10.0.1.12, etc.

# Security group: Allow port 6443 within VPC
```

### DigitalOcean VPC
```bash
# Enable VPC for all droplets
# Use private network IPs for k3s communication
```

### Configuration
Same as Option 1, but use private network IPs instead of public IPs:

```bash
# Master
curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="server \
  --node-ip <private-ip> \
  --advertise-address <private-ip>" sh -

# Agents
K3S_URL="https://<master-private-ip>:6443"
```

---

## Security Best Practices

### Regardless of Option Chosen:

1. **Restrict k3s API Access:**
```bash
# Only allow necessary IPs
sudo ufw default deny incoming
sudo ufw allow ssh
sudo ufw allow from <trusted-ip-range> to any port 6443
```

2. **Use Strong Join Token:**
```bash
# The k3s token is automatically generated
# Keep it secure - it's like a cluster password
sudo cat /var/lib/rancher/k3s/server/node-token
```

3. **Enable Audit Logging:**
```bash
# Add to k3s server args
--kube-apiserver-arg=audit-log-path=/var/log/k3s-audit.log
```

4. **Regular Updates:**
```bash
# Update k3s periodically
curl -sfL https://get.k3s.io | sh -
```

---

## Comparison Table

| Option | Security | Complexity | Cost | Best For |
|--------|----------|------------|------|----------|
| Public IP + Firewall | Medium | Low | Free | Simple setups, same datacenter |
| WireGuard VPN | High | Medium | Free | Security-conscious deployments |
| Tailscale | High | Very Low | Free/Paid | Multi-cloud, ease of use |
| Cloud VPC | High | Low | Included | Cloud-native deployments |

---

## Recommendation

**For production**: Use **WireGuard (Option 2)** or **Tailscale (Option 3)**
- Provides encryption and security
- Isolates cluster communication from public internet
- Allows flexible deployment across providers

**For testing/development**: Use **Public IP + Firewall (Option 1)**
- Quick to set up
- Good enough with proper firewall rules

**For cloud deployments**: Use **Cloud Provider Private Network (Option 4)**
- Native integration
- Best performance
- Included in cloud pricing

---

## Troubleshooting

### Agent Cannot Connect to Master

**Check connectivity:**
```bash
# From agent node
nc -zv <master-ip> 6443

# If using VPN, check VPN connection first
ping <master-vpn-ip>
```

**Check firewall:**
```bash
# On master
sudo ufw status
sudo iptables -L -n | grep 6443
```

**Check k3s logs:**
```bash
# On master
sudo journalctl -u k3s -f

# On agent
sudo journalctl -u k3s-agent -f
```

### Certificate Errors

If you see certificate errors, ensure `--tls-san` includes all IPs/hostnames agents will use:

```bash
# Reinstall with correct TLS SANs
curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="server \
  --tls-san <public-ip> \
  --tls-san <private-ip> \
  --tls-san <vpn-ip> \
  --tls-san <hostname>" sh -
```

---

## Important Notes

### Cloudflare Tunnel is NOT for Cluster Communication

**Cloudflare Tunnel is used for:**
- ✅ HTTP/HTTPS traffic to your API service
- ✅ Web dashboard access
- ✅ User-facing services

**Cloudflare Tunnel is NOT used for:**
- ❌ Kubernetes API server (port 6443)
- ❌ Agent-to-master communication
- ❌ Cluster infrastructure

### Port Requirements

| Port | Protocol | Purpose | Source |
|------|----------|---------|--------|
| 6443 | TCP | Kubernetes API | Agents → Master |
| 51820 | UDP | WireGuard VPN | All nodes (if using WireGuard) |
| 7000-8000 | TCP/UDP | Game server ports | Players → Agents |
| 22 | TCP | SSH management | Admin |

### Game Server Connectivity is Separate

Remember: **Game server traffic is completely separate from cluster communication.**

- **Cluster communication** (agents ↔ master): Can use private IPs/VPN
- **Game server traffic** (players → agents): Must use public IPs on agents

Players connect directly to agent nodes' public IPs on ports 7000-8000. This traffic never goes through the master or Cloudflare tunnel.

---

## Next Steps

After setting up networking:

1. Follow [GETTING_STARTED.md](./GETTING_STARTED.md) for application deployment
2. Verify agents can join the cluster: `kubectl get nodes`
3. Label agent nodes with their **public IPs** (for player connections):
   ```bash
   kubectl label node agent-01 platform.io/public-ip=<agent-public-ip>
   ```
4. Deploy game servers and test connectivity
