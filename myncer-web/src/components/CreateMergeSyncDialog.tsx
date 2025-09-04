import { useState } from "react"
import {
  Dialog,
  DialogTrigger,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { useForm, useFieldArray } from "react-hook-form"
import { DatasourceSelector } from "./DatasourceSelector"
import { PlaylistSelector } from "./PlaylistSelector"
import { useCreateSync } from "@/hooks/useCreateSync"
import { Loader2, Plus, X } from "lucide-react"
import { useDatasources } from "@/hooks/useDatasources"
import { useListPlaylists } from "@/hooks/useListPlaylists"
import { Checkbox } from "@/components/ui/checkbox"
import { Label } from "@/components/ui/label"
import type { Datasource } from "@/generated_grpc/myncer/datasource_pb"

type SourcePlaylist = {
  datasource?: Datasource
  playlistId: string
}

type FormValues = {
  sources: SourcePlaylist[]
  targetDatasource: Datasource
  targetPlaylistId: string
  overwriteExisting: boolean
}

export const CreateMergeSyncDialog = () => {
  const [open, setOpen] = useState(false)
  const { datasources: connectedDatasources, loading: datasourcesLoading } = useDatasources()

  const {
    control,
    watch,
    handleSubmit,
    formState: { isValid },
  } = useForm<FormValues>({
    mode: "onChange",
    defaultValues: {
      sources: [{ datasource: undefined, playlistId: "" }, { datasource: undefined, playlistId: "" }],
      overwriteExisting: false,
    },
  })

  const { fields, append, remove } = useFieldArray({
    control,
    name: "sources",
  })

  const { mutate: createSync, isPending: creating } = useCreateSync()

  const watchedSources = watch("sources")
  const targetDatasource = watch("targetDatasource")

  const {
    playlists: targetPlaylists,
    loading: targetPlaylistsLoading,
  } = useListPlaylists({ datasource: targetDatasource })

  const onSubmit = (data: FormValues) => {
    // Filtrar las fuentes que están completas
    const completeSources = data.sources.filter(
      source => source.datasource && source.playlistId
    )

    createSync({
      syncVariant: {
        case: 'playlistMergeSync',
        value: {
          sources: completeSources.map(source => ({
            datasource: source.datasource,
            playlistId: source.playlistId
          })),
          destination: {
            datasource: data.targetDatasource,
            playlistId: data.targetPlaylistId,
          },
          overwriteExisting: data.overwriteExisting,
        },
      },
    })
    setOpen(false)
  }

  const addSource = () => {
    append({ datasource: undefined, playlistId: "" })
  }

  const removeSource = (index: number) => {
    if (fields.length > 2) {
      remove(index)
    }
  }

  const isFormLoading = datasourcesLoading || targetPlaylistsLoading

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button variant="outline">Create Playlist Merge</Button>
      </DialogTrigger>
      <DialogContent className="max-w-2xl" aria-describedby="create a playlist merge sync">
        <DialogHeader>
          <DialogTitle>Create Playlist Merge Sync</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-6 py-2">
          <div className="flex flex-col space-y-4">
            {/* Sources Section */}
            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <h3 className="text-sm font-medium">Source Playlists</h3>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={addSource}
                  className="flex items-center space-x-1"
                >
                  <Plus className="h-4 w-4" />
                  <span>Add Playlist</span>
                </Button>
              </div>
              
              {fields.map((field, index) => {
                const sourceDatasource = watchedSources[index]?.datasource
                const { playlists: sourcePlaylists, loading: sourcePlaylistsLoading } = useListPlaylists({ 
                  datasource: sourceDatasource 
                })
                
                return (
                  <div key={field.id} className="flex items-end space-x-2 p-3 border rounded-lg">
                    <div className="flex-1 grid grid-cols-2 gap-3">
                      <DatasourceSelector<FormValues>
                        name={`sources.${index}.datasource` as const}
                        control={control}
                        datasources={connectedDatasources}
                        label={`Source ${index + 1} Datasource`}
                      />
                      <PlaylistSelector<FormValues>
                        name={`sources.${index}.playlistId` as const}
                        control={control}
                        playlists={sourcePlaylists}
                        label={`Source ${index + 1} Playlist`}
                        disabled={!sourceDatasource || sourcePlaylistsLoading}
                      />
                    </div>
                    {fields.length > 2 && (
                      <Button
                        type="button"
                        variant="destructive"
                        size="sm"
                        onClick={() => removeSource(index)}
                        className="shrink-0"
                      >
                        <X className="h-4 w-4" />
                      </Button>
                    )}
                  </div>
                )
              })}
            </div>

            {/* Merge Arrow */}
            <div className="text-center text-2xl text-muted-foreground py-2">
              ⇣ MERGE ⇣
            </div>

            {/* Target Section */}
            <div className="space-y-3">
              <h3 className="text-sm font-medium">Target Playlist</h3>
              <div className="p-3 border rounded-lg">
                <div className="grid grid-cols-2 gap-3">
                  <DatasourceSelector<FormValues>
                    name="targetDatasource"
                    control={control}
                    datasources={connectedDatasources}
                    label="Target Datasource"
                  />
                  <PlaylistSelector<FormValues>
                    name="targetPlaylistId"
                    control={control}
                    playlists={targetPlaylists}
                    label="Target Playlist"
                    disabled={!targetDatasource}
                  />
                </div>
              </div>
            </div>

            {/* Options */}
            <div className="flex items-center space-x-2">
              <Checkbox
                id="overwriteExisting"
                {...control.register("overwriteExisting")}
              />
              <Label htmlFor="overwriteExisting" className="text-sm">
                Overwrite existing songs in target playlist
              </Label>
            </div>
          </div>

          <Button
            type="submit"
            disabled={!isValid || isFormLoading || creating}
            className="w-full"
          >
            {(isFormLoading || creating) ? (
              <div className="flex items-center justify-center space-x-2">
                <Loader2 className="h-4 w-4 animate-spin" />
                <span>{isFormLoading ? "Loading..." : "Creating..."}</span>
              </div>
            ) : (
              "Create Merge Sync"
            )}
          </Button>
        </form>
      </DialogContent>
    </Dialog>
  )
}
