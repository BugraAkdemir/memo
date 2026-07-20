package app

import (
	"fmt"
	"log"
	"memo/internal/logx"
	"os"
	"strings"
	"time"

	"memo/internal/api"
	"memo/internal/config"
	"memo/internal/llama"
	"memo/internal/provider"
)

// StartLocalModel loads and starts a local llama.cpp model.
func (a *App) StartLocalModel(modelPath string, ctxSize, port, gpuLayers int) error {
	if a.isEmbeddingModelPath(modelPath) {
		return fmt.Errorf("embedding modeli ana sohbet modeli olarak başlatılamaz; Hafıza (Embedding) Modeli olarak başlatın")
	}

	a.cfgMu.RLock()
	binaryPath := a.cfg.Llama.BinaryPath
	engineMode := a.cfg.Llama.EngineMode
	a.cfgMu.RUnlock()

	if err := a.llamaServer.Start(binaryPath, modelPath, ctxSize, port, gpuLayers, false, engineMode); err != nil {
		return err
	}

	if err := a.llamaServer.WaitReady(180 * time.Second); err != nil {
		a.llamaServer.Stop()
		return fmt.Errorf("Model yükleme zaman aşımına uğradı (3 dk). (Hata: %w)", err)
	}

	newBaseURL := a.llamaServer.GetBaseURL()
	a.clientMu.Lock()
	a.client = api.NewClient(newBaseURL, a.cfg.API.TimeoutSeconds)
	a.clientMu.Unlock()
	logx.Printf("API client redirected to local llama-server: %s", newBaseURL)

	if memEnabled := a.GetMemoryEnabled(); memEnabled && !a.llamaEmbedServer.IsRunning() {
		a.autoStartEmbeddingModel()
	} else if !memEnabled {
		logx.Info("Memory disabled — skipping embedding model auto-start")
	}

	return nil
}

// StopLocalModel stops the running local model and reverts the API client.
func (a *App) StopLocalModel() error {
	if err := a.llamaServer.Stop(); err != nil {
		return err
	}

	a.clientMu.Lock()
	a.client = api.NewClient(a.originalBaseURL, a.cfg.API.TimeoutSeconds)
	revertedClient := a.client
	a.clientMu.Unlock()
	logx.Printf("API client reverted to: %s", a.originalBaseURL)

	if !a.llamaEmbedServer.IsRunning() {
		a.reinitMemoryStore(revertedClient, a.cfg.API.EmbeddingModel)
	}

	return nil
}

// GetLocalModelStatus returns the current llama server status.
// When Memo Swarm's coordinator is running, report that server instead so
// EngineStrip / model status reflect the process chat is actually using.
func (a *App) GetLocalModelStatus() llama.ServerStatus {
	a.swarmMu.Lock()
	swarmSrv := a.swarmServer
	a.swarmMu.Unlock()

	if swarmSrv != nil && swarmSrv.IsRunning() {
		st := swarmSrv.GetStatus()
		// Prefix so the status strip is obviously "swarm", not a normal local load.
		if st.ModelName != "" && !strings.HasPrefix(st.ModelName, "swarm · ") {
			st.ModelName = "swarm · " + st.ModelName
		} else if st.ModelName == "" {
			st.ModelName = "swarm"
		}
		return st
	}
	if a.llamaServer == nil {
		return llama.ServerStatus{}
	}
	return a.llamaServer.GetStatus()
}

// DetectGPU probes GPU availability.
func (a *App) DetectGPU() llama.GPUInfo {
	return llama.DetectGPU()
}

// GetLlamaConfig returns the llama configuration.
func (a *App) GetLlamaConfig() config.LlamaConfig {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	return a.cfg.Llama
}

