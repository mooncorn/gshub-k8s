import { Link } from "react-router-dom"
import { ServerStatusDropdown } from "./ServerStatusDropdown"
import type { Server } from "@/api/servers"
import { useStartServer, useStopServer } from "@/hooks/useServerActions"
import { GAMES, HERO_IMAGES, GAME_ICONS } from "@/lib/constants"

interface ServerCardProps {
  server: Server
}

export function ServerCard({ server }: ServerCardProps) {
  const game = GAMES[server.game]
  const heroImage = HERO_IMAGES[server.game]
  const gameIcon = GAME_ICONS[server.game]
  const startServer = useStartServer()
  const stopServer = useStopServer()

  return (
    <Link to={`/servers/${server.id}`} className="block group">
      <div className="relative overflow-hidden rounded-lg border border-border/50 bg-card">
        {/* Hero background */}
        <div className="absolute inset-0 pointer-events-none">
          <div
            className="absolute inset-0 bg-cover bg-center bg-no-repeat opacity-70 transition-all duration-300 group-hover:opacity-80 group-hover:scale-105"
            style={{ backgroundImage: `url(${heroImage})` }}
          />
          <div className="absolute inset-0 bg-gradient-to-r from-background/80 via-background/60 to-transparent" />
        </div>

        {/* Content */}
        <div className="relative z-10 flex items-center justify-between gap-4 p-4">
          <div className="flex items-center gap-3 min-w-0 flex-1">
            <img
              src={gameIcon}
              alt={game?.name || server.game}
              className="h-10 w-10 shrink-0 rounded"
            />
            <div className="min-w-0">
              <h3 className="truncate font-medium text-foreground">
                {server.display_name}
              </h3>
              <p className="text-sm text-muted-foreground">
                {game?.name || server.game}
              </p>
            </div>
          </div>
          <ServerStatusDropdown
            status={server.status}
            onStart={() => startServer.mutate(server.id)}
            onStop={() => stopServer.mutate(server.id)}
            isStartPending={startServer.isPending}
            isStopPending={stopServer.isPending}
          />
        </div>
      </div>
    </Link>
  )
}
