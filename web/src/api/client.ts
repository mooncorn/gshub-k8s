import axios, { type AxiosError, type InternalAxiosRequestConfig } from "axios"

const API_URL = import.meta.env.VITE_API_URL || "http://localhost:8080"

const client = axios.create({
  baseURL: API_URL,
  headers: {
    "Content-Type": "application/json",
  },
})

// Request interceptor - add auth token
client.interceptors.request.use((config: InternalAxiosRequestConfig) => {
  const token = localStorage.getItem("access_token")
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// Auth endpoints that should not trigger token refresh or redirect
const AUTH_ENDPOINTS = ["/auth/login", "/auth/register", "/auth/refresh", "/auth/forgot-password", "/auth/reset-password", "/auth/verify-email"]

// Response interceptor - handle 401 and token refresh
client.interceptors.response.use(
  (response) => response,
  async (error: AxiosError) => {
    const originalRequest = error.config

    // Skip refresh/redirect logic for auth endpoints
    const isAuthEndpoint = originalRequest?.url && AUTH_ENDPOINTS.some(endpoint => originalRequest.url?.includes(endpoint))
    if (isAuthEndpoint) {
      return Promise.reject(error)
    }

    if (error.response?.status === 401 && originalRequest) {
      const refreshToken = localStorage.getItem("refresh_token")
      if (refreshToken) {
        try {
          const response = await axios.post(`${API_URL}/auth/refresh`, {
            refresh_token: refreshToken,
          })
          const { access_token, refresh_token } = response.data

          localStorage.setItem("access_token", access_token)
          localStorage.setItem("refresh_token", refresh_token)

          originalRequest.headers.Authorization = `Bearer ${access_token}`
          return client(originalRequest)
        } catch {
          localStorage.removeItem("access_token")
          localStorage.removeItem("refresh_token")
          window.location.href = "/login"
        }
      } else {
        window.location.href = "/login"
      }
    }
    return Promise.reject(error)
  }
)

export default client
