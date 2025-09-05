package datasources

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/hansbala/myncer/core"
	"github.com/hansbala/myncer/matching"
	myncer_pb "github.com/hansbala/myncer/proto/myncer"
	"github.com/hansbala/myncer/sync_engine"
)

const (
	cTidalAuthURL      = "https://auth.tidal.com/v1/oauth2/authorize"
	cTidalTokenURL     = "https://auth.tidal.com/v1/oauth2/token"
	cTidalAPIBaseURL   = "https://openapi.tidal.com/v2"
	cTidalPageLimit    = 50
	cTidalAcceptHeader = "application/vnd.api+json"
)

// TidalResourceIdentifier is a JSON:API resource identifier
type TidalResourceIdentifier struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// TidalRelationship is a JSON:API relationship
type TidalRelationship struct {
	Data []TidalResourceIdentifier `json:"data"`
}

// TidalTrackAttributes contains the attributes of a track
type TidalTrackAttributes struct {
	Title   string          `json:"title"`
	ISRC    string          `json:"isrc"`
	Album   TidalV2Album    `json:"album"`
	Artists []TidalV2Artist `json:"artists"`
}

// TidalV2TrackResource is a track resource object
type TidalV2TrackResource struct {
	TidalResourceIdentifier
	Attributes    TidalTrackAttributes         `json:"attributes"`
	Relationships map[string]TidalRelationship `json:"relationships"`
}

// TidalV2Artist contains artist data
type TidalV2Artist struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// TidalV2Album contains album data
type TidalV2Album struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

// PlaylistAttributes contains the attributes of a playlist
type PlaylistAttributes struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	// Assuming there's an image-related field, though not explicit in the provided structs
	// If the API provides image URLs, they would be added here.
}

// PlaylistResource is a playlist resource object
type PlaylistResource struct {
	TidalResourceIdentifier
	Attributes PlaylistAttributes `json:"attributes"`
}

// PlaylistsV2Response is the response for a list of playlists
type PlaylistsV2Response struct {
	Data  []PlaylistResource `json:"data"`
	Links struct {
		Self string `json:"self"`
		Next string `json:"next,omitempty"`
	} `json:"links"`
}

// PlaylistItemsV2Response is the response for playlist items
type PlaylistItemsV2Response struct {
	Data     []PlaylistItemIdentifier `json:"data"`
	Included []TidalV2TrackResource   `json:"included"`
	Links    struct {
		Self string `json:"self"`
		Next string `json:"next,omitempty"`
	} `json:"links"`
}

// PlaylistItemIdentifier includes the crucial 'itemId' for deletion
type PlaylistItemIdentifier struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Meta struct {
		ItemID string `json:"itemId"`
	} `json:"meta"`
}

// TidalUserAttributes contains user attributes including country
type TidalUserAttributes struct {
	Country string `json:"country"`
}

// TidalMeData contains user data with attributes
type TidalMeData struct {
	ID         string              `json:"id"`
	Type       string              `json:"type"`
	Attributes TidalUserAttributes `json:"attributes"`
}

// TidalMeResponse is the response for /users/me
type TidalMeResponse struct {
	Data TidalMeData `json:"data"`
}

// SearchV2Response is the search response document
type SearchV2Response struct {
	Data     []TidalResourceIdentifier `json:"data"`
	Included []TidalV2TrackResource    `json:"included"`
	Links    struct {
		Self string `json:"self"`
		Next string `json:"next,omitempty"`
	} `json:"links"`
}

// SinglePlaylistV2Response is the response for getting a single playlist
type SinglePlaylistV2Response struct {
	Data PlaylistResource `json:"data"`
}

// TracksV2Response is the response for getting tracks
type TracksV2Response struct {
	Data     []TidalV2TrackResource `json:"data"`
	Included []TidalV2TrackResource `json:"included"`
}

// UserCollectionPlaylistsResponse is the response for user collection playlists relationship
type UserCollectionPlaylistsResponse struct {
	Data     []TidalResourceIdentifier `json:"data"`
	Included []PlaylistResource        `json:"included"`
	Links    struct {
		Self string `json:"self"`
		Next string `json:"next,omitempty"`
	} `json:"links"`
}

