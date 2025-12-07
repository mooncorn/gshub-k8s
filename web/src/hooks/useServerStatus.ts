import { useEffect, useRef, useState } from "react"
import { useQueryClient } from "@tanstack/react-query"
import { createStatusStream, type StatusEvent, type ConnectedEvent } from "@/api/status"
import type { ServerDetailResponse, ServerStatus } from "@/api/servers"

interface UseServerStatusOptions {
  enabled?: boolean
}

export function useServerStatus(options: UseServerStatusOptions = {}) {
  const { enabled = true } = options
  const [isConnected, setIsConnected] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const eventSourceRef = useRef<EventSource | null>(null)
  const queryClient = useQueryClient()

  useEffect(() => {
    if (!enabled) {
      return
    }

    // Reset state on new connection
    setError(null)
    setIsConnected(false)

    const es = createStatusStream({
      onStatus: (event: StatusEvent) => {
        // Update React Query cache for this specific server
        queryClient.setQueryData<ServerDetailResponse>(
          ["servers", event.server_id],
          (old) => {
            if (!old) return old
            return {
              ...old,
              server: {
                ...old.server,
                status: event.status as ServerStatus,
                status_message: event.status_message,
              },
            }
          }
        )

        // Also invalidate the servers list query to refresh totals
        queryClient.invalidateQueries({ queryKey: ["servers"], exact: true })
      },
      onConnected: (data: ConnectedEvent) => {
        setIsConnected(true)
        setError(null)

        // Update cache with initial state for all servers
        data.servers.forEach((server) => {
          queryClient.setQueryData<ServerDetailResponse>(
            ["servers", server.server_id],
            (old) => {
              if (!old) return old
              return {
                ...old,
                server: {
                  ...old.server,
                  status: server.status as ServerStatus,
                  status_message: server.status_message,
                },
              }
            }
          )
        })
      },
      onError: (err) => {
        setError(err.message)
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
  }, [enabled, queryClient])

  return { isConnected, error }
}
