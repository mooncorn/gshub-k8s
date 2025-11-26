# Game Server Hosting Frontend Design

## Overview

Minimalistic React frontend built with Vite. Users can create a server and start playing in under 2 minutes with zero configuration required.

---

## Tech Stack

| Component | Choice                          |
| --------- | ------------------------------- |
| Framework | Vite                            |
| Styling   | Tailwind CSS + shadcn/ui        |
| State     | React Query (TanStack Query)    |
| Forms     | React Hook Form + Zod           |
| Auth      | JWT (stored in httpOnly cookie) |
| WebSocket | Native WebSocket (console)      |

---

## Pages

|Route|Page|Auth|
|---|---|---|
|`/`|Landing|No|
|`/login`|Login|No|
|`/register`|Register|No|
|`/dashboard`|Server list|Yes|
|`/servers/new`|Create server|Yes|
|`/servers/[id]`|Server details|Yes|
|`/servers/[id]/console`|Live console|Yes|
|`/servers/[id]/files`|File browser|Yes|
|`/account`|Profile & billing|Yes|

---

## User Flow

```
Landing → Register → Select Game → Select Plan → Done
                                                   │
              ← 3 clicks, ~60 seconds →            │
                                                   ▼
                                              Dashboard
                                                   │
                                    Click server name
                                                   │
                                                   ▼
                                             Server Page
                                    (Console, Settings, Files)
```

---

## Page Designs

### 1. Create Server (`/servers/new`)

Single page, 3-step form. No page transitions.

```
┌─────────────────────────────────────────────────────────────────┐
│  ← Back                                                          │
│                                                                  │
│  Create Server                                                   │
│                                                                  │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ① Select Game                                                   │
│                                                                  │
│  ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌────────────┐    │
│  │    🎮      │ │    ⚔️      │ │    🦖      │ │    🔫      │    │
│  │ Minecraft  │ │  Valheim   │ │    ARK     │ │   Rust     │    │
│  │   Java     │ │            │ │            │ │            │    │
│  │     ✓      │ │            │ │            │ │            │    │
│  └────────────┘ └────────────┘ └────────────┘ └────────────┘    │
│                                                                  │
│  ┌────────────┐ ┌────────────┐                                  │
│  │    🎮      │ │    🧟      │                                  │
│  │ Minecraft  │ │  7 Days    │                                  │
│  │  Bedrock   │ │  to Die    │                                  │
│  └────────────┘ └────────────┘                                  │
│                                                                  │
│                                                                  │
│  ② Select Plan                                                   │
│                                                                  │
│  ┌───────────────┐ ┌───────────────┐ ┌───────────────┐          │
│  │   Starter     │ │   Standard    │ │  Performance  │          │
│  │               │ │      ✓        │ │               │          │
│  │  2-5 players  │ │  5-15 players │ │ 15-30 players │          │
│  │  2 GB RAM     │ │  4 GB RAM     │ │  8 GB RAM     │          │
│  │               │ │               │ │               │          │
│  │    $3/mo      │ │    $6/mo      │ │    $12/mo     │          │
│  └───────────────┘ └───────────────┘ └───────────────┘          │
│                                                                  │
│                                                                  │
│  ③ Display Name                                                  │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │ My Minecraft Server                                      │    │
│  └─────────────────────────────────────────────────────────┘    │
│  Auto-generated. You can change this later.                      │
│                                                                  │
│                                                                  │
│                            ┌─────────────────────────────────┐   │
│                            │   Create Server — $6/mo  →      │   │
│                            └─────────────────────────────────┘   │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

#### Behavior

- Selecting a game auto-updates display name to "My [Game] Server"
- Plans shown are specific to selected game
- Button disabled until game + plan selected
- Clicking button → Stripe Checkout → redirect to dashboard on success

#### Component

```tsx
// app/servers/new/page.tsx
"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useQuery, useMutation } from "@tanstack/react-query";
import { api } from "@/lib/api";

