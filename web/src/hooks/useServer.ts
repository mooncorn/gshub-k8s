import { useQuery } from "@tanstack/react-query"
import { serversApi, type ServerStatus } from "@/api/servers"

const TRANSITIONAL_STATES: ServerStatus[] = [
  "pending",
  "starting",
  "stopping",
  "deleting",
]

export function useServer(id: string) {
  return useQuery({
    queryKey: ["servers", id],
    queryFn: async () => {
      const res = await serversApi.get(id)
      return res.data
    },
    refetchInterval: (query) => {
      const data = query.state.data
      if (data && TRANSITIONAL_STATES.includes(data.server.status)) {
        return 5000
      }
      return false
    },
  })
}
