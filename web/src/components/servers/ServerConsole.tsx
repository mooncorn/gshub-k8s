import { useRef, useEffect, useState, useCallback } from "react"
import { ArrowDown } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import type { LogEvent } from "@/api/logs"

interface ServerConsoleProps {
  logs: LogEvent[]
  isConnected: boolean
  error: string | null
  onClear: () => void
  className?: string
}

export function ServerConsole({
  logs,
  isConnected,
  error,
  onClear,
  className,
}: ServerConsoleProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const [isUserScrolledUp, setIsUserScrolledUp] = useState(false)

  // Track scroll position to determine if user has scrolled up
  const handleScroll = useCallback(() => {
    const container = containerRef.current
    if (!container) return

    // Check if we're at the bottom (within 50px threshold)
    const isAtBottom =
      container.scrollHeight - container.scrollTop - container.clientHeight < 50

    setIsUserScrolledUp(!isAtBottom)
  }, [])

  // Scroll container to bottom (without affecting page scroll)
  const scrollContainerToBottom = useCallback((smooth = true) => {
    const container = containerRef.current
    if (!container) return

    if (smooth) {
      container.scrollTo({ top: container.scrollHeight, behavior: "smooth" })
    } else {
      container.scrollTop = container.scrollHeight
    }
  }, [])

  // Auto-scroll only when at bottom
  useEffect(() => {
    if (!isUserScrolledUp && logs.length > 0) {
      scrollContainerToBottom(false)
    }
  }, [logs, isUserScrolledUp, scrollContainerToBottom])

  // Scroll to bottom handler
  const scrollToBottom = useCallback(() => {
    scrollContainerToBottom(true)
    setIsUserScrolledUp(false)
  }, [scrollContainerToBottom])

  return (
    <div className={className}>
      <div className="relative">
          <div
            ref={containerRef}
            onScroll={handleScroll}
            className="h-96 overflow-auto rounded bg-zinc-800 p-3 font-mono text-xs"
          >
            {logs.length === 0 && !error && (
              <p className="text-zinc-500">
                {isConnected ? "Waiting for logs..." : "Connecting..."}
              </p>
            )}
            {error && <p className="text-red-400">Error: {error}</p>}
            {logs.map((log, i) => (
              <div key={i} className="whitespace-pre-wrap text-zinc-300">
                {log.line}
              </div>
            ))}
          </div>

          {/* Scroll to bottom button */}
          {isUserScrolledUp && logs.length > 0 && (
            <Button
              size="sm"
              variant="secondary"
              className="absolute bottom-4 right-4 gap-1"
              onClick={scrollToBottom}
            >
              <ArrowDown className="h-3 w-3" />
            </Button>
          )}
        </div>
    </div>
  )
}
