package datasources

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	youtube "google.golang.org/api/youtube/v3"

	"github.com/hansbala/myncer/core"
	"github.com/hansbala/myncer/matching"
	myncer_pb "github.com/hansbala/myncer/proto/myncer"
	"github.com/hansbala/myncer/sync_engine"
)

const (
	cYouTubeAuthURL  = "https://accounts.google.com/o/oauth2/auth"
	cYouTubeTokenURL = "https://oauth2.googleapis.com/token"
)

// Regex to find common artist separators in YouTube titles.
var artistSeparators = regexp.MustCompile(`\s*[,&]\s*|\s+(?:feat|ft)\.?\s+`)

func NewYouTubeClient() core.DatasourceClient {
	return &youtubeClientImpl{}
}

type youtubeClientImpl struct{}

var _ core.DatasourceClient = (*youtubeClientImpl)(nil)

func (c *youtubeClientImpl) ExchangeCodeForToken(
	ctx context.Context,
	code string,
) (*oauth2.Token, error) {
	conf := c.getOAuthConfig(ctx)
	tok, err := conf.Exchange(ctx, code)
	if err != nil {
		return nil, core.WrappedError(err, "failed to exchange auth code with YouTube")
	}
	return tok, nil
}

func (c *youtubeClientImpl) GetPlaylists(
	ctx context.Context,
	userInfo *myncer_pb.User, /*const*/
) ([]*myncer_pb.Playlist, error) {
	svc, err := c.getService(ctx, userInfo)
	if err != nil {
		return nil, core.WrappedError(err, "failed to get YouTube service")
	}

	call := svc.Playlists.List([]string{"snippet"}).Mine(true).MaxResults(50)
	resp, err := call.Do()
	if err != nil {
		return nil, core.WrappedError(err, "failed to fetch playlists")
	}

	var playlists []*myncer_pb.Playlist
	for _, p := range resp.Items {
		playlists = append(
			playlists,
			&myncer_pb.Playlist{
				MusicSource: createMusicSource(
					myncer_pb.Datasource_DATASOURCE_YOUTUBE,
					p.Id,
				),
				Name:        p.Snippet.Title,
				Description: p.Snippet.Description,
				ImageUrl:    getBestThumbnailUrl(p.Snippet.Thumbnails),
			},
		)
	}
	return playlists, nil
}

func (c *youtubeClientImpl) GetPlaylist(
	ctx context.Context,
	userInfo *myncer_pb.User, /*const*/
	id string,
) (*myncer_pb.Playlist, error) {
	svc, err := c.getService(ctx, userInfo)
	if err != nil {
		return nil, core.WrappedError(err, "failed to get YouTube service")
	}
	call := svc.Playlists.List([]string{"snippet"}).Id(id)
	resp, err := call.Do()
	if err != nil || len(resp.Items) == 0 {
		return nil, core.WrappedError(err, "failed to fetch playlist %s", id)
	}

	p := resp.Items[0]
	return &myncer_pb.Playlist{
		MusicSource: createMusicSource(myncer_pb.Datasource_DATASOURCE_YOUTUBE, p.Id),
		Name:        p.Snippet.Title,
		Description: p.Snippet.Description,
		ImageUrl:    getBestThumbnailUrl(p.Snippet.Thumbnails),
	}, nil
}

func (c *youtubeClientImpl) GetPlaylistSongs(
	ctx context.Context,
	userInfo *myncer_pb.User, /*const*/
	playlistId string,
) ([]core.Song, error) {
	svc, err := c.getService(ctx, userInfo)
	if err != nil {
		return nil, core.WrappedError(err, "failed to get YouTube service")
	}

	songs := []core.Song{}
	nextPageToken := ""
	for {
		call := svc.PlaylistItems.
			List([]string{"snippet"}).
			PlaylistId(playlistId).
			MaxResults(50).
			PageToken(nextPageToken)
		resp, err := call.Do()
		if err != nil {
			return nil, core.WrappedError(err, "failed to fetch playlist items")
		}

		for _, item := range resp.Items {
			videoId := item.Snippet.ResourceId.VideoId
			if len(videoId) == 0 {
				continue
			}
			songs = append(songs, buildSongFromYouTubePlaylistItem(item))
		}
		if resp.NextPageToken == "" {
			break
		}
		nextPageToken = resp.NextPageToken
	}

	return songs, nil
}

