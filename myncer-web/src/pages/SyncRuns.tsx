import { PageWrapper } from "@/components/PageWrapper";
import { SyncRunRender } from "@/components/SyncRunRender";
import { PageLoader } from "@/components/ui/page-loader";
import { useListSyncRuns } from "@/hooks/useListSyncRuns";
import { useSearchParams } from "react-router-dom";
import { useSync } from "@/hooks/useSync";
import { SyncRender } from "@/components/Sync";

export const SyncRuns = () => {
  const [searchParams] = useSearchParams();
  const syncId = searchParams.get("syncId");

  const { sync, loading: syncLoading } = useSync(syncId || undefined);
  const { syncRuns, loading: runsLoading } = useListSyncRuns();

  if (runsLoading || (syncId && syncLoading)) {
    return <PageLoader />;
  }

  const relevantRuns = syncId
    ? syncRuns.filter((run) => run.syncId === syncId)
    : syncRuns;

  return (
    <PageWrapper>
      <div className="max-w-5xl px-4 py-8 space-y-8">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold">
              {syncId ? "Sync Run History" : "All Sync Runs"}
            </h1>
            <p className="text-muted-foreground mt-1 text-sm">
              {syncId
                ? `Showing all runs for a specific sync.`
                : "All sync runs are listed here."}
            </p>
          </div>
        </div>

        {syncId && sync && (
          <div className="mb-8">
            <h2 className="text-2xl font-semibold mb-4 border-b pb-2">Sync Configuration</h2>
            <SyncRender sync={sync} />
          </div>
        )}

        <div className="space-y-4">
          {relevantRuns.length === 0 ? (
            <div className="rounded-xl border bg-card p-6 shadow-sm">
              <p className="text-muted-foreground">
                {syncId ? "This sync has not been run yet." : "Run a sync to see it here."}
              </p>
            </div>
          ) : (
            relevantRuns.map((syncRun) => (
              <SyncRunRender
                key={syncRun.runId}
                syncRun={syncRun}
                showViewSyncButton={!syncId}
              />
            ))
          )}
        </div>
      </div>
    </PageWrapper>
  );
};
