// SPDX-License-Identifier: AGPL-3.0-or-later
// memo-perf is an opt-in, serial longevity benchmark adapter, never a production entrypoint.
package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"memo/internal/api"
	"memo/internal/app"
	"memo/internal/cliproxy"
	"memo/internal/config"
	"memo/internal/memory"
	"memo/internal/provider"
)

func main() {
	mode := flag.String("mode", "serve", "serve, rag, or subscriptions")
	root := flag.String("data", "", "isolated fixture data directory (required)")
	endpoint := flag.String("endpoint", "http://127.0.0.1:18082/v1", "fixture endpoint")
	port := flag.Int("port", 18090, "loopback API port")
	flag.Parse()
	if *root == "" {
		panic("-data is required")
	}
	abs, err := filepath.Abs(*root)
	must(err)
	must(os.Setenv("MEMO_DATA_DIR", abs))
	must(os.Setenv("MEMO_TELEMETRY", "off"))
	config.ResetForTests()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if *mode == "subscriptions" {
		m := cliproxy.New(abs)
		c, stop := context.WithTimeout(ctx, 50*time.Second)
		err = m.Start(c)
		stop()
		must(err)
		defer m.Stop()
		models, err := m.Models(ctx)
		must(err)
		emit(map[string]any{"ready": true, "models": models})
		<-ctx.Done()
		return
	}
	if *mode == "rag" {
		s, err := memory.NewStore(memory.StoreConfig{Dir: filepath.Join(abs, "memory"), Dimension: 768, EmbeddingFunc: memory.NewEmbeddingFunc(api.NewClient(*endpoint, 30), "benchmark-hash-768"), RecencyHalfLifeDays: 30})
		must(err)
		defer s.Close()
		var queries []string
		must(json.NewDecoder(os.Stdin).Decode(&queries))
		out := []any{}
		for _, scope := range []struct {
			name  string
			value memory.Scope
		}{{"all", memory.AllProjects}, {"project", memory.ProjectScope("benchmark-project")}} {
			for _, q := range queries {
				t := time.Now()
				results, err := s.RetrieveContext(ctx, scope.value, q, 5, 0.15)
				row := map[string]any{"scope": scope.name, "query": q, "ms": float64(time.Since(t).Microseconds()) / 1000, "results": results}
				if err != nil {
					row["error"] = err.Error()
				}
				out = append(out, row)
			}
		}
		emit(out)
		return
	}
	if *mode != "serve" {
		panic("unknown mode")
	}
	cfg := config.Default()
	cfg.API.BaseURL = *endpoint
	cfg.API.EmbeddingModel = "benchmark-hash-768"
	cfg.Memory.PersistDir = filepath.Join(abs, "memory")
	cfg.Memory.EmbeddingDimension = 768
	cfg.Memory.MemoryEnabled = true
	cfg.Memory.EmbeddingAutoStart = false
	cfg.Memory.AutoFactExtraction = false
	cfg.Memory.DreamEnabled = false
	cfg.Memory.RecencyHalfLifeDays = 30
	cfg.Memory.MinSimilarity = 0.15
	cfg.WhatsApp.Enabled = false
	cfg.Proactive.Enabled = false
	cfg.Sync.Enabled = false
	cfg.Mood.Enabled = false
	cfg.Whisper.Enabled = false
	cfg.Learning.SingleModelEnabled = false
	cfg.Llama.ModelsDir = filepath.Join(abs, "models")
	cfg.Llama.EmbeddingPort = 18083
	cfg.Llama.Port = 18084
	cfg.Llama.MaxContextTokens = 16384
	cfg.Identity.UserName = "Benchmark User"
	cfg.Identity.UILanguage = "en"
	cfg.AgentMode.Enabled = false
	cfg.WebSearch.Enabled = false
	cfg.Onboarding.Completed = true
	cfg.ActiveProvider = "benchmark-fixture"
	must(config.Save(cfg))
	a := app.NewApp(embed.FS{}, "performance-benchmark")
	start := time.Now()
	a.Startup(ctx)
	must(a.UpdateProvider(provider.ProviderConfig{Type: provider.ProviderCustom, Name: "benchmark-fixture", BaseURL: *endpoint, APIKey: "benchmark-not-a-secret", Model: "benchmark-fake-local", Enabled: true}))
	a.SetActiveProvider("benchmark-fixture")
	must(a.StartWebServerHTTP(*port))
	emit(map[string]any{"ready": true, "startup_ms": float64(time.Since(start).Microseconds()) / 1000, "port": a.WebServerPort()})
	<-ctx.Done()
	c, stop := context.WithTimeout(context.Background(), 10*time.Second)
	defer stop()
	a.Shutdown(c)
}
func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func emit(v any) { must(json.NewEncoder(os.Stdout).Encode(v)) }
