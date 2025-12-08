import { Outlet, Navigate, useLocation, Link } from "react-router-dom"
import { Gamepad2 } from "lucide-react"
import { useAuth } from "@/hooks/useAuth"
import { Skeleton } from "@/components/ui/skeleton"
import { Button } from "@/components/ui/button"

export function AuthLayout() {
  const { isAuthenticated, isLoading } = useAuth()
  const location = useLocation()

  // Get the intended destination from state, default to dashboard
  const from = location.state?.from?.pathname || "/"

  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background">
        <Skeleton className="h-8 w-32" />
      </div>
    )
  }

  if (isAuthenticated) {
    return <Navigate to={from} replace />
  }

  const isLoginPage = location.pathname === "/login"

  return (
    <div className="min-h-screen bg-background">
      <header className="border-b border-border/30 bg-background/50 backdrop-blur-sm">
        <div className="container mx-auto flex h-14 items-center justify-between px-4">
          <Link to="/welcome" className="flex items-center gap-2 text-lg font-semibold text-foreground">
            <Gamepad2 className="h-6 w-6" />
            GSHUB
          </Link>
          <div className="flex items-center gap-3">
            {isLoginPage ? (
              <Link to="/register" state={{ from: location.state?.from }}>
                <Button size="sm">Sign up</Button>
              </Link>
            ) : (
              <Link to="/login" state={{ from: location.state?.from }}>
                <Button variant="ghost" size="sm">Sign in</Button>
              </Link>
            )}
          </div>
        </div>
      </header>
      <div className="flex min-h-[calc(100vh-3.5rem)] items-center justify-center p-4">
        <div className="w-full max-w-sm">
          <Outlet />
        </div>
      </div>
    </div>
  )
}
