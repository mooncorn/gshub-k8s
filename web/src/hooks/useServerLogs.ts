import { useState, useEffect, useRef, useCallback } from "react"
import { createLogStream, type LogEvent } from "@/api/logs"

const MAX_LOG_LINES = 1000

export function useServerLogs(serverId: string | undefined, enabled: boolean = true) {
  const [logs, setLogs] = useState<LogEvent[]>([])
  const [isConnected, setIsConnected] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const eventSourceRef = useRef<EventSource | null>(null)

  const clearLogs = useCallback(() => {
    setLogs([])
  }, [])

  useEffect(() => {
    if (!serverId || !enabled) {
      return
    }

    // Reset state on new connection
    setError(null)
    setIsConnected(false)

    const es = createLogStream(serverId, {
      onLog: (log) => {
        setLogs((prev) => {
          const next = [...prev, log]
          // Keep only the last MAX_LOG_LINES to prevent memory issues
          return next.slice(-MAX_LOG_LINES)
        })
      },
      onConnected: () => {
        setIsConnected(true)
        setError(null)
      },
      onError: (err) => {
        setError(err.message)
        setIsConnected(false)
      },
      onEnd: () => {
        setIsConnected(false)
      },
      onHeartbeat: () => {
        // Keep-alive, no action needed
      },
    })

    eventSourceRef.current = es

    return () => {
      es.close()
      eventSourceRef.current = null
    }
  }, [serverId, enabled])

  return { logs, isConnected, error, clearLogs }
}
