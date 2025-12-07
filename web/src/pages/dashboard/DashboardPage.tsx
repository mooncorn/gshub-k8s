import { Link } from "react-router-dom"
import { useServers } from "@/hooks/useServers"
import { ServerCard } from "@/components/servers/ServerCard"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { Card, CardContent } from "@/components/ui/card"

export function DashboardPage() {
  const { data: servers, isLoading, error } = useServers()

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">Servers</h1>
        <Link to="/servers/new">
          <Button size="sm">New Server</Button>
        </Link>
      </div>

      {isLoading && (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {[1, 2, 3].map((i) => (
            <Card key={i}>
              <CardContent className="p-4">
                <div className="space-y-2">
                  <Skeleton className="h-5 w-32" />
                  <Skeleton className="h-4 w-24" />
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {error && (
        <Card>
          <CardContent className="p-6 text-center">
            <p className="text-sm text-muted-foreground">
              Failed to load servers. Please try again.
            </p>
          </CardContent>
        </Card>
      )}

      {!isLoading && !error && servers?.length === 0 && (
        <Card>
          <CardContent className="p-6 text-center">
            <p className="text-sm text-muted-foreground">
              No servers yet. Create your first server to get started.
            </p>
            <Link to="/servers/new">
              <Button className="mt-4" size="sm">
                Create Server
              </Button>
            </Link>
          </CardContent>
        </Card>
      )}

      {!isLoading && !error && servers && servers.length > 0 && (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {servers.map((server) => (
            <ServerCard key={server.id} server={server} />
          ))}
        </div>
      )}
    </div>
  )
}
