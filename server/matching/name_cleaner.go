package matching

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var (
	// Regex to remove featured artist information (e.g., feat, ft).
	featRegex = regexp.MustCompile(`(?i)\[\s*(feat|ft)\.?\s*[^\]]+\]|\(\s*(feat|ft)\.?\s*[^)]+\)`)

	// Regex to remove common tags like (remix, live, edit, etc.).
	tagsRegex = regexp.MustCompile(`(?i)[\(\[].*?(remix|edit|live|version|explicit|clean|instrumental|deluxe|mastered).*?[\)\]]`)

	// Regex to remove all non-alphanumeric characters, replacing them with a space.
	nonAlphanumericRegex = regexp.MustCompile(`[^a-z0-9\s]+`)

	// Regex to collapse multiple whitespace characters into a single space.
	extraWhitespaceRegex = regexp.MustCompile(`\s+`)
)

// Clean normalizes a song's metadata string to improve matching accuracy.
// It performs the following operations in order:
// 1. Converts the string to lowercase.
// 2. Removes diacritics (e.g., "é" -> "e").
// 3. Removes featured artist information like "(feat. ...)" or "[ft. ...]".
// 4. Removes common tags like "(remix)", "(live)", "(radio edit)", etc.
// 5. Removes all non-alphanumeric characters.
// 6. Collapses multiple whitespace sequences into a single space.
// 7. Trims whitespace from the beginning and end of the string.
func Clean(s string) string {
	// 1. Convert to lowercase
	s = strings.ToLower(s)

	// 2. Remove diacritics
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	s, _, _ = transform.String(t, s)

	// 3. Remove "feat" and "ft"
	s = featRegex.ReplaceAllString(s, "")

	// 4. Remove other common tags
	s = tagsRegex.ReplaceAllString(s, "")

	// 5. Remove non-alphanumeric characters
	s = nonAlphanumericRegex.ReplaceAllString(s, " ")

	// 6. Collapse whitespace
	s = extraWhitespaceRegex.ReplaceAllString(s, " ")

	// 7. Trim leading and trailing spaces
	return strings.TrimSpace(s)
}