// Helper function to get the user ID and country code from Tidal
func getTidalUserInfo(ctx context.Context, client *http.Client) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/users/me", cTidalAPIBaseURL), nil)
	if err != nil {
		return "", "", core.WrappedError(err, "failed to create request for Tidal user info")
	}
	req.Header.Set("Accept", cTidalAcceptHeader)

	core.Printf("Tidal: Fetching user info from %s", req.URL)
	resp, err := client.Do(req)
	if err != nil {
		return "", "", core.WrappedError(err, "failed to get current user from Tidal")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", core.WrappedError(err, "failed to read response body from Tidal /users/me")
	}

	core.Printf("Tidal: Response from %s -> Status: %s", req.URL, resp.Status)

	if resp.StatusCode != http.StatusOK {
		core.Errorf("Tidal API Error for /users/me. Status: %s, Body: %s", resp.Status, string(body))
		return "", "", core.NewError("Tidal API returned status %d for /users/me. Body: %s", resp.StatusCode, string(body))
	}

	var userResponse TidalMeResponse
	if err := json.Unmarshal(body, &userResponse); err != nil {
		return "", "", core.WrappedError(err, "failed to decode Tidal user response")
	}

	if userResponse.Data.ID == "" {
		return "", "", core.NewError("Tidal user ID not found in response")
	}

	if userResponse.Data.Attributes.Country == "" {
		return "", "", core.NewError("Tidal user country code not found in response")
	}

	return userResponse.Data.ID, userResponse.Data.Attributes.Country, nil
}

func NewTidalClient() core.DatasourceClient {
	return &tidalClientImpl{}
}

type tidalClientImpl struct{}

var _ core.DatasourceClient = (*tidalClientImpl)(nil)

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
		Scopes: []string{"user.read", "playlists.read", "playlists.write", "collection.read", "search.read"},
	}
}

func (c *tidalClientImpl) ExchangeCodeForToken(ctx context.Context, authCode string, codeVerifier string) (*oauth2.Token, error) {
	conf := c.getOAuthConfig(ctx)
	opts := []oauth2.AuthCodeOption{
		oauth2.SetAuthURLParam("code_verifier", codeVerifier),
	}
	token, err := conf.Exchange(ctx, authCode, opts...)
	if err != nil {
		return nil, core.WrappedError(err, "failed to exchange auth code with Tidal")
	}
	return token, nil
}

