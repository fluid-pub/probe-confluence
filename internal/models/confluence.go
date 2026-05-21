package models

import (
	"strings"
	"time"
)

func parseTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t, err = time.Parse(time.RFC3339Nano, s)
	}
	return t, err
}

// Page is a Confluence page entity collected by the probe.
type Page struct {
	ID    string `json:"id" yaml:"id"`
	Title string `json:"title" yaml:"title"`
	// RagForTitle is indexed for RAG (control plane: usable_in_rag + rag_for_title).
	RagForTitle string `json:"rag_for_title,omitempty" yaml:"rag_for_title,omitempty"`
	Status     string `json:"status" yaml:"status"`
	SpaceID    string `json:"space_id" yaml:"space_id"`
	ParentID   string `json:"parent_id,omitempty" yaml:"parent_id,omitempty"`
	ParentType string `json:"parent_type,omitempty" yaml:"parent_type,omitempty"`
	AuthorID   string `json:"author_id,omitempty" yaml:"author_id,omitempty"`
	CreatedAt  time.Time  `json:"created_at" yaml:"created_at"`
	Version    PageVersion `json:"version" yaml:"version"`
	Links      PageLinks  `json:"_links,omitempty" yaml:"_links,omitempty"`
	// SourceURL is the browser "view source" URL (wiki base + _links.webui), set by the probe.
	SourceURL string `json:"source_url,omitempty" yaml:"source_url,omitempty"`
	// Body is raw storage XML from the API when config has fields.body.rag: true.
	Body string `json:"body,omitempty" yaml:"body,omitempty"`
	// RagForBody is plain text for RAG (control plane: usable_in_rag + rag_for_body).
	RagForBody string `json:"rag_for_body,omitempty" yaml:"rag_for_body,omitempty"`
}

// PageVersion holds version metadata.
type PageVersion struct {
	Number    int       `json:"number" yaml:"number"`
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
	Message   string    `json:"message,omitempty" yaml:"message,omitempty"`
	AuthorID  string    `json:"author_id,omitempty" yaml:"author_id,omitempty"`
}

// PageLinks holds API links (webui, editui, tinyui).
type PageLinks struct {
	WebUI  string `json:"webui,omitempty" yaml:"webui,omitempty"`
	EditUI string `json:"editui,omitempty" yaml:"editui,omitempty"`
	TinyUI string `json:"tinyui,omitempty" yaml:"tinyui,omitempty"`
}

// PagesResponse is the Confluence API v2 response for GET /wiki/api/v2/pages.
type PagesResponse struct {
	Results []PageResult `json:"results"`
	Links   struct {
		Next string `json:"next,omitempty"`
	} `json:"_links,omitempty"`
}

// PageResult is the raw page object from the Confluence API v2.
type PageResult struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	Title      string `json:"title"`
	SpaceID    string `json:"spaceId"`
	ParentID   string `json:"parentId,omitempty"`
	ParentType string `json:"parentType,omitempty"`
	AuthorID   string `json:"authorId,omitempty"`
	CreatedAt  string `json:"createdAt"`
	Version    struct {
		Number    int    `json:"number"`
		CreatedAt string `json:"createdAt"`
		Message   string `json:"message,omitempty"`
		AuthorID  string `json:"authorId,omitempty"`
	} `json:"version,omitempty"`
	Links struct {
		WebUI  string `json:"webui,omitempty"`
		EditUI string `json:"editui,omitempty"`
		TinyUI string `json:"tinyui,omitempty"`
	} `json:"_links,omitempty"`
}

// WikiPageURL builds the browser URL from wiki base and Confluence webui path.
func WikiPageURL(wikiBase, webuiPath string) string {
	wikiBase = strings.TrimSpace(wikiBase)
	webuiPath = strings.TrimSpace(webuiPath)
	if wikiBase == "" || webuiPath == "" {
		return ""
	}
	wikiBase = strings.TrimRight(wikiBase, "/")
	if !strings.HasPrefix(webuiPath, "/") {
		webuiPath = "/" + webuiPath
	}
	return wikiBase + webuiPath
}

// PageFromResult converts an API page result to the probe Page model.
// wikiBaseURL is confluence.wiki_base_url; when set with a webui link, SourceURL is populated.
func PageFromResult(r PageResult, wikiBaseURL string) (Page, error) {
	createdAt, _ := parseTime(r.CreatedAt)
	versionCreatedAt, _ := parseTime(r.Version.CreatedAt)

	title := strings.TrimSpace(r.Title)
	p := Page{
		ID:          r.ID,
		Title:       r.Title,
		RagForTitle: title,
		Status:      r.Status,
		SpaceID:     r.SpaceID,
		ParentID:    r.ParentID,
		ParentType:  r.ParentType,
		AuthorID:    r.AuthorID,
		CreatedAt:   createdAt,
		Version: PageVersion{
			Number:    r.Version.Number,
			CreatedAt: versionCreatedAt,
			Message:   r.Version.Message,
			AuthorID:  r.Version.AuthorID,
		},
		Links: PageLinks{
			WebUI:  r.Links.WebUI,
			EditUI: r.Links.EditUI,
			TinyUI: r.Links.TinyUI,
		},
	}
	if su := WikiPageURL(wikiBaseURL, r.Links.WebUI); su != "" {
		p.SourceURL = su
	}
	return p, nil
}
