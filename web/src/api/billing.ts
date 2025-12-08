import client from "./client"
import type { GameType, ServerPlan, ServerStatus } from "./servers"

export interface SubscriptionInfo {
  subscription_id: string
  status: string
  current_period_end: string
  cancel_at_period_end: boolean
  canceled_at?: string
  cancels_at?: string
}

export interface ServerSubscription {
  server_id: string
  display_name: string
  game: GameType
  plan: ServerPlan
  status: ServerStatus
  subscription?: SubscriptionInfo
  expired_at?: string
  delete_after?: string
}

export interface BillingResponse {
  subscriptions: ServerSubscription[]
}

export interface CancelResponse {
  status: string
  message: string
  cancel_at_period_end: boolean
  current_period_end: string
}

export interface ResubscribeResponse {
  session_id: string
  checkout_url: string
}

export interface ResumeResponse {
  status: string
  message: string
}

export const billingApi = {
  getBilling: () => client.get<BillingResponse>("/billing"),

  cancelSubscription: (serverId: string) =>
    client.post<CancelResponse>(`/billing/servers/${serverId}/cancel`),

  resumeSubscription: (serverId: string) =>
    client.post<ResumeResponse>(`/billing/servers/${serverId}/resume`),

  resubscribe: (serverId: string) =>
    client.post<ResubscribeResponse>(`/billing/servers/${serverId}/resubscribe`),
}
