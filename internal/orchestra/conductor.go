package orchestra

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"memo/internal/provider"
)

// ProviderFactory is a function type for creating providers within the orchestra.
type ProviderFactory func(cfg provider.ProviderConfig) (provider.Provider, error)

// Conductor orchestrates the multi-model workflow.
type Conductor struct {
	config     OrchestraConfig
	pf         ProviderFactory
	getConfigs func() []provider.ProviderConfig
	mu         sync.RWMutex
}

// NewConductor creates a new orchestra conductor.
func NewConductor(cfg OrchestraConfig, pf ProviderFactory, getConfigs func() []provider.ProviderConfig) *Conductor {
	return &Conductor{
		config:     cfg.Sanitize(),
		pf:         pf,
		getConfigs: getConfigs,
	}
}

// UpdateConfig updates the conductor's configuration and persists it.
func (c *Conductor) UpdateConfig(cfg OrchestraConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cfg.Roles = MergeRoles(cfg.Roles)
	c.config = cfg.Sanitize()
}

// Config returns a copy of the current configuration.
func (c *Conductor) Config() OrchestraConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	// Deep copy the roles slice to avoid shared backing array
	cfg := c.config
	cfg.Roles = append([]RoleConfig{}, cfg.Roles...)
	return cfg
}

// Run executes a full orchestra workflow: plan → execute → synthesize.
func (c *Conductor) Run(ctx context.Context, userMessage string) (string, []OrchestraResult, error) {
	return c.RunWithProgress(ctx, userMessage, nil)
}

// RunWithProgress executes the full orchestra workflow with optional progress streaming.
// It checks ctx.Done() between each phase so cancellation propagates promptly.
func (c *Conductor) RunWithProgress(ctx context.Context, userMessage string, onProgress ProgressFn) (string, []OrchestraResult, error) {
	c.mu.RLock()
	cfg := c.config
	c.mu.RUnlock()

	if !cfg.Enabled {
		return "", nil, fmt.Errorf("orchestra mode is not enabled")
	}

	select {
	case <-ctx.Done():
		return "", nil, ctx.Err()
	default:
	}

	if onProgress != nil {
		c.safeProgress(onProgress, ProgressUpdate{Type: ProgressPlan, Content: "🧠 **Şef planlıyor...**\n\n"})
	}

	log.Println("ORCHESTRA: chief planning...")
	plan, err := c.createPlan(ctx, cfg, userMessage, onProgress)
	if err != nil {
		return "", nil, fmt.Errorf("chief planning failed: %w", err)
	}
	if len(plan.Tasks) == 0 {
		return "", nil, fmt.Errorf("chief returned no tasks")
	}
	log.Printf("ORCHESTRA: chief created %d tasks", len(plan.Tasks))

	select {
	case <-ctx.Done():
		return "", nil, ctx.Err()
	default:
	}

	results, err := c.executeTasks(ctx, cfg, *plan, onProgress)
	if err != nil {
		return "", nil, fmt.Errorf("task execution failed: %w", err)
	}
	log.Printf("ORCHESTRA: %d/%d tasks completed", len(results), len(plan.Tasks))

	select {
	case <-ctx.Done():
		return "", nil, ctx.Err()
	default:
	}

	if onProgress != nil {
		c.safeProgress(onProgress, ProgressUpdate{Type: ProgressSynthChunk, Content: "📝 **Şef sentezliyor...**\n\n"})
	}

	log.Println("ORCHESTRA: chief synthesizing...")
	finalResponse, err := c.synthesize(ctx, cfg, userMessage, plan.Tasks, results, onProgress)
	if err != nil {
		return "", nil, fmt.Errorf("synthesis failed: %w", err)
	}
	return finalResponse, results, nil
}

// safeProgress calls onProgress without holding any lock.
// fn must be goroutine-safe (protect any shared state like string builders at the call site).
// Calling fn under a mutex caused deadlocks when fn blocked on a full output channel.
func (c *Conductor) safeProgress(fn ProgressFn, up ProgressUpdate) {
	fn(up)
}

