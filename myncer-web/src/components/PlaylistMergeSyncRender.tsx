import { Badge } from "@/components/ui/badge"
import type { PlaylistMergeSync } from "@/generated_grpc/myncer/sync_pb"
import { Datasource } from "@/generated_grpc/myncer/datasource_pb"

interface PlaylistMergeSyncRenderProps {
  sync: PlaylistMergeSync
}

export const PlaylistMergeSyncRender = ({ sync }: PlaylistMergeSyncRenderProps) => {
  const getDatasourceName = (datasource: number) => {
    switch (datasource) {
      case Datasource.SPOTIFY:
        return "Spotify"
      case Datasource.YOUTUBE:
        return "YouTube"
      case Datasource.TIDAL:
        return "Tidal"
      default:
        return "Unknown"
    }
  }

  const getDatasourceColor = (datasource: number) => {
    switch (datasource) {
      case Datasource.SPOTIFY:
        return "bg-green-500"
      case Datasource.YOUTUBE:
        return "bg-red-500"
      case Datasource.TIDAL:
        return "bg-white-500"
      default:
        return "bg-gray-500"
    }
  }

  return (
    <div className="space-y-4">
      {/* Sources Section */}
      <div className="space-y-2">
        <h4 className="text-sm font-medium text-muted-foreground">Source Playlists</h4>
        <div className="grid gap-2">
          {sync.sources.map((source, index) => (
            <div
              key={index}
              className="flex items-center justify-between p-3 bg-muted/50 rounded-lg border"
            >
              <div className="flex items-center space-x-3">
                <div
                  className={`w-3 h-3 rounded-full ${getDatasourceColor(source.datasource)}`}
                />
                <div>
                  <div className="font-medium text-sm">
                    {getDatasourceName(source.datasource)}
                  </div>
                  <div className="text-xs text-muted-foreground">
                    Playlist ID: {source.playlistId}
                  </div>
                </div>
              </div>
              <Badge variant="secondary" className="text-xs">
                Source {index + 1}
              </Badge>
            </div>
          ))}
        </div>
      </div>

      {/* Merge Indicator */}
      <div className="flex justify-center py-2">
        <div className="flex items-center space-x-2 text-muted-foreground">
          <div className="text-lg">⇣</div>
          <span className="text-sm font-medium">MERGE</span>
          <div className="text-lg">⇣</div>
        </div>
      </div>

      {/* Destination Section */}
      <div className="space-y-2">
        <h4 className="text-sm font-medium text-muted-foreground">Target Playlist</h4>
        <div className="flex items-center justify-between p-3 bg-muted/50 rounded-lg border">
          <div className="flex items-center space-x-3">
            <div
              className={`w-3 h-3 rounded-full ${getDatasourceColor(sync.destination?.datasource || 0)}`}
            />
            <div>
              <div className="font-medium text-sm">
                {getDatasourceName(sync.destination?.datasource || 0)}
              </div>
              <div className="text-xs text-muted-foreground">
                Playlist ID: {sync.destination?.playlistId}
              </div>
            </div>
          </div>
          <Badge variant="default" className="text-xs">
            Target
          </Badge>
        </div>
      </div>

      {/* Options */}
      {sync.overwriteExisting && (
        <div className="flex items-center space-x-2 text-sm text-amber-600">
          <div className="w-2 h-2 bg-amber-500 rounded-full" />
          <span>Will overwrite existing songs in target playlist</span>
        </div>
      )}
    </div>
  )
}
