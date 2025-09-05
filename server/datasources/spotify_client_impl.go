package datasources

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	spotify "github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2"

	"github.com/hansbala/myncer/core"
	"github.com/hansbala/myncer/matching"
	myncer_pb "github.com/hansbala/myncer/proto/myncer"
	"github.com/hansbala/myncer/sync_engine"
)

const (
	cPageLimit       = 50
	cSpotifyAuthUrl  = "https://accounts.spotify.com/authorize"
	cSpotifyTokenUrl = "https://accounts.spotify.com/api/token"
)

func NewSpotifyClient() core.DatasourceClient {
	return &spotifyClientImpl{}
}

type spotifyClientImpl struct{}

var _ core.DatasourceClient = (*spotifyClientImpl)(nil)

// ExchangeCodeForToken makes an API request to spotify to to retrieve the access and refresh token.
func (s *spotifyClientImpl) ExchangeCodeForToken(
	ctx context.Context,
	authCode string,
	codeVerifier string,
) (*oauth2.Token, error) {
	authenticator := s.getAuthenticator(ctx)
	token, err := authenticator.Exchange(ctx, authCode)
	if err != nil {
		return nil, core.WrappedError(err, "failed to exchange auth code with spotify")
	}
	return token, nil
}

func (s *spotifyClientImpl) GetPlaylist(
	ctx context.Context,
	userInfo *myncer_pb.User, /*const*/
	playlistId string,
) (*myncer_pb.Playlist, error) {
	client, err := s.getClient(ctx, userInfo)
	if err != nil {
		return nil, core.WrappedError(err, "failed to get spotify client")
	}
	if len(playlistId) == 0 {
		return nil, core.NewError("invalid playlist id")
	}
	playlist, err := client.GetPlaylist(ctx, spotify.ID(playlistId))
	if err != nil {
		return nil, core.WrappedError(err, "failed to get spotify playlist with id %s", playlistId)
	}
	return spotifyPlaylistToProto(playlist), nil
}

func (s *spotifyClientImpl) GetPlaylistSongs(
	ctx context.Context,
	userInfo *myncer_pb.User, /*const*/
	playlistId string,
) ([]core.Song, error) {
	client, err := s.getClient(ctx, userInfo)
	if err != nil {
		return nil, core.WrappedError(err, "failed to get spotify client")
	}
	// Use GetPlaylistItems to fetch all songs in the playlist.
	if len(playlistId) == 0 {
		return nil, core.NewError("invalid playlist id")
	}
	allSongs := []core.Song{}
	offset := 0
	for {
		playlistTracks, err := client.GetPlaylistItems(
			ctx,
			spotify.ID(playlistId),
			spotify.Limit(cPageLimit),
			spotify.Offset(offset),
		)
		if err != nil {
			if spotifyErr, ok := err.(spotify.Error); ok &&
				spotifyErr.Status == http.StatusTooManyRequests {
				core.Printf("Spotify API rate limit hit, with message: %s", spotifyErr.Message)
			}
			return nil, core.WrappedError(
				err,
				"failed to get playlist items for playlist %s at offset %d",
				playlistId,
				offset,
			)
		}
		for _, item := range playlistTracks.Items {
			if item.Track.Track != nil {
				allSongs = append(allSongs, buildSongFromSpotifyTrack(ctx, item.Track.Track))
			}
		}
		if len(playlistTracks.Items) < cPageLimit {
			// No more items left to fetch.
			break
		}
		offset += cPageLimit
	}
	return allSongs, nil
}

func (s *spotifyClientImpl) GetPlaylists(
	ctx context.Context,
	userInfo *myncer_pb.User, /*const*/
) ([]*myncer_pb.Playlist, error) {
	client, err := s.getClient(ctx, userInfo)
	if err != nil {
		return nil, core.WrappedError(err, "failed to get spotify client")
	}

	r := []*myncer_pb.Playlist{}
	for offset := 0; ; offset += cPageLimit {
		page, err := client.CurrentUsersPlaylists(
			ctx,
			spotify.Limit(cPageLimit),
			spotify.Offset(offset),
		)
		if err != nil {
			return nil, core.WrappedError(
				err,
				"failed to get current user playlists at offset %d",
				offset,
			)
		}

		for _, p := range page.Playlists {
			r = append(
				r,
				&myncer_pb.Playlist{
					MusicSource: createMusicSource(
						myncer_pb.Datasource_DATASOURCE_SPOTIFY,
						p.ID.String(),
					),
					Name:        p.Name,
					Description: p.Description,
					ImageUrl:    getBestSpotifyImageURL(p.Images),
				},
			)
		}

		if len(page.Playlists) < cPageLimit {
			// No more pages left to get.
			break
		}
	}

	return r, nil
}