func (c *youtubeClientImpl) AddToPlaylist(
	ctx context.Context,
	userInfo *myncer_pb.User, /*const*/
	playlistId string,
	songs []core.Song,
) error {
	svc, err := c.getService(ctx, userInfo)
	if err != nil {
		return core.WrappedError(err, "failed to get YouTube service")
	}

	for _, song := range songs {
		if _, err := svc.PlaylistItems.Insert(
			[]string{"snippet"},
			&youtube.PlaylistItem{
				Snippet: &youtube.PlaylistItemSnippet{
					PlaylistId: playlistId,
					ResourceId: &youtube.ResourceId{
						Kind:    "youtube#video",
						VideoId: song.GetId(),
					},
				},
			},
		).
			Do(); err != nil {
			return core.WrappedError(err, "failed to insert video %s", song.GetName())
		}
	}
	return nil
}

func (c *youtubeClientImpl) ClearPlaylist(
	ctx context.Context,
	userInfo *myncer_pb.User, /*const*/
	playlistId string,
) error {
	svc, err := c.getService(ctx, userInfo)
	if err != nil {
		return core.WrappedError(err, "failed to get YouTube service")
	}

	var nextPageToken string
	for {
		resp, err := svc.PlaylistItems.
			List([]string{"id"}).
			PlaylistId(playlistId).
			MaxResults(50).
			PageToken(nextPageToken).
			Do()
		if err != nil {
			return core.WrappedError(err, "failed to list playlist items")
		}

		for _, item := range resp.Items {
			if err := svc.PlaylistItems.Delete(item.Id).Do(); err != nil {
				return core.WrappedError(err, "failed to delete playlist item %s", item.Id)
			}
		}

		if resp.NextPageToken == "" {
			break
		}
		nextPageToken = resp.NextPageToken
	}

	return nil
}

// buildYouTubeQueries builds a list of search strings from most specific to most general.
func buildYouTubeQueries(songToSearch core.Song) []string {
	queries := []string{}
	cleanTrack := matching.Clean(songToSearch.GetName())

	cleanArtists := []string{}
	for _, artist := range songToSearch.GetArtistNames() {
		cleanArtists = append(cleanArtists, matching.Clean(artist))
	}
	cleanArtist := strings.Join(cleanArtists, " ")

	cleanAlbum := matching.Clean(songToSearch.GetAlbum())

	// For YouTube, it's better to use more natural queries as there are no specific operators like "track:"
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

func (s *youtubeClientImpl) Search(
	ctx context.Context,
	userInfo *myncer_pb.User,
	names core.Set[string],
	artistNames core.Set[string],
	albumNames core.Set[string],
) (core.Song, error) {
	svc, err := s.getService(ctx, userInfo)
	if err != nil {
		return nil, core.WrappedError(err, "failed to get YouTube service")
	}

	// Build a `core.Song` representation for the search.
	songToSearch := sync_engine.NewSong(&myncer_pb.Song{
		Name:       names.ToArray()[0], // Assuming a single name for simplicity
		ArtistName: artistNames.ToArray(),
		AlbumName:  albumNames.ToArray()[0], // Assuming a single album
	})

	// Search by metadata using multiple queries
	queries := buildYouTubeQueries(songToSearch)
	var bestMatch core.Song
	highestScore := 0.0

	for _, query := range queries {
		call := svc.Search.List([]string{"snippet"}).
			Q(query).
			Type("video").
			MaxResults(5) // We search for more results to compare

		resp, err := call.Do()
		if err != nil {
			core.Warningf("YouTube search failed for query %q, trying next. Error: %v", query, err)
			continue
		}

		if len(resp.Items) == 0 {
			core.Warningf("No results found for YouTube query %q", query)
			continue
		}

		for _, item := range resp.Items {
			foundSong, err := buildSongFormYoutubeSearchResultItem(item)
			if err != nil {
				core.Warningf("Failed to build song from YouTube result: %v", err)
				continue
			}

			score := matching.CalculateSimilarity(songToSearch, foundSong)

			if score > highestScore {
				highestScore = score
				bestMatch = foundSong
			}

			// If we find a nearly perfect match, we can stop.
			if highestScore > 95.0 {
				return bestMatch, nil
			}
		}
	}

	if bestMatch == nil {
		return nil, core.NewError("no suitable video found after trying all queries for: %s", songToSearch.GetName())
	}

	return bestMatch, nil
}

func (c *youtubeClientImpl) getService(
	ctx context.Context,
	userInfo *myncer_pb.User, /*const*/
) (*youtube.Service, error) {
	oAuthToken, err := core.ToMyncerCtx(ctx).DB.DatasourceTokenStore.GetToken(
		ctx,
		userInfo.GetId(),
		myncer_pb.Datasource_DATASOURCE_YOUTUBE,
	)
	if err != nil {
		return nil, core.WrappedError(err, "failed to get youtube token for user %s", userInfo.GetId())
	}
	httpClient := oauth2.NewClient(
		ctx,
		c.getOAuthConfig(ctx).TokenSource(ctx, core.ProtoOAuthTokenToOAuth2(oAuthToken)),
	)
	svc, err := youtube.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, core.WrappedError(err, "failed to create YouTube service")
	}
	return svc, nil
}