export default function CreateServerPage() {
  const router = useRouter();
  const [selectedGame, setSelectedGame] = useState<string | null>(null);
  const [selectedPlan, setSelectedPlan] = useState<string | null>(null);
  const [displayName, setDisplayName] = useState("");

  const { data: games } = useQuery({
    queryKey: ["games"],
    queryFn: api.getGames,
  });

  const { data: plans } = useQuery({
    queryKey: ["plans", selectedGame],
    queryFn: () => api.getPlans(selectedGame!),
    enabled: !!selectedGame,
  });

  const createServer = useMutation({
    mutationFn: api.createServer,
    onSuccess: (data) => {
      // Redirect to Stripe Checkout
      window.location.href = data.checkoutUrl;
    },
  });

  const handleGameSelect = (gameId: string) => {
    setSelectedGame(gameId);
    setSelectedPlan(null);
    const game = games?.find((g) => g.id === gameId);
    setDisplayName(`My ${game?.name} Server`);
  };

  const handleSubmit = () => {
    createServer.mutate({
      game: selectedGame!,
      plan: selectedPlan!,
      displayName,
    });
  };

  return (
    <div className="max-w-3xl mx-auto py-8 px-4">
      <h1 className="text-2xl font-bold mb-8">Create Server</h1>

      {/* Step 1: Select Game */}
      <section className="mb-8">
        <h2 className="text-sm font-medium text-gray-500 mb-3">① Select Game</h2>
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
          {games?.map((game) => (
            <GameCard
              key={game.id}
              game={game}
              selected={selectedGame === game.id}
              onClick={() => handleGameSelect(game.id)}
            />
          ))}
        </div>
      </section>

      {/* Step 2: Select Plan */}
      {selectedGame && (
        <section className="mb-8">
          <h2 className="text-sm font-medium text-gray-500 mb-3">② Select Plan</h2>
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
            {plans?.map((plan) => (
              <PlanCard
                key={plan.id}
                plan={plan}
                selected={selectedPlan === plan.id}
                onClick={() => setSelectedPlan(plan.id)}
              />
            ))}
          </div>
        </section>
      )}

      {/* Step 3: Display Name */}
      {selectedPlan && (
        <section className="mb-8">
          <h2 className="text-sm font-medium text-gray-500 mb-3">③ Display Name</h2>
          <input
            type="text"
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
            className="w-full px-4 py-2 border rounded-lg"
            placeholder="My Server"
          />
          <p className="text-sm text-gray-400 mt-1">
            Auto-generated. You can change this later.
          </p>
        </section>
      )}

      {/* Submit */}
      <div className="flex justify-end">
        <button
          onClick={handleSubmit}
          disabled={!selectedGame || !selectedPlan || !displayName}
          className="px-6 py-3 bg-blue-600 text-white rounded-lg disabled:opacity-50"
        >
          Create Server — ${plans?.find((p) => p.id === selectedPlan)?.price}/mo →
        </button>
      </div>
    </div>
  );
}
```

---

### 2. Dashboard (`/dashboard`)

Minimal server list. Click server name to view details.

```
┌─────────────────────────────────────────────────────────────────┐
│  ☰ GameHost                                    user@email.com ▼ │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  My Servers                                      [+ New Server]  │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                                                             ││
│  │  My Minecraft Server                            ┌─────────┐ ││
│  │  mc-a1b2c3.play.example.com:25565     [Copy]    │ 3/10 🟢 ▼│ ││
│  │  CPU: 45%  •  RAM: 2.1/4 GB  •  Disk: 1.2/25 GB └─────────┘ ││
│  │                                                             ││
│  └─────────────────────────────────────────────────────────────┘│
│                                                                  │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                                                             ││
│  │  Valheim Server                                 ┌─────────┐ ││
│  │  vh-x7k2m9.play.example.com:2456      [Copy]    │ 0/4  🔴 ▼│ ││
│  │  Stopped                                        └─────────┘ ││
│  │                                                             ││
│  └─────────────────────────────────────────────────────────────┘│
│                                                                  │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                                                             ││
│  │  ARK Server                                     ┌─────────┐ ││
│  │  ark-p3q8r2.play.example.com:7777     [Copy]    │ 8/25 🟡 ▼│ ││
│  │  CPU: 89%  •  RAM: 11.2/12 GB  •  Disk: 45/60 GB└─────────┘ ││
│  │                                                             ││
│  └─────────────────────────────────────────────────────────────┘│
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

