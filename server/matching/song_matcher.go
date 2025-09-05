package matching

import (
	"strings"

	"github.com/hansbala/myncer/core"
	"github.com/lithammer/fuzzywuzzy"
)

// CalculateSimilarity calculates a weighted similarity score between two songs.
// It prioritizes an exact ISRC match and falls back to a weighted fuzzy match
// on cleaned metadata if no ISRC is available.
func CalculateSimilarity(songA, songB core.Song) float64 {
	// 1. Exact identifier check (ISRC). If it matches, it's 100% the same song.
	isrcA := songA.GetSpec().GetIsrc()
	isrcB := songB.GetSpec().GetIsrc()
	if isrcA != "" && isrcA == isrcB {
		return 100.0
	}

	// 2. Weighted fuzzy matching on clean metadata.
	// Ratio is used for strings where the complete order is important (title, album).
	titleScore := float64(fuzzywuzzy.Ratio(
		Clean(songA.GetName()),
		Clean(songB.GetName()),
	))

	artistA := strings.Join(songA.GetArtistNames(), " ")
	artistB := strings.Join(songB.GetArtistNames(), " ")

	// TokenSetRatio is ideal for artists, as it ignores word order and duplicates.
	artistScore := float64(fuzzywuzzy.TokenSetRatio(
		Clean(artistA),
		Clean(artistB),
	))

	albumScore := float64(fuzzywuzzy.Ratio(
		Clean(songA.GetAlbum()),
		Clean(songB.GetAlbum()),
	))

	// Weighting: 45% title, 45% artist, 10% album.
	weightedScore := (titleScore*0.45) + (artistScore*0.45) + (albumScore*0.10)

	return weightedScore
}

// AreDuplicates compares two songs to determine if they are duplicates based on a similarity threshold.
func AreDuplicates(songA, songB core.Song, threshold float64) bool {
	return CalculateSimilarity(songA, songB) >= threshold
}

// DeduplicateSongs filters a list of songs, returning only the unique ones based on the similarity threshold.
func DeduplicateSongs(songs []core.Song, threshold float64) ([]core.Song, error) {
	uniqueSongs := []core.Song{}
	for _, song := range songs {
		isDuplicate := false
		for _, uniqueSong := range uniqueSongs {
			if AreDuplicates(song, uniqueSong, threshold) {
				isDuplicate = true
				break
			}
		}
		if !isDuplicate {
			uniqueSongs = append(uniqueSongs, song)
		}
	}
	return uniqueSongs, nil
}