// SaveConfig persists the orchestra configuration to a JSON file.
func SaveConfig(filePath string, cfg OrchestraConfig) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("orchestra config dir: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("orchestra config marshal: %w", err)
	}
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("orchestra config write: %w", err)
	}
	log.Printf("ORCHESTRA: config saved to %s", filePath)
	return nil
}

// LoadConfig loads the orchestra configuration from a JSON file.
// Returns the default config if file doesn't exist.
func LoadConfig(filePath string) OrchestraConfig {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("ORCHESTRA: config read error: %v", err)
		}
		return DefaultConfig()
	}
	var cfg OrchestraConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Printf("ORCHESTRA: config parse error: %v, using defaults", err)
		return DefaultConfig()
	}
	// Ensure all built-in roles are present (merge missing roles from defaults)
	cfg.Roles = MergeRoles(cfg.Roles)
	return cfg.Sanitize()
}

// findProviderConfig looks up a provider config by name from the stored configs.
// Falls back to type match for backward compatibility with old configs.
func (c *Conductor) findProviderConfig(providerName string) *provider.ProviderConfig {
	configs := c.getConfigs()
	for _, cfg := range configs {
		if cfg.Name == providerName && cfg.Enabled {
			return &cfg
		}
	}
	for _, cfg := range configs {
		if string(cfg.Type) == providerName && cfg.Enabled {
			return &cfg
		}
	}
	return nil
}

// createProviderForType creates a provider for the given model type and model name.
// Returns a descriptive error if the provider is "local" (unsupported) or not found.
func (c *Conductor) createProviderForType(modelType, modelName string) (provider.Provider, error) {
	// Final guard: trim stray whitespace so a dirty type/model id can never reach
	// the provider HTTP request (see OrchestraConfig.Sanitize).
	modelType = strings.TrimSpace(modelType)
	modelName = strings.TrimSpace(modelName)

	// "local" is not supported in orchestra mode — all roles must use external providers
	if modelType == "" || modelType == "local" {
		enabled := c.getConfigs()
		available := make([]string, 0, len(enabled))
		for _, cfg := range enabled {
			if cfg.Enabled {
				available = append(available, string(cfg.Type))
			}
		}
		availStr := "hiçbiri"
		if len(available) > 0 {
			availStr = strings.Join(available, ", ")
		}
		return nil, fmt.Errorf("orkestra modu yerel modelleri desteklemez. Kullanılabilir provider'lar: %s. Lütfen bir provider seçip bu role ata", availStr)
	}

	// First try exact match
	pCfg := c.findProviderConfig(modelType)
	if pCfg != nil {
		cfg := *pCfg // copy to avoid mutating the config manager's internal state
		cfg.Model = modelName
		return c.pf(cfg)
	}

	// Fallback: use any enabled provider (e.g. OpenRouter) but log a warning
	configs := c.getConfigs()
	for _, cfg := range configs {
		if cfg.Enabled {
			log.Printf("ORCHESTRA WARNING: provider %s/%s not found, falling back to %s/%s", modelType, modelName, cfg.Type, cfg.Model)
			cfg.Model = modelName
			return c.pf(cfg)
		}
	}

	var enabledTypes []string
	for _, cfg := range configs {
		if cfg.Enabled {
			enabledTypes = append(enabledTypes, fmt.Sprintf("%s (%s)", cfg.Type, cfg.Model))
		}
	}
	availStr := "hiçbiri etkin değil"
	if len(enabledTypes) > 0 {
		availStr = strings.Join(enabledTypes, ", ")
	}
	return nil, fmt.Errorf("'%s' provider'ı bulunamadı. Mevcut etkin provider'lar: %s. Lütfen API Providers sayfasından '%s' tipinde bir provider ekleyip etkinleştir", modelType, availStr, modelType)
}

// ─── Planning ──────────────────────────────────────────────────

