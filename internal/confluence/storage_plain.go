package confluence

import (
	"html"
	"regexp"
	"strings"
)

var storageTagPattern = regexp.MustCompile(`(?s)<[^>]+>`)

// StorageToPlain extracts approximate plain text from Confluence storage format (XML/HTML).
func StorageToPlain(storage string) string {
	if storage == "" {
		return ""
	}
	t := storageTagPattern.ReplaceAllString(storage, " ")
	t = html.UnescapeString(t)
	t = strings.Join(strings.Fields(t), " ")
	return strings.TrimSpace(t)
}
