import { useAuth } from "@/hooks/useAuth"
import { Skeleton } from "@/components/ui/skeleton"
import { RootLayout } from "@/components/layout/RootLayout"
import { PublicLayout } from "@/components/layout/PublicLayout"

export function MixedRoute() {
  const { isAuthenticated, isLoading } = useAuth()

  if (isLoading) {
    return (
      <div className="flex h-screen items-center justify-center bg-background">
        <Skeleton className="h-8 w-32" />
      </div>
    )
  }

  // Render the appropriate layout - each has its own <Outlet /> for nested routes
  return isAuthenticated ? <RootLayout /> : <PublicLayout />
}