func (c *tidalClientImpl) GetPlaylists(ctx context.Context, userInfo *myncer_pb.User) ([]*myncer_pb.Playlist, error) {
	client, err := c.getHTTPClient(ctx, userInfo)
	if err != nil {
		return nil, core.WrappedError(err, "failed to get Tidal HTTP client")
	}

	tidalUserID, countryCode, err := getTidalUserInfo(ctx, client)
	if err != nil {
		return nil, core.WrappedError(err, "failed to get Tidal user info")
	}

	// Use a map for deduplication
	playlistsMap := make(map[string]*myncer_pb.Playlist)

	// --- 1. Fetch playlists from user's collection (favorites/added) ---
	collectionNextURL := fmt.Sprintf("%s/userCollections/%s/relationships/playlists?countryCode=%s&include=playlists&limit=%d",
		cTidalAPIBaseURL, tidalUserID, countryCode, cTidalPageLimit)

	for collectionNextURL != "" {
		req, err := http.NewRequestWithContext(ctx, "GET", collectionNextURL, nil)
		if err != nil {
			return nil, core.WrappedError(err, "failed to create request for Tidal user collection playlists")
		}
		req.Header.Set("Accept", cTidalAcceptHeader)

		core.Printf("Tidal: Fetching user collection playlists for user %s from URL: %s", tidalUserID, collectionNextURL)
		resp, err := client.Do(req)
		if err != nil {
			return nil, core.WrappedError(err, "failed to get Tidal user collection playlists from URL: %s", collectionNextURL)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
			return nil, core.WrappedError(err, "failed to read response body from Tidal user collection playlists")
		}
		resp.Body.Close()

		core.Printf("Tidal: Response from %s -> Status: %s", collectionNextURL, resp.Status)

		if resp.StatusCode != http.StatusOK {
			core.Errorf("Tidal API Error for user collection playlists. Status: %s, Body: %s", resp.Status, string(body))
			// Continue to the next fetch type instead of failing completely
			break
		}

		var playlistsResp UserCollectionPlaylistsResponse
		if err := json.Unmarshal(body, &playlistsResp); err != nil {
			core.Errorf("Failed to decode Tidal user collection playlists response: %v. Body: %s", err, string(body))
			// Continue to the next fetch type
			break
		}

		for _, p := range playlistsResp.Included {
			if p.Type == "playlists" && playlistsMap[p.ID] == nil {
				playlistsMap[p.ID] = &myncer_pb.Playlist{
					MusicSource: createMusicSource(myncer_pb.Datasource_DATASOURCE_TIDAL, p.ID),
					Name:        p.Attributes.Name,
					Description: p.Attributes.Description,
				}
			}
		}

		if playlistsResp.Links.Next != "" {
			collectionNextURL = fmt.Sprintf("%s%s", "https://openapi.tidal.com", playlistsResp.Links.Next)
		} else {
			collectionNextURL = ""
		}
	}

	// --- 2. Fetch playlists owned by the user (created by them) ---
	ownedNextURL := fmt.Sprintf("%s/playlists?filter[owners.id]=%s&countryCode=%s&limit=%d",
		cTidalAPIBaseURL, tidalUserID, countryCode, cTidalPageLimit)

	for ownedNextURL != "" {
		req, err := http.NewRequestWithContext(ctx, "GET", ownedNextURL, nil)
		if err != nil {
			return nil, core.WrappedError(err, "failed to create request for Tidal owned playlists")
		}
		req.Header.Set("Accept", cTidalAcceptHeader)

		core.Printf("Tidal: Fetching owned playlists for user %s from URL: %s", tidalUserID, ownedNextURL)
		resp, err := client.Do(req)
		if err != nil {
			return nil, core.WrappedError(err, "failed to get Tidal owned playlists from URL: %s", ownedNextURL)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
			return nil, core.WrappedError(err, "failed to read response body from Tidal owned playlists")
		}
		resp.Body.Close()

		core.Printf("Tidal: Response from %s -> Status: %s", ownedNextURL, resp.Status)

		if resp.StatusCode != http.StatusOK {
			core.Errorf("Tidal API Error for owned playlists. Status: %s, Body: %s", resp.Status, string(body))
			// Break the loop on error but don't discard what we already have
			break
		}

		var playlistsResp PlaylistsV2Response
		if err := json.Unmarshal(body, &playlistsResp); err != nil {
			core.Errorf("Failed to decode Tidal owned playlists response: %v. Body: %s", err, string(body))
			break
		}

		for _, p := range playlistsResp.Data {
			if p.Type == "playlists" && playlistsMap[p.ID] == nil {
				playlistsMap[p.ID] = &myncer_pb.Playlist{
					MusicSource: createMusicSource(myncer_pb.Datasource_DATASOURCE_TIDAL, p.ID),
					Name:        p.Attributes.Name,
					Description: p.Attributes.Description,
				}
			}
		}

		if playlistsResp.Links.Next != "" {
			ownedNextURL = fmt.Sprintf("%s%s", "https://openapi.tidal.com", playlistsResp.Links.Next)
		} else {
			ownedNextURL = ""
		}
	}

	// Convert map back to slice
	var finalPlaylists []*myncer_pb.Playlist
	for _, p := range playlistsMap {
		finalPlaylists = append(finalPlaylists, p)
	}

	core.Printf("Tidal: Found a total of %d unique playlists for user %s.", len(finalPlaylists), tidalUserID)

	return finalPlaylists, nil
}

