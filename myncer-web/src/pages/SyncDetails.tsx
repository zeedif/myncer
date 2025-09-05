import { PageWrapper } from "@/components/PageWrapper"
import { SyncRender } from "@/components/Sync"
import { SyncRunRender } from "@/components/SyncRunRender"
import { PageLoader } from "@/components/ui/page-loader"
import { useListSyncRuns } from "@/hooks/useListSyncRuns"
import { useSync } from "@/hooks/useSync"
import { useParams } from "react-router-dom"

export const SyncDetails = () => {
  const { syncId } = useParams<{ syncId: string }>()
  const { sync, loading: syncLoading } = useSync(syncId)
  const { syncRuns, loading: runsLoading } = useListSyncRuns()

  if (syncLoading || runsLoading) {
    return <PageLoader />
  }

  if (!syncId || !sync) {
    return <div className="text-red-500 p-5">Sync not found or ID is missing.</div>
  }

  const relevantRuns = syncRuns.filter((run) => run.syncId === syncId)

  return (
    <PageWrapper>
      <div className="max-w-5xl px-4 py-8 space-y-8">
        {/* Muestra la configuración actual de la sincronización */}
        <SyncRender key={sync.id} sync={sync} />

        {/* Muestra el historial de ejecuciones */}
        <div className="space-y-4">
          <h2 className="text-2xl font-bold border-b pb-2">Run History</h2>
          {relevantRuns.length > 0 ? (
            relevantRuns.map((run) => <SyncRunRender key={run.runId} syncRun={run} />)
          ) : (
            <div className="rounded-xl border bg-card p-6 shadow-sm">
              <p className="text-muted-foreground">This sync has not been run yet.</p>
            </div>
          )}
        </div>
      </div>
    </PageWrapper>
  )
}
