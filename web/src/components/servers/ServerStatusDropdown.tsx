import { ChevronDown, Play, Square } from "lucide-react"
import { statusConfig } from "@/components/servers/ServerStatusBadge"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { cn } from "@/lib/utils"
import type { ServerStatus } from "@/api/servers"

interface ServerStatusDropdownProps {
  status: ServerStatus
  onStart?: () => void
  onStop?: () => void
  isStartPending?: boolean
  isStopPending?: boolean
  className?: string
}

export function ServerStatusDropdown({
  status,
  onStart,
  onStop,
  isStartPending = false,
  isStopPending = false,
  className,
}: ServerStatusDropdownProps) {
  const statusStyle = statusConfig[status]
  const canStart = status === "stopped"
  const canStop = status === "running"
  const isActionPending = isStartPending || isStopPending

  const handleStart = (e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    if (canStart && onStart) {
      onStart()
    }
  }

  const handleStop = (e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    if (canStop && onStop) {
      onStop()
    }
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          onClick={(e) => e.preventDefault()}
          className={cn(
            "flex items-center justify-between gap-2 rounded-md border px-3 py-2 cursor-pointer text-sm font-medium transition-colors",
            statusStyle.className,
            className
          )}
        >
          <span>{statusStyle.label}</span>
          <ChevronDown className="h-4 w-4" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-[--radix-dropdown-menu-trigger-width]">
        <DropdownMenuItem
          onClick={handleStart}
          disabled={!canStart || isActionPending}
          className={cn(!canStart && "opacity-50 cursor-not-allowed")}
        >
          <Play className="h-4 w-4" />
          Start Server
        </DropdownMenuItem>
        <DropdownMenuItem
          onClick={handleStop}
          disabled={!canStop || isActionPending}
          className={cn(!canStop && "opacity-50 cursor-not-allowed")}
        >
          <Square className="h-4 w-4" />
          Stop Server
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
