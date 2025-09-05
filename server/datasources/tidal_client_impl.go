package datasources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/oauth2"

	"github.com/hansbala/myncer/core"
	"github.com/hansbala/myncer/matching"
	myncer_pb "github.com/hansbala/myncer/proto/myncer"
	"github.com/hansbala/myncer/sync_engine"
)

const (
	cTidalAuthURL    = "https://auth.tidal.com/v1/oauth2/authorize"
	cTidalTokenURL   = "https://auth.tidal.com/v1/oauth2/token"
	cTidalAPIBaseURL = "https://api.tidal.com/v1"
	cTidalPageLimit  = 50
)

// TidalTrack represents a Tidal track
type TidalTrack struct {
	ID                   int           `json:"id"`
	Title                string        `json:"title"`
	Duration             int           `json:"duration"`
	ReplayGain           float64       `json:"replayGain"`
	Peak                 float64       `json:"peak"`
	AllowStreaming       bool          `json:"allowStreaming"`
	StreamReady          bool          `json:"streamReady"`
	StreamStartDate      string        `json:"streamStartDate"`
	PremiumStreamingOnly bool          `json:"premiumStreamingOnly"`
	TrackNumber          int           `json:"trackNumber"`
	VolumeNumber         int           `json:"volumeNumber"`
	Version              string        `json:"version"`
	Popularity           int           `json:"popularity"`
	Copyright            string        `json:"copyright"`
	URL                  string        `json:"url"`
	ISRC                 string        `json:"isrc"`
	Editable             bool          `json:"editable"`
	Explicit             bool          `json:"explicit"`
	AudioQuality         string        `json:"audioQuality"`
	AudioModes           []string      `json:"audioModes"`
	Artist               TidalArtist   `json:"artist"`
	Artists              []TidalArtist `json:"artists"`
	Album                TidalAlbum    `json:"album"`
}

// TidalArtist represents a Tidal artist
type TidalArtist struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	URL  string `json:"url"`
}

// TidalAlbum represents a Tidal album
type TidalAlbum struct {
	ID                   int           `json:"id"`
	Title                string        `json:"title"`
	Duration             int           `json:"duration"`
	StreamReady          bool          `json:"streamReady"`
	StreamStartDate      string        `json:"streamStartDate"`
	AllowStreaming       bool          `json:"allowStreaming"`
	PremiumStreamingOnly bool          `json:"premiumStreamingOnly"`
	NumberOfTracks       int           `json:"numberOfTracks"`
	NumberOfVideos       int           `json:"numberOfVideos"`
	NumberOfVolumes      int           `json:"numberOfVolumes"`
	ReleaseDate          string        `json:"releaseDate"`
	Copyright            string        `json:"copyright"`
	Type                 string        `json:"type"`
	Version              string        `json:"version"`
	URL                  string        `json:"url"`
	Cover                string        `json:"cover"`
	VideoCover           string        `json:"videoCover"`
	Explicit             bool          `json:"explicit"`
	UPC                  string        `json:"upc"`
	Popularity           int           `json:"popularity"`
	AudioQuality         string        `json:"audioQuality"`
	AudioModes           []string      `json:"audioModes"`
	Artist               TidalArtist   `json:"artist"`
	Artists              []TidalArtist `json:"artists"`
}

// TidalPlaylist represents a Tidal playlist
type TidalPlaylist struct {
	UUID            string        `json:"uuid"`
	Title           string        `json:"title"`
	NumberOfTracks  int           `json:"numberOfTracks"`
	NumberOfVideos  int           `json:"numberOfVideos"`
	Creator         TidalUser     `json:"creator"`
	Description     string        `json:"description"`
	Duration        int           `json:"duration"`
	LastUpdated     string        `json:"lastUpdated"`
	Created         string        `json:"created"`
	Type            string        `json:"type"`
	PublicPlaylist  bool          `json:"publicPlaylist"`
	URL             string        `json:"url"`
	Image           string        `json:"image"`
	Popularity      int           `json:"popularity"`
	SquareImage     string        `json:"squareImage"`
	PromotedArtists []TidalArtist `json:"promotedArtists"`
	LastItemAddedAt string        `json:"lastItemAddedAt"`
}

// TidalUser represents a Tidal user
type TidalUser struct {
	ID          int    `json:"id"`
	Username    string `json:"username"`
	FirstName   string `json:"firstName"`
	LastName    string `json:"lastName"`
	Email       string `json:"email"`
	CountryCode string `json:"countryCode"`
	Created     string `json:"created"`
}

// TidalSearchResponse represents the Tidal search response
type TidalSearchResponse struct {
	Tracks TidalSearchTracks `json:"tracks"`
}