func (c *Conductor) createPlan(ctx context.Context, cfg OrchestraConfig, userMessage string, onProgress ProgressFn) (*OrchestraPlan, error) {
	prov, err := c.createProviderForType(cfg.ChiefType, cfg.ChiefModel)
	if err != nil {
		return nil, fmt.Errorf("chief provider (%s/%s) oluşturulamadı: %w. API key ve provider konfigürasyonunu kontrol et.", cfg.ChiefType, cfg.ChiefModel, err)
	}

	log.Printf("ORCHESTRA: chief planning with %s/%s...", cfg.ChiefType, cfg.ChiefModel)
	roleInfo := c.buildRoleInfo(cfg)
	systemMsg := ChiefSystemPrompt + "\n\nMevcut roller ve yetenekleri:\n" + roleInfo

	req := provider.ChatRequest{
		Model: cfg.ChiefModel,
		Messages: []provider.Message{
			provider.TextMessage("system", systemMsg),
			provider.TextMessage("user", userMessage),
		},
		Temperature: 0.3,
		MaxTokens:   4096,
	}

	planCtx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()

	var content string
	if onProgress != nil {
		// The chief's raw output is JSON — accumulate it INTERNALLY and never
		// stream it to the UI. Showing raw `{"tasks":[...]}` to the user was
		// confusing and unprofessional. A clean ProgressPlanReady is emitted once
		// the plan is parsed below.
		log.Printf("ORCHESTRA: chief opening stream to %s/%s...", cfg.ChiefType, cfg.ChiefModel)
		streamCh, streamErr := prov.ChatCompletionStream(planCtx, req)
		if streamErr != nil {
			return nil, fmt.Errorf("chief (%s/%s) stream başlatılamadı: %w", cfg.ChiefType, cfg.ChiefModel, streamErr)
		}
		log.Printf("ORCHESTRA: chief stream established, waiting for first chunk...")
		var sb strings.Builder
		gotChunk := false
		for chunk := range streamCh {
			if !gotChunk {
				gotChunk = true
				log.Printf("ORCHESTRA: chief first chunk received")
			}
			if chunk.Error != "" {
				return nil, fmt.Errorf("chief (%s/%s) stream hatası: %s", cfg.ChiefType, cfg.ChiefModel, chunk.Error)
			}
			sb.WriteString(chunk.Content)
		}
		if !gotChunk {
			return nil, fmt.Errorf("chief (%s/%s) stream hiç veri döndürmedi — model adı geçerli mi ve sağlayıcı streaming destekliyor mu kontrol et", cfg.ChiefType, cfg.ChiefModel)
		}
		content = strings.TrimSpace(sb.String())
	} else {
		var resp *provider.ChatResponse
		err = callWithRetry(planCtx, fmt.Sprintf("chief/%s", cfg.ChiefType), func() error {
			var innerErr error
			resp, innerErr = prov.ChatCompletion(planCtx, req)
			return innerErr
		})
		if err != nil {
			return nil, fmt.Errorf("chief (%s/%s) yanıt vermedi: %w", cfg.ChiefType, cfg.ChiefModel, err)
		}
		content = strings.TrimSpace(resp.Content)
	}

	log.Printf("ORCHESTRA: chief raw response (%d chars): %s", len(content), content[:min(len(content), 200)])

	content = extractJSON(content)

	var plan OrchestraPlan
	if err := json.Unmarshal([]byte(content), &plan); err != nil {
		log.Printf("ORCHESTRA: chief JSON parse error: %v", err)
		log.Printf("ORCHESTRA: extracted JSON: %s", content)
		return nil, fmt.Errorf("chief JSON parse hatası: %w\nGelen cevap: %s", err, content)
	}

	for i, task := range plan.Tasks {
		// Fill in model info from config
		found := false
		for _, role := range cfg.Roles {
			if role.Role == task.Role && role.Enabled {
				plan.Tasks[i].ModelType = role.ModelType
				plan.Tasks[i].ModelName = role.ModelName
				found = true
				break
			}
		}
		if !found {
			originalRole := task.Role
			// Chief picked a disabled/missing role → fall back to first enabled role
			for _, role := range cfg.Roles {
				if role.Enabled {
					plan.Tasks[i].Role = role.Role
					plan.Tasks[i].ModelType = role.ModelType
					plan.Tasks[i].ModelName = role.ModelName
					found = true
					log.Printf("ORCHESTRA WARNING: chief assigned task to '%s' but it's disabled/missing, falling back to '%s'", originalRole, role.Role)
					if onProgress != nil {
						c.safeProgress(onProgress, ProgressUpdate{
							Type: ProgressError,
							Content: fmt.Sprintf("⚠️ Chief %s rolünü seçti ama bu rol etkin değil. %s rolüne yönlendirildi.\n", originalRole, role.Role),
						})
					}
					break
				}
			}
		}
		if !found {
			return nil, fmt.Errorf("hiçbir rol etkin değil, orkestrasyon çalıştırılamaz")
		}
	}

	// Plan is ready — hand the UI a clean, structured breakdown (never raw JSON).
	if onProgress != nil {
		c.safeProgress(onProgress, ProgressUpdate{Type: ProgressPlanReady, Tasks: plan.Tasks})
	}

	return &plan, nil
}