#### Dropdown (Expanded)

```
                                                     ┌─────────┐
                                                     │ 3/10 🟢 ▼│
                                                     ├─────────┤
                                                     │ Start   │  ← disabled when running
                                                     │ Stop    │
                                                     │ Restart │
                                                     └─────────┘
```

#### Status Colors

|Status|Border Color|Dot|
|---|---|---|
|Running (healthy)|Green|🟢|
|Running (high CPU/RAM >80%)|Yellow|🟡|
|Stopped|Red|🔴|
|Starting/Stopping|Yellow|🟡 (pulsing)|
|Error|Red|🔴|

#### Component

```tsx
// app/dashboard/page.tsx
"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { api } from "@/lib/api";
import { ServerCard } from "@/components/server-card";

export default function DashboardPage() {
  const { data: servers, isLoading } = useQuery({
    queryKey: ["servers"],
    queryFn: api.getServers,
    refetchInterval: 10000, // Poll every 10s for status updates
  });

  return (
    <div className="max-w-4xl mx-auto py-8 px-4">
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-2xl font-bold">My Servers</h1>
        <Link
          href="/servers/new"
          className="px-4 py-2 bg-blue-600 text-white rounded-lg"
        >
          + New Server
        </Link>
      </div>

      <div className="space-y-4">
        {servers?.map((server) => (
          <ServerCard key={server.id} server={server} />
        ))}

        {servers?.length === 0 && (
          <div className="text-center py-12 text-gray-500">
            No servers yet. Create your first one!
          </div>
        )}
      </div>
    </div>
  );
}
```

