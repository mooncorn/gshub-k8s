import { NavLink, Link } from "react-router-dom"
import { LayoutDashboard, Settings, FolderOpen, ArrowLeft } from "lucide-react"
import { useServerDetail } from "@/contexts/ServerDetailContext"
import { ServerStatusDropdown } from "@/components/servers/ServerStatusDropdown"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { GAMES, PLANS } from "@/lib/constants"

const navItems = [
  { path: "", label: "Dashboard", icon: LayoutDashboard },
  { path: "configuration", label: "Configuration", icon: Settings },
  { path: "files", label: "Files", icon: FolderOpen },
]

export function ServerSidebar() {
  const { server, isLoading, startServer, stopServer } = useServerDetail()

  const game = server ? GAMES[server.game] : null
  const plan = server ? PLANS[server.plan] : null

  return (
    <div className="space-y-6">
      {/* Back button */}
      <Link to="/">
        <Button variant="ghost" size="sm" className="gap-2">
          <ArrowLeft className="h-4 w-4" />
          Back to Servers
        </Button>
      </Link>

      {/* Server header */}
      <div className="space-y-2">
        {isLoading ? (
          <>
            <Skeleton className="h-6 w-32" />
            <Skeleton className="h-4 w-24" />
          </>
        ) : server ? (
          <>
            <h1 className="text-lg font-semibold truncate">{server.display_name}</h1>
            <p className="text-sm text-muted-foreground">
              {game?.name || server.game} &bull; {plan?.name || server.plan}
            </p>
            
          </>
        ) : null}
      </div>

      {/* Navigation */}
      <nav className="space-y-1">
        {navItems.map((item) => (
          <NavLink
            key={item.path}
            to={item.path}
            end={item.path === ""}
            className={({ isActive }) =>
              `flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors ${
                isActive
                  ? "bg-accent/70 text-accent-foreground"
                  : "text-muted-foreground hover:bg-accent/50 hover:text-accent-foreground"
              }`
            }
          >
            <item.icon className="h-4 w-4" />
            {item.label}
          </NavLink>
        ))}
      </nav>

      {/* Actions dropdown */}
      {server && (
        <ServerStatusDropdown
          status={server.status}
          onStart={() => startServer.mutate(server.id)}
          onStop={() => stopServer.mutate(server.id)}
          isStartPending={startServer.isPending}
          isStopPending={stopServer.isPending}
          className="w-full"
        />
      )}
    </div>
  )
}
