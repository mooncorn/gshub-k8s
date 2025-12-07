const API_URL = import.meta.env.VITE_API_URL || "http://localhost:8080"

export interface LogEvent {
  line: string
  timestamp: string
}

export interface ConnectedEvent {
  server_id: string
  status: string
}

export interface ErrorEvent {
  message: string
  details?: string
}

export interface LogStreamCallbacks {
  onLog: (log: LogEvent) => void
  onConnected: (data: ConnectedEvent) => void
  onError: (error: ErrorEvent) => void
  onEnd: () => void
  onHeartbeat?: () => void
}

export function createLogStream(
  serverId: string,
  callbacks: LogStreamCallbacks
): EventSource {
  const token = localStorage.getItem("access_token")
  const url = `${API_URL}/servers/${serverId}/logs?token=${encodeURIComponent(token || "")}`

  const eventSource = new EventSource(url)

  eventSource.addEventListener("log", (event) => {
    try {
      callbacks.onLog(JSON.parse(event.data))
    } catch (e) {
      console.error("Failed to parse log event:", e)
    }
  })

  eventSource.addEventListener("connected", (event) => {
    try {
      callbacks.onConnected(JSON.parse(event.data))
    } catch (e) {
      console.error("Failed to parse connected event:", e)
    }
  })

  eventSource.addEventListener("error", (event: Event) => {
    const messageEvent = event as MessageEvent
    if (messageEvent.data) {
      try {
        callbacks.onError(JSON.parse(messageEvent.data))
      } catch (e) {
        console.error("Failed to parse error event:", e)
      }
    }
  })

  eventSource.addEventListener("end", () => {
    callbacks.onEnd()
    eventSource.close()
  })

  eventSource.addEventListener("heartbeat", () => {
    callbacks.onHeartbeat?.()
  })

  // Handle connection errors (network issues, auth failures, etc.)
  eventSource.onerror = () => {
    // EventSource auto-reconnects by default on network errors
    // For auth failures (401), we should close the connection
    if (eventSource.readyState === EventSource.CLOSED) {
      callbacks.onError({ message: "Connection closed" })
    }
  }

  return eventSource
}
