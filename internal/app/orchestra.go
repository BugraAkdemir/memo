package app

import (
	"fmt"
	"log"

	"memo/internal/config"
	"memo/internal/orchestra"
)

// GetOrchestraConfig returns the current orchestra configuration.
func (a *App) GetOrchestraConfig() orchestra.OrchestraConfig {
	if a.orchestraConductor == nil {
		return orchestra.DefaultConfig()
	}
	return a.orchestraConductor.Config()
}

// UpdateOrchestraConfig saves an updated orchestra configuration.
func (a *App) UpdateOrchestraConfig(cfg orchestra.OrchestraConfig) error {
	if a.orchestraConductor == nil {
		return fmt.Errorf("orchestra system not initialized")
	}
	a.orchestraConductor.UpdateConfig(cfg)
	if err := orchestra.SaveConfig(config.DataPath("orchestra.json"), a.orchestraConductor.Config()); err != nil {
		log.Printf("ORCHESTRA: config save error: %v", err)
		return err
	}
	log.Printf("Orchestra config updated: enabled=%v, chief=%s/%s", cfg.Enabled, cfg.ChiefType, cfg.ChiefModel)
	return nil
}
