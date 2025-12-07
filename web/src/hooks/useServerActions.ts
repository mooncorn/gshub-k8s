import { useMutation, useQueryClient } from "@tanstack/react-query"
import { serversApi } from "@/api/servers"

export function useStartServer() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (id: string) => serversApi.start(id),
    onSuccess: (_, id) => {
      queryClient.invalidateQueries({ queryKey: ["servers"] })
      queryClient.invalidateQueries({ queryKey: ["servers", id] })
    },
  })
}

export function useStopServer() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (id: string) => serversApi.stop(id),
    onSuccess: (_, id) => {
      queryClient.invalidateQueries({ queryKey: ["servers"] })
      queryClient.invalidateQueries({ queryKey: ["servers", id] })
    },
  })
}
