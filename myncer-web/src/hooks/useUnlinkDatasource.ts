import {
  listDatasources,
  unlinkDatasource,
} from "@/generated_grpc/myncer/datasource-DatasourceService_connectquery"
import { createConnectQueryKey, useMutation } from "@connectrpc/connect-query"
import { useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"

export const useUnlinkDatasource = () => {
  const queryClient = useQueryClient()
  return useMutation(unlinkDatasource, {
    onSuccess: () => {
      toast.success("Account unlinked successfully!")
      // Invalidate the datasources list to refetch and update the UI
      queryClient.invalidateQueries({
        queryKey: createConnectQueryKey({
          schema: listDatasources,
          cardinality: undefined,
        }),
      })
    },
    onError: (error) => {
      toast.error(`Failed to unlink account: ${error.message}`)
    },
  })
}
