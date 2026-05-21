package confluence

import (
	"html"
	"regexp"
	"strings"
)

var storageTagPattern = regexp.MustCompile(`(?s)<[^>]+>`)

// StorageToPlain extrait un texte lisible approximatif depuis le format storage Confluence (XML/HTML).
func StorageToPlain(storage string) string {
	if storage == "" {
		return ""
	}
	t := storageTagPattern.ReplaceAllString(storage, " ")
	t = html.UnescapeString(t)
	t = strings.Join(strings.Fields(t), " ")
	return strings.TrimSpace(t)
}
