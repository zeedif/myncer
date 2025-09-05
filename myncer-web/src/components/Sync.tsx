import { Button } from "@/components/ui/button"
import { CircleCheck, CircleX, AlertCircle, Loader2, Trash2, Play, History } from "lucide-react"
import { OneWaySyncRender } from "./OneWaySyncRender"
import { PlaylistMergeSyncRender } from "./PlaylistMergeSyncRender"
import { useDeleteSync } from "@/hooks/useDeleteSync"
import { useRunSync } from "@/hooks/useRunSync"
import { SyncStatus, type Sync } from "@/generated_grpc/myncer/sync_pb"
import { protoTimestampToDate } from "@/lib/utils"
import { useListSyncRuns } from "@/hooks/useListSyncRuns"
import { Link } from "react-router-dom"

export const SyncRender = ({ sync, showHistoryButton = true }: { sync: Sync; showHistoryButton?: boolean }) => {
  const { runSync, isRunningSync } = useRunSync()
  const { deleteSync, isDeleting } = useDeleteSync()
  const { syncRuns } = useListSyncRuns()
  const { syncVariant, id, createdAt } = sync

  // 1. Find the most recent run for this Sync
  const mostRecentRun = syncRuns
    .filter((run) => run.syncId === sync.id)
    .sort((a, b) => {
      // Sort by update date descending to get the most recent
      const dateA = a.updatedAt ? protoTimestampToDate(a.updatedAt).getTime() : 0
      const dateB = b.updatedAt ? protoTimestampToDate(b.updatedAt).getTime() : 0
      return dateB - dateA
    })[0] // Take the first element, which is the most recent

  // 2. Determine the visual status
  let status: 'RUNNING' | 'PENDING' | 'FAILED' | 'COMPLETED' | 'IDLE' = 'IDLE'
  if (mostRecentRun) {
    switch (mostRecentRun.syncStatus) {
      case SyncStatus.RUNNING:
        status = 'RUNNING'
        break
      case SyncStatus.PENDING:
        status = 'PENDING'
        break
      case SyncStatus.FAILED:
        status = 'FAILED'
        break
      case SyncStatus.COMPLETED:
        status = 'COMPLETED'
        break
    }
  }

  // React-query mutation state has priority if a click just happened
  const isSyncing = isRunningSync || status === 'RUNNING' || status === 'PENDING'

  const renderVariantLabel = () => {
    switch (syncVariant.case) {
      case "oneWaySync":
        return "One-Way"
      case "playlistMergeSync":
        return "Playlist Merge"
      default:
        return "Unknown"
    }
  }

  const renderStatusFooter = () => {
    if (!mostRecentRun) {
      return <div>Created at {createdAt ? protoTimestampToDate(createdAt).toLocaleString() : "Unknown"}</div>
    }

    const runDate = protoTimestampToDate(mostRecentRun.updatedAt!).toLocaleString()

    switch (status) {
      case 'COMPLETED':
        return (
          <div className="flex items-center gap-2 text-green-600">
            <CircleCheck className="w-4 h-4" />
            <span>Last synced successfully at {runDate}</span>
          </div>
        )
      case 'FAILED':
        return (
          <div className="flex items-center gap-2 text-red-600">
            <AlertCircle className="w-4 h-4" />
            <span>Failed at {runDate}</span>
          </div>
        )
      case 'RUNNING':
        return (
          <div className="flex items-center gap-2 text-blue-600 animate-pulse">
            <Loader2 className="w-4 h-4 animate-spin" />
            <span>Syncing now...</span>
          </div>
        )
      case 'PENDING':
        return (
          <div className="flex items-center gap-2 text-yellow-600">
            <Loader2 className="w-4 h-4" />
            <span>Pending execution...</span>
          </div>
        )
      default:
        return <div>Created at {createdAt ? protoTimestampToDate(createdAt).toLocaleString() : "Unknown"}</div>
    }
  }

  return (
    <div className="w-full rounded-lg border bg-card p-4 shadow-sm space-y-4">
      {/* Header: Variant label + Actions */}
      <div className="flex items-center justify-between">
        <span className="text-xs text-muted-foreground">
          {renderVariantLabel()} Sync
        </span>
      </div>

      {/* Sync Details */}
      <div className="mb-8">
        {syncVariant.case === "oneWaySync" && (
          <OneWaySyncRender sync={syncVariant.value} />
        )}
        {syncVariant.case === "playlistMergeSync" && (
          <PlaylistMergeSyncRender sync={syncVariant.value} />
        )}
      </div>

      {/* Footer: Updated */}
      <div className="flex justify-between items-center">
        <div className="space-y-1 text-xs text-muted-foreground">
          {renderStatusFooter()}
        </div>
        {/* Action Buttons */}
        <div className="flex items-center gap-2">
          {/* View History */}
          {showHistoryButton && (
            <Button size="sm" variant="outline" asChild>
              <Link to={`/syncruns?syncId=${id}`}>
                <History className="w-4 h-4 mr-2" />
                View History
              </Link>
            </Button>
          )}

          {/* Delete Sync */}
          <Button
            size="sm"
            variant="destructive"
            onClick={() => deleteSync({ syncId: id })}
            disabled={isDeleting || isSyncing}
          >
            {isDeleting ? (
              <>
                <Loader2 className="w-4 h-4 animate-spin mr-2" />
                Deleting
              </>
            ) : (
              <>
                <Trash2 className="w-4 h-4 mr-2" />
                Delete
              </>
            )}
          </Button>
          {/* Run/Retry Sync Button - Updated */}
          <Button
            size="sm"
            onClick={() => runSync({ syncId: id })}
            disabled={isSyncing || isDeleting}
          >
            {isSyncing ? (
              <>
                <Loader2 className="w-4 h-4 animate-spin mr-2" />
                {status === 'PENDING' ? 'Pending' : 'Running'}
              </>
            ) : (
              <>
                {status === 'FAILED' ? <CircleX className="w-4 h-4 mr-2" /> : <Play className="w-4 h-4 mr-2" />}
                {status === 'FAILED' ? 'Retry' : 'Run Sync'}
              </>
            )}
          </Button>
        </div>
      </div>
    </div>
  )
}