// TidalSearchTracks represents the tracks in a search response
type TidalSearchTracks struct {
	Limit              int          `json:"limit"`
	Offset             int          `json:"offset"`
	TotalNumberOfItems int          `json:"totalNumberOfItems"`
	Items              []TidalTrack `json:"items"`
}

// TidalPlaylistsResponse represents the response for playlists lists
type TidalPlaylistsResponse struct {
	Limit              int             `json:"limit"`
	Offset             int             `json:"offset"`
	TotalNumberOfItems int             `json:"totalNumberOfItems"`
	Items              []TidalPlaylist `json:"items"`
}

// TidalPlaylistTracksResponse represents the songs in a playlist
type TidalPlaylistTracksResponse struct {
	Limit              int          `json:"limit"`
	Offset             int          `json:"offset"`
	TotalNumberOfItems int          `json:"totalNumberOfItems"`
	Items              []TidalTrack `json:"items"`
}

func NewTidalClient() core.DatasourceClient {
	return &tidalClientImpl{}
}

type tidalClientImpl struct{}

var _ core.DatasourceClient = (*tidalClientImpl)(nil)

// getOAuthConfig creates the OAuth2 configuration for Tidal
func (c *tidalClientImpl) getOAuthConfig(ctx context.Context) *oauth2.Config {
	tidalCfg := core.ToMyncerCtx(ctx).Config.TidalConfig
	return &oauth2.Config{
		ClientID:     tidalCfg.ClientId,
		ClientSecret: tidalCfg.ClientSecret,
		RedirectURL:  tidalCfg.RedirectUri,
		Endpoint: oauth2.Endpoint{
			AuthURL:  cTidalAuthURL,
			TokenURL: cTidalTokenURL,
		},
		Scopes: []string{"r_usr", "w_usr", "w_sub"},
	}
}

// ExchangeCodeForToken exchanges the authorization code for a token
func (c *tidalClientImpl) ExchangeCodeForToken(ctx context.Context, authCode string) (*oauth2.Token, error) {
	conf := c.getOAuthConfig(ctx)
	token, err := conf.Exchange(ctx, authCode)
	if err != nil {
		return nil, core.WrappedError(err, "failed to exchange auth code with Tidal")
	}
	return token, nil
}

// GetPlaylists gets the user's playlists
func (c *tidalClientImpl) GetPlaylists(ctx context.Context, userInfo *myncer_pb.User) ([]*myncer_pb.Playlist, error) {
	client, err := c.getHTTPClient(ctx, userInfo)
	if err != nil {
		return nil, core.WrappedError(err, "failed to get Tidal HTTP client")
	}

	r := []*myncer_pb.Playlist{}
	offset := 0

	for {
		url := fmt.Sprintf("%s/users/%s/playlists?limit=%d&offset=%d",
			cTidalAPIBaseURL,
			userInfo.GetId(), // This might need the user's Tidal ID
			cTidalPageLimit,
			offset)

		resp, err := client.Get(url)
		if err != nil {
			return nil, core.WrappedError(err, "failed to get Tidal playlists at offset %d", offset)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, core.NewError("Tidal API returned status %d", resp.StatusCode)
		}

		var playlistsResp TidalPlaylistsResponse
		if err := json.NewDecoder(resp.Body).Decode(&playlistsResp); err != nil {
			return nil, core.WrappedError(err, "failed to decode Tidal playlists response")
		}

		for _, p := range playlistsResp.Items {
			r = append(r, &myncer_pb.Playlist{
				MusicSource: createMusicSource(
					myncer_pb.Datasource_DATASOURCE_TIDAL,
					p.UUID,
				),
				Name:        p.Title,
				Description: p.Description,
				ImageUrl:    p.Image,
			})
		}

		if len(playlistsResp.Items) < cTidalPageLimit {
			break
		}
		offset += cTidalPageLimit
	}

	return r, nil
}

// GetPlaylist gets a specific playlist
func (c *tidalClientImpl) GetPlaylist(ctx context.Context, userInfo *myncer_pb.User, playlistId string) (*myncer_pb.Playlist, error) {
	client, err := c.getHTTPClient(ctx, userInfo)
	if err != nil {
		return nil, core.WrappedError(err, "failed to get Tidal HTTP client")
	}

	url := fmt.Sprintf("%s/playlists/%s", cTidalAPIBaseURL, playlistId)
	resp, err := client.Get(url)
	if err != nil {
		return nil, core.WrappedError(err, "failed to get Tidal playlist %s", playlistId)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, core.NewError("Tidal API returned status %d for playlist %s", resp.StatusCode, playlistId)
	}

	var playlist TidalPlaylist
	if err := json.NewDecoder(resp.Body).Decode(&playlist); err != nil {
		return nil, core.WrappedError(err, "failed to decode Tidal playlist response")
	}

	return &myncer_pb.Playlist{
		MusicSource: createMusicSource(
			myncer_pb.Datasource_DATASOURCE_TIDAL,
			playlist.UUID,
		),
		Name:        playlist.Title,
		Description: playlist.Description,
		ImageUrl:    playlist.Image,
	}, nil
}

