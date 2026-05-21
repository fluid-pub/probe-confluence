package confluence

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
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
		log.Printf("Confluence: using Basic auth (email set)")
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

// SetContext sets the context used for HTTP requests (cancellation, deadlines).
func (c *Client) SetContext(ctx context.Context) {
	if ctx == nil {
		c.ctx = context.Background()
		return
	}
	c.ctx = ctx
}

// GetPages returns all pages (paginated)
func (c *Client) GetPages() ([]models.Page, error) {
	startURL := c.baseURL + apiPathPages + "?limit=" + fmt.Sprintf("%d", pageLimit)
	log.Printf("Confluence API: GET %s", startURL)
	return c.getPagesHTTP(startURL)
}

func (c *Client) getPagesHTTP(startURL string) ([]models.Page, error) {
	var all []models.Page
	for url := startURL; url != ""; {
		body, status, err := c.doGET(url)
		if err != nil {
			return nil, err
		}
		if status != http.StatusOK {
			return nil, fmt.Errorf("API error %d: %s", status, string(body))
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

func (c *Client) getBytes(ctx context.Context, url string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	body, status, err := c.doGETWithContext(ctx, url)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("API error %d: %s", status, string(body))
	}
	return body, nil
}

func (c *Client) doGET(url string) ([]byte, int, error) {
	ctx := c.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return c.doGETWithContext(ctx, url)
}

func (c *Client) doGETWithContext(ctx context.Context, url string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("reading response: %w", err)
	}
	return raw, resp.StatusCode, nil
}

func (c *Client) setAuth(req *http.Request) {
	if c.useBasic && c.email != "" && c.token != "" {
		req.SetBasicAuth(c.email, c.token)
		return
	}
	req.Header.Set("Authorization", c.authHeader)
}

// GetPageStorage returns a page body in Confluence storage format (XML).
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
