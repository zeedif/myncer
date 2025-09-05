import { PageWrapper } from "@/components/PageWrapper";
import { SyncRunRender } from "@/components/SyncRunRender";
import { PageLoader } from "@/components/ui/page-loader";
import { useListSyncRuns } from "@/hooks/useListSyncRuns";
import { useSearchParams, Link, useNavigate } from "react-router-dom";
import { useSync } from "@/hooks/useSync";
import { SyncRender } from "@/components/Sync";
import { useSyncs } from "@/hooks/useSyncs";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Button } from "@/components/ui/button";
import { ListRestart, Settings } from "lucide-react";

export const SyncRuns = () => {
  const [searchParams, setSearchParams] = useSearchParams();
  const navigate = useNavigate();
  const syncId = searchParams.get("syncId");

  const { sync, loading: syncLoading } = useSync(syncId || undefined);
  const { syncs, loading: syncsLoading } = useSyncs(); // For filter dropdown
  const { syncRuns, loading: runsLoading } = useListSyncRuns();

  if (runsLoading || syncsLoading || (syncId && syncLoading)) {
    return <PageLoader />;
  }

  const relevantRuns = syncId
    ? syncRuns.filter((run) => run.syncId === syncId)
    : syncRuns;

  const handleFilterChange = (selectedSyncId: string) => {
    setSearchParams({ syncId: selectedSyncId });
  };

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
                ? `Showing runs for a specific sync.`
                : "Filter by sync or view all runs."}
            </p>
          </div>
          <div className="flex items-center gap-2">
            {/* Button to manage syncs */}
            <Button variant="outline" asChild>
              <Link to="/syncs">
                <Settings className="w-4 h-4 mr-2" />
                Manage Syncs
              </Link>
            </Button>
            {/* Clear filter button - only visible when filter is applied */}
            {syncId && (
              <Button variant="secondary" onClick={() => navigate("/syncruns")}>
                <ListRestart className="w-4 h-4 mr-2" />
                View All Runs
              </Button>
            )}
          </div>
        </div>

        {/* Show filter dropdown or sync configuration based on context */}
        {syncId && sync ? (
          <div className="mb-8">
            <h2 className="text-2xl font-semibold mb-4 border-b pb-2">Sync Configuration</h2>
            <SyncRender sync={sync} showHistoryButton={false} />
          </div>
        ) : (
          <div className="w-full md:w-1/2">
            <Select onValueChange={handleFilterChange}>
              <SelectTrigger>
                <SelectValue placeholder="Filter by Sync..." />
              </SelectTrigger>
              <SelectContent>
                {syncs.map((s) => (
                  <SelectItem key={s.id} value={s.id}>
                    {s.syncVariant.case === "oneWaySync" ? "One-Way" : "Merge"} Sync - {s.id.substring(0, 8)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        )}

        <div className="space-y-4">
          {relevantRuns.length === 0 ? (
            <div className="rounded-xl border bg-card p-6 shadow-sm">
              <p className="text-muted-foreground">
                {syncId ? "This sync has not been run yet." : "No sync runs found."}
              </p>
            </div>
          ) : (
            relevantRuns.map((syncRun) => (
              <SyncRunRender
                key={syncRun.runId}
                syncRun={syncRun}
                showViewSyncButton={!syncId} // Button only appears when NO filter is applied
              />
            ))
          )}
        </div>
      </div>
    </PageWrapper>
  );
};
