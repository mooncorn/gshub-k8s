import { useQuery } from "@tanstack/react-query"
import { serversApi } from "@/api/servers"

export function useServers() {
  return useQuery({
    queryKey: ["servers"],
    queryFn: async () => {
      const res = await serversApi.list()
      return res.data.servers
    },
  })
}