func (c *tidalClientImpl) GetPlaylist(ctx context.Context, userInfo *myncer_pb.User, playlistId string) (*myncer_pb.Playlist, error) {
	client, err := c.getHTTPClient(ctx, userInfo)
	if err != nil {
		return nil, core.WrappedError(err, "failed to get Tidal HTTP client")
	}

	_, countryCode, err := getTidalUserInfo(ctx, client)
	if err != nil {
		return nil, core.WrappedError(err, "failed to get Tidal user info")
	}

	url := fmt.Sprintf("%s/playlists/%s?countryCode=%s", cTidalAPIBaseURL, playlistId, countryCode)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, core.WrappedError(err, "failed to create request for Tidal playlist")
	}
	req.Header.Set("Accept", cTidalAcceptHeader)

	core.Printf("Tidal: Fetching playlist details for playlist %s", playlistId)
	resp, err := client.Do(req)
	if err != nil {
		return nil, core.WrappedError(err, "failed to get Tidal playlist %s", playlistId)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, core.WrappedError(err, "failed to read response body from Tidal playlist details")
	}

	core.Printf("Tidal: Response from %s -> Status: %s", url, resp.Status)

	if resp.StatusCode != http.StatusOK {
		core.Errorf("Tidal API Error for playlist %s. Status: %s, Body: %s", playlistId, resp.Status, string(body))
		return nil, core.NewError("Tidal API returned status %d for playlist %s. Body: %s", resp.StatusCode, playlistId, string(body))
	}

	var playlistResp SinglePlaylistV2Response
	if err := json.Unmarshal(body, &playlistResp); err != nil {
		return nil, core.WrappedError(err, "failed to decode single Tidal playlist response")
	}

	p := playlistResp.Data
	return &myncer_pb.Playlist{
		MusicSource: createMusicSource(myncer_pb.Datasource_DATASOURCE_TIDAL, p.ID),
		Name:        p.Attributes.Name,
		Description: p.Attributes.Description,
	}, nil
}

func (c *tidalClientImpl) GetPlaylistSongs(ctx context.Context, userInfo *myncer_pb.User, playlistId string) ([]core.Song, error) {
	client, err := c.getHTTPClient(ctx, userInfo)
	if err != nil {
		return nil, core.WrappedError(err, "failed to get Tidal HTTP client")
	}

	var allSongs []core.Song
	_, countryCode, err := getTidalUserInfo(ctx, client)
	if err != nil {
		return nil, core.WrappedError(err, "failed to get Tidal user info")
	}

	nextURL := fmt.Sprintf("%s/playlists/%s/relationships/items?countryCode=%s&include=items&limit=%d",
		cTidalAPIBaseURL,
		playlistId,
		countryCode,
		cTidalPageLimit)

	for nextURL != "" {
		req, err := http.NewRequestWithContext(ctx, "GET", nextURL, nil)
		if err != nil {
			return nil, core.WrappedError(err, "failed to create request for Tidal playlist items")
		}
		req.Header.Set("Accept", cTidalAcceptHeader)

		core.Printf("Tidal: Fetching songs for playlist %s from URL: %s", playlistId, nextURL)
		resp, err := client.Do(req)
		if err != nil {
			return nil, core.WrappedError(err, "failed to get Tidal playlist items from URL: %s", nextURL)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
			return nil, core.WrappedError(err, "failed to read response body from Tidal playlist items")
		}
		resp.Body.Close()

		core.Printf("Tidal: Response from %s -> Status: %s", nextURL, resp.Status)

		if resp.StatusCode != http.StatusOK {
			core.Errorf("Tidal API Error for playlist items. Status: %s, Body: %s", resp.Status, string(body))
			return nil, core.NewError("Tidal API returned status %d for playlist items. Body: %s", resp.StatusCode, string(body))
		}

		var itemsResp PlaylistItemsV2Response
		if err := json.Unmarshal(body, &itemsResp); err != nil {
			return nil, core.WrappedError(err, "failed to decode Tidal v2 playlist items response")
		}

		for _, trackResource := range itemsResp.Included {
			if trackResource.Type == "tracks" {
				allSongs = append(allSongs, buildSongFromTidalV2Track(trackResource))
			}
		}

		if itemsResp.Links.Next != "" {
			nextURL = fmt.Sprintf("%s%s", "https://openapi.tidal.com", itemsResp.Links.Next)
		} else {
			nextURL = ""
		}
	}
	return allSongs, nil
}