func (s *spotifyClientImpl) AddToPlaylist(
	ctx context.Context,
	userInfo *myncer_pb.User, /*const*/
	playlistId string, /*const*/
	songs []core.Song, /*const*/
) error {
	client, err := s.getClient(ctx, userInfo)
	if err != nil {
		return core.WrappedError(err, "failed to get spotify client")
	}
	trackIds := []spotify.ID{}
	for _, song := range songs {
		trackIds = append(trackIds, spotify.ID(song.GetId()))
	}
	if _, err := client.AddTracksToPlaylist(ctx, spotify.ID(playlistId), trackIds...); err != nil {
		return core.WrappedError(err, "failed to add tracks to playlist %s", playlistId)
	}
	return nil
}

func (s *spotifyClientImpl) ClearPlaylist(
	ctx context.Context,
	userInfo *myncer_pb.User, /*const*/
	playlistId string, /*const*/
) error {
	client, err := s.getClient(ctx, userInfo)
	if err != nil {
		return core.WrappedError(err, "failed to get spotify client")
	}
	// Fetch all track URIs to remove
	playlistTracks, err := client.GetPlaylistItems(ctx, spotify.ID(playlistId))
	if err != nil {
		return core.WrappedError(err, "failed to fetch playlist items")
	}

	trackIDs := []spotify.ID{}
	for _, item := range playlistTracks.Items {
		if item.Track.Track != nil {
			trackIDs = append(trackIDs, item.Track.Track.ID)
		}
	}

	if len(trackIDs) == 0 {
		return nil
	}
	_, err = client.RemoveTracksFromPlaylist(ctx, spotify.ID(playlistId), trackIDs...)
	if err != nil {
		return core.WrappedError(err, "failed to clear playlist")
	}
	return nil
}

// buildSpotifyQueries builds a list of search strings from most specific to most general.
func buildSpotifyQueries(songToSearch core.Song) []string {
	queries := []string{}
	cleanTrack := matching.Clean(songToSearch.GetName())

	var cleanArtists []string
	for _, artist := range songToSearch.GetArtistNames() {
		cleanArtists = append(cleanArtists, matching.Clean(artist))
	}
	// For Spotify, it's often better to use only the first artist in complex searches.
	mainArtist := ""
	if len(cleanArtists) > 0 {
		mainArtist = cleanArtists[0]
	}

	cleanAlbum := matching.Clean(songToSearch.GetAlbum())

	// Most specific: Title + Main Artist + Album
	if cleanTrack != "" && mainArtist != "" && cleanAlbum != "" {
		queries = append(queries, fmt.Sprintf("track:\"%s\" artist:\"%s\" album:\"%s\"", cleanTrack, mainArtist, cleanAlbum))
	}
	// Title + Main Artist
	if cleanTrack != "" && mainArtist != "" {
		queries = append(queries, fmt.Sprintf("track:\"%s\" artist:\"%s\"", cleanTrack, mainArtist))
	}
	// Title + Album
	if cleanTrack != "" && cleanAlbum != "" {
		queries = append(queries, fmt.Sprintf("track:\"%s\" album:\"%s\"", cleanTrack, cleanAlbum))
	}
	// Only Title as last resort
	if cleanTrack != "" {
		queries = append(queries, fmt.Sprintf("track:\"%s\"", cleanTrack))
	}

	return queries
}

