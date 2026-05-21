package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"fluid/probes/core"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Probe        core.ProbeConfig         `yaml:"probe"`
	Confluence   ConfluenceConfig         `yaml:"confluence"`
	Data         core.DataConfig          `yaml:"data"`
	State        core.StateConfig         `yaml:"state"`
	Controlplane *core.ControlplaneConfig `yaml:"controlplane,omitempty"`
	// Filled from config/schema.yml (usable_in_rag); not serialized.
	RAGFieldAllowlist core.RAGFieldSet `yaml:"-"`
}

type ConfluenceConfig struct {
	BaseURL string `yaml:"base_url"`
	// WikiBaseURL is the browser wiki root (e.g. https://tenant.atlassian.net/wiki), no trailing slash.
	// Not used by the API client; expose via control plane runtime_config for RAG source links.
	WikiBaseURL string `yaml:"wiki_base_url,omitempty"`
	Token       string `yaml:"token"`
	Email       string `yaml:"email,omitempty"` // If set, use Basic auth (email:token)
}

func LoadConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("error reading configuration file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("error parsing configuration: %w", err)
	}

	if err := config.resolveEnvironmentVariables(); err != nil {
		return nil, fmt.Errorf("error resolving environment variables: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	schemaPath := filepath.Join(filepath.Dir(configPath), "schema.yml")
	schemaData, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("schema.yml required (same directory as config) to validate fields.*.rag: %w", err)
	}
	allowed, err := core.ParseRAGFieldSetFromSchemaYAML(schemaData)
	if err != nil {
		return nil, fmt.Errorf("schema.yml (RAG): %w", err)
	}
	if err := core.ValidateRAGEntityFields(config.Data.Entities, allowed); err != nil {
		return nil, err
	}
	config.RAGFieldAllowlist = allowed

	return &config, nil
}

func (c *Config) Validate() error {
	if c.Probe.Name == "" {
		return fmt.Errorf("probe name is missing")
	}

	if c.Confluence.BaseURL == "" {
		return fmt.Errorf("confluence base URL is missing")
	}

	if c.Confluence.Token == "" {
		return fmt.Errorf("confluence token is missing")
	}

	if len(c.Data.Entities) == 0 {
		return fmt.Errorf("at least one entity must be configured")
	}

	if c.State.Dir == "" {
		return fmt.Errorf("state directory is missing")
	}

	if c.State.CleanupInterval <= 0 {
		c.State.CleanupInterval = 1
	}

	return nil
}

func (c *Config) resolveEnvironmentVariables() error {
	if c.Confluence.Token != "" {
		if resolved, isEnvVar := resolveEnvVar(c.Confluence.Token); isEnvVar {
			if resolved == "" {
				return fmt.Errorf("environment variable for confluence token is not defined")
			}
			c.Confluence.Token = resolved
		}
	}

	if c.Confluence.Email != "" {
		if resolved, isEnvVar := resolveEnvVar(c.Confluence.Email); isEnvVar {
			c.Confluence.Email = resolved
		}
	}

	if c.Controlplane != nil {
		if c.Controlplane.WebSocketURL != "" {
			if resolved, isEnvVar := resolveEnvVar(c.Controlplane.WebSocketURL); isEnvVar {
				if resolved == "" {
					log.Printf("Warning: environment variable not defined for websocket_url, controlplane disabled")
					c.Controlplane = nil
					return nil
				}
				c.Controlplane.WebSocketURL = resolved
			}
		}

		if c.Controlplane != nil && c.Controlplane.Parameters == nil {
			log.Printf("Warning: controlplane parameters missing, controlplane disabled")
			c.Controlplane = nil
			return nil
		}

		if c.Controlplane != nil && c.Controlplane.Parameters != nil {
			if c.Controlplane.Parameters.OrganizationUUID != "" {
				if resolved, isEnvVar := resolveEnvVar(c.Controlplane.Parameters.OrganizationUUID); isEnvVar && resolved != "" {
					c.Controlplane.Parameters.OrganizationUUID = resolved
				}
			}
			if c.Controlplane.Parameters.Token != "" {
				if resolved, isEnvVar := resolveEnvVar(c.Controlplane.Parameters.Token); isEnvVar && resolved != "" {
					c.Controlplane.Parameters.Token = resolved
				}
			}
			if c.Controlplane.Parameters.OrganizationUUID == "" || c.Controlplane.Parameters.Token == "" {
				log.Printf("Warning: controlplane parameters incomplete, controlplane disabled")
				c.Controlplane = nil
			}
		}
	}

	return nil
}

func resolveEnvVar(value string) (string, bool) {
	if !strings.HasPrefix(value, "${") || !strings.HasSuffix(value, "}") {
		return value, false
	}
	envVar := strings.TrimPrefix(strings.TrimSuffix(value, "}"), "${")
	return os.Getenv(envVar), true
}

func (c *Config) GetProbeName() string                      { return c.Probe.Name }
func (c *Config) GetProbeVersion() string                   { return c.Probe.Version }
func (c *Config) GetStateDir() string                       { return c.State.Dir }
func (c *Config) GetCleanupInterval() int                   { return c.State.CleanupInterval }
func (c *Config) GetEntities() []core.EntityConfig          { return c.Data.Entities }
func (c *Config) GetControlplane() *core.ControlplaneConfig { return c.Controlplane }
