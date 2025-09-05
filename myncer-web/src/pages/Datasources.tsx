import { useState } from "react"
import { PageWrapper } from "@/components/PageWrapper"
import { Button } from "@/components/ui/button"
import { PageLoader } from "@/components/ui/page-loader"
import { Datasource } from "@/generated_grpc/myncer/datasource_pb"
import { useDatasources } from "@/hooks/useDatasources"
import {
  getDatasourceLabel,
  getSpotifyAuthUrl,
  getTidalAuthUrl,
  getYoutubeAuthUrl,
} from "@/lib/utils"
import { ArrowRightIcon, Loader2, Trash2 } from "lucide-react"
import { useUnlinkDatasource } from "@/hooks/useUnlinkDatasource"
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { toast } from "sonner"

const DatasourceCard = ({
  datasource,
  onConnect,
  onDisconnect,
  isConnected,
  isDisconnecting,
}: {
  datasource: Datasource
  onConnect: () => void | Promise<void>
  onDisconnect: () => void
  isConnected: boolean
  isDisconnecting?: boolean
}) => {
  const name = getDatasourceLabel(datasource)

  return (
    <div className="rounded-xl border bg-card p-6 shadow-sm">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold">{name}</h2>
          <p className="text-sm text-muted-foreground">
            Connect account to sync playlists.
          </p>
        </div>
        {isConnected ? (
          <Dialog>
            <DialogTrigger asChild>
              <Button variant="destructive" className="flex items-center gap-1">
                <Trash2 className="w-4 h-4" />
                Disconnect
              </Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Disconnect {name}?</DialogTitle>
                <DialogDescription>
                  Are you sure you want to disconnect your {name} account? Any
                  syncs that depend on it will stop working.
                </DialogDescription>
              </DialogHeader>
              <DialogFooter>
                <DialogClose asChild>
                  <Button variant="outline">Cancel</Button>
                </DialogClose>
                <Button
                  variant="destructive"
                  onClick={onDisconnect}
                  disabled={isDisconnecting}
                >
                  {isDisconnecting ? (
                    <>
                      <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                      Disconnecting...
                    </>
                  ) : (
                    "Disconnect"
                  )}
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        ) : (
          <Button onClick={onConnect} className="flex items-center gap-1">
            Connect
            <ArrowRightIcon className="w-4 h-4" />
          </Button>
        )}
      </div>
    </div>
  )
}

export const Datasources = () => {
  const { datasources, loading } = useDatasources()
  const { mutate: unlinkDatasource, isPending: isUnlinking } =
    useUnlinkDatasource()
  const [unlinkingDs, setUnlinkingDs] = useState<Datasource | null>(null)

  const handleConnectSpotify = () => {
    window.location.href = getSpotifyAuthUrl()
  }
  const handleConnectYoutube = () => {
    window.location.href = getYoutubeAuthUrl()
  }
  const handleConnectTidal = async () => {
    try {
      const authUrl = await getTidalAuthUrl()
      window.location.href = authUrl
    } catch (error) {
      console.error("Failed to generate Tidal Auth URL:", error)
      toast.error("Could not initiate connection with Tidal. Please try again.")
    }
  }

  const handleDisconnect = (datasource: Datasource) => {
    setUnlinkingDs(datasource)
    unlinkDatasource({ datasource })
  }

  if (loading) {
    return <PageLoader />
  }

  const allDatasources = [
    { dsEnum: Datasource.SPOTIFY, onConnect: handleConnectSpotify },
    { dsEnum: Datasource.YOUTUBE, onConnect: handleConnectYoutube },
    { dsEnum: Datasource.TIDAL, onConnect: handleConnectTidal },
  ]

  return (
    <PageWrapper>
      <div className="max-w-5xl space-y-8 px-4 py-8">
        <div>
          <h1 className="text-3xl font-bold">Datasources</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Manage integrations with third-party services.
          </p>
        </div>

        <div className="flex flex-col gap-4">
          {allDatasources.map(({ dsEnum, onConnect }) => (
            <DatasourceCard
              key={dsEnum}
              datasource={dsEnum}
              isConnected={datasources?.some((ds) => ds === dsEnum)}
              onConnect={onConnect}
              onDisconnect={() => handleDisconnect(dsEnum)}
              isDisconnecting={isUnlinking && unlinkingDs === dsEnum}
            />
          ))}
        </div>
      </div>
    </PageWrapper>
  )
}
