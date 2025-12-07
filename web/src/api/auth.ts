import client from "./client"

export interface User {
  id: string
  email: string
  email_verified: boolean
  created_at: string
}

export interface AuthResponse {
  access_token: string
  refresh_token: string
  user: User
}

export const authApi = {
  register: (email: string, password: string) =>
    client.post<{ message: string; user: User }>("/auth/register", {
      email,
      password,
    }),

  login: (email: string, password: string) =>
    client.post<AuthResponse>("/auth/login", { email, password }),

  logout: (refreshToken: string) =>
    client.post("/auth/logout", { refresh_token: refreshToken }),

  refresh: (refreshToken: string) =>
    client.post<{ access_token: string; refresh_token: string }>(
      "/auth/refresh",
      { refresh_token: refreshToken }
    ),

  verifyEmail: (token: string) =>
    client.post("/auth/verify-email", { token }),

  resendVerification: (email: string) =>
    client.post("/auth/resend-verification", { email }),

  forgotPassword: (email: string) =>
    client.post("/auth/forgot-password", { email }),

  resetPassword: (token: string, password: string) =>
    client.post("/auth/reset-password", { token, password }),

  getProfile: () => client.get<User>("/me"),
}
