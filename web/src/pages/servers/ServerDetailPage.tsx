import { useParams, Link } from "react-router-dom"
import { useServer } from "@/hooks/useServer"
import { useStartServer, useStopServer } from "@/hooks/useServerActions"
import { ServerStatusBadge } from "@/components/servers/ServerStatusBadge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { Separator } from "@/components/ui/separator"
import { GAMES, PLANS } from "@/lib/constants"

export function ServerDetailPage() {
  const { id } = useParams<{ id: string }>()
  const { data, isLoading, error } = useServer(id!)
  const startServer = useStartServer()
  const stopServer = useStopServer()

  if (isLoading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-8 w-48" />
        <Card>
          <CardContent className="p-6">
            <div className="space-y-4">
              <Skeleton className="h-4 w-full" />
              <Skeleton className="h-4 w-3/4" />
              <Skeleton className="h-4 w-1/2" />
            </div>
          </CardContent>
        </Card>
      </div>
    )
  }

  if (error || !data) {
    return (
      <div className="space-y-6">
        <Link to="/">
          <Button variant="ghost" size="sm">
            ← Back
          </Button>
        </Link>
        <Card>
          <CardContent className="p-6 text-center">
            <p className="text-sm text-muted-foreground">
              Server not found or failed to load.
            </p>
          </CardContent>
        </Card>
      </div>
    )
  }

  const { server, k8s_state } = data
  const game = GAMES[server.game]
  const plan = PLANS[server.plan]

  const canStart = server.status === "stopped" || server.status === "failed"
  const canStop =
    server.status === "running" ||
    server.status === "starting" ||
    server.status === "pending"

  const gamePort = server.ports?.find((p) => p.name === "game")
  const connectionAddress =
    server.node_ip && gamePort?.host_port
      ? `${server.node_ip}:${gamePort.host_port}`
      : null

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Link to="/">
          <Button variant="ghost" size="sm">
            ← Back
          </Button>
        </Link>
      </div>

      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-xl font-semibold">{server.display_name}</h1>
          <p className="text-sm text-muted-foreground">
            {game?.name || server.game} • {plan?.name || server.plan}
          </p>
        </div>
        <ServerStatusBadge status={server.status} />
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Status
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <span className="text-sm">Server Status</span>
                <ServerStatusBadge status={server.status} />
              </div>
              {k8s_state && (
                <div className="flex items-center justify-between">
                  <span className="text-sm">K8s State</span>
                  <span className="text-sm text-muted-foreground">
                    {k8s_state}
                  </span>
                </div>
              )}
              {server.status_message && (
                <div className="pt-2">
                  <span className="text-xs text-muted-foreground">
                    {server.status_message}
                  </span>
                </div>
              )}
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Connection
            </CardTitle>
          </CardHeader>
          <CardContent>
            {connectionAddress ? (
              <div className="space-y-2">
                <code className="block rounded bg-muted px-2 py-1 text-sm">
                  {connectionAddress}
                </code>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => navigator.clipboard.writeText(connectionAddress)}
                >
                  Copy address
                </Button>
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">
                {server.status === "running"
                  ? "Waiting for connection info..."
                  : "Start the server to get connection info"}
              </p>
            )}
          </CardContent>
        </Card>
      </div>

      <Separator />

      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-sm font-medium text-muted-foreground">
            Actions
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex gap-2">
            {canStart && (
              <Button
                size="sm"
                onClick={() => startServer.mutate(server.id)}
                disabled={startServer.isPending}
              >
                {startServer.isPending ? "Starting..." : "Start"}
              </Button>
            )}
            {canStop && (
              <Button
                variant="outline"
                size="sm"
                onClick={() => stopServer.mutate(server.id)}
                disabled={stopServer.isPending}
              >
                {stopServer.isPending ? "Stopping..." : "Stop"}
              </Button>
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