// ─── Execution ─────────────────────────────────────────────────

func (c *Conductor) executeTasks(ctx context.Context, cfg OrchestraConfig, plan OrchestraPlan, onProgress ProgressFn) ([]OrchestraResult, error) {
	results := make([]OrchestraResult, len(plan.Tasks))

	// Check if any task has dependencies
	hasDeps := false
	for _, t := range plan.Tasks {
		if len(t.DependsOn) > 0 {
			hasDeps = true
			break
		}
	}

	if hasDeps || !plan.Parallel {
		c.executeSequential(ctx, cfg, plan.Tasks, results, onProgress)
	} else {
		c.executeParallel(ctx, cfg, plan.Tasks, results, onProgress)
	}
	return results, nil
}

func (c *Conductor) executeSequential(ctx context.Context, cfg OrchestraConfig, tasks []OrchestraTask, results []OrchestraResult, onProgress ProgressFn) {
	completed := make(map[int]bool) // keyed by task index, not role name
	remaining := make([]int, len(tasks))
	for i := range tasks {
		remaining[i] = i
	}

	maxIterations := len(tasks) * 2 // safety: avoid infinite loop on circular deps
	for iteration := 0; iteration < maxIterations && len(remaining) > 0; iteration++ {
		progress := false
		for i := 0; i < len(remaining); i++ {
			idx := remaining[i]
			task := tasks[idx]

			// Check if all dependencies are met (resolve DependsOn role names to task indices)
			depsMet := true
			for _, dep := range task.DependsOn {
				depResolved := false
				for j, t := range tasks {
					if string(t.Role) == dep && completed[j] {
						depResolved = true
						break
					}
				}
				if !depResolved {
					depsMet = false
					break
				}
			}
			if !depsMet {
				continue
			}

			// Sequential tasks stream token-by-token — only one runs at a time
			// so the live output stays coherent.
			results[idx] = c.executeSingleTask(ctx, cfg, task, idx, onProgress, true)
			completed[idx] = true

			remaining = append(remaining[:i], remaining[i+1:]...)
			i-- // adjust index after removal
			progress = true
			break
		}
		if !progress {
			// No task could be executed — circular dependency or unresolvable deps
			var pending []string
			for _, idx := range remaining {
				t := tasks[idx]
				pending = append(pending, fmt.Sprintf("task[%d]%s(depends_on:%v)", idx, t.Role, t.DependsOn))
			}
			log.Printf("ORCHESTRA: deadlock in sequential execution, pending: %v", pending)
			// Mark remaining as failed
			for _, idx := range remaining {
				results[idx] = OrchestraResult{
					TaskIndex:  idx,
					Role:       tasks[idx].Role,
					ModelType:  tasks[idx].ModelType,
					ModelName:  tasks[idx].ModelName,
					Error:      fmt.Sprintf("deadlock: dependency never satisfied (task[%d] role=%s, needs=%v)", idx, tasks[idx].Role, tasks[idx].DependsOn),
					DurationMs: 0,
				}
			}
			break
		}
	}
}