func (c *tidalClientImpl) AddToPlaylist(ctx context.Context, userInfo *myncer_pb.User, playlistId string, songs []core.Song) error {
	client, err := c.getHTTPClient(ctx, userInfo)
	if err != nil {
		return core.WrappedError(err, "failed to get Tidal HTTP client")
	}

	_, countryCode, err := getTidalUserInfo(ctx, client)
	if err != nil {
		return core.WrappedError(err, "failed to get Tidal user info")
	}

	var resourceIdentifiers []TidalResourceIdentifier
	for _, song := range songs {
		resourceIdentifiers = append(resourceIdentifiers, TidalResourceIdentifier{ID: song.GetId(), Type: "tracks"})
	}

	// The API adds items in batches of max 20.
	for i := 0; i < len(resourceIdentifiers); i += 20 {
		end := i + 20
		if end > len(resourceIdentifiers) {
			end = len(resourceIdentifiers)
		}
		batch := resourceIdentifiers[i:end]

		payload := map[string][]TidalResourceIdentifier{"data": batch}
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return core.WrappedError(err, "failed to marshal add tracks payload")
		}

		url := fmt.Sprintf("%s/playlists/%s/relationships/items?countryCode=%s", cTidalAPIBaseURL, playlistId, countryCode)
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(payloadBytes))
		if err != nil {
			return core.WrappedError(err, "failed to create add tracks request")
		}
		req.Header.Set("Content-Type", "application/vnd.api+json")
		req.Header.Set("Accept", cTidalAcceptHeader)

		core.Printf("Tidal: Adding %d tracks to playlist %s", len(batch), playlistId)
		resp, err := client.Do(req)
		if err != nil {
			return core.WrappedError(err, "failed to add tracks to Tidal playlist %s", playlistId)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return core.WrappedError(err, "failed to read response body when adding tracks")
		}

		core.Printf("Tidal: Response from POST %s -> Status: %s", url, resp.Status)

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			core.Errorf("Tidal API Error adding tracks. Status: %s, Body: %s", resp.Status, string(body))
			return core.NewError("Tidal API returned status %d when adding tracks. Body: %s", resp.StatusCode, string(body))
		}
	}

	return nil
}

func (c *tidalClientImpl) ClearPlaylist(ctx context.Context, userInfo *myncer_pb.User, playlistId string) error {
	client, err := c.getHTTPClient(ctx, userInfo)
	if err != nil {
		return core.WrappedError(err, "failed to get Tidal HTTP client")
	}

	// Fetch all item identifiers with their unique itemId for deletion.
	var itemsToRemove []PlaylistItemIdentifier
	nextURL := fmt.Sprintf("%s/playlists/%s/relationships/items", cTidalAPIBaseURL, playlistId)

	for nextURL != "" {
		req, err := http.NewRequestWithContext(ctx, "GET", nextURL, nil)
		if err != nil {
			return core.WrappedError(err, "failed to create request to get playlist items for deletion")
		}
		req.Header.Set("Accept", cTidalAcceptHeader)

		core.Printf("Tidal: Fetching items to clear from playlist %s", playlistId)
		resp, err := client.Do(req)
		if err != nil {
			return core.WrappedError(err, "failed to get playlist items for deletion")
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
			return core.WrappedError(err, "failed to read body of playlist items for deletion")
		}
		resp.Body.Close()

		core.Printf("Tidal: Response from %s -> Status: %s", nextURL, resp.Status)

		if resp.StatusCode != http.StatusOK {
			core.Errorf("Tidal API Error getting items to clear. Status: %s, Body: %s", resp.Status, string(body))
			return core.NewError("Tidal API returned status %d getting items to clear. Body: %s", resp.StatusCode, string(body))
		}

		var itemsResp PlaylistItemsV2Response
		if err := json.Unmarshal(body, &itemsResp); err != nil {
			return core.WrappedError(err, "failed to decode playlist items for deletion")
		}

		itemsToRemove = append(itemsToRemove, itemsResp.Data...)

		if itemsResp.Links.Next != "" {
			nextURL = fmt.Sprintf("%s%s", "https://openapi.tidal.com", itemsResp.Links.Next)
		} else {
			nextURL = ""
		}
	}

	if len(itemsToRemove) == 0 {
		core.Printf("Tidal: Playlist %s is already empty. Nothing to clear.", playlistId)
		return nil // Nothing to clear
	}

	// Delete items in batches of 20
	for i := 0; i < len(itemsToRemove); i += 20 {
		end := i + 20
		if end > len(itemsToRemove) {
			end = len(itemsToRemove)
		}
		batch := itemsToRemove[i:end]

		payload := map[string][]PlaylistItemIdentifier{"data": batch}
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return core.WrappedError(err, "failed to marshal delete payload")
		}

		deleteURL := fmt.Sprintf("%s/playlists/%s/relationships/items", cTidalAPIBaseURL, playlistId)
		req, err := http.NewRequestWithContext(ctx, "DELETE", deleteURL, bytes.NewBuffer(payloadBytes))
		if err != nil {
			return core.WrappedError(err, "failed to create delete request")
		}
		req.Header.Set("Content-Type", "application/vnd.api+json")
		req.Header.Set("Accept", cTidalAcceptHeader)

		core.Printf("Tidal: Clearing %d tracks from playlist %s", len(batch), playlistId)
		resp, err := client.Do(req)
		if err != nil {
			return core.WrappedError(err, "failed to clear batch from playlist")
		}
		defer resp.Body.Close()

		core.Printf("Tidal: Response from DELETE %s -> Status: %s", deleteURL, resp.Status)

		if resp.StatusCode != http.StatusNoContent {
			body, _ := io.ReadAll(resp.Body)
			core.Errorf("Tidal API Error when clearing playlist. Status: %s, Body: %s", resp.Status, string(body))
			return core.NewError("Tidal API returned status %d when clearing playlist. Body: %s", resp.StatusCode, string(body))
		}
	}

	return nil
}

