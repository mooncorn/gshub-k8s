import client from "./client"

export type ServerStatus =
  | "pending"
  | "starting"
  | "running"
  | "stopping"
  | "stopped"
  | "expired"
  | "failed"
  | "deleting"
  | "deleted"

export type GameType = "minecraft" | "valheim"
export type ServerPlan = "small" | "medium" | "large"

export interface ServerPort {
  id: string
  name: string
  container_port: number
  host_port?: number
  node_ip?: string
  protocol: string
}

export interface Server {
  id: string
  user_id: string
  display_name: string
  game: GameType
  subdomain: string
  plan: ServerPlan
  status: ServerStatus
  status_message?: string
  ports?: ServerPort[]
  created_at: string
  updated_at: string
}

export interface ServerListResponse {
  servers: Server[]
  total: number
}

export interface ServerDetailResponse {
  server: Server
  k8s_state?: string
}

export interface CheckoutResponse {
  session_id: string
  checkout_url: string
  pending_request_id: string
}

export const serversApi = {
  list: () => client.get<ServerListResponse>("/servers"),

  get: (id: string) => client.get<ServerDetailResponse>(`/servers/${id}`),

  start: (id: string) =>
    client.post<{ status: string; message: string }>(`/servers/${id}/start`),

  stop: (id: string) =>
    client.post<{ status: string; message: string }>(`/servers/${id}/stop`),

  checkout: (
    displayName: string,
    subdomain: string,
    game: GameType,
    plan: ServerPlan
  ) =>
    client.post<CheckoutResponse>("/servers/checkout", {
      display_name: displayName,
      subdomain,
      game,
      plan,
    }),
}
