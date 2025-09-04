package matching

import (
	"strings"
	"github.com/hansbala/myncer/core"
)

// simpleFuzzRatio calculates a basic similarity metric between two strings
func simpleFuzzRatio(s1, s2 string) float64 {
	if s1 == s2 {
		return 100.0
	}
	
	// Simple implementation of normalized Levenshtein distance
	s1Lower := strings.ToLower(s1)
	s2Lower := strings.ToLower(s2)
	
	if s1Lower == s2Lower {
		return 95.0
	}
	
	// If one contains the other, high similarity
	if strings.Contains(s1Lower, s2Lower) || strings.Contains(s2Lower, s1Lower) {
		return 85.0
	}
	
	// Check for common words
	words1 := strings.Fields(s1Lower)
	words2 := strings.Fields(s2Lower)
	
	commonWords := 0
	totalWords := len(words1) + len(words2)
	
	for _, w1 := range words1 {
		for _, w2 := range words2 {
			if w1 == w2 {
				commonWords++
				break
			}
		}
	}
	
	if totalWords == 0 {
		return 0.0
	}
	
	return float64(commonWords*2) / float64(totalWords) * 100.0
}

// tokenSetRatio calculates similarity by comparing token sets
func tokenSetRatio(s1, s2 string) float64 {
	words1 := strings.Fields(strings.ToLower(s1))
	words2 := strings.Fields(strings.ToLower(s2))
	
	// Create map of unique words
	allWords := make(map[string]bool)
	for _, w := range words1 {
		allWords[w] = true
	}
	for _, w := range words2 {
		allWords[w] = true
	}
	
	commonCount := 0
	for word := range allWords {
		inS1 := false
		inS2 := false
		
		for _, w := range words1 {
			if w == word {
				inS1 = true
				break
			}
		}
		for _, w := range words2 {
			if w == word {
				inS2 = true
				break
			}
		}
		
		if inS1 && inS2 {
			commonCount++
		}
	}
	
	if len(allWords) == 0 {
		return 0.0
	}
	
	return float64(commonCount) / float64(len(allWords)) * 100.0
}

// AreDuplicates compares two songs to see if they are duplicates.
func AreDuplicates(songA, songB core.Song, threshold float64) bool {
	// 1. Check exact identifiers (ISRC, service IDs)
	if songA.GetSpec().GetIsrc() != "" && songA.GetSpec().GetIsrc() == songB.GetSpec().GetIsrc() {
		return true
	}
	
	// 2. Weighted fuzzy matching if no exact IDs
	titleScore := simpleFuzzRatio(
		songA.GetName(),
		songB.GetName(),
	)

	artistScore := tokenSetRatio(
		strings.Join(songA.GetArtistNames(), " "),
		strings.Join(songB.GetArtistNames(), " "),
	)
	
	albumScore := simpleFuzzRatio(
		songA.GetAlbum(),
		songB.GetAlbum(),
	)
	
	// Weighting: 45% title, 45% artist, 10% album
	weightedScore := (titleScore * 0.45) + (artistScore * 0.45) + (albumScore * 0.10)
	
	return weightedScore >= threshold
}

// DeduplicateSongs removes duplicate songs from a list.
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