// buildTidalQueries constructs a list of search queries from most to least specific.
func buildTidalQueries(songToSearch core.Song) []string {
	queries := []string{}
	seen := make(map[string]bool)

	addQuery := func(q string) {
		if q != "" && !seen[q] {
			queries = append(queries, q)
			seen[q] = true
		}
	}

	// Raw metadata
	rawTrack := songToSearch.GetName()
	rawArtists := strings.Join(songToSearch.GetArtistNames(), " ")
	rawAlbum := songToSearch.GetAlbum()

	// Cleaned metadata for fallback
	cleanTrack := matching.Clean(rawTrack)
	cleanArtists := matching.Clean(rawArtists)
	cleanAlbum := matching.Clean(rawAlbum)

	// Phase 1: Raw, specific queries
	if rawTrack != "" && rawArtists != "" && rawAlbum != "" {
		addQuery(fmt.Sprintf("%s %s %s", rawArtists, rawTrack, rawAlbum))
	}
	if rawTrack != "" && rawArtists != "" {
		addQuery(fmt.Sprintf("%s %s", rawArtists, rawTrack))
	}

	// Phase 2: Cleaned, specific queries
	if cleanTrack != "" && cleanArtists != "" && cleanAlbum != "" {
		addQuery(fmt.Sprintf("%s %s %s", cleanArtists, cleanTrack, cleanAlbum))
	}
	if cleanTrack != "" && cleanArtists != "" {
		addQuery(fmt.Sprintf("%s %s", cleanArtists, cleanTrack))
	}

	// Phase 3: More generic queries
	if cleanTrack != "" && cleanAlbum != "" {
		addQuery(fmt.Sprintf("%s %s", cleanTrack, cleanAlbum))
	}
	if cleanTrack != "" {
		addQuery(cleanTrack)
	}

	return queries
}

