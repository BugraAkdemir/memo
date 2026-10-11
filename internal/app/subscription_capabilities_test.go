package app

import (
	"context"
	"memo/internal/config"
	"memo/internal/modelcatalog"
	"memo/internal/provider"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSubscriptionBudgetUsesPublishedModelAndUnknownGauge(t *testing.T) {
	t.Setenv("MEMO_DATA_DIR", t.TempDir())
	config.ResetForTests()
	t.Cleanup(config.ResetForTests)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"claude":[{"id":"published-1m","owned_by":"anthropic","context_length":1000000,"thinking":{"levels":["medium","high"]}}]}`))
	}))
	defer server.Close()
	catalog := modelcatalog.New(server.URL, config.DataPath("cliproxy", "model-capabilities.json"), server.Client())
	if err := catalog.Ensure(context.Background()); err != nil {
		t.Fatal(err)
	}
	mgr := provider.NewConfigManager(t.TempDir(), nil)
	p := provider.ProviderConfig{Type: provider.ProviderCustom, Name: subsProviderName, BaseURL: "http://127.0.0.1:1/v1", Model: "published-1m", Enabled: true}
	mgr.Set(p)
	a := &App{cfg: &config.AppConfig{}, providerCfgMgr: mgr, activeProviderName: subsProviderName}
	if got := a.contextBudgetFor(subsProviderName); got != 1000000 {
		t.Fatalf("budget=%d", got)
	}
	if got := a.ContextReport("none").Window; got != 1000000 {
		t.Fatalf("gauge=%d", got)
	}
	if got := supportedSubscriptionEffort(p.Model, "high"); got != "high" {
		t.Fatalf("supported effort=%q", got)
	}
	if got := supportedSubscriptionEffort(p.Model, "xhigh"); got != "" {
		t.Fatalf("unsupported effort=%q", got)
	}
	p.Model = "unknown-model"
	mgr.Set(p)
	if got := a.contextBudgetFor(subsProviderName); got != 128*1024 {
		t.Fatalf("safety budget=%d", got)
	}
	if got := a.ContextReport("none").Window; got != 0 {
		t.Fatalf("unknown capacity shown as %d", got)
	}
}
