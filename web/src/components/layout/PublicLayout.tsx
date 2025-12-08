import { Link, Outlet, useLocation } from "react-router-dom"
import { Gamepad2 } from "lucide-react"
import { Button } from "@/components/ui/button"

export function PublicLayout() {
  const location = useLocation()

  return (
    <div className="min-h-screen bg-background">
      <header className="border-b border-border/30 bg-background/50 backdrop-blur-sm">
        <div className="container mx-auto flex h-14 items-center justify-between px-4">
          <Link to="/welcome" className="flex items-center gap-2 text-lg font-semibold text-foreground">
            <Gamepad2 className="h-6 w-6" />
            GSHUB
          </Link>
          <div className="flex items-center gap-3">
            <Link to="/login" state={{ from: location }}>
              <Button variant="ghost" size="sm">
                Sign in
              </Button>
            </Link>
            <Link to="/register" state={{ from: location }}>
              <Button size="sm">Sign up</Button>
            </Link>
          </div>
        </div>
      </header>
      <main className="container mx-auto px-4 py-6">
        <Outlet />
      </main>
    </div>
  )
}