```tsx
// components/server-card.tsx
"use client";

import { useState } from "react";
import Link from "next/link";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { Server } from "@/types";
import { CopyButton } from "./copy-button";

interface ServerCardProps {
  server: Server;
}

export function ServerCard({ server }: ServerCardProps) {
  const [dropdownOpen, setDropdownOpen] = useState(false);
  const queryClient = useQueryClient();

  const statusColor = getStatusColor(server);
  const playerCount = server.players ?? 0;
  const maxPlayers = server.maxPlayers ?? 10;

  const startServer = useMutation({
    mutationFn: () => api.startServer(server.id),
    onSuccess: () => queryClient.invalidateQueries(["servers"]),
  });

  const stopServer = useMutation({
    mutationFn: () => api.stopServer(server.id),
    onSuccess: () => queryClient.invalidateQueries(["servers"]),
  });

  const restartServer = useMutation({
    mutationFn: () => api.restartServer(server.id),
    onSuccess: () => queryClient.invalidateQueries(["servers"]),
  });

  const isRunning = server.status === "running";
  const isStopped = server.status === "stopped";
  const isLoading = startServer.isLoading || stopServer.isLoading || restartServer.isLoading;

  return (
    <div className={`border-l-4 ${statusColor.border} bg-white rounded-lg shadow p-4`}>
      <div className="flex justify-between items-start">
        {/* Left side */}
        <div className="space-y-1">
          <Link
            href={`/servers/${server.id}`}
            className="text-lg font-semibold hover:text-blue-600"
          >
            {server.name}
          </Link>

          <div className="flex items-center gap-2 text-sm text-gray-600">
            <span className="font-mono">{server.dnsRecord}:{server.port}</span>
            <CopyButton text={`${server.dnsRecord}:${server.port}`} />
          </div>

          {isRunning ? (
            <div className="text-sm text-gray-500">
              CPU: {server.cpuPercent}% • RAM: {server.memoryUsed}/{server.memoryLimit} GB • Disk: {server.storageUsed}/{server.storageLimit} GB
            </div>
          ) : (
            <div className="text-sm text-gray-400">
              {server.status === "stopped" ? "Stopped" : server.status}
            </div>
          )}
        </div>

        {/* Right side - Dropdown */}
        <div className="relative">
          <button
            onClick={() => setDropdownOpen(!dropdownOpen)}
            disabled={isLoading}
            className={`flex items-center gap-2 px-3 py-1.5 border-2 ${statusColor.border} rounded-lg text-sm font-medium`}
          >
            <span>{playerCount}/{maxPlayers}</span>
            <span className={statusColor.dot}>●</span>
            <span>▼</span>
          </button>

          {dropdownOpen && (
            <div className="absolute right-0 mt-1 w-32 bg-white border rounded-lg shadow-lg z-10">
              <button
                onClick={() => { startServer.mutate(); setDropdownOpen(false); }}
                disabled={isRunning || isLoading}
                className="w-full px-4 py-2 text-left text-sm hover:bg-gray-50 disabled:text-gray-300"
              >
                Start
              </button>
              <button
                onClick={() => { stopServer.mutate(); setDropdownOpen(false); }}
                disabled={isStopped || isLoading}
                className="w-full px-4 py-2 text-left text-sm hover:bg-gray-50 disabled:text-gray-300"
              >
                Stop
              </button>
              <button
                onClick={() => { restartServer.mutate(); setDropdownOpen(false); }}
                disabled={isStopped || isLoading}
                className="w-full px-4 py-2 text-left text-sm hover:bg-gray-50 disabled:text-gray-300"
              >
                Restart
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function getStatusColor(server: Server) {
  if (server.status === "stopped") {
    return { border: "border-red-500", dot: "text-red-500" };
  }
  if (server.status === "starting" || server.status === "stopping") {
    return { border: "border-yellow-500", dot: "text-yellow-500 animate-pulse" };
  }
  if (server.status === "running") {
    // High resource usage = yellow
    if (server.cpuPercent > 80 || (server.memoryUsed / server.memoryLimit) > 0.8) {
      return { border: "border-yellow-500", dot: "text-yellow-500" };
    }
    return { border: "border-green-500", dot: "text-green-500" };
  }
  return { border: "border-red-500", dot: "text-red-500" };
}
```

---

### 3. Server Page (`/servers/[id]`)

Tabbed interface for server management.

```
┌─────────────────────────────────────────────────────────────────┐
│  ← Dashboard                                                     │
│                                                                  │
│  My Minecraft Server                                    🟢 Online│
│  mc-a1b2c3.play.example.com:25565                        [Copy] │
├─────────────────────────────────────────────────────────────────┤
│  [Overview]  [Console]  [Settings]  [Files]                      │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  RESOURCES                                                       │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ CPU        ████████░░░░░░░░░░░░░░░░░░░░░░░░░░░░  45%       ││
│  │ Memory     ██████████████░░░░░░░░░░░░░░░░░░░░░░  2.1/4 GB  ││
│  │ Storage    ████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░  1.2/25 GB ││
│  └─────────────────────────────────────────────────────────────┘│
│                                                                  │
│  PLAYERS ONLINE (3)                                              │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ Steve • Alex • Notch                                        ││
│  └─────────────────────────────────────────────────────────────┘│
│                                                                  │
│  DETAILS                                                         │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ Game          │ Minecraft: Java Edition                     ││
│  │ Plan          │ Standard (5-15 players) • $6/mo             ││
│  │ Version       │ 1.20.4 (Paper)                              ││
│  │ Created       │ Jan 15, 2025                                ││
│  └─────────────────────────────────────────────────────────────┘│
│                                                                  │
│  ACTIONS                                                         │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ [Stop Server]  [Restart Server]                             ││
│  └─────────────────────────────────────────────────────────────┘│
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

### 4. Server Settings (`/servers/[id]?tab=settings`)

```
┌─────────────────────────────────────────────────────────────────┐
│  ← Dashboard                                                     │
│                                                                  │
│  My Minecraft Server                                    🟢 Online│
│  mc-a1b2c3.play.example.com:25565                        [Copy] │
├─────────────────────────────────────────────────────────────────┤
│  [Overview]  [Console]  [Settings]  [Files]                      │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  BASIC                                                           │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ Display Name      │ My Minecraft Server              [Edit] ││
│  │ Server Version    │ 1.20.4                               ▼  ││
│  │ Server Type       │ Paper                                ▼  ││
│  └─────────────────────────────────────────────────────────────┘│
│                                                                  │
│  GAMEPLAY                                                        │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ Gamemode          │ Survival                             ▼  ││
│  │ Difficulty        │ Normal                               ▼  ││
│  │ Max Players       │ 10                                [Edit] ││
│  │ Allow PVP         │ [✓]                                     ││
│  └─────────────────────────────────────────────────────────────┘│
│                                                                  │
│  ▶ ADVANCED                                                      │
│                                                                  │
│  DANGER ZONE                                                     │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ [Upgrade Plan]  [Reinstall Server]  [Delete Server]         ││
│  └─────────────────────────────────────────────────────────────┘│
│                                                                  │
│                                       [Save Changes & Restart]   │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