// GetPlaylistSongs gets the songs from a playlist
func (c *tidalClientImpl) GetPlaylistSongs(ctx context.Context, userInfo *myncer_pb.User, playlistId string) ([]core.Song, error) {
	client, err := c.getHTTPClient(ctx, userInfo)
	if err != nil {
		return nil, core.WrappedError(err, "failed to get Tidal HTTP client")
	}

	allSongs := []core.Song{}
	offset := 0

	for {
		url := fmt.Sprintf("%s/playlists/%s/tracks?limit=%d&offset=%d",
			cTidalAPIBaseURL,
			playlistId,
			cTidalPageLimit,
			offset)

		resp, err := client.Get(url)
		if err != nil {
			return nil, core.WrappedError(err, "failed to get Tidal playlist tracks at offset %d", offset)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, core.NewError("Tidal API returned status %d", resp.StatusCode)
		}

		var tracksResp TidalPlaylistTracksResponse
		if err := json.NewDecoder(resp.Body).Decode(&tracksResp); err != nil {
			return nil, core.WrappedError(err, "failed to decode Tidal playlist tracks response")
		}

		for _, track := range tracksResp.Items {
			allSongs = append(allSongs, buildSongFromTidalTrack(track))
		}

		if len(tracksResp.Items) < cTidalPageLimit {
			break
		}
		offset += cTidalPageLimit
	}

	return allSongs, nil
}

// AddToPlaylist adds songs to a playlist
func (c *tidalClientImpl) AddToPlaylist(ctx context.Context, userInfo *myncer_pb.User, playlistId string, songs []core.Song) error {
	client, err := c.getHTTPClient(ctx, userInfo)
	if err != nil {
		return core.WrappedError(err, "failed to get Tidal HTTP client")
	}

	trackIds := []string{}
	for _, song := range songs {
		trackIds = append(trackIds, song.GetId())
	}

	// Tidal API might require a specific format to add tracks
	data := url.Values{}
	data.Set("trackIds", strings.Join(trackIds, ","))

	url := fmt.Sprintf("%s/playlists/%s/tracks", cTidalAPIBaseURL, playlistId)
	resp, err := client.PostForm(url, data)
	if err != nil {
		return core.WrappedError(err, "failed to add tracks to Tidal playlist %s", playlistId)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return core.NewError("Tidal API returned status %d when adding tracks", resp.StatusCode)
	}

	return nil
}

// ClearPlaylist removes all songs from a playlist
func (c *tidalClientImpl) ClearPlaylist(ctx context.Context, userInfo *myncer_pb.User, playlistId string) error {
	// First, we get all the songs
	songs, err := c.GetPlaylistSongs(ctx, userInfo, playlistId)
	if err != nil {
		return core.WrappedError(err, "failed to get playlist songs for clearing")
	}

	if len(songs) == 0 {
		return nil
	}

	client, err := c.getHTTPClient(ctx, userInfo)
	if err != nil {
		return core.WrappedError(err, "failed to get Tidal HTTP client")
	}

	// We delete all the songs
	trackIds := []string{}
	for _, song := range songs {
		trackIds = append(trackIds, song.GetId())
	}

	data := url.Values{}
	data.Set("trackIds", strings.Join(trackIds, ","))

	url := fmt.Sprintf("%s/playlists/%s/tracks", cTidalAPIBaseURL, playlistId)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, strings.NewReader(data.Encode()))
	if err != nil {
		return core.WrappedError(err, "failed to create delete request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return core.WrappedError(err, "failed to clear Tidal playlist %s", playlistId)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return core.NewError("Tidal API returned status %d when clearing playlist", resp.StatusCode)
	}

	return nil
}

// buildTidalQueries builds search queries for Tidal
func buildTidalQueries(songToSearch core.Song) []string {
	queries := []string{}
	cleanTrack := matching.Clean(songToSearch.GetName())

	cleanArtists := []string{}
	for _, artist := range songToSearch.GetArtistNames() {
		cleanArtists = append(cleanArtists, matching.Clean(artist))
	}
	cleanArtist := strings.Join(cleanArtists, " ")

	cleanAlbum := matching.Clean(songToSearch.GetAlbum())

	// Queries from most specific to most general
	if cleanTrack != "" && cleanArtist != "" && cleanAlbum != "" {
		queries = append(queries, fmt.Sprintf("%s %s %s", cleanTrack, cleanArtist, cleanAlbum))
	}
	if cleanTrack != "" && cleanArtist != "" {
		queries = append(queries, fmt.Sprintf("%s %s", cleanTrack, cleanArtist))
	}
	if cleanTrack != "" && cleanAlbum != "" {
		queries = append(queries, fmt.Sprintf("%s %s", cleanTrack, cleanAlbum))
	}
	if cleanTrack != "" {
		queries = append(queries, cleanTrack)
	}

	return queries
}

