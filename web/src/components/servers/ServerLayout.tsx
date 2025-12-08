import { Outlet } from "react-router-dom"
import { ServerDetailProvider, useServerDetail } from "@/contexts/ServerDetailContext"
import { ServerSidebar } from "@/components/servers/ServerSidebar"
import { Card, CardContent } from "@/components/ui/card"
import { HERO_IMAGES } from "@/lib/constants"

function ServerLayoutContent() {
  const { server, error, isLoading } = useServerDetail()
  const heroImage = server?.game ? HERO_IMAGES[server.game] : null

  if (error && !isLoading) {
    return (
      <div className="relative">
        <div className="flex flex-col md:flex-row gap-6 relative z-10">
          <aside className="w-full md:w-56 shrink-0">
            <ServerSidebar />
          </aside>
          <main className="flex-1 min-w-0">
            <Card>
              <CardContent className="p-6 text-center">
                <p className="text-sm text-muted-foreground">
                  Server not found or failed to load.
                </p>
              </CardContent>
            </Card>
          </main>
        </div>
      </div>
    )
  }

  return (
    <div className="relative">
      {heroImage && (
        <div className="absolute -top-6 left-1/2 h-64 w-screen -translate-x-1/2 overflow-hidden pointer-events-none">
          <div
            className="absolute inset-0 bg-cover bg-top bg-no-repeat opacity-50"
            style={{ backgroundImage: `url(${heroImage})` }}
          />
          <div className="absolute inset-0 bg-gradient-to-b from-transparent via-transparent to-background" />
        </div>
      )}
      <div className="flex flex-col md:flex-row gap-6 relative z-10">
        <aside className="w-full md:w-56 shrink-0">
          <ServerSidebar />
        </aside>
        <main className="flex-1 min-w-0">
          <Outlet />
        </main>
      </div>
    </div>
  )
}

export function ServerLayout() {
  return (
    <ServerDetailProvider>
      <ServerLayoutContent />
    </ServerDetailProvider>
  )
}