#### Settings Schema (from API)

```typescript
// types/settings.ts
interface SettingField {
  key: string;
  label: string;
  type: "text" | "number" | "boolean" | "select" | "password";
  options?: string[];  // for select
  min?: number;        // for number
  max?: number;        // for number
  default: string | number | boolean;
}

interface SettingsSchema {
  basic: SettingField[];
  gameplay: SettingField[];
  advanced: SettingField[];
}
```

#### Component

```tsx
// app/servers/[id]/settings/page.tsx
"use client";

import { useState } from "react";
import { useQuery, useMutation } from "@tanstack/react-query";
import { api } from "@/lib/api";

export default function ServerSettingsPage({ params }: { params: { id: string } }) {
  const { data: server } = useQuery({
    queryKey: ["server", params.id],
    queryFn: () => api.getServer(params.id),
  });

  const { data: schema } = useQuery({
    queryKey: ["settings-schema", server?.game],
    queryFn: () => api.getSettingsSchema(server!.game),
    enabled: !!server,
  });

  const [settings, setSettings] = useState<Record<string, any>>(server?.settings ?? {});
  const [advancedOpen, setAdvancedOpen] = useState(false);

  const updateSettings = useMutation({
    mutationFn: (data: Record<string, any>) => api.updateServerSettings(params.id, data),
  });

  const handleSave = () => {
    updateSettings.mutate(settings);
  };

  return (
    <div className="space-y-6">
      {/* Basic Settings */}
      <SettingsSection
        title="Basic"
        fields={schema?.basic ?? []}
        values={settings}
        onChange={setSettings}
      />

      {/* Gameplay Settings */}
      <SettingsSection
        title="Gameplay"
        fields={schema?.gameplay ?? []}
        values={settings}
        onChange={setSettings}
      />

      {/* Advanced Settings (collapsible) */}
      <div>
        <button
          onClick={() => setAdvancedOpen(!advancedOpen)}
          className="text-sm font-medium text-gray-500"
        >
          {advancedOpen ? "▼" : "▶"} Advanced
        </button>
        {advancedOpen && (
          <SettingsSection
            title=""
            fields={schema?.advanced ?? []}
            values={settings}
            onChange={setSettings}
          />
        )}
      </div>

      {/* Danger Zone */}
      <div className="border border-red-200 rounded-lg p-4">
        <h3 className="text-sm font-medium text-red-600 mb-3">Danger Zone</h3>
        <div className="flex gap-3">
          <button className="px-4 py-2 border rounded-lg text-sm">
            Upgrade Plan
          </button>
          <button className="px-4 py-2 border rounded-lg text-sm">
            Reinstall Server
          </button>
          <button className="px-4 py-2 border border-red-300 text-red-600 rounded-lg text-sm">
            Delete Server
          </button>
        </div>
      </div>

      {/* Save Button */}
      <div className="flex justify-end">
        <button
          onClick={handleSave}
          disabled={updateSettings.isLoading}
          className="px-6 py-2 bg-blue-600 text-white rounded-lg"
        >
          {updateSettings.isLoading ? "Saving..." : "Save Changes & Restart"}
        </button>
      </div>
    </div>
  );
}
```

