import { runSync, listSyncRuns } from "@/generated_grpc/myncer/sync-SyncService_connectquery"
import { useMutation, createConnectQueryKey } from "@connectrpc/connect-query"
import { useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner";

export const useRunSync = () => {
  const queryClient = useQueryClient();

  const { mutateAsync, isPending: isRunningSync } = useMutation(runSync, {
    onSuccess: () => {
      toast.success("Sync started!");

      queryClient.refetchQueries({
        queryKey: createConnectQueryKey({
          schema: listSyncRuns,
          cardinality: undefined,
        })
      });
    },
    onError: (error) => {
      toast.error(`Sync failed: ${error.message}`);
    },
  })
  return {
    runSync: mutateAsync,
    isRunningSync,
  }
}
