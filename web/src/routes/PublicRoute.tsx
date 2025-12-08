import { Navigate, Outlet } from "react-router-dom"
import { useAuth } from "@/hooks/useAuth"
import { Skeleton } from "@/components/ui/skeleton"

export function PublicRoute() {
  const { isAuthenticated, isLoading } = useAuth()

  if (isLoading) {
    return (
      <div className="flex h-screen items-center justify-center bg-background">
        <Skeleton className="h-8 w-32" />
      </div>
    )
  }

  // Redirect authenticated users to dashboard
  if (isAuthenticated) {
    return <Navigate to="/" replace />
  }

  return <Outlet />
}