func (c *Conductor) executeParallel(ctx context.Context, cfg OrchestraConfig, tasks []OrchestraTask, results []OrchestraResult, onProgress ProgressFn) {
	var wg sync.WaitGroup
	for i, task := range tasks {
		wg.Add(1)
		go func(idx int, t OrchestraTask) {
			defer wg.Done()
			// Parallel tasks must NOT stream token-by-token: concurrent streams
			// would interleave into one garbled blob. Run without chunk streaming
			// so each task's full result is emitted atomically on completion.
			results[idx] = c.executeSingleTask(ctx, cfg, t, idx, onProgress, false)
		}(i, task)
	}
	wg.Wait()
}

func (c *Conductor) executeSingleTask(ctx context.Context, cfg OrchestraConfig, task OrchestraTask, index int, onProgress ProgressFn, streamChunks bool) OrchestraResult {
	start := time.Now()
	log.Printf("ORCHESTRA: task %d starting: role=%s model=%s/%s", index, task.Role, task.ModelType, task.ModelName)

	if onProgress != nil {
		c.safeProgress(onProgress, ProgressUpdate{Type: ProgressTaskStart, Role: task.Role, Index: index, ModelType: task.ModelType, ModelName: task.ModelName, Content: fmt.Sprintf("🎯 **%s** (%s/%s) çalışıyor...\n", task.Role, task.ModelType, task.ModelName)})
	}

	var systemPrompt string
	for _, role := range cfg.Roles {
		if role.Role == task.Role {
			systemPrompt = role.SystemPrompt
			break
		}
	}

	result := OrchestraResult{
		TaskIndex: index,
		Role:      task.Role,
		ModelType: task.ModelType,
		ModelName: task.ModelName,
	}

	p, err := c.createProviderForType(task.ModelType, task.ModelName)
	if err != nil {
		errMsg := fmt.Sprintf("%s (%s) provider hatası: %v. Provider'ı API Providers'dan kontrol et.", task.ModelType, task.ModelName, err)
		log.Printf("ORCHESTRA: %s", errMsg)
		result.Error = errMsg
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}

	messages := []provider.Message{
		provider.TextMessage("system", systemPrompt),
	}
	contextMsg := task.Context
	if contextMsg == "" {
		contextMsg = task.Prompt // fallback for backward compatibility
	}
	messages = append(messages, provider.TextMessage("user", contextMsg))

	req := provider.ChatRequest{
		Model: task.ModelName,
		Messages:    messages,
		Temperature: 0.7,
		MaxTokens:   8192,
	}

	taskCtx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()

	if onProgress != nil && streamChunks {
		streamCh, streamErr := p.ChatCompletionStream(taskCtx, req)
		if streamErr == nil {
			var sb strings.Builder
			var lastErr error
			for chunk := range streamCh {
				if chunk.Error != "" {
					lastErr = fmt.Errorf("stream error: %s", chunk.Error)
					break
				}
				sb.WriteString(chunk.Content)
				c.safeProgress(onProgress, ProgressUpdate{
					Type:      ProgressTaskChunk,
					Role:      task.Role,
					Index:     index,
					ModelType: task.ModelType,
					ModelName: task.ModelName,
					Content:   chunk.Content,
				})
			}
			result.DurationMs = time.Since(start).Milliseconds()
			result.Content = sb.String()
			result.TokensIn = estimateTokens(systemPrompt + contextMsg)
			result.TokensOut = estimateTokens(sb.String())
			if lastErr == nil {
				if onProgress != nil {
					c.safeProgress(onProgress, ProgressUpdate{
						Type:       ProgressTaskDone,
						Role:       task.Role,
						Index:      index,
						ModelType:  task.ModelType,
						ModelName:  task.ModelName,
						DurationMs: result.DurationMs,
						Content:    sb.String(),
					})
				}
				return result
			}
			result.Error = lastErr.Error()
			if onProgress != nil {
				c.safeProgress(onProgress, ProgressUpdate{
					Type:       ProgressTaskDone,
					Role:       task.Role,
					Index:      index,
					ModelType:  task.ModelType,
					ModelName:  task.ModelName,
					DurationMs: result.DurationMs,
					Error:      lastErr.Error(),
				})
			}
			return result
		}
		log.Printf("ORCHESTRA: stream start failed for %s/%s, falling back to non-streaming: %v", task.ModelType, task.ModelName, streamErr)
	}

	var resp *provider.ChatResponse
	err = c.retryTask(taskCtx, fmt.Sprintf("task/%s/%s", task.Role, task.ModelType), func() error {
		var innerErr error
		resp, innerErr = p.ChatCompletion(taskCtx, req)
		return innerErr
	})
	result.DurationMs = time.Since(start).Milliseconds()
	result.TokensIn = estimateTokens(systemPrompt + contextMsg)
	if err != nil {
		log.Printf("ORCHESTRA: task %d (%s/%s) error: %v", index, task.ModelType, task.ModelName, err)
		result.Error = err.Error()
		if onProgress != nil {
			c.safeProgress(onProgress, ProgressUpdate{Type: ProgressTaskDone, Role: task.Role, Index: index, ModelType: task.ModelType, ModelName: task.ModelName, DurationMs: result.DurationMs, Error: err.Error()})
		}
	} else {
		log.Printf("ORCHESTRA: task %d (%s/%s) OK %dms", index, task.ModelType, task.ModelName, result.DurationMs)
		result.Content = resp.Content
		result.TokensOut = estimateTokens(resp.Content)
		if onProgress != nil {
			c.safeProgress(onProgress, ProgressUpdate{Type: ProgressTaskDone, Role: task.Role, Index: index, ModelType: task.ModelType, ModelName: task.ModelName, DurationMs: result.DurationMs, Content: resp.Content})
		}
	}
	return result
}

