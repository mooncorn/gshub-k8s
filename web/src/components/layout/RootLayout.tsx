import { Outlet } from "react-router-dom"
import { Navbar } from "./Navbar"
import { useServerStatus } from "@/hooks/useServerStatus"

export function RootLayout() {
  // Connect to SSE for real-time server status updates
  // This runs for all authenticated users on protected routes
  useServerStatus()

  return (
    <div className="min-h-screen bg-background">
      <Navbar />
      <main className="container mx-auto px-4 py-6">
        <Outlet />
      </main>
    </div>
  )
}
