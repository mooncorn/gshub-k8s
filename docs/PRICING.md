# Pricing Strategy Analysis for gshub-k8s

## Your Infrastructure Costs

| Component | Specs | Monthly Cost (CAD) |
|-----------|-------|-------------------|
| Agent VPS | 4 vCores, 8GB RAM, 75GB Storage | $8 |
| Control Plane | 4 vCores, 8GB RAM | $8 |
| **Total Fixed** | | **$16 CAD (~$12 USD)** |

---

## Server Capacity Per Agent Node

Based on your game catalog resource requirements:

### Minecraft
| Plan | CPU | RAM | Storage | Max Servers/Node | Bottleneck |
|------|-----|-----|---------|------------------|------------|
| Small | 1 | 2Gi | 5Gi | **4** | RAM/CPU |
| Medium | 2 | 4Gi | 10Gi | **2** | RAM/CPU |
| Large | 4 | 8Gi | 20Gi | **1** | RAM/CPU |

### Valheim
| Plan | CPU | RAM | Storage | Max Servers/Node | Bottleneck |
|------|-----|-----|---------|------------------|------------|
| Small | 2 | 4Gi | 5Gi | **2** | RAM/CPU |
| Medium | 3 | 6Gi | 10Gi | **1** | RAM/CPU |

---

## Break-Even Analysis

### Scenario 1: Single Agent Node (current setup)
- Fixed costs: $16 CAD/month ($8 control + $8 agent)
- Control plane can support many agent nodes before scaling

**Per-Server Break-Even (to cover $8 agent + proportional control plane):**

| Game | Plan | Servers/Node | Break-Even/Server |
|------|------|--------------|-------------------|
| Minecraft | Small | 4 | **$4 CAD ($3 USD)** |
| Minecraft | Medium | 2 | **$8 CAD ($6 USD)** |
| Minecraft | Large | 1 | **$16 CAD ($12 USD)** |
| Valheim | Small | 2 | **$8 CAD ($6 USD)** |
| Valheim | Medium | 1 | **$16 CAD ($12 USD)** |

### Scenario 2: Scaling (10+ customers)
Control plane cost becomes negligible per customer:
- At 20 small Minecraft servers (5 agent nodes): $48 CAD total
- Per-server cost: **$2.40 CAD ($1.75 USD)**

---

## Competitive Market Pricing (2025)

### Minecraft Hosting
| Tier | Price Range (USD) | RAM | Competitors |
|------|-------------------|-----|-------------|
| Budget | $1-3/mo | 1-2GB | PebbleHost, SparkedHost, ScalaCube |
| Mid-range | $4-8/mo | 2-4GB | Shockbyte, Apex Hosting |
| Premium | $10-15/mo | 4-8GB | Apex, BisectHosting |

### Valheim Hosting
| Tier | Price Range (USD) | RAM | Competitors |
|------|-------------------|-----|-------------|
| Budget | $5-8/mo | 2-4GB | IONOS, Cybrancee, 4NetPlayers |
| Mid-range | $10-15/mo | 4-8GB | ScalaCube, Host Havoc |
| Premium | $18-25/mo | 8GB+ | G-Portal, Nitrado |

---

## Recommended Pricing Strategy

### Minimum Viable Pricing (Break-Even + Small Margin)

| Game | Plan | Min Price (USD) | Min Price (CAD) | Margin |
|------|------|-----------------|-----------------|--------|
| Minecraft | Small | **$4/mo** | $5.50/mo | 33% |
| Minecraft | Medium | **$7/mo** | $9.50/mo | 17% |
| Minecraft | Large | **$14/mo** | $19/mo | 17% |
| Valheim | Small | **$7/mo** | $9.50/mo | 17% |
| Valheim | Medium | **$14/mo** | $19/mo | 17% |

### Competitive Pricing (Maximum while staying attractive)

| Game | Plan | Competitive Price (USD) | Competitive Price (CAD) | Margin |
|------|------|------------------------|-------------------------|--------|
| Minecraft | Small | **$5/mo** | $7/mo | 67% |
| Minecraft | Medium | **$9/mo** | $12/mo | 50% |
| Minecraft | Large | **$17/mo** | $23/mo | 42% |
| Valheim | Small | **$9/mo** | $12/mo | 50% |
| Valheim | Medium | **$15/mo** | $20/mo | 25% |

---

## Final Recommended Pricing Tiers

### Launch Pricing (Aggressive - to gain customers)

| Game | Plan | Price (USD) | Price (CAD) | vs Market |
|------|------|-------------|-------------|-----------|
| Minecraft | Small | **$3.99** | $5.49 | Cheapest tier |
| Minecraft | Medium | **$6.99** | $9.49 | Below average |
| Minecraft | Large | **$13.99** | $18.99 | Competitive |
| Valheim | Small | **$6.99** | $9.49 | Below average |
| Valheim | Medium | **$12.99** | $17.49 | Competitive |

### Sustainable Pricing (After gaining traction)

| Game | Plan | Price (USD) | Price (CAD) | vs Market |
|------|------|-------------|-------------|-----------|
| Minecraft | Small | **$4.99** | $6.99 | Competitive |
| Minecraft | Medium | **$8.99** | $12.49 | Average |
| Minecraft | Large | **$16.99** | $22.99 | Average |
| Valheim | Small | **$8.99** | $12.49 | Average |
| Valheim | Medium | **$14.99** | $20.49 | Average |

---

## Key Insights

### Profitability Thresholds
- **2 Minecraft Small servers** covers one agent node ($8 CAD)
- **4 Minecraft Small servers** covers entire infrastructure ($16 CAD)
- After 4 customers: pure profit on existing infrastructure

### Scaling Economics
- Each new agent node ($8 CAD) adds 4 small Minecraft slots
- At scale, your cost-per-server drops to ~$2 CAD
- This gives room for promotions and discounts

### Competitive Advantages
- Kubernetes/Agones = better uptime than shared hosting
- Static ports = no random port assignments
- Can emphasize "dedicated resources" vs. oversold competitors

### Risks
- Large plans are less profitable (1 server/node)
- Valheim requires more resources than Minecraft
- Need 4+ small customers or 2+ medium to break even per node

---

## Sources
- [WiseHosting - Cheapest Minecraft Hosting 2025](https://wisehosting.com/blog/cheapest-minecraft-server-hosting-providers)
- [HostAdvice - Cheap Minecraft Hosting](https://hostadvice.com/game-server-hosting/best-minecraft/cheap/)
- [Apex Hosting Pricing](https://apexminecrafthosting.com/pricing/)
- [4NetPlayers Valheim](https://www.4netplayers.com/en-us/gameserver-hosting/valheim/)
- [CyberNews - Valheim Hosting 2025](https://cybernews.com/best-web-hosting/valheim-server-hosting/)
- [Geekflare - Valheim Hosting](https://geekflare.com/hosting/best-valheim-server-hosting/)