func (c *tidalClientImpl) Search(ctx context.Context, userInfo *myncer_pb.User, names core.Set[string], artistNames core.Set[string], albumNames core.Set[string]) (core.Song, error) {
	client, err := c.getHTTPClient(ctx, userInfo)
	if err != nil {
		return nil, core.WrappedError(err, "failed to get Tidal HTTP client")
	}

	_, countryCode, err := getTidalUserInfo(ctx, client)
	if err != nil {
		return nil, core.WrappedError(err, "failed to get Tidal user info")
	}

	songToSearch := sync_engine.NewSong(&myncer_pb.Song{
		Name:       names.ToArray()[0],
		ArtistName: artistNames.ToArray(),
		AlbumName:  albumNames.ToArray()[0],
	})

	// 1. Try searching by ISRC first, as it's the most accurate
	if isrc := songToSearch.GetSpec().GetIsrc(); isrc != "" {
		core.Printf("Tidal: Searching song '%s' for track by ISRC %s", songToSearch.GetName(), isrc)
		isrcURL := fmt.Sprintf("%s/tracks?filter[isrc]=%s&countryCode=%s&include=albums,artists", cTidalAPIBaseURL, isrc, countryCode)
		req, _ := http.NewRequestWithContext(ctx, "GET", isrcURL, nil)
		req.Header.Set("Accept", cTidalAcceptHeader)

		resp, err := client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			body, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr == nil {
				var tracksResp TracksV2Response
				if json.Unmarshal(body, &tracksResp) == nil && len(tracksResp.Data) > 0 {
					core.Printf("Tidal: Found track by ISRC %s", isrc)
					return buildSongFromTidalV2Track(tracksResp.Data[0]), nil
				}
			}
		}
		if resp != nil {
			resp.Body.Close()
		}
	} else {
		core.Printf("Tidal: No ISRC found for song '%s'. Proceeding with metadata search.", songToSearch.GetName())
	}

	// 2. Fallback to metadata search
	queries := buildTidalQueries(songToSearch)
	var bestMatch core.Song
	highestScore := 0.0

	for _, query := range queries {
		time.Sleep(250 * time.Millisecond)

		searchURL := fmt.Sprintf("%s/searchResults/%s/relationships/tracks?countryCode=%s&include=tracks&limit=5",
			cTidalAPIBaseURL, url.QueryEscape(query), countryCode)

		req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
		if err != nil {
			core.Warningf("Failed to create Tidal search request for query %q: %v", query, err)
			continue
		}
		req.Header.Set("Accept", cTidalAcceptHeader)

		core.Printf("Tidal: Searching for track with query: %s", query)
		resp, err := client.Do(req)
		if err != nil {
			core.Warningf("Tidal search failed for query %q, trying next. Error: %v", query, err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			core.Warningf("Tidal search returned status %d for query %q. Body: %s", resp.StatusCode, query, string(body))
			resp.Body.Close()
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			core.Warningf("Failed to read response body for Tidal search query %q: %v", query, err)
			continue
		}

		var searchResp SearchV2Response
		if err := json.Unmarshal(body, &searchResp); err != nil {
			core.Warningf("Failed to decode Tidal search response for query %q: %v. Body: %s", query, err, string(body))
			continue
		}

		for _, trackResource := range searchResp.Included {
			if trackResource.Type == "tracks" {
				foundSong := buildSongFromTidalV2Track(trackResource)
				score := matching.CalculateSimilarity(songToSearch, foundSong)

				if score > highestScore {
					highestScore = score
					bestMatch = foundSong
				}
				if highestScore > 95.0 {
					return bestMatch, nil
				}
			}
		}

		if highestScore > 85.0 {
			break
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

// buildSongFromTidalV2Track converts a v2 track resource to core.Song
func buildSongFromTidalV2Track(trackResource TidalV2TrackResource) core.Song {
	artists := []string{}
	for _, artist := range trackResource.Attributes.Artists {
		artists = append(artists, artist.Name)
	}
	// The track ID from the API can be a string, but our internal representation for other datasources might be an int.
	// We'll keep it as a string as per the JSON:API spec.
	trackID := trackResource.ID

	return sync_engine.NewSong(&myncer_pb.Song{
		Name:             trackResource.Attributes.Title,
		ArtistName:       artists,
		AlbumName:        trackResource.Attributes.Album.Title,
		Datasource:       myncer_pb.Datasource_DATASOURCE_TIDAL,
		DatasourceSongId: trackID,
		Isrc:             trackResource.Attributes.ISRC,
	})
}