---

### 5. Server Console (`/servers/[id]?tab=console`)

```
┌─────────────────────────────────────────────────────────────────┐
│  ← Dashboard                                                     │
│                                                                  │
│  My Minecraft Server                                    🟢 Online│
│  mc-a1b2c3.play.example.com:25565                        [Copy] │
├─────────────────────────────────────────────────────────────────┤
│  [Overview]  [Console]  [Settings]  [Files]                      │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ [15:32:01] [Server thread/INFO]: Starting Minecraft server  ││
│  │ [15:32:02] [Server thread/INFO]: Loading properties         ││
│  │ [15:32:02] [Server thread/INFO]: Default game type: SURVIVAL││
│  │ [15:32:03] [Server thread/INFO]: Preparing level "world"    ││
│  │ [15:32:05] [Server thread/INFO]: Done (4.123s)!             ││
│  │ [15:32:45] [Server thread/INFO]: Steve joined the game      ││
│  │ [15:33:12] [Server thread/INFO]: Alex joined the game       ││
│  │ [15:34:01] [Server thread/INFO]: <Steve> Hello everyone!    ││
│  │                                                             ││
│  │                                                             ││
│  │                                                             ││
│  │                                                             ││
│  └─────────────────────────────────────────────────────────────┘│
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ Enter command...                                      [Send]││
│  └─────────────────────────────────────────────────────────────┘│
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

#### Component

```tsx
// components/console.tsx
"use client";

import { useEffect, useRef, useState } from "react";

interface ConsoleProps {
  serverId: string;
}

