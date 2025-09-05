import { SyncStatus, type SyncRun } from "@/generated_grpc/myncer/sync_pb"
import { type Song } from "@/generated_grpc/myncer/song_pb"
import { protoTimestampToDate, getDatasourceLabel } from "@/lib/utils"
import { Button } from "./ui/button"
import { Link } from "react-router-dom"
import clsx from "clsx"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
  DialogDescription,
} from "./ui/dialog"
import { AlertTriangle, ListMusic } from "lucide-react"

const syncStatusToUI: Record<
  SyncStatus,
  { label: string; className: string }
> = {
  [SyncStatus.UNSPECIFIED]: {
    label: "Unspecified",
    className: "bg-gray-200 text-gray-700",
  },
  [SyncStatus.PENDING]: {
    label: "Pending",
    className: "bg-yellow-100 text-yellow-800",
  },
  [SyncStatus.RUNNING]: {
    label: "Running",
    className: "bg-blue-100 text-blue-800 animate-pulse",
  },
  [SyncStatus.COMPLETED]: {
    label: "Completed",
    className: "bg-green-100 text-green-800",
  },
  [SyncStatus.FAILED]: {
    label: "Failed",
    className: "bg-red-100 text-red-800",
  },
  [SyncStatus.CANCELLED]: {
    label: "Cancelled",
    className: "bg-muted text-muted-foreground",
  },
}

// Componente para mostrar la lista de canciones no encontradas
const UnmatchedSongsList = ({ songs }: { songs: Song[] }) => (
  <div className="mt-4">
    <h3 className="text-lg font-semibold mb-2">Unmatched Songs</h3>
    <div className="border rounded-lg max-h-96 overflow-y-auto">
      {songs.map((song, index) => (
        <div key={index} className="p-3 border-b last:border-b-0">
          <p className="font-medium text-sm">{song.name}</p>
          <p className="text-xs text-muted-foreground">{song.artistName.join(", ")}</p>
          <p className="text-xs text-muted-foreground mt-1">
            (From: {getDatasourceLabel(song.datasource)})
          </p>
        </div>
      ))}
    </div>
  </div>
)

interface SyncRunRenderProps {
  syncRun: SyncRun
}
export const SyncRunRender = ({ syncRun }: SyncRunRenderProps) => {
  const createdAt = syncRun.createdAt
    ? protoTimestampToDate(syncRun.createdAt).toLocaleString()
    : "Unknown"
  const updatedAt = syncRun.updatedAt
    ? protoTimestampToDate(syncRun.updatedAt).toLocaleString()
    : "Unknown"

  const status = syncStatusToUI[syncRun.syncStatus] ?? syncStatusToUI[SyncStatus.UNSPECIFIED]

  return (
    <div className="rounded-xl border bg-card p-6 shadow-sm">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold">Run ID: {syncRun.runId}</h2>
          <div
            className={clsx(
              "inline-block rounded-full px-3 py-0.5 text-xs font-medium mt-2",
              status.className
            )}
          >
            {status.label}
          </div>
          <p className="text-sm text-muted-foreground mt-2">Created: {createdAt}</p>
          <p className="text-sm text-muted-foreground">Updated: {updatedAt}</p>
          {syncRun.errorMessage && (
            <div className="mt-2 flex items-center gap-2 text-sm text-red-600">
              <AlertTriangle className="h-4 w-4" />
              <span>{syncRun.errorMessage}</span>
            </div>
          )}
        </div>
        <div className="flex gap-2">
          <Dialog>
            <DialogTrigger asChild>
              <Button variant="outline">View Details</Button>
            </DialogTrigger>
            <DialogContent className="max-w-3xl">
              <DialogHeader>
                <DialogTitle>Sync Run Details</DialogTitle>
                <DialogDescription>Run ID: {syncRun.runId}</DialogDescription>
              </DialogHeader>
              {syncRun.unmatchedSongs && syncRun.unmatchedSongs.length > 0 ? (
                <UnmatchedSongsList songs={syncRun.unmatchedSongs} />
              ) : (
                <div className="text-center py-8 text-muted-foreground">
                  <ListMusic className="mx-auto h-8 w-8 mb-2" />
                  <p>All songs were matched successfully in this run.</p>
                </div>
              )}
            </DialogContent>
          </Dialog>
          <Button variant="outline" asChild>
            <Link to={`/syncs/${syncRun.syncId}`}>View Sync</Link>
          </Button>
        </div>
      </div>
    </div>
  )
}
