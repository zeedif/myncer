package sync_engine

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/hansbala/myncer/core"
	"github.com/hansbala/myncer/matching"
	myncer_pb "github.com/hansbala/myncer/proto/myncer"
)

func NewSyncEngine() core.SyncEngine {
	return &syncEngineImpl{}
}

type syncEngineImpl struct{}

var _ core.SyncEngine = (*syncEngineImpl)(nil)

func (s *syncEngineImpl) RunSync(
	ctx context.Context,
	userInfo *myncer_pb.User, /*const*/
	sync *myncer_pb.Sync, /*const*/
) error {
	// Create the initial SyncRun object with a PENDING status.
	syncRun := &myncer_pb.SyncRun{
		SyncId:     sync.GetId(),
		RunId:      uuid.NewString(),
		SyncStatus: myncer_pb.SyncStatus_SYNC_STATUS_PENDING,
	}

	// Use a deferred function to guarantee the final status is set, stored, and broadcast.
	defer func() {
		// If a panic occurs during the sync, recover and set status to FAILED.
		if r := recover(); r != nil {
			core.Errorf("Recovered from panic in RunSync: %v", r)
			syncRun.SyncStatus = myncer_pb.SyncStatus_SYNC_STATUS_FAILED
		}
		
		// Store and broadcast the final status.
		if _, err := s.storeAndBroadcastSyncRun(ctx, syncRun, false); err != nil {
			core.Errorf(core.WrappedError(err, "CRITICAL: failed to store final sync run state for run %s", syncRun.GetRunId()))
		}
	}()

	// 1. Store and broadcast the initial PENDING status.
	storedRun, err := s.storeAndBroadcastSyncRun(ctx, syncRun, true /* isCreate */)
	if err != nil {
		return core.WrappedError(err, "failed to store initial sync run")
	}
	syncRun = storedRun // Use the run object returned from DB (with correct timestamps)

	// 2. Update status to RUNNING, then store and broadcast.
	syncRun.SyncStatus = myncer_pb.SyncStatus_SYNC_STATUS_RUNNING
	storedRun, err = s.storeAndBroadcastSyncRun(ctx, syncRun, false /* isCreate */)
	if err != nil {
		return core.WrappedError(err, "failed to store running sync status")
	}
	syncRun = storedRun

	// 3. Execute the main sync logic.
	var syncErr error
	switch v := sync.GetSyncVariant().(type) {
	case *myncer_pb.Sync_OneWaySync:
		syncErr = s.runOneWaySync(ctx, userInfo, v.OneWaySync)
	case *myncer_pb.Sync_PlaylistMergeSync:
		syncErr = s.runPlaylistMergeSync(ctx, userInfo, v.PlaylistMergeSync)
	default:
		syncErr = core.NewError("unreachable: unknown sync variant: %T", sync.GetSyncVariant())
	}

	// 4. Determine final status based on the error. The deferred function will handle storage.
	if syncErr != nil {
		syncRun.SyncStatus = myncer_pb.SyncStatus_SYNC_STATUS_FAILED
		return core.WrappedError(syncErr, "sync execution failed")
	}

	syncRun.SyncStatus = myncer_pb.SyncStatus_SYNC_STATUS_COMPLETED
	return nil
}


// storeAndBroadcastSyncRun is a helper to centralize storing and broadcasting logic.
func (s *syncEngineImpl) storeAndBroadcastSyncRun(
	ctx context.Context,
	syncRun *myncer_pb.SyncRun,
	isCreate bool,
) (*myncer_pb.SyncRun, error) {
	myncerCtx := core.ToMyncerCtx(ctx)
	syncRunStore := myncerCtx.DB.SyncRunStore

	if isCreate {
		if err := syncRunStore.CreateSyncRun(ctx, syncRun); err != nil {
			return nil, core.WrappedError(err, "failed to create sync run in database")
		}
	} else {
		if err := syncRunStore.UpdateSyncRun(ctx, syncRun); err != nil {
			return nil, core.WrappedError(err, "failed to update sync run in database")
		}
	}

	// Always re-fetch from the database to get the latest state, including DB-generated timestamps.
	runs, err := syncRunStore.GetSyncs(ctx, core.NewSet(syncRun.GetRunId()), nil)
	if err != nil || runs.IsEmpty() {
		core.Warningf("Failed to re-fetch sync run %s after store, broadcast will use in-memory object. Error: %v", syncRun.GetRunId(), err)
		myncerCtx.SyncStatusBroadcaster.Broadcast(syncRun)
		return syncRun, nil
	}
	
	refreshedSyncRun := runs.ToArray()[0]
	
	core.Printf("Broadcasting status '%s' for sync run %s", refreshedSyncRun.GetSyncStatus().String(), refreshedSyncRun.GetRunId())
	myncerCtx.SyncStatusBroadcaster.Broadcast(refreshedSyncRun)
	
	return refreshedSyncRun, nil
}

func (s *syncEngineImpl) validateSync(sync *myncer_pb.Sync /*const*/) error {
	switch sync.GetSyncVariant().(type) {
	case *myncer_pb.Sync_OneWaySync:
		return nil
	case *myncer_pb.Sync_PlaylistMergeSync:
		return nil
	default:
		return core.NewError(fmt.Sprintf("unknown sync variant: %T", sync.GetSyncVariant()))
	}
}

