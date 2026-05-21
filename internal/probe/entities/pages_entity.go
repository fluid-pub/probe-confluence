package entities

import (
	"fmt"
	"log"

	"fluid/probes/confluence/internal/confluence"
	"fluid/probes/confluence/internal/models"
	"fluid/probes/core"
	"fluid/probes/core/state"
)

type PagesEntity struct {
	cfg state.ConfigProvider
}

func NewPagesEntity(cfg state.ConfigProvider) *PagesEntity {
	return &PagesEntity{cfg: cfg}
}

func (e *PagesEntity) Name() string {
	return "pages"
}

func (e *PagesEntity) Refresh(client core.Client) (interface{}, error) {
	confluenceClient, ok := client.(*confluence.Client)
	if !ok {
		return nil, fmt.Errorf("invalid client type for pages entity, expected *confluence.Client")
	}

	log.Printf("Fetching Confluence pages...")

	pages, err := confluenceClient.GetPages()
	if err != nil {
		return nil, fmt.Errorf("fetch pages: %w", err)
	}

	log.Printf("Fetched %d pages", len(pages))

	if pagesBodyRAGEnabled(e.cfg) {
		log.Printf("RAG enabled for body (fields.body.rag): fetching storage for %d pages", len(pages))
		for i := range pages {
			storage, err := confluenceClient.GetPageStorage(pages[i].ID)
			if err != nil {
				log.Printf("page body id=%s: %v", pages[i].ID, err)
				continue
			}
			pages[i].Body = storage
			pages[i].RagForBody = confluence.StorageToPlain(storage)
		}
	}

	return pages, nil
}

func pagesBodyRAGEnabled(cfg state.ConfigProvider) bool {
	if cfg == nil {
		return false
	}
	for _, ec := range cfg.GetEntities() {
		if ec.Name == "pages" && ec.FieldRAG("body") {
			return true
		}
	}
	return false
}

func (e *PagesEntity) Save(stateManager core.StateManager, data interface{}) error {
	pages, ok := data.([]models.Page)
	if !ok {
		return fmt.Errorf("invalid data type for pages entity")
	}

	if err := stateManager.SaveEntity(e.Name(), pages); err != nil {
		return fmt.Errorf("save pages state: %w", err)
	}

	log.Printf("Pages state saved")
	return nil
}