// retryTask calls fn, retrying on all errors (including rate limits) up to 2 times.
// For rate limit errors it uses callWithRetry internally; for other errors it uses a simple backoff.
// Total attempts: up to maxRetries + 1 on top of callWithRetry's own retries.
func (c *Conductor) retryTask(ctx context.Context, label string, fn func() error) error {
	maxRetries := 2
	baseDelay := 3 * time.Second

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			delay := baseDelay * time.Duration(1<<(attempt-1))
			log.Printf("ORCHESTRA: %s failed, retrying in %v (attempt %d/%d): %v", label, delay, attempt, maxRetries, lastErr)
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
		err := fn()
		if err == nil {
			return nil
		}
		lastErr = err
		// For rate limit errors, use callWithRetry's specialized backoff instead
		if isRateLimitError(err) {
			return callWithRetry(ctx, label, fn)
		}
	}
	return fmt.Errorf("%v (retried %d times)", lastErr, maxRetries)
}

// ─── Synthesis ────────────────────────────────────────────────

func (c *Conductor) synthesize(ctx context.Context, cfg OrchestraConfig, userMessage string, tasks []OrchestraTask, results []OrchestraResult, onProgress ProgressFn) (string, error) {
	// Check task results
	successCount := 0
	for _, r := range results {
		if r.Error == "" && r.Content != "" {
			successCount++
		}
	}

	if successCount == 0 {
		var sb strings.Builder
		sb.WriteString("Orkestrasyon tamamlandı ancak tüm görevler başarısız:\n\n")
		for _, r := range results {
			sb.WriteString(fmt.Sprintf("- %s (%s): %s\n", r.Role, r.ModelType, r.Error))
		}
		return sb.String(), nil
	}

	var sb strings.Builder
	sb.WriteString("Kullanıcının orijinal mesajı:\n")
	sb.WriteString(userMessage)
	sb.WriteString("\n\nUzmanlara dağıtılan görevler ve sonuçları:\n\n")

	for i, task := range tasks {
		sb.WriteString(fmt.Sprintf("=== Görev %d: %s (%s) ===\n", i+1, task.Role, task.ModelType))
		if results[i].Error != "" {
			sb.WriteString(fmt.Sprintf("HATA: %s\n", results[i].Error))
			sb.WriteString("Bu görev başarısız oldu, atlandı.\n")
		} else {
			sb.WriteString(fmt.Sprintf("Görev: %s\n", task.Context))
			sb.WriteString("Sonuç:\n")
			sb.WriteString(results[i].Content)
		}
		sb.WriteString("\n\n")
	}
	sb.WriteString("Yukarıdaki sonuçları kullanarak kullanıcıya tek bir tutarlı cevap oluştur. Başarısız görevleri belirt. Cevabı kısa ve öz tut.")

	synthesisMsg := `Sen bir Orkestra Şefi'sin. Alt uzmanların sonuçlarını alıp kullanıcıya tek bir cevap olarak sunuyorsun.
Kurallar:
- Başarılı sonuçları birleştir
- Başarısız görevleri belirt
- Kullanıcının sorusuna doğrudan cevap ver
- Doğal ve akıcı ol
- Cevabı kısa tut`

	p, err := c.createProviderForType(cfg.ChiefType, cfg.ChiefModel)
	if err != nil {
		return "", fmt.Errorf("synthesis provider: %w", err)
	}

	req := provider.ChatRequest{
		Model: cfg.ChiefModel,
		Messages: []provider.Message{
			provider.TextMessage("system", synthesisMsg),
			provider.TextMessage("user", sb.String()),
		},
		Temperature: 0.5,
		MaxTokens:   2048,
	}

	var finalContent string
	synthCtx, synthCancel := context.WithTimeout(ctx, 300*time.Second)
	defer synthCancel()

	if onProgress != nil {
		// Stream the synthesis response
		streamCh, streamErr := p.ChatCompletionStream(synthCtx, req)
		if streamErr != nil {
			return "", fmt.Errorf("synthesis stream başlatılamadı: %w", streamErr)
		}
		var sb strings.Builder
		for chunk := range streamCh {
			if chunk.Error != "" {
				return "", fmt.Errorf("synthesis stream hatası: %s", chunk.Error)
			}
			sb.WriteString(chunk.Content)
			c.safeProgress(onProgress, ProgressUpdate{Type: ProgressSynthChunk, Content: chunk.Content})
		}
		finalContent = sb.String()
	} else {
		var resp *provider.ChatResponse
		err = callWithRetry(synthCtx, fmt.Sprintf("synthesis/%s", cfg.ChiefType), func() error {
			var innerErr error
			resp, innerErr = p.ChatCompletion(synthCtx, req)
			return innerErr
		})
		if err != nil {
			return "", fmt.Errorf("synthesis chat completion: %w", err)
		}
		finalContent = resp.Content
	}
	return finalContent, nil
}

