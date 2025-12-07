import type { GameType, ServerPlan } from "@/api/servers"

export const GAMES: Record<
  GameType,
  {
    id: GameType
    name: string
    description: string
    plans: ServerPlan[]
  }
> = {
  minecraft: {
    id: "minecraft",
    name: "Minecraft: Java Edition",
    description: "Build, explore, and survive in a blocky world",
    plans: ["small", "medium", "large"],
  },
  valheim: {
    id: "valheim",
    name: "Valheim",
    description: "Viking survival and exploration",
    plans: ["small", "medium"],
  },
}

export const PLANS: Record<
  ServerPlan,
  {
    id: ServerPlan
    name: string
    description: string
  }
> = {
  small: {
    id: "small",
    name: "Small",
    description: "Perfect for small groups (2-5 players)",
  },
  medium: {
    id: "medium",
    name: "Medium",
    description: "Great for communities (5-15 players)",
  },
  large: {
    id: "large",
    name: "Large",
    description: "For large servers (15+ players)",
  },
}

export interface EnvVarDefinition {
  key: string
  label: string
  description: string
  default?: string
  type?: "text" | "boolean" | "select"
  options?: { value: string; label: string }[]
}

export const GAME_ENV_VARS: Record<GameType, EnvVarDefinition[]> = {
  minecraft: [
    {
      key: "EULA",
      label: "Accept EULA",
      description: "Required to run the server",
      default: "TRUE",
      type: "boolean",
    },
    {
      key: "TYPE",
      label: "Server Type",
      description: "Server software type",
      default: "PAPER",
      type: "select",
      options: [
        { value: "PAPER", label: "Paper (Recommended)" },
        { value: "SPIGOT", label: "Spigot" },
        { value: "VANILLA", label: "Vanilla" },
        { value: "FABRIC", label: "Fabric" },
        { value: "FORGE", label: "Forge" },
      ],
    },
    {
      key: "VERSION",
      label: "Game Version",
      description: "Minecraft version (e.g., 1.20.4, LATEST)",
      type: "text",
    },
    {
      key: "MOTD",
      label: "Server Message",
      description: "Message shown in server list",
      type: "text",
    },
    {
      key: "MAX_PLAYERS",
      label: "Max Players",
      description: "Maximum player count",
      type: "text",
    },
    {
      key: "DIFFICULTY",
      label: "Difficulty",
      description: "Game difficulty level",
      type: "select",
      options: [
        { value: "peaceful", label: "Peaceful" },
        { value: "easy", label: "Easy" },
        { value: "normal", label: "Normal" },
        { value: "hard", label: "Hard" },
      ],
    },
    {
      key: "MODE",
      label: "Game Mode",
      description: "Default game mode for players",
      type: "select",
      options: [
        { value: "survival", label: "Survival" },
        { value: "creative", label: "Creative" },
        { value: "adventure", label: "Adventure" },
        { value: "spectator", label: "Spectator" },
      ],
    },
  ],
  valheim: [
    {
      key: "SERVER_PUBLIC",
      label: "Public Server",
      description: "List in server browser",
      default: "false",
      type: "boolean",
    },
    {
      key: "SERVER_NAME",
      label: "Server Name",
      description: "Name shown in server browser",
      type: "text",
    },
    {
      key: "WORLD_NAME",
      label: "World Name",
      description: "Name of the world save",
      type: "text",
    },
    {
      key: "SERVER_PASS",
      label: "Server Password",
      description: "Password to join (min 5 characters)",
      type: "text",
    },
  ],
}