// Search searches for a song on Tidal
func (c *tidalClientImpl) Search(
	ctx context.Context,
	userInfo *myncer_pb.User,
	names core.Set[string],
	artistNames core.Set[string],
	albumNames core.Set[string],
) (core.Song, error) {
	client, err := c.getHTTPClient(ctx, userInfo)
	if err != nil {
		return nil, core.WrappedError(err, "failed to get Tidal HTTP client")
	}

	// Build song for searching
	songToSearch := sync_engine.NewSong(&myncer_pb.Song{
		Name:       names.ToArray()[0],
		ArtistName: artistNames.ToArray(),
		AlbumName:  albumNames.ToArray()[0],
	})

	// First, try searching by ISRC if available
	if isrc := songToSearch.GetSpec().GetIsrc(); isrc != "" {
		searchURL := fmt.Sprintf("%s/search/tracks?query=isrc:%s&limit=1", cTidalAPIBaseURL, isrc)
		resp, err := client.Get(searchURL)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				var searchResp TidalSearchResponse
				if err := json.NewDecoder(resp.Body).Decode(&searchResp); err == nil {
					if len(searchResp.Tracks.Items) > 0 {
						return buildSongFromTidalTrack(searchResp.Tracks.Items[0]), nil
					}
				}
			}
		}
	}

	// Search by metadata
	queries := buildTidalQueries(songToSearch)
	var bestMatch core.Song
	highestScore := 0.0

	for _, query := range queries {
		searchURL := fmt.Sprintf("%s/search/tracks?query=%s&limit=5",
			cTidalAPIBaseURL,
			url.QueryEscape(query))

		resp, err := client.Get(searchURL)
		if err != nil {
			core.Warningf("Tidal search failed for query %q, trying next. Error: %v", query, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			core.Warningf("Tidal search returned status %d for query %q", resp.StatusCode, query)
			continue
		}

		var searchResp TidalSearchResponse
		if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
			core.Warningf("Failed to decode Tidal search response for query %q: %v", query, err)
			continue
		}

		for _, track := range searchResp.Tracks.Items {
			foundSong := buildSongFromTidalTrack(track)
			score := matching.CalculateSimilarity(songToSearch, foundSong)

			if score > highestScore {
				highestScore = score
				bestMatch = foundSong
			}

			// If we find an almost perfect match, we can stop
			if highestScore > 95.0 {
				return bestMatch, nil
			}
		}
	}

	if bestMatch == nil {
		return nil, core.NewError("no suitable track found after trying all queries for: %s", songToSearch.GetName())
	}

	return bestMatch, nil
}

// getHTTPClient gets an authenticated HTTP client for Tidal
func (c *tidalClientImpl) getHTTPClient(ctx context.Context, userInfo *myncer_pb.User) (*http.Client, error) {
	oAuthToken, err := core.ToMyncerCtx(ctx).DB.DatasourceTokenStore.GetToken(
		ctx,
		userInfo.GetId(),
		myncer_pb.Datasource_DATASOURCE_TIDAL,
	)
	if err != nil {
		return nil, core.WrappedError(err, "failed to get Tidal token for user %s", userInfo.GetId())
	}

	tokenSource := c.getOAuthConfig(ctx).TokenSource(ctx, core.ProtoOAuthTokenToOAuth2(oAuthToken))
	return oauth2.NewClient(ctx, tokenSource), nil
}

// buildSongFromTidalTrack converts a Tidal track to a core.Song
func buildSongFromTidalTrack(track TidalTrack) core.Song {
	artists := []string{}
	for _, artist := range track.Artists {
		artists = append(artists, artist.Name)
	}

	// If there are no artists in the array, use the main artist
	if len(artists) == 0 && track.Artist.Name != "" {
		artists = append(artists, track.Artist.Name)
	}

	return sync_engine.NewSong(&myncer_pb.Song{
		Name:             track.Title,
		ArtistName:       artists,
		AlbumName:        track.Album.Title,
		Datasource:       myncer_pb.Datasource_DATASOURCE_TIDAL,
		DatasourceSongId: strconv.Itoa(track.ID),
		Isrc:             track.ISRC,
	})
}
