import {
  createContext,
  useState,
  useEffect,
  useCallback,
  useRef,
  type ReactNode,
} from "react"
import { authApi, type User } from "@/api/auth"

interface AuthContextType {
  user: User | null
  isLoading: boolean
  isAuthenticated: boolean
  login: (email: string, password: string) => Promise<void>
  logout: () => Promise<void>
  register: (email: string, password: string) => Promise<void>
  refreshUser: () => Promise<void>
}

// eslint-disable-next-line react-refresh/only-export-components
export const AuthContext = createContext<AuthContextType | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const initialized = useRef(false)

  const refreshUser = useCallback(async () => {
    try {
      const res = await authApi.getProfile()
      setUser(res.data)
    } catch {
      localStorage.removeItem("access_token")
      localStorage.removeItem("refresh_token")
      setUser(null)
    }
  }, [])

  useEffect(() => {
    if (initialized.current) return
    initialized.current = true

    const token = localStorage.getItem("access_token")
    if (token) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      refreshUser().finally(() => setIsLoading(false))
    } else {
      setIsLoading(false)
    }
  }, [refreshUser])

  const login = async (email: string, password: string) => {
    const res = await authApi.login(email, password)
    localStorage.setItem("access_token", res.data.access_token)
    localStorage.setItem("refresh_token", res.data.refresh_token)
    setUser(res.data.user)
  }

  const logout = async () => {
    const refreshToken = localStorage.getItem("refresh_token")
    if (refreshToken) {
      try {
        await authApi.logout(refreshToken)
      } catch {
        // Ignore logout errors
      }
    }
    localStorage.removeItem("access_token")
    localStorage.removeItem("refresh_token")
    setUser(null)
  }

  const register = async (email: string, password: string) => {
    await authApi.register(email, password)
    // Auto-login after successful registration
    await login(email, password)
  }

  return (
    <AuthContext.Provider
      value={{
        user,
        isLoading,
        isAuthenticated: !!user,
        login,
        logout,
        register,
        refreshUser,
      }}
    >
      {children}
    </AuthContext.Provider>
  )
}
