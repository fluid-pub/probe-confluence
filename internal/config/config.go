package config

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"fluid/probes/core"

	"gopkg.in/yaml.v3"
)

const markConfigPath = "~/.config/mark"

type Config struct {
	Probe        core.ProbeConfig         `yaml:"probe"`
	Confluence   ConfluenceConfig         `yaml:"confluence"`
	Data         core.DataConfig          `yaml:"data"`
	State        core.StateConfig         `yaml:"state"`
	Controlplane *core.ControlplaneConfig `yaml:"controlplane,omitempty"`
	// RAGFieldAllowlist est rempli depuis config/schema.yml (champs usable_in_rag) ; non sérialisé.
	RAGFieldAllowlist core.RAGFieldSet `yaml:"-"`
}

type ConfluenceConfig struct {
	BaseURL string `yaml:"base_url"`
	// WikiBaseURL : base des URLs navigateur (ex. https://tenant.atlassian.net/wiki), sans slash final.
	// Non utilisée par le client API ; à reporter dans le runtime_config du control plane pour les liens RAG.
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

	// Optionally fill Confluence credentials from ~/.config/mark (email, token, base_url)
	if err := config.loadFromMarkConfig(); err != nil {
		log.Printf("Warning: load from %s: %v", markConfigPath, err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	schemaPath := filepath.Join(filepath.Dir(configPath), "schema.yml")
	schemaData, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("schema.yml requis (même répertoire que la config) pour valider fields.*.rag: %w", err)
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

// loadFromMarkConfig fills Confluence credentials from ~/.config/mark if not already set.
// File format: key = "value" (username -> email, password -> token, base-url -> base_url).
func (c *Config) loadFromMarkConfig() error {
	path := markConfigPath
	if strings.HasPrefix(path, "~/") {
		path = filepath.Join(os.Getenv("HOME"), path[2:])
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	kv := parseMarkConfig(data)
	if len(kv) == 0 {
		return nil
	}

	if c.Confluence.Email == "" && kv["username"] != "" {
		c.Confluence.Email = kv["username"]
		log.Printf("Using Confluence email from %s", markConfigPath)
	}
	if c.Confluence.Token == "" && kv["password"] != "" {
		c.Confluence.Token = kv["password"]
		log.Printf("Using Confluence token from %s", markConfigPath)
	}
	if c.Confluence.BaseURL == "" || c.Confluence.BaseURL == "https://your-site.atlassian.net" {
		if u := kv["base-url"]; u != "" {
			u = strings.TrimSuffix(u, "/")
			u = strings.TrimSuffix(u, "/wiki")
			c.Confluence.BaseURL = u
			log.Printf("Using Confluence base_url from %s", markConfigPath)
		}
	}

	return nil
}

// parseMarkConfig parses key = "value" lines and returns a map (keys lowercased).
func parseMarkConfig(data []byte) map[string]string {
	out := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(strings.ToLower(line[:idx]))
		val := strings.TrimSpace(line[idx+1:])
		val = strings.Trim(val, `"`)
		out[key] = val
	}
	return out
}

func (c *Config) GetProbeName() string   { return c.Probe.Name }
func (c *Config) GetProbeVersion() string { return c.Probe.Version }
func (c *Config) GetStateDir() string    { return c.State.Dir }
func (c *Config) GetCleanupInterval() int { return c.State.CleanupInterval }
func (c *Config) GetEntities() []core.EntityConfig { return c.Data.Entities }
func (c *Config) GetControlplane() *core.ControlplaneConfig { return c.Controlplane }
