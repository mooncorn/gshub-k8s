import { useQuery } from "@tanstack/react-query"
import { serversApi } from "@/api/servers"

// Note: Real-time status updates are handled by useServerStatus hook via SSE.
// This hook no longer polls - it only fetches the initial state.
// The SSE stream updates the React Query cache directly.

export function useServer(id: string) {
  return useQuery({
    queryKey: ["servers", id],
    queryFn: async () => {
      const res = await serversApi.get(id)
      return res.data
    },
    // No refetchInterval - SSE handles real-time updates
  })
}
