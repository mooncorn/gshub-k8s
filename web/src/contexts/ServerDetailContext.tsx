import { createContext, useContext, useState, type ReactNode } from "react"
import { useParams } from "react-router-dom"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useServer } from "@/hooks/useServer"
import { useStartServer, useStopServer } from "@/hooks/useServerActions"
import { useServerLogs } from "@/hooks/useServerLogs"
import { serversApi, type Server, type GameConfigInfo } from "@/api/servers"
import type { LogEvent } from "@/api/logs"

interface EnvUpdateMessage {
  type: "success" | "error"
  text: string
}

interface ServerDetailContextValue {
  // Server data
  server: Server | null
  k8sState: string | null
  gameConfig: GameConfigInfo | null
  isLoading: boolean
  error: Error | null

  // Server actions
  startServer: ReturnType<typeof useStartServer>
  stopServer: ReturnType<typeof useStopServer>

  // Environment update
  updateEnv: ReturnType<typeof useMutation<unknown, Error, Record<string, string>>>
  envUpdateMessage: EnvUpdateMessage | null

  // Logs
  logs: LogEvent[]
  isConnected: boolean
  logError: string | null
  clearLogs: () => void
  showLogs: boolean
}

const ServerDetailContext = createContext<ServerDetailContextValue | null>(null)

export function ServerDetailProvider({ children }: { children: ReactNode }) {
  const { id } = useParams<{ id: string }>()
  const queryClient = useQueryClient()

  // Fetch server data
  const { data, isLoading, error } = useServer(id!)

  // Server actions
  const startServer = useStartServer()
  const stopServer = useStopServer()

  // Environment update state
  const [envUpdateMessage, setEnvUpdateMessage] = useState<EnvUpdateMessage | null>(null)

  const updateEnv = useMutation({
    mutationFn: (envOverrides: Record<string, string>) =>
      serversApi.updateEnv(id!, envOverrides),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["server", id] })
      setEnvUpdateMessage({
        type: "success",
        text: "Environment variables saved. Restart server for changes to take effect.",
      })
      setTimeout(() => setEnvUpdateMessage(null), 5000)
    },
    onError: () => {
      setEnvUpdateMessage({
        type: "error",
        text: "Failed to save environment variables.",
      })
      setTimeout(() => setEnvUpdateMessage(null), 5000)
    },
  })

  // Log streaming
  const server = data?.server ?? null
  const showLogs =
    server?.status === "running" ||
    server?.status === "starting" ||
    server?.status === "stopping"

  const { logs, isConnected, error: logError, clearLogs } = useServerLogs(id, showLogs)

  const value: ServerDetailContextValue = {
    server,
    k8sState: data?.k8s_state ?? null,
    gameConfig: data?.game_config ?? null,
    isLoading,
    error: error as Error | null,
    startServer,
    stopServer,
    updateEnv,
    envUpdateMessage,
    logs,
    isConnected,
    logError,
    clearLogs,
    showLogs,
  }

  return (
    <ServerDetailContext.Provider value={value}>
      {children}
    </ServerDetailContext.Provider>
  )
}

export function useServerDetail() {
  const context = useContext(ServerDetailContext)
  if (!context) {
    throw new Error("useServerDetail must be used within a ServerDetailProvider")
  }
  return context
}