// UpdateLlamaConfig merges partial updates into the llama configuration.
// Temperature/TopP/MaxTokens are pointers (see LlamaConfigUpdate) precisely
// so an explicit 0 — greedy decoding, or "no token limit" — can be applied;
// a nil pointer means the caller didn't send that field at all.
func (a *App) UpdateLlamaConfig(cfg config.LlamaConfigUpdate) error {
	a.cfgMu.Lock()
	if cfg.EngineMode != "" {
		a.cfg.Llama.EngineMode = cfg.EngineMode
	}
	if cfg.BinaryPath != "" {
		a.cfg.Llama.BinaryPath = cfg.BinaryPath
	}
	if cfg.Port != 0 {
		a.cfg.Llama.Port = cfg.Port
	}
	if cfg.EmbeddingPort != 0 {
		a.cfg.Llama.EmbeddingPort = cfg.EmbeddingPort
	}
	if cfg.CtxSize != 0 {
		a.cfg.Llama.CtxSize = cfg.CtxSize
	}
	if cfg.MaxHistory != 0 {
		a.cfg.Llama.MaxHistory = cfg.MaxHistory
	}
	if cfg.ModelsDir != "" {
		a.cfg.Llama.ModelsDir = cfg.ModelsDir
	}
	if cfg.Temperature != nil {
		a.cfg.Llama.Temperature = *cfg.Temperature
	}
	if cfg.TopP != nil {
		a.cfg.Llama.TopP = *cfg.TopP
	}
	if cfg.MaxTokens != nil {
		a.cfg.Llama.MaxTokens = *cfg.MaxTokens
	}
	a.cfgMu.Unlock()
	return config.Save(a.cfg)
}

// SetLlamaBinaryPath updates the path to the llama.cpp binary.
func (a *App) SetLlamaBinaryPath(path string) error {
	a.cfgMu.Lock()
	a.cfg.Llama.BinaryPath = path
	a.cfgMu.Unlock()
	return config.Save(a.cfg)
}

// CheckLlamaInstallation reports whether the llama.cpp binary is installed.
func (a *App) CheckLlamaInstallation() bool {
	a.cfgMu.RLock()
	binaryPath := a.cfg.Llama.BinaryPath
	a.cfgMu.RUnlock()
	return a.llamaInstaller.IsInstalled(binaryPath)
}

// InstallLlamaServer compiles and installs the llama.cpp binary.
func (a *App) InstallLlamaServer() error {
	logger := func(msg string) {
		logx.Info("INSTALL:", msg)
	}

	binPath, err := a.llamaInstaller.Install(a.lifecycleCtx, logger)
	if err != nil {
		return err
	}

	a.cfgMu.Lock()
	a.cfg.Llama.BinaryPath = binPath
	a.cfgMu.Unlock()
	_ = os.Remove(config.DataPath(".force_cpu"))
	return config.Save(a.cfg)
}

// SkipLlamaGPUInstall creates a .force_cpu marker to bypass GPU detection.
func (a *App) SkipLlamaGPUInstall() error {
	if err := os.MkdirAll(config.DataDir(), 0755); err != nil {
		return err
	}
	forceCPUFile := config.DataPath(".force_cpu")
	f, err := os.Create(forceCPUFile)
	if err != nil {
		return err
	}
	f.Close()
	logx.Info("Created .force_cpu bypass file. Future starts will use CPU.")
	return nil
}

// autoStartEmbeddingModel finds the first embedding model and starts it.
func (a *App) autoStartEmbeddingModel() {
	models := a.modelStore.ListLocalModels()
	var embeddingPath string
	for _, m := range models {
		if m.IsEmbedding {
			embeddingPath = m.Path
			break
		}
	}
	if embeddingPath == "" {
		msg := "⚠️ No embedding model found — RAG will NOT function."
		logx.Info(msg)
		a.emitEvent("memory:error", msg)
		return
	}

	logx.Printf("Auto-starting embedding model: %s", embeddingPath)
	if err := a.StartEmbeddingModel(embeddingPath, -1); err != nil {
		msg := fmt.Sprintf("⚠️ Failed to auto-start embedding model: %v", err)
		log.Print(msg)
		a.emitEvent("memory:error", msg)
	} else {
		logx.Info("✅ Embedding model auto-started — memory/RAG is active.")
	}
}

// isEmbeddingModelPath reports whether the given model path is tagged as an embedding model.
func (a *App) isEmbeddingModelPath(modelPath string) bool {
	if a.modelStore == nil {
		return false
	}
	for _, m := range a.modelStore.ListLocalModels() {
		if m.Path == modelPath {
			return m.IsEmbedding
		}
	}
	return false
}

// resolveAgentProviderForLlama builds a one-off local router pointing at the llama-server.
// It is used by the embedding server startup path to avoid circular dependencies.
func resolveLocalLlamaRouter(baseURL, modelName string) *provider.Router {
	cfg := provider.ProviderConfig{
		Type:    provider.ProviderLlamaCPP,
		Name:    "Local (llama.cpp)",
		BaseURL: baseURL,
		Model:   modelName,
		Enabled: true,
	}
	return provider.NewRouter([]provider.ProviderConfig{cfg})
}
