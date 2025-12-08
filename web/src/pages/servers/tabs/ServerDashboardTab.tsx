import { Link } from "react-router-dom"
import { Settings } from "lucide-react"
import { useServerDetail } from "@/contexts/ServerDetailContext"
import { ServerConsole } from "@/components/servers/ServerConsole"
import { CopyableText } from "@/components/ui/copyable-text"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { PLANS } from "@/lib/constants"

export function ServerDashboardTab() {
  const {
    server,
    k8sState,
    isLoading,
    startServer,
    stopServer,
    logs,
    isConnected,
    logError,
    clearLogs,
    showLogs,
  } = useServerDetail()

  if (isLoading) {
    return (
      <div className="space-y-6">
        <div className="grid gap-4 sm:grid-cols-2">
          <div className="rounded-lg bg-card/50 border border-border/50 p-4">
            <Skeleton className="h-4 w-32" />
          </div>
          <div className="rounded-lg bg-card/50 border border-border/50 p-4">
            <Skeleton className="h-4 w-48" />
          </div>
        </div>
        <Skeleton className="h-96 w-full rounded-lg" />
      </div>
    )
  }

  if (!server) {
    return null
  }

  const gamePort = server.ports?.find((p) => p.name === "game")
  const connectionAddress =
    gamePort?.node_ip && gamePort?.host_port
      ? `${gamePort.node_ip}:${gamePort.host_port}`
      : null

  const plan = PLANS[server.plan]

  return (
    <TooltipProvider>
      <div className="space-y-6">
        {/* Info Boxes */}
        <div className="grid gap-4 sm:grid-cols-2">
          {/* Plan Box */}
          <div className="flex items-center justify-between rounded-lg bg-card/50 border border-border/50 px-4 py-3">
            <div className="flex items-center gap-2">
              <span className="text-sm text-muted-foreground">Plan</span>
              <span className="text-sm font-medium">{plan?.name || server.plan}</span>
            </div>
            <Tooltip>
              <TooltipTrigger asChild>
                <Link
                  to="/settings/billing"
                  className="text-muted-foreground hover:text-foreground transition-colors"
                >
                  <Settings className="h-4 w-4" />
                </Link>
              </TooltipTrigger>
              <TooltipContent>
                <p>Manage plan</p>
              </TooltipContent>
            </Tooltip>
          </div>

          {/* IP Address Box */}
          <div className="flex items-center justify-between rounded-lg bg-card/50 border border-border/50 px-4 py-3">
            <span className="text-sm text-muted-foreground">IP Address</span>
            {connectionAddress ? (
              <CopyableText value={connectionAddress} className="bg-transparent p-0" />
            ) : (
              <span className="text-sm text-muted-foreground">
                {server.status === "running" ? "Loading..." : "—"}
              </span>
            )}
          </div>
        </div>

        {/* Console */}
        {showLogs && (
          <ServerConsole
            logs={logs}
            isConnected={isConnected}
            error={logError}
            onClear={clearLogs}
          />
        )}
      </div>
    </TooltipProvider>
  )
}
