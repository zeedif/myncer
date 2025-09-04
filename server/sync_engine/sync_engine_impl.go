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
	// Validates the sync is valid and implemented.
	if err := s.validateSync(sync); err != nil {
		return core.WrappedError(err, "failed to validate sync")
	}

	// Store the sync run run state in the database.
	syncRun := s.getSyncRun(sync)
	if err := s.storeSyncRun(ctx, syncRun, true /*create*/); err != nil {
		return core.WrappedError(err, "failed to store sync run")
	}

	// Run the sync.
	var err error = nil
	switch v := sync.GetSyncVariant().(type) {
	case *myncer_pb.Sync_OneWaySync:
		if err = s.runOneWaySync(ctx, userInfo, v.OneWaySync); err != nil {
			err = core.WrappedError(err, "failed to run one-way sync")
		}
	case *myncer_pb.Sync_PlaylistMergeSync:
		if err = s.runPlaylistMergeSync(ctx, userInfo, v.PlaylistMergeSync); err != nil {
			err = core.WrappedError(err, "failed to run playlist merge sync")
		}
	default:
		// We should never reach here if the sync was validated correctly.
		err = core.NewError(fmt.Sprintf("unreachable: unknown sync variant: %T", sync.GetSyncVariant()))
	}

	// Update the status of the sync run in the database.
	if err != nil {
		syncRun.SyncStatus = myncer_pb.SyncStatus_SYNC_STATUS_FAILED
	} else {
		syncRun.SyncStatus = myncer_pb.SyncStatus_SYNC_STATUS_COMPLETED
	}
	if err := s.storeSyncRun(ctx, syncRun, false /*create*/); err != nil {
		return core.WrappedError(err, "failed to update sync run in database")
	}

	return nil
}

func (s *syncEngineImpl) getSyncRun(sync *myncer_pb.Sync /*const*/) *myncer_pb.SyncRun {
	return &myncer_pb.SyncRun{
		SyncId:     sync.GetId(),
		RunId:      uuid.NewString(),
		SyncStatus: myncer_pb.SyncStatus_SYNC_STATUS_RUNNING,
	}
}

func (s *syncEngineImpl) storeSyncRun(
	ctx context.Context,
	syncRun *myncer_pb.SyncRun, /*const*/
	create bool, // if true, create a new sync run, otherwise update an existing one.
) error {
	syncRunStore := core.ToMyncerCtx(ctx).DB.SyncRunStore
	if create {
		if err := syncRunStore.CreateSyncRun(ctx, syncRun); err != nil {
			return core.WrappedError(err, "failed to create sync run in database")
		}
	} else {
		if err := syncRunStore.UpdateSyncRun(ctx, syncRun); err != nil {
			return core.WrappedError(err, "failed to update sync run in database")
		}
	}
	return nil
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
