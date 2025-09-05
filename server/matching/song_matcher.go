package matching

import (
	"math"
	"strings"

	"github.com/hansbala/myncer/core"
	"github.com/lithammer/fuzzysearch/fuzzy"
)

// normalizedLevenshtein converts an absolute Levenshtein distance into a similarity ratio from 0.0 to 100.0.
// A score of 100.0 means the strings are identical.
func normalizedLevenshtein(s1, s2 string) float64 {
	distance := fuzzy.LevenshteinDistance(s1, s2)
	maxLen := math.Max(float64(len(s1)), float64(len(s2)))
	if maxLen == 0 {
		return 100.0
	}
	return (1.0 - (float64(distance) / maxLen)) * 100.0
}

// tokenSetRatio calculates similarity based on the intersection and union of word sets.
// It's ideal for comparing artist names where order doesn't matter.
func tokenSetRatio(s1, s2 string) float64 {
	words1 := core.ToSet(strings.Fields(s1))
	words2 := core.ToSet(strings.Fields(s2))

	if words1.IsEmpty() && words2.IsEmpty() {
		return 100.0
	}
	if words1.IsEmpty() || words2.IsEmpty() {
		return 0.0
	}

	intersection := core.NewSet[string]()
	for word := range words1 {
		if words2.Contains(word) {
			intersection.Add(word)
		}
	}

	union := core.NewSet[string]()
	for word := range words1 {
		union.Add(word)
	}
	for word := range words2 {
		union.Add(word)
	}

	return (float64(len(intersection)) / float64(len(union))) * 100.0
}


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
	titleScore := normalizedLevenshtein(
		Clean(songA.GetName()),
		Clean(songB.GetName()),
	)

	artistA := strings.Join(songA.GetArtistNames(), " ")
	artistB := strings.Join(songB.GetArtistNames(), " ")
	artistScore := tokenSetRatio(
		Clean(artistA),
		Clean(artistB),
	)

	albumScore := normalizedLevenshtein(
		Clean(songA.GetAlbum()),
		Clean(songB.GetAlbum()),
	)

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
