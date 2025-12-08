import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { billingApi } from "@/api/billing"

export function useBilling() {
  return useQuery({
    queryKey: ["billing"],
    queryFn: async () => {
      const res = await billingApi.getBilling()
      return res.data.subscriptions
    },
  })
}

export function useCancelSubscription() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (serverId: string) => billingApi.cancelSubscription(serverId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["billing"] })
      queryClient.invalidateQueries({ queryKey: ["servers"] })
    },
  })
}

export function useResumeSubscription() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (serverId: string) => billingApi.resumeSubscription(serverId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["billing"] })
      queryClient.invalidateQueries({ queryKey: ["servers"] })
    },
  })
}

export function useResubscribe() {
  return useMutation({
    mutationFn: (serverId: string) => billingApi.resubscribe(serverId),
    onSuccess: (response) => {
      // Redirect to Stripe checkout
      window.location.href = response.data.checkout_url
    },
  })
}