func (s *spotifyClientImpl) Search(
	ctx context.Context,
	userInfo *myncer_pb.User,
	names core.Set[string],
	artistNames core.Set[string],
	albumNames core.Set[string],
) (core.Song, error) {
	client, err := s.getClient(ctx, userInfo)
	if err != nil {
		return nil, core.WrappedError(err, "failed to get spotify client")
	}

	// Build a `core.Song` representation for the search.
	songToSearch := sync_engine.NewSong(&myncer_pb.Song{
		Name:       names.ToArray()[0], // Assuming a single name for simplicity
		ArtistName: artistNames.ToArray(),
		AlbumName:  albumNames.ToArray()[0], // Assuming a single album
	})

	// First, if the original song has an ISRC, use it for a high-precision search.
	if isrc := songToSearch.GetSpec().GetIsrc(); isrc != "" {
		query := fmt.Sprintf("isrc:%s", isrc)
		searchResult, err := client.Search(ctx, query, spotify.SearchTypeTrack, spotify.Limit(1))
		if err == nil && searchResult.Tracks != nil && len(searchResult.Tracks.Tracks) > 0 {
			return buildSongFromSpotifyTrack(ctx, &searchResult.Tracks.Tracks[0]), nil
		}
	}

	// If no ISRC or it fails, proceed with metadata search.
	queries := buildSpotifyQueries(songToSearch)
	var bestMatch core.Song
	highestScore := 0.0

	for _, query := range queries {
		searchResult, err := client.Search(ctx, query, spotify.SearchTypeTrack, spotify.Limit(5))
		if err != nil {
			core.Warningf("Spotify search failed for query %q, trying next. Error: %v", query, err)
			continue
		}

		if searchResult.Tracks != nil {
			for _, track := range searchResult.Tracks.Tracks {
				foundSong := buildSongFromSpotifyTrack(ctx, &track)
				score := matching.CalculateSimilarity(songToSearch, foundSong)

				if score > highestScore {
					highestScore = score
					bestMatch = foundSong
				}

				// If we find a nearly perfect match, we can stop early.
				if highestScore > 95.0 {
					return bestMatch, nil
				}
			}
		}
		// If we found a good candidate with a specific query, don't continue with more generic ones.
		if highestScore > 85.0 {
			break
		}
	}

	if bestMatch == nil {
		return nil, core.NewError("no suitable track found after trying all queries for: %s", songToSearch.GetName())
	}

	return bestMatch, nil
}

func (s *spotifyClientImpl) getClient(
	ctx context.Context,
	userInfo *myncer_pb.User, /*const*/
) (*spotify.Client, error) {
	oAuthToken, err := core.ToMyncerCtx(ctx).DB.DatasourceTokenStore.GetToken(
		ctx,
		userInfo.GetId(),
		myncer_pb.Datasource_DATASOURCE_SPOTIFY,
	)
	if err != nil {
		return nil, core.WrappedError(err, "failed to get spotify token for user %s", userInfo.GetId())
	}

	tokenSource := s.getOAuthConfig(ctx).TokenSource(ctx, core.ProtoOAuthTokenToOAuth2(oAuthToken))
	httpClient := oauth2.NewClient(ctx, tokenSource)
	return spotify.New(httpClient), nil
}

func (s *spotifyClientImpl) getAuthenticator(ctx context.Context) *spotifyauth.Authenticator {
	spotifyConfig := core.ToMyncerCtx(ctx).Config.SpotifyConfig
	return spotifyauth.New(
		spotifyauth.WithClientID(spotifyConfig.ClientId),
		spotifyauth.WithClientSecret(spotifyConfig.ClientSecret),
		spotifyauth.WithRedirectURL(spotifyConfig.RedirectUri),
	)
}

func (s *spotifyClientImpl) getOAuthConfig(ctx context.Context) *oauth2.Config {
	spotifyConfig := core.ToMyncerCtx(ctx).Config.SpotifyConfig
	return &oauth2.Config{
		ClientID:     spotifyConfig.ClientId,
		ClientSecret: spotifyConfig.ClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  cSpotifyAuthUrl,
			TokenURL: cSpotifyTokenUrl,
		},
		RedirectURL: spotifyConfig.RedirectUri,
	}
}

func buildSongFromSpotifyTrack(
	_ context.Context,
	track *spotify.FullTrack,
) core.Song {
	isrc, _ := track.ExternalIDs["isrc"]
	var artists []string
	for _, artist := range track.Artists {
		artists = append(artists, artist.Name)
	}

	return sync_engine.NewSong(
		&myncer_pb.Song{
			Name:             track.Name,
			ArtistName:       artists,
			AlbumName:        track.Album.Name,
			Datasource:       myncer_pb.Datasource_DATASOURCE_SPOTIFY,
			DatasourceSongId: track.ID.String(),
			Isrc:             isrc,
		},
	)
}

func filterEmpty(vals []string) (out []string) {
	for _, v := range vals {
		v = strings.TrimSpace(v)
		if v != "" {
			out = append(out, v)
		}
	}
	return
}