func (c *youtubeClientImpl) getOAuthConfig(ctx context.Context) *oauth2.Config {
	youtubeCfg := core.ToMyncerCtx(ctx).Config.YoutubeConfig
	return &oauth2.Config{
		ClientID:     youtubeCfg.ClientId,
		ClientSecret: youtubeCfg.ClientSecret,
		RedirectURL:  youtubeCfg.RedirectUri,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/auth",
			TokenURL: "https://oauth2.googleapis.com/token",
		},
	}
}

// parseArtistsFromYouTubeTitle attempts to extract artist names from a video title.
// YouTube Music titles often follow patterns like "Artist - Song" or "Song (feat. Artist)".
func parseArtistsFromYouTubeTitle(title, channelTitle string) (string, []string) {
	// We use the general metadata cleaner first, but keep the original for splitting.
	cleanedTitleForMatching := matching.Clean(title)

	// Common case: "Artist - Title"
	parts := strings.SplitN(title, " - ", 2)
	if len(parts) == 2 {
		songTitle := strings.TrimSpace(parts[1])
		// The first part may contain multiple artists.
		artistsStr := strings.TrimSpace(parts[0])
		artists := artistSeparators.Split(artistsStr, -1)

		// Clean and remove duplicates
		cleanedArtists := core.NewSet[string]()
		for _, artist := range artists {
			if a := strings.TrimSpace(artist); a != "" {
				cleanedArtists.Add(a)
			}
		}

		if !cleanedArtists.IsEmpty() {
			return songTitle, cleanedArtists.ToArray()
		}
	}

	// If there's no clear separator, use the cleaned title as the song name
	// and the channel name as the artist (fallback).
	// We remove "- Topic", which YouTube adds to many artist channels.
	artistFallback := strings.TrimSuffix(channelTitle, " - Topic")
	return cleanedTitleForMatching, []string{artistFallback}
}

func buildSongFromYouTubePlaylistItem(
	pi *youtube.PlaylistItem, /*const*/
) core.Song {
	cleanTitle, artists := parseArtistsFromYouTubeTitle(pi.Snippet.Title, pi.Snippet.ChannelTitle)

	return sync_engine.NewSong(
		&myncer_pb.Song{
			Name:             cleanTitle,
			ArtistName:       artists,
			Datasource:       myncer_pb.Datasource_DATASOURCE_YOUTUBE,
			DatasourceSongId: pi.Snippet.ResourceId.VideoId, // Use the VideoId as the ID
		},
	)
}

func buildSongFormYoutubeSearchResultItem(
	item *youtube.SearchResult, /*const*/
) (core.Song, error) {
	videoId := ""
	if item.Id != nil && item.Id.VideoId != "" {
		videoId = item.Id.VideoId
	} else {
		return nil, core.NewError("missing video ID in YouTube search result")
	}

	cleanTitle, artists := parseArtistsFromYouTubeTitle(item.Snippet.Title, item.Snippet.ChannelTitle)

	return sync_engine.NewSong(
		&myncer_pb.Song{
			Name:             cleanTitle,
			ArtistName:       artists,
			Datasource:       myncer_pb.Datasource_DATASOURCE_YOUTUBE,
			DatasourceSongId: videoId,
		},
	), nil
}

// Helper to get the first available thumbnail URL from the YouTube API response.
// Prefers higher resolution thumbnails if available.
func getBestThumbnailUrl(thumbnails *youtube.ThumbnailDetails /*const*/) string {
	switch {
	case thumbnails.Maxres != nil:
		return thumbnails.Maxres.Url
	case thumbnails.Standard != nil:
		return thumbnails.Standard.Url
	case thumbnails.High != nil:
		return thumbnails.High.Url
	case thumbnails.Medium != nil:
		return thumbnails.Medium.Url
	case thumbnails.Default != nil:
		return thumbnails.Default.Url
	default:
		return ""
	}
}