export function Console({ serverId }: ConsoleProps) {
  const [logs, setLogs] = useState<string[]>([]);
  const [command, setCommand] = useState("");
  const logsEndRef = useRef<HTMLDivElement>(null);
  const wsRef = useRef<WebSocket | null>(null);

  useEffect(() => {
    // Connect to WebSocket for live logs
    const ws = new WebSocket(`wss://api.example.com/servers/${serverId}/console`);
    wsRef.current = ws;

    ws.onmessage = (event) => {
      setLogs((prev) => [...prev.slice(-500), event.data]); // Keep last 500 lines
    };

    return () => ws.close();
  }, [serverId]);

  useEffect(() => {
    // Auto-scroll to bottom
    logsEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [logs]);

  const sendCommand = () => {
    if (command.trim() && wsRef.current) {
      wsRef.current.send(JSON.stringify({ type: "command", data: command }));
      setCommand("");
    }
  };

  return (
    <div className="flex flex-col h-[500px]">
      {/* Log output */}
      <div className="flex-1 bg-gray-900 text-gray-100 font-mono text-sm p-4 rounded-t-lg overflow-y-auto">
        {logs.map((log, i) => (
          <div key={i} className="whitespace-pre-wrap">
            {log}
          </div>
        ))}
        <div ref={logsEndRef} />
      </div>

      {/* Command input */}
      <div className="flex border-t border-gray-700">
        <input
          type="text"
          value={command}
          onChange={(e) => setCommand(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && sendCommand()}
          placeholder="Enter command..."
          className="flex-1 bg-gray-800 text-white px-4 py-3 font-mono text-sm outline-none rounded-bl-lg"
        />
        <button
          onClick={sendCommand}
          className="px-6 py-3 bg-blue-600 text-white text-sm rounded-br-lg"
        >
          Send
        </button>
      </div>
    </div>
  );
}
```

---

## API Client

```typescript
// lib/api.ts
const BASE_URL = process.env.NEXT_PUBLIC_API_URL;

async function fetchAPI<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE_URL}${path}`, {
    ...options,
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...options?.headers,
    },
  });

  if (!res.ok) {
    throw new Error(await res.text());
  }

  return res.json();
}

export const api = {
  // Auth
  login: (email: string, password: string) =>
    fetchAPI("/auth/login", {
      method: "POST",
      body: JSON.stringify({ email, password }),
    }),

  register: (email: string, password: string) =>
    fetchAPI("/auth/register", {
      method: "POST",
      body: JSON.stringify({ email, password }),
    }),

  // Games
  getGames: () => fetchAPI<Game[]>("/api/games"),
  getPlans: (gameId: string) => fetchAPI<Plan[]>(`/api/games/${gameId}/plans`),
  getSettingsSchema: (gameId: string) =>
    fetchAPI<SettingsSchema>(`/api/games/${gameId}/settings-schema`),

  // Servers
  getServers: () => fetchAPI<Server[]>("/api/servers"),
  getServer: (id: string) => fetchAPI<Server>(`/api/servers/${id}`),
  createServer: (data: CreateServerRequest) =>
    fetchAPI<{ checkoutUrl: string }>("/api/servers", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  deleteServer: (id: string) =>
    fetchAPI(`/api/servers/${id}`, { method: "DELETE" }),

  // Server actions
  startServer: (id: string) =>
    fetchAPI(`/api/servers/${id}/start`, { method: "POST" }),
  stopServer: (id: string) =>
    fetchAPI(`/api/servers/${id}/stop`, { method: "POST" }),
  restartServer: (id: string) =>
    fetchAPI(`/api/servers/${id}/restart`, { method: "POST" }),

  // Server settings
  updateServerSettings: (id: string, settings: Record<string, any>) =>
    fetchAPI(`/api/servers/${id}/settings`, {
      method: "PATCH",
      body: JSON.stringify(settings),
    }),
};
```

---

## Types

```typescript
// types/index.ts
export interface Game {
  id: string;
  name: string;
  icon: string;
}

export interface Plan {
  id: string;
  name: string;
  players: string;
  memory: string;
  price: number;
}

export interface Server {
  id: string;
  name: string;
  game: string;
  plan: string;
  status: "pending" | "running" | "stopped" | "starting" | "stopping" | "error";
  dnsRecord: string;
  port: number;
  players?: number;
  maxPlayers?: number;
  cpuPercent?: number;
  memoryUsed?: number;
  memoryLimit?: number;
  storageUsed?: number;
  storageLimit?: number;
  settings: Record<string, any>;
  createdAt: string;
}

export interface CreateServerRequest {
  game: string;
  plan: string;
  displayName: string;
}
```

---

## Directory Structure

```
/frontend
├── app/
│   ├── layout.tsx
│   ├── page.tsx                    # Landing
│   ├── login/page.tsx
│   ├── register/page.tsx
│   ├── dashboard/page.tsx
│   ├── servers/
│   │   ├── new/page.tsx
│   │   └── [id]/
│   │       ├── page.tsx            # Overview (default)
│   │       ├── console/page.tsx
│   │       ├── settings/page.tsx
│   │       └── files/page.tsx
│   └── account/page.tsx
├── components/
│   ├── ui/                         # shadcn components
│   ├── server-card.tsx
│   ├── console.tsx
│   ├── settings-section.tsx
│   ├── copy-button.tsx
│   └── navbar.tsx
├── lib/
│   ├── api.ts
│   └── utils.ts
├── types/
│   └── index.ts
└── package.json
```

---

## Key UX Principles

1. **Zero config on create** — defaults work, customize later
2. **Status always visible** — color-coded borders and dots
3. **One-click copy** — server address everywhere
4. **Click server name → details** — not buried in dropdowns
5. **Actions in dropdown** — keeps dashboard clean
6. **Settings grouped** — basic visible, advanced collapsed
7. **Confirm destructive actions** — delete, reinstall require confirmation
8. **Mobile-friendly** — dashboard works on phone