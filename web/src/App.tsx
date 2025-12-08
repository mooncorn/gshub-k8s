import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom"
import { AuthProvider } from "@/contexts/AuthContext"
import { ProtectedRoute } from "@/routes/ProtectedRoute"
import { PublicRoute } from "@/routes/PublicRoute"
import { MixedRoute } from "@/routes/MixedRoute"
import { AuthLayout } from "@/components/layout/AuthLayout"
import { RootLayout } from "@/components/layout/RootLayout"

import { LandingPage } from "@/pages/LandingPage"
import { LoginPage } from "@/pages/auth/LoginPage"
import { RegisterPage } from "@/pages/auth/RegisterPage"
import { ForgotPasswordPage } from "@/pages/auth/ForgotPasswordPage"
import { ResetPasswordPage } from "@/pages/auth/ResetPasswordPage"
import { VerifyEmailPage } from "@/pages/auth/VerifyEmailPage"
import { DashboardPage } from "@/pages/dashboard/DashboardPage"
import { CreateServerPage } from "@/pages/servers/CreateServerPage"
import { ServerLayout } from "@/components/servers/ServerLayout"
import { ServerDashboardTab } from "@/pages/servers/tabs/ServerDashboardTab"
import { ServerConfigurationTab } from "@/pages/servers/tabs/ServerConfigurationTab"
import { ServerFilesTab } from "@/pages/servers/tabs/ServerFilesTab"

function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          {/* Landing page for non-authenticated users */}
          <Route element={<PublicRoute />}>
            <Route path="/welcome" element={<LandingPage />} />
          </Route>

          {/* Auth routes */}
          <Route element={<AuthLayout />}>
            <Route path="/login" element={<LoginPage />} />
            <Route path="/register" element={<RegisterPage />} />
            <Route path="/forgot-password" element={<ForgotPasswordPage />} />
            <Route path="/reset-password" element={<ResetPasswordPage />} />
            <Route path="/verify-email" element={<VerifyEmailPage />} />
          </Route>

          {/* Create server - accessible to both visitors and authenticated users */}
          <Route element={<MixedRoute />}>
            <Route path="/servers/new" element={<CreateServerPage />} />
          </Route>

          {/* Protected routes */}
          <Route element={<ProtectedRoute />}>
            <Route element={<RootLayout />}>
              <Route path="/" element={<DashboardPage />} />
              <Route path="/servers/:id" element={<ServerLayout />}>
                <Route index element={<ServerDashboardTab />} />
                <Route path="configuration" element={<ServerConfigurationTab />} />
                <Route path="files" element={<ServerFilesTab />} />
              </Route>
            </Route>
          </Route>

          {/* Catch all */}
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  )
}

export default App
