package probe

import (
	"fmt"
	"log"

	"fluid/probes/core"
	"fluid/probes/core/state"
	"fluid/probes/confluence/internal/probe/entities"
	"fluid/probes/confluence/internal/config"
	"fluid/probes/confluence/internal/confluence"
	"fluid/probes/confluence/internal/manager"
)

type Probe struct {
	*core.Probe
	config *config.Config
	client *confluence.Client
}

func getConfluenceConfig(cfg state.ConfigProvider) *config.Config {
	if c, ok := cfg.(*config.Config); ok {
		return c
	}
	if m, ok := cfg.(*core.MergedConfigProvider); ok {
		if c, ok := m.Local().(*config.Config); ok {
			return c
		}
	}
	return nil
}

// NewProbe accepts state.ConfigProvider (*config.Config or *core.MergedConfigProvider).
// When MergedConfigProvider is used, config is merged with controlplane at startup and on configuration_changed.
func NewProbe(cfg state.ConfigProvider) (*Probe, error) {
	confluenceCfg := getConfluenceConfig(cfg)
	if confluenceCfg == nil {
		return nil, fmt.Errorf("NewProbe requires *config.Config or *core.MergedConfigProvider with *config.Config as local")
	}
	client := confluence.NewClient(&confluenceCfg.Confluence)
	stateManager, err := manager.NewManager(cfg)
	if err != nil {
		return nil, err
	}
	coreProbe := core.NewProbe(cfg, client, stateManager)

	a := &Probe{
		Probe:  coreProbe,
		config: confluenceCfg,
		client: client,
	}

	coreProbe.RegisterEntity(entities.NewPagesEntity(cfg))

	if merged, ok := cfg.(*core.MergedConfigProvider); ok && stateManager.GetPushManager() != nil {
		stateManager.SetConfigCallbacks(
			merged.GetConfigVersion,
			func(runtimeJSON []byte, configVersion string) {
				runtime, version, err := core.ParseRuntimeConfig(runtimeJSON)
				if err != nil {
					log.Printf("Parse runtime config on reload: %v", err)
					return
				}
				if version != "" {
					configVersion = version
				}
				if err := merged.SetRemote(runtime, configVersion); err != nil {
					log.Printf("Set remote config on reload: %v", err)
					return
				}
				if err := coreProbe.ReloadConfig(); err != nil {
					log.Printf("ReloadConfig failed: %v", err)
				}
			},
		)
	}

	return a, nil
}

func (a *Probe) Start() error {
	return a.Probe.Start()
}

func (a *Probe) GetStatus() map[string]interface{} {
	status := a.Probe.GetStatus()
	status["confluence_configured"] = true
	return status
}
