import { useServerDetail } from "@/contexts/ServerDetailContext"
import { EnvEditor } from "@/components/servers/EnvEditor"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Skeleton } from "@/components/ui/skeleton"

export function ServerConfigurationTab() {
  const { server, gameConfig, updateEnv, envUpdateMessage, isLoading } = useServerDetail()

  if (isLoading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-64 w-full" />
      </div>
    )
  }

  if (!server) {
    return null
  }

  return (
    <div className="space-y-6">
      {envUpdateMessage && (
        <Alert
          variant={envUpdateMessage.type === "error" ? "destructive" : "default"}
        >
          <AlertDescription>{envUpdateMessage.text}</AlertDescription>
        </Alert>
      )}

      <EnvEditor
        game={server.game}
        gameConfig={gameConfig ?? undefined}
        onSave={async (envOverrides) => {
          await updateEnv.mutateAsync(envOverrides)
        }}
        disabled={updateEnv.isPending}
      />
    </div>
  )
}
