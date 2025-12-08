import { Badge } from "@/components/ui/badge"
import type { ServerStatus } from "@/api/servers"
import { cn } from "@/lib/utils"

export const statusConfig: Record<ServerStatus, { label: string; className: string }> =
  {
    pending: {
      label: "Pending",
      className: "bg-yellow-500/20 text-yellow-400 border-yellow-500/50",
    },
    starting: {
      label: "Starting",
      className: "bg-blue-500/20 text-blue-400 border-blue-500/50",
    },
    running: {
      label: "Running",
      className: "bg-green-500/20 text-green-400 border-green-500/50",
    },
    stopping: {
      label: "Stopping",
      className: "bg-orange-500/20 text-orange-400 border-orange-500/50",
    },
    stopped: {
      label: "Stopped",
      className: "bg-gray-500/20 text-gray-400 border-gray-500/50",
    },
    expired: {
      label: "Expired",
      className: "bg-red-500/20 text-red-400 border-red-500/50",
    },
    failed: {
      label: "Failed",
      className: "bg-red-500/20 text-red-400 border-red-500/50",
    },
    deleting: {
      label: "Deleting",
      className: "bg-red-500/20 text-red-400 border-red-500/50",
    },
    deleted: {
      label: "Deleted",
      className: "bg-gray-500/20 text-gray-400 border-gray-500/50",
    },
  }

interface ServerStatusBadgeProps {
  status: ServerStatus
}

export function ServerStatusBadge({ status }: ServerStatusBadgeProps) {
  const config = statusConfig[status] || statusConfig.pending

  return (
    <Badge variant="outline" className={cn("font-medium", config.className)}>
      {config.label}
    </Badge>
  )
}
