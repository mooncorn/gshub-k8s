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
