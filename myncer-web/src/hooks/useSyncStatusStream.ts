import { useState, useEffect } from "react";
import { createClient } from "@connectrpc/connect";
import { useTransport } from "@connectrpc/connect-query";
import { SyncService } from "@/generated_grpc/myncer/sync_pb";
import type { SyncRun } from "@/generated_grpc/myncer/sync_pb";
import { ConnectError, Code } from "@connectrpc/connect";

interface UseSyncStatusStreamOptions {
  enabled?: boolean;
  onMessage?: (syncRun: SyncRun) => void;
  onError?: (error: Error) => void;
}

/**
 * A custom React hook to subscribe to real-time sync status updates
 * using a gRPC server-side stream.
 */
export const useSyncStatusStream = (
  syncId: string,
  options: UseSyncStatusStreamOptions = {}
) => {
  const { enabled = true, onMessage, onError } = options;

  const [latestRun, setLatestRun] = useState<SyncRun | undefined>(undefined);
  const [isConnected, setIsConnected] = useState(false);
  const [error, setError] = useState<Error | undefined>(undefined);

  const transport = useTransport();

  useEffect(() => {
    // Do not connect if the hook is disabled or syncId is missing.
    if (!enabled || !syncId) {
      // Ensure we clean up state if the hook is disabled.
      setIsConnected(false);
      setLatestRun(undefined);
      return;
    }

    const client = createClient(SyncService, transport);
    const controller = new AbortController();
    let isMounted = true;

    async function connectStream() {
      if (!isMounted) return;
      setIsConnected(false);
      setError(undefined);

      try {
        // This creates an async iterator over the stream.
        const stream = client.subscribeToSyncStatus(
          { syncId },
          { signal: controller.signal },
        );
        
        if (isMounted) {
          setIsConnected(true);
        }

        // The for-await-of loop will run for each message from the server.
        for await (const syncRun of stream) {
          if (!isMounted) break;
          setLatestRun(syncRun);
          onMessage?.(syncRun);
        }
      } catch (err) {
        if (!isMounted) return;
        
        // Don't report an error if the stream was intentionally aborted by the component unmounting.
        if (err instanceof ConnectError && err.code === Code.Canceled) {
          console.log(`Stream for sync ${syncId} was intentionally canceled.`);
          return;
        }

        const streamError = err instanceof Error ? err : new Error(String(err));
        setError(streamError);
        onError?.(streamError);
        setIsConnected(false);
      } finally {
        if (isMounted) {
          setIsConnected(false);
        }
      }
    }

    connectStream();

    // Cleanup function: This is critical to prevent memory leaks.
    // It runs when the component unmounts or dependencies change.
    return () => {
      isMounted = false;
      controller.abort();
    };
    // The dependency array ensures this effect re-runs if any of these values change.
  }, [syncId, transport, enabled, onMessage, onError]);

  return {
    latestRun,
    isConnected,
    error,
    isError: !!error,
  };
};
