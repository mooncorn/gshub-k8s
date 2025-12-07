import { Link } from "react-router-dom"
import { Card, CardContent } from "@/components/ui/card"
import { ServerStatusBadge } from "./ServerStatusBadge"
import type { Server } from "@/api/servers"
import { GAMES } from "@/lib/constants"

interface ServerCardProps {
  server: Server
}

export function ServerCard({ server }: ServerCardProps) {
  const game = GAMES[server.game]

  return (
    <Link to={`/servers/${server.id}`}>
      <Card className="transition-colors hover:bg-accent/50">
        <CardContent className="p-4">
          <div className="flex items-start justify-between gap-4">
            <div className="min-w-0 flex-1">
              <h3 className="truncate font-medium text-foreground">
                {server.display_name}
              </h3>
              <p className="text-sm text-muted-foreground">
                {game?.name || server.game} • {server.plan}
              </p>
            </div>
            <ServerStatusBadge status={server.status} />
          </div>
        </CardContent>
      </Card>
    </Link>
  )
}
