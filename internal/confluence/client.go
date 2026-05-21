package confluence

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"fluid/probes/confluence/internal/config"
	"fluid/probes/confluence/internal/models"
)

const (
	apiPathPages = "/wiki/api/v2/pages"
	pageLimit    = 50
)

// Client is the Confluence Cloud API v2 client
type Client struct {
	baseURL     string
	wikiBaseURL string
	authHeader  string
	email       string
	token       string
	useBasic    bool
	httpClient  *http.Client
	ctx         context.Context
}

// NewClient creates a Confluence client
func NewClient(cfg *config.ConfluenceConfig) *Client {
	baseURL := strings.TrimSuffix(cfg.BaseURL, "/")
	wikiBaseURL := strings.TrimSpace(cfg.WikiBaseURL)

	authHeader := "Bearer " + cfg.Token
	useBasic := false
	if cfg.Email != "" {
		authHeader = "Basic " + base64.StdEncoding.EncodeToString([]byte(cfg.Email+":"+cfg.Token))
		useBasic = true
		log.Printf("Confluence: auth Basic (email présent)")
	} else {
		log.Printf("Confluence: auth Bearer")
	}

	return &Client{
		baseURL:     baseURL,
		wikiBaseURL: wikiBaseURL,
		authHeader:  authHeader,
		email:       cfg.Email,
		token:       cfg.Token,
		useBasic:    useBasic,
		httpClient:  &http.Client{Timeout: 60 * time.Second},
		ctx:         context.Background(),
	}
}

// SetContext fixe le contexte pour les requêtes HTTP (annulation, deadline).
func (c *Client) SetContext(ctx context.Context) {
	if ctx == nil {
		c.ctx = context.Background()
		return
	}
	c.ctx = ctx
}

// fetchPagesViaCurl appelle curl pour une URL et retourne la réponse parsée + next URL si présente
func (c *Client) fetchPagesViaCurl(fetchURL string) (*models.PagesResponse, error) {
	email, token := os.Getenv("CONFLUENCE_EMAIL"), os.Getenv("CONFLUENCE_TOKEN")
	if email == "" || token == "" {
		return nil, fmt.Errorf("CONFLUENCE_EMAIL/CONFLUENCE_TOKEN required for curl fallback")
	}
	out, err := exec.Command("curl", "-s", "-u", email+":"+token, "-H", "Accept: application/json", fetchURL).Output()
	if err != nil {
		return nil, err
	}
	body := strings.TrimSpace(string(out))
	if body == "" {
		return nil, fmt.Errorf("curl returned empty body")
	}
	var data models.PagesResponse
	if json.Unmarshal([]byte(body), &data) != nil {
		snippet := body
		if len(snippet) > 200 {
			snippet = snippet[:200] + "..."
		}
		return nil, fmt.Errorf("invalid JSON (starts with: %q)", snippet)
	}
	return &data, nil
}

// GetPages returns all pages (paginated)
func (c *Client) GetPages() ([]models.Page, error) {
	var all []models.Page
	url := c.baseURL + apiPathPages + "?limit=" + fmt.Sprintf("%d", pageLimit)
	log.Printf("Confluence API: GET %s", url)

	// Fallback: si le client HTTP Go renvoie 404 (routage Atlassian), utiliser curl avec pagination
	if c.useBasic {
		email, token := os.Getenv("CONFLUENCE_EMAIL"), os.Getenv("CONFLUENCE_TOKEN")
		if email != "" && token != "" {
			for fetchURL := url; fetchURL != ""; {
				data, err := c.fetchPagesViaCurl(fetchURL)
				if err != nil {
					return nil, fmt.Errorf("curl fallback: %w", err)
				}
				for _, r := range data.Results {
					p, _ := models.PageFromResult(r, c.wikiBaseURL)
					all = append(all, p)
				}
				fetchURL = ""
				if data.Links.Next != "" {
					if strings.HasPrefix(data.Links.Next, "http") {
						fetchURL = data.Links.Next
					} else {
						fetchURL = c.baseURL + data.Links.Next
					}
				}
			}
			log.Printf("Confluence API: curl fallback OK (%d pages total)", len(all))
			return all, nil
		}
	}

	for url != "" {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Probe", "curl/7.68.0")
		if c.useBasic {
			email, token := os.Getenv("CONFLUENCE_EMAIL"), os.Getenv("CONFLUENCE_TOKEN")
			if email != "" && token != "" {
				req.SetBasicAuth(email, token)
			} else {
				req.Header.Set("Authorization", c.authHeader)
			}
		} else {
			req.Header.Set("Authorization", c.authHeader)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("reading response: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
		}

		var data models.PagesResponse
		if err := json.Unmarshal(body, &data); err != nil {
			return nil, fmt.Errorf("decoding response: %w", err)
		}

		for _, r := range data.Results {
			p, err := models.PageFromResult(r, c.wikiBaseURL)
			if err != nil {
				continue
			}
			all = append(all, p)
		}

		// Pagination: next URL can be in _links.next (relative or absolute)
		url = ""
		if data.Links.Next != "" {
			if strings.HasPrefix(data.Links.Next, "http") {
				url = data.Links.Next
			} else {
				url = c.baseURL + data.Links.Next
			}
		}
	}

	return all, nil
}

// fetchURLViaCurl exécute un GET JSON (même stratégie que la liste de pages en fallback curl).
func (c *Client) fetchURLViaCurl(fetchURL string) ([]byte, error) {
	email, token := os.Getenv("CONFLUENCE_EMAIL"), os.Getenv("CONFLUENCE_TOKEN")
	if email == "" || token == "" {
		return nil, fmt.Errorf("CONFLUENCE_EMAIL/CONFLUENCE_TOKEN required for curl fallback")
	}
	out, err := exec.Command("curl", "-s", "-u", email+":"+token, "-H", "Accept: application/json", fetchURL).Output()
	if err != nil {
		return nil, err
	}
	body := strings.TrimSpace(string(out))
	if body == "" {
		return nil, fmt.Errorf("curl returned empty body")
	}
	return []byte(body), nil
}

func (c *Client) getBytes(ctx context.Context, url string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if c.useBasic {
		email, token := os.Getenv("CONFLUENCE_EMAIL"), os.Getenv("CONFLUENCE_TOKEN")
		if email != "" && token != "" {
			return c.fetchURLViaCurl(url)
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Probe", "curl/7.68.0")
	if c.useBasic {
		email, token := os.Getenv("CONFLUENCE_EMAIL"), os.Getenv("CONFLUENCE_TOKEN")
		if email != "" && token != "" {
			req.SetBasicAuth(email, token)
		} else {
			req.Header.Set("Authorization", c.authHeader)
		}
	} else {
		req.Header.Set("Authorization", c.authHeader)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(raw))
	}
	return raw, nil
}

// GetPageStorage récupère le corps d’une page au format storage (XML Confluence).
func (c *Client) GetPageStorage(pageID string) (string, error) {
	if pageID == "" {
		return "", fmt.Errorf("page id is empty")
	}
	url := c.baseURL + "/wiki/api/v2/pages/" + pageID + "?body-format=storage"
	ctx := c.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	raw, err := c.getBytes(ctx, url)
	if err != nil {
		return "", err
	}
	var parsed struct {
		Body *struct {
			Storage *struct {
				Value string `json:"value"`
			} `json:"storage"`
		} `json:"body"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("decoding page body: %w", err)
	}
	if parsed.Body == nil || parsed.Body.Storage == nil {
		return "", nil
	}
	return parsed.Body.Storage.Value, nil
}
