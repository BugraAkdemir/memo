package app

import (
	"context"
	"memo/internal/provider"
	"os"
	"path/filepath"
	"testing"
)

func TestProviderSaveFailureKeepsRunningRouter(t *testing.T) {
	for _, operation := range []string{"update", "model", "delete"} {
		t.Run(operation, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "providers.json")
			cm := provider.NewConfigManager(path, make([]byte, 32))
			cfg := provider.ProviderConfig{Type: provider.ProviderCustom, Name: "test", Model: "original", BaseURL: "http://localhost:1/v1", Enabled: true}
			if err := cm.Set(cfg); err != nil {
				t.Fatal(err)
			}
			router := provider.NewRouter(cm.GetAll())
			router.SetActiveProvider("test")
			a := &App{providerCfgMgr: cm, providerRouter: router, activeProviderName: "test", lifecycleCtx: context.Background()}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(path, 0700); err != nil {
				t.Fatal(err)
			}
			var err error
			switch operation {
			case "update":
				cfg.EffortLevel = "high"
				err = a.UpdateProvider(cfg)
			case "model":
				err = a.SetProviderModel("test", "replacement")
			case "delete":
				err = a.DeleteProvider(provider.ProviderCustom, "test")
			}
			if err == nil {
				t.Fatal("app reported failed persistence as success")
			}
			if a.providerRouter != router || a.activeProviderName != "test" {
				t.Fatal("failed persistence rebuilt or deselected running router")
			}
			got := cm.GetAll()
			if len(got) != 1 || got[0].Model != "original" || got[0].EffortLevel != "" {
				t.Fatalf("failed edit changed settings: %+v", got)
			}
		})
	}
}