// ─── Helpers ───────────────────────────────────────────────────

func (c *Conductor) buildRoleInfo(cfg OrchestraConfig) string {
	var sb strings.Builder
	for _, role := range cfg.Roles {
		if role.Enabled {
			custom := ""
			if !IsBuiltinRole(role.Role) {
				custom = " (custom)"
			}
			prompt := truncateUTF8(role.SystemPrompt, 100)
			sb.WriteString(fmt.Sprintf("- %s%s: model=%s, prompt=%s\n", role.Role, custom, role.ModelType, prompt))
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// estimateTokens provides a rough token count estimate.
// Uses word-based heuristic: ~1.3 tokens per word for mixed text (code + natural language).
// This is more accurate than len(text)/4 which fails for multi-byte and code-heavy text.
func estimateTokens(s string) int {
	if s == "" {
		return 0
	}
	wordCount := len(strings.Fields(s))
	charCount := len(s)
	// Heuristic: tokens ≈ words * 1.3, but ensure minimum based on chars
	fromWords := int(float64(wordCount) * 1.3)
	fromChars := charCount / 6 // lower bound: ~6 chars per token for dense code
	if fromWords > fromChars {
		return fromWords
	}
	return fromChars
}

// truncateUTF8 truncates a string to n runes without breaking multi-byte characters.
func truncateUTF8(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}

func extractJSON(text string) string {
	// Strategy 1: ```json ... ``` block
	if idx := strings.Index(text, "```json"); idx >= 0 {
		start := idx + 7
		if end := strings.Index(text[start:], "```"); end >= 0 {
			return strings.TrimSpace(text[start : start+end])
		}
	}
	// Strategy 2: ``` ... ``` block (any language or no language tag)
	if idx := strings.Index(text, "```"); idx >= 0 {
		start := idx + 3
		if nl := strings.Index(text[start:], "\n"); nl >= 0 {
			start += nl + 1
		}
		if end := strings.Index(text[start:], "```"); end >= 0 {
			extracted := strings.TrimSpace(text[start : start+end])
			// Validate it looks like JSON
			if strings.HasPrefix(extracted, "{") || strings.HasPrefix(extracted, "[") {
				return extracted
			}
		}
	}
	// Strategy 3: bare JSON object in the text (find matching braces)
	if idx := strings.Index(text, "{"); idx >= 0 {
		depth := 0
		for i := idx; i < len(text); i++ {
			switch text[i] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					return text[idx : i+1]
				}
			}
		}
	}
	// Strategy 4: bare JSON array
	if idx := strings.Index(text, "["); idx >= 0 {
		depth := 0
		for i := idx; i < len(text); i++ {
			switch text[i] {
			case '[':
				depth++
			case ']':
				depth--
				if depth == 0 {
					return text[idx : i+1]
				}
			}
		}
	}
	return text
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// retryPattern matches "try again in Xs" or "try again in X.Ys" from rate limit errors.
var retryPattern = regexp.MustCompile(`try again in ([\d.]+)s`)

// callWithRetry calls fn, retrying on rate limit errors with exponential backoff.
func callWithRetry(ctx context.Context, label string, fn func() error) error {
	maxRetries := 3
	baseDelay := 5 * time.Second

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Parse retry duration from error if available
			delay := baseDelay * time.Duration(1<<(attempt-1)) // exponential: 5s, 10s, 20s
			if matches := retryPattern.FindStringSubmatch(lastErr.Error()); len(matches) > 1 {
				if d, err := time.ParseDuration(matches[1] + "s"); err == nil {
					delay = d + time.Duration(rand.Intn(3))*time.Second // add jitter
				}
			}
			log.Printf("ORCHESTRA: %s rate limited, retrying in %v (attempt %d/%d)", label, delay, attempt, maxRetries)

			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}

		err := fn()
		if err == nil {
			return nil
		}
		lastErr = err

		if !isRateLimitError(err) {
			return err
		}
	}
	return fmt.Errorf("%w after %d retries: %v", provider.ErrRateLimited, maxRetries, lastErr)
}

// isRateLimitError checks if an error is a rate limit error.
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	if strings.Contains(err.Error(), "rate_limit_exceeded") ||
		strings.Contains(err.Error(), "Rate limit") ||
		strings.Contains(err.Error(), "rate limit") ||
		strings.Contains(err.Error(), "Request too large") ||
		strings.Contains(err.Error(), "Too Many Requests") ||
		strings.Contains(err.Error(), "TooManyRequests") ||
		strings.Contains(err.Error(), "Please slow down") ||
		strings.Contains(err.Error(), "slowdown") ||
		strings.Contains(err.Error(), "429") ||
		strings.Contains(err.Error(), "503") {
		return true
	}
	return false
}