func (s *syncEngineImpl) runOneWaySync(
	ctx context.Context,
	userInfo *myncer_pb.User, /*const*/
	sync *myncer_pb.OneWaySync, /*const*/
) error {
	sourceClient, err := s.getClient(ctx, sync.GetSource().GetDatasource())
	if err != nil {
		return err
	}
	destClient, err := s.getClient(ctx, sync.GetDestination().GetDatasource())
	if err != nil {
		return err
	}

	// Fetch songs from source playlist
	sourceSongs, err := sourceClient.GetPlaylistSongs(ctx, userInfo, sync.GetSource().GetPlaylistId())
	if err != nil {
		return core.WrappedError(err, "failed to fetch source playlist")
	}

	// Normalize songs if supported.
	var normalizedSongs *core.SongList
	if s.shouldNormalize(ctx) {
		normalizedSongs, err = NewLlmSongsNormalizer().NormalizeSongs(
			ctx,
			core.NewSongList(sourceSongs),
		)
		if err != nil {
			return core.WrappedError(err, "failed to normalize songs")
		}
	} else {
		normalizedSongs = core.NewSongList(sourceSongs)
	}

	searchedSongs, err := s.getSearchedSongs(
		ctx,
		userInfo,
		normalizedSongs.GetSongs(),
		sync.GetDestination().GetDatasource(),
	)
	if err != nil {
		return core.WrappedError(err, "failed to get searched songs for destination datasource")
	}

	// Optionally clear destination playlist
	destPlaylistId := sync.GetDestination().GetPlaylistId()
	if sync.OverwriteExisting {
		core.Printf("Clearing destination playlist")
		if err := destClient.ClearPlaylist(ctx, userInfo, destPlaylistId); err != nil {
			return core.WrappedError(err, "failed to clear destination playlist")
		}
	}

	// Add source songs to destination
	if err := destClient.AddToPlaylist(ctx, userInfo, destPlaylistId, searchedSongs); err != nil {
		return core.WrappedError(err, "failed to add songs to destination playlist")
	}
	return nil
}

func (s *syncEngineImpl) getSearchedSongs(
	ctx context.Context,
	userInfo *myncer_pb.User, /*const*/
	songs []core.Song, /*const*/
	datasource myncer_pb.Datasource, /*const*/
) ([]core.Song, error) {
	r := []core.Song{}
	for _, song := range songs {
		newDatasourceSongId, err := song.GetIdByDatasource(ctx, userInfo, datasource)
		if err != nil {
			// Just log the error and continue with the next song.
			core.Errorf(
				core.NewError("failed to get datasource ID for song %s: %s", song.GetName(), err.Error()),
			)
			continue
		}
		r = append(
			r,
			NewSong(
				&myncer_pb.Song{
					Name:             song.GetName(),
					ArtistName:       song.GetArtistNames(),
					AlbumName:        song.GetAlbum(),
					DatasourceSongId: newDatasourceSongId,
				},
			),
		)
	}
	return r, nil
}

func (s *syncEngineImpl) shouldNormalize(ctx context.Context) bool {
	return core.ToMyncerCtx(ctx).Config.GetLlmConfig().GetEnabled()
}

func (s *syncEngineImpl) getClient(
	ctx context.Context,
	datasource myncer_pb.Datasource,
) (core.DatasourceClient, error) {
	dsClients := core.ToMyncerCtx(ctx).DatasourceClients
	switch datasource {
	case myncer_pb.Datasource_DATASOURCE_SPOTIFY:
		return dsClients.SpotifyClient, nil
	case myncer_pb.Datasource_DATASOURCE_YOUTUBE:
		return dsClients.YoutubeClient, nil
	case myncer_pb.Datasource_DATASOURCE_TIDAL:
		return dsClients.TidalClient, nil
	default:
		return nil, core.NewError("unsupported datasource: %v", datasource)
	}
}

func (s *syncEngineImpl) runPlaylistMergeSync(
	ctx context.Context,
	userInfo *myncer_pb.User, /*const*/
	sync *myncer_pb.PlaylistMergeSync, /*const*/
) error {
	allSongs := []core.Song{}

	// 1. Collect songs from all sources
	for _, source := range sync.GetSources() {
		sourceClient, err := s.getClient(ctx, source.GetDatasource())
		if err != nil {
			return core.WrappedError(err, "failed to get source client for datasource %v", source.GetDatasource())
		}
		songs, err := sourceClient.GetPlaylistSongs(ctx, userInfo, source.GetPlaylistId())
		if err != nil {
			core.Warningf("Could not fetch songs from playlist %s, skipping.", source.GetPlaylistId())
			continue
		}
		allSongs = append(allSongs, songs...)
	}

	// 2. Remove duplicates (decoupled logic)
	uniqueSongs, err := matching.DeduplicateSongs(allSongs, 90.0) // 90.0 is the similarity threshold
	if err != nil {
		return core.WrappedError(err, "failed to deduplicate songs")
	}

	// 3. Get destination client
	destClient, err := s.getClient(ctx, sync.GetDestination().GetDatasource())
	if err != nil {
		return core.WrappedError(err, "failed to get destination client")
	}

	destPlaylistId := sync.GetDestination().GetPlaylistId()

	// 4. (Optional) Clear destination playlist
	if sync.GetOverwriteExisting() {
		if err := destClient.ClearPlaylist(ctx, userInfo, destPlaylistId); err != nil {
			return core.WrappedError(err, "failed to clear destination playlist")
		}
	}
	
	// 5. Add songs to destination list
	// You may need to search for each song on the destination platform first.
	searchedSongs, err := s.getSearchedSongs(ctx, userInfo, uniqueSongs, sync.GetDestination().GetDatasource())
	if err != nil {
		return core.WrappedError(err, "failed to search for songs on destination platform")
	}

	if err := destClient.AddToPlaylist(ctx, userInfo, destPlaylistId, searchedSongs); err != nil {
		return core.WrappedError(err, "failed to add songs to destination playlist")
	}

	return nil
}
