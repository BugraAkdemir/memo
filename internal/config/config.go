package config

import (
	"fmt"
	"memo/internal/fileutil"
	"memo/internal/logx"
	"memo/internal/remoteauth"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

var (
	dataDirOnce sync.Once
	dataDirVal  string
)

// DataDir returns the writable base directory for Memo's persistent data
// (sessions, memory, models, providers, etc.).
//
// On Windows the application is normally installed under Program Files, which is
// read-only for standard (non-admin) users, so a process-relative "data" folder
// there cannot be written. Instead data lives under %ProgramData%\Memo\data —
// the same location installer.iss pre-creates with users-full permissions.
//
// On Linux/macOS the historical process-relative "data" directory is kept so
// portable (AppImage) and development runs are unaffected.
//
// The MEMO_DATA_DIR environment variable overrides the resolved location.
func DataDir() string {
	dataDirOnce.Do(func() {
		dataDirVal = resolveDataDir()
	})
	return dataDirVal
}

// DataPath joins one or more elements onto the resolved data directory.
func DataPath(elem ...string) string {
	return filepath.Join(append([]string{DataDir()}, elem...)...)
}

// ConfigDir returns the directory that holds config.yaml. It is the "config"
// sibling of the data directory's parent, so it tracks DataDir automatically:
//   - Linux/macOS:      "config" (relative, next to "data")
//   - Windows installer: %ProgramData%\Memo\config
//   - runner workspaces: <MEMO_DATA_DIR parent>\config (e.g. %USERPROFILE%\.memo\config)
//
// This keeps config readable AND writable in every distribution mode, since the
// install directory (Program Files) is read-only for standard users.
func ConfigDir() string {
	return filepath.Join(filepath.Dir(DataDir()), "config")
}

// ConfigFilePath returns the full path to config.yaml under ConfigDir.
func ConfigFilePath() string {
	return filepath.Join(ConfigDir(), "config.yaml")
}

func resolveDataDir() string {
	if v := strings.TrimSpace(os.Getenv("MEMO_DATA_DIR")); v != "" {
		return v
	}
	if runtime.GOOS == "windows" {
		base := os.Getenv("ProgramData")
		if base == "" {
			base = os.Getenv("ALLUSERSPROFILE")
		}
		if base != "" {
			return filepath.Join(base, "Memo", "data")
		}
	}
	return "data"
}

type AppConfig struct {
	API            APIConfig          `yaml:"api"`
	Identity       IdentityConfig     `yaml:"identity"`
	Memory         MemoryConfig       `yaml:"memory"`
	RemoteAccess   RemoteAccessConfig `yaml:"remote_access"`
	Llama          LlamaConfig        `yaml:"llama"`
	Whisper        WhisperConfig      `yaml:"whisper" json:"whisper"`
	TTS            TTSConfig          `yaml:"tts" json:"tts"`
	Sync           SyncConfig         `yaml:"sync"`
	WhatsApp       WhatsAppConfig     `yaml:"whatsapp"`
	Proactive      ProactiveConfig    `yaml:"proactive" json:"proactive"`
	Learning       LearningConfig     `yaml:"learning" json:"learning"`
	Calendar       CalendarConfig     `yaml:"calendar" json:"calendar"`
	Mood           MoodConfig         `yaml:"mood" json:"mood"`
	WebSearch      WebSearchConfig    `yaml:"web_search" json:"web_search"`
	AgentMode      AgentModeConfig    `yaml:"agent_mode" json:"agent_mode"`
	Browser        BrowserConfig      `yaml:"browser" json:"browser"`
	DevGateway     DevGatewayConfig   `yaml:"dev_gateway" json:"dev_gateway"`
	Swarm          SwarmConfig        `yaml:"swarm" json:"swarm"`
	Onboarding     OnboardingConfig   `yaml:"onboarding" json:"onboarding"`
	LiveMode       LiveModeConfig     `yaml:"live_mode" json:"live_mode"`
	TaskLoop       TaskLoopConfig     `yaml:"task_loop" json:"task_loop"`
	ActiveProvider string             `yaml:"active_provider" json:"active_provider"`

	// Beta gates genuinely experimental features (e.g. Memo Swarm). Off by
	// default: beta features are hidden and never run. The embedded
	// Tailscale tunnel graduated out of Beta and has its own independent
	// on/off toggle (RemoteAccess.Enabled/TunnelMode) — it is unaffected by
	// this flag. Live Mode graduated the same way (see LiveModeConfig) —
	// its own Enabled toggle, no longer gated by Beta.
	Beta bool `yaml:"beta" json:"beta"`
}

// TaskLoopConfig tunes the Self-Driving task loop (v4.4.0). Both switches
// default on: the release theme is full autonomy. Existing installs keep
// whatever their config.yaml carries once Load() overlays it.
type TaskLoopConfig struct {
	// PlanningSelfConfig lets the loop pick this task's provider/model (and
	// optionally toggle Orchestra) during the planning phase instead of just
	// inheriting the global active provider.
	PlanningSelfConfig bool `yaml:"planning_self_config" json:"planning_self_config"`
	// SubAgents allows a large item to fan out to parallel read-only
	// sub-agents plus one writer.
	SubAgents bool `yaml:"sub_agents" json:"sub_agents"`

	// --- v4.5.0 planner/executor mode ("planlayıcı") ---
	// Per-role model, each "provider/model" or a bare model name. Empty means
	// "unset" — resolveRoleModels then reads a Task.md header, an AGENTS.md
	// directive, and finally falls back to the active provider. No role has a
	// location bias.
	PlannerModel  string `yaml:"planner_model" json:"planner_model"`
	CoderModel    string `yaml:"coder_model" json:"coder_model"`
	VerifierModel string `yaml:"verifier_model" json:"verifier_model"`
	// StepGranularity: "intent" | "literal" | "hybrid" (default "hybrid").
	StepGranularity string `yaml:"step_granularity" json:"step_granularity"`
	// AutoApprovePlan skips the Plan.md approval gate.
	AutoApprovePlan bool `yaml:"auto_approve_plan" json:"auto_approve_plan"`
	// TaskMemory: when false, planner/executor turns skip the RAG/memory block.
	TaskMemory bool `yaml:"task_memory" json:"task_memory"`
	// MaxExecutorAttempts before a step escalates (default 3).
	MaxExecutorAttempts int `yaml:"max_executor_attempts" json:"max_executor_attempts"`
	// MaxParallelSteps run at once (default 3).
	MaxParallelSteps int `yaml:"max_parallel_steps" json:"max_parallel_steps"`
	// HandoffStateMaxTokens before the state doc is compacted (default 2000).
	HandoffStateMaxTokens int `yaml:"handoff_state_max_tokens" json:"handoff_state_max_tokens"`
	// StreamIdleTimeoutSec: a planner/coder/escalator LLM stream is aborted
	// only after this many seconds with NO new token (default 300). Total run
	// time is unbounded as long as tokens keep flowing — a 3-hour plan is fine.
	StreamIdleTimeoutSec int `yaml:"stream_idle_timeout_sec" json:"stream_idle_timeout_sec"`
	// AcceptanceCommandTimeoutSec caps a single `command` acceptance check
	// (e.g. `go test ./...`) — default 300.
	AcceptanceCommandTimeoutSec int `yaml:"acceptance_command_timeout_sec" json:"acceptance_command_timeout_sec"`
	// MaxConcurrentLists caps how many task lists may run at the same time.
	// 0 (default) means unlimited — since v4.6.0 lists no longer serialise
	// against each other, so this is only a safety valve for very
	// resource-constrained hosts.
	MaxConcurrentLists int `yaml:"max_concurrent_lists" json:"max_concurrent_lists"`
	// ProviderRoaming is the global default for cross-provider fallback in a
	// running task. false (default) = a task runs ONLY on the provider it
	// started with; if that provider fails, the task parks and notifies rather
	// than silently trying every other entry in data/providers.json. A Task.md
	// "# sağlayıcı: otomatik" header opts one list back into roaming; a
	// "# sağlayıcı: <name>" header pins it to a specific provider.
	ProviderRoaming bool `yaml:"provider_roaming" json:"provider_roaming"`
}

// LiveModeConfig controls voice Live Mode independently of Beta — it
// graduated out of Beta the same way RemoteAccess (embedded Tailscale) did:
// its own on/off toggle instead of piggybacking on the experimental-features
// flag. See docs/plans/PLAN_live_mode_v2.md for the full design.
type LiveModeConfig struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
	// ActiveEngine selects which voice engine powers Live Mode:
	// "local" (existing Whisper+Piper pipeline, config.Whisper/config.TTS),
	// "google_live", "openai_realtime", "elevenlabs", or "custom". Per-engine
	// API keys/model/voice choices live in data/livemode_engines.json
	// (internal/livemode.ConfigManager), not here — this field only says
	// which one is active.
	ActiveEngine string `yaml:"active_engine" json:"active_engine"`
	// WorkMode is meaningful only when ActiveEngine is "google_live" or
	// "openai_realtime" — the two engines with a genuine native
	// audio-to-audio reasoning model that can itself act as the "live
	// model" in Memo's two-model design (see PLAN_live_mode_v2.md §4/§4b):
	//   "delegate"   (default) — the live model's only tool is
	//                 delegate_to_main_model; real work is handed off to
	//                 Memo's existing main chat/agent model and narrated
	//                 back once done.
	//   "standalone" — the live model instead gets the same full tool
	//                 registry agent mode gives the main model, and
	//                 executes tool calls itself, with no second model
	//                 involved at all. A deliberate, user-opted-into
	//                 tradeoff (faster/simpler, but the smaller live model
	//                 gets direct file/command access) — AgentPermissionPolicy
	//                 below is the safety net for it.
	// Ignored (no meaning) for "local"/"elevenlabs"/"custom": those engines
	// have no separate live-model brain at all — transcribed text always
	// goes straight to Memo's existing main model routing.
	WorkMode string `yaml:"work_mode" json:"work_mode"`
	// AgentPermissionPolicy controls how a delegated/standalone tool call's
	// permission prompt (danger level Medium/Dangerous) is resolved during a
	// live voice session:
	//   "voice_prompt"    (default) — Safe tools auto-allow (same as
	//                      PermissionManager's normal behavior); Medium/
	//                      Dangerous tools are asked out loud and resolved
	//                      from the user's spoken answer.
	//   "auto_allow_once" — every request is resolved as AllowOnce
	//                      immediately, still fully audited.
	AgentPermissionPolicy string `yaml:"agent_permission_policy" json:"agent_permission_policy"`
	// BargeInSensitivity controls how eagerly the native realtime engine
	// (google_live; also passed to openai_realtime) treats incoming user
	// audio as an interruption of the model's own speech —
	// Gemini Live's realtimeInputConfig.automaticActivityDetection
	// .startOfSpeechSensitivity:
	//   "high" (default) — interrupt readily; the user can talk over Memo
	//                      and it stops. Best for a quiet room.
	//   "low"            — needs a more confident speech signal before
	//                      interrupting; keyboard/background noise won't cut
	//                      the model off mid-sentence, but a soft or distant
	//                      "stop" may be missed.
	// An empty value is treated as "high". Ignored for local/elevenlabs/
	// custom (no server-side barge-in VAD to configure).
	BargeInSensitivity string `yaml:"barge_in_sensitivity" json:"barge_in_sensitivity"`
}

// OnboardingConfig tracks whether the Flutter client's first-run wizard
// (language/theme/persona/model-download steps, SetupWizardOverlay) has been
// completed. This used to live only in the browser's own SharedPreferences/
// localStorage (per-origin, per-browser) — a client reachable from more than
// one origin (e.g. a LAN IP and a Cloudflare tunnel hostname pointed at the
// same backend) has no shared storage between them, so the wizard reappeared
// on every origin that hadn't independently completed it, even though the
// backend was already fully configured. Completed is a durable server-side
// fact instead, checked by every client regardless of which origin/browser
// it's using.
type OnboardingConfig struct {
	Completed bool `yaml:"completed" json:"completed"`
}

// LearningConfig controls the learning system's model routing. When
// SingleModelEnabled is true the intent extractor and proactive engine both use
// ModelID directly instead of routing through Orchestra.
type LearningConfig struct {
	SingleModelEnabled bool   `yaml:"single_model_enabled" json:"single_model_enabled"`
	ModelID            string `yaml:"model_id" json:"model_id"`
}

// CalendarConfig controls the calendar system.
type CalendarConfig struct {
	// ReminderLeadMinutes is how many minutes before an event the reminder
	// notification fires. Default 30.
	ReminderLeadMinutes int `yaml:"reminder_lead_minutes" json:"reminder_lead_minutes"`

	// DisableTimeGuess turns off creating calendar events when the message did
	// not state an explicit time (e.g. "yarın dışarı çıkalım"). When false
	// (default) Memo infers a time; when true such vague mentions are ignored.
	// The zero value preserves the original inferring behaviour.
	DisableTimeGuess bool `yaml:"disable_time_guess" json:"disable_time_guess"`
}

// WebSearchConfig web arama özelliğini kontrol eder.
type WebSearchConfig struct {
	Enabled    bool `yaml:"enabled" json:"enabled"`
	MaxResults int  `yaml:"max_results" json:"max_results"`
}

// AgentModeConfig persists the agent-mode (tool-execution) toggle so it
// survives a backend restart. Agent mode used to be an in-memory-only flag
// reset to false on every App init, while the Flutter/CLI clients cached
// their last value — so after the desktop app relaunched its bundled
// backend (an update, a --kill/relaunch) the UI still showed agent mode
// "on" while the backend had silently reverted to off, and every message
// routed as a plain toolless reply ("agent mode is off, turn it on...").
// Mirrors WebSearchConfig.Enabled, which is persisted for the same reason.
type AgentModeConfig struct {
	Enabled bool `yaml:"enabled" json:"enabled"`

	// WorkingSetEnabled keeps a per-chat digest of the files and commands an
	// agent turn has already touched this session and injects it (compactly)
	// into later turns, so the model doesn't re-read a file it read two turns
	// ago just because the tool output fell out of history. Default on.
	WorkingSetEnabled bool `yaml:"working_set_enabled" json:"working_set_enabled"`
	// WorkingSetMaxTokens hard-caps that injected digest. Default 600.
	WorkingSetMaxTokens int `yaml:"working_set_max_tokens" json:"working_set_max_tokens"`
}

// BrowserConfig controls the optional headless-browser fallback
// (internal/browserengine) fetch_page uses for JavaScript-rendered pages
// gosearch.Fetch alone can't read.
type BrowserConfig struct {
	// KeepAlive: false (default) launches a fresh browser per fetch and
	// closes it immediately after — zero idle memory. true keeps one
	// browser process alive across fetches instead, trading steady-state
	// RAM (~150-250MB) for lower per-fetch latency.
	KeepAlive bool `yaml:"keep_alive" json:"keep_alive"`
}

// MoodConfig Stokastik Duygu Motorunu kontrol eder.
type MoodConfig struct {
	Enabled          bool    `yaml:"enabled" json:"enabled"`
	Alpha            float64 `yaml:"alpha" json:"alpha"`
	Beta             float64 `yaml:"beta" json:"beta"`
	SigmaMin         float64 `yaml:"sigma_min" json:"sigma_min"`
	SigmaMax         float64 `yaml:"sigma_max" json:"sigma_max"`
	SelfInterest     bool    `yaml:"self_interest" json:"self_interest"`
	SystemManagement bool    `yaml:"system_management" json:"system_management"`
}

// WhisperConfig holds speech-to-text settings for whisper.cpp.
type WhisperConfig struct {
	BinaryPath string `yaml:"binary_path" json:"binary_path"`
	ModelPath  string `yaml:"model_path" json:"model_path"`
	Language   string `yaml:"language" json:"language"` // "auto", "tr", "en"
	Port       int    `yaml:"port" json:"port"`         // default 9877
	Enabled    bool   `yaml:"enabled" json:"enabled"`   // default false — whisper-server + its ~500MB model ship with every install but sit idle until a user opts in from Settings
}

// TTSConfig holds text-to-speech settings for Piper (internal/tts). Unlike
// WhisperConfig there is no Port/Language: Piper has no persistent server
// mode (a one-shot subprocess per call, see internal/tts's package doc),
// and Faz 1 (docs/plans/PLAN_voice_live_mode_faz1.md) has no voice
// auto-selection, so ModelPath must point at a specific .onnx voice file —
// no default is guessed. Enabled defaults to false, unlike Whisper's true:
// the Piper binary/voice model aren't bundled with the app yet at this
// stage of Voice Live Mode's development.
type TTSConfig struct {
	BinaryPath string `yaml:"binary_path" json:"binary_path"`
	ModelPath  string `yaml:"model_path" json:"model_path"` // path to a .onnx voice file; its .onnx.json sidecar must sit alongside it
	Enabled    bool   `yaml:"enabled" json:"enabled"`       // default false
}

// ProactiveConfig controls the learning system's proactive engine. Disabled by
// default (privacy-first): the observer still records, but nothing is suggested.
type ProactiveConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Level   string `yaml:"level" json:"level"` // off | subtle | normal | assertive
}

// SyncConfig holds Google Drive backup settings.
// ClientID and ClientSecret come from the user's own Google Cloud project
// (OAuth 2.0 Desktop App credentials). Passphrase derives the AES-256 key;
// leave empty to use a hardware-derived key automatically.
type SyncConfig struct {
	Enabled          bool   `yaml:"enabled" json:"enabled"`
	ClientID         string `yaml:"client_id" json:"client_id"`
	ClientSecret     string `yaml:"client_secret" json:"client_secret"`
	Passphrase       string `yaml:"passphrase" json:"passphrase,omitempty"`
	TokenPath        string `yaml:"token_path" json:"token_path"`
	IntervalMessages int    `yaml:"interval_messages" json:"interval_messages"`
}

// WhatsAppConfig holds WhatsApp integration settings.
type WhatsAppConfig struct {
	Enabled        bool   `yaml:"enabled" json:"enabled"`
	DataDir        string `yaml:"data_dir" json:"data_dir"`
	AutoIndex      bool   `yaml:"auto_index" json:"auto_index"`
	MaxHistoryDays int    `yaml:"max_history_days" json:"max_history_days"`
	// SelfChatAssistant makes Memo auto-reply, as a normal chat turn, to
	// messages arriving in the user's own WhatsApp "Message Yourself" chat
	// — turning that chat into another interface to talk to Memo. Opt-in
	// (default false): unlike read-only features, this one autonomously
	// sends WhatsApp messages on every self-chat turn, so an existing user
	// upgrading must not have it silently switched on underneath them.
	SelfChatAssistant bool `yaml:"self_chat_assistant" json:"self_chat_assistant"`
	// AutoApprovePermissions, when true, skips the agent's tool-call
	// permission prompt entirely for the WhatsApp self-chat assistant
	// (AllowOnce on every request) instead of asking a y/n question in the
	// chat itself — see the matching field on telegram.State for the
	// Telegram side. Opt-in (default false): the safe default is asking,
	// same reasoning as SelfChatAssistant itself.
	AutoApprovePermissions bool `yaml:"auto_approve_permissions" json:"auto_approve_permissions"`
}

// DevGatewayConfig controls the OpenAI/Anthropic-compatible local API
// gateway (Settings > Developer) — lets external tools (Claude Code via
// ANTHROPIC_BASE_URL, or anything OpenAI-compatible) use whichever
// model/provider is configured in Memo. Both fields default to the more
// permissive/private option: no key required (matches how plain localhost
// access is already unauthenticated elsewhere — see remoteAuthOK), and
// memory left out of gateway requests (an external coding agent's traffic
// should not silently mix into the user's personal RAG memory unless they
// explicitly opt in).
type DevGatewayConfig struct {
	RequireAPIKey bool `yaml:"require_api_key" json:"require_api_key"`
	UseMemory     bool `yaml:"use_memory" json:"use_memory"`
	// SystemPrompt, when non-empty, is appended to every gateway request's
	// system message (see mergeMemoryBlock's merge pattern in
	// internal/app/devgateway.go, which this reuses) — additive on top of
	// whatever the calling tool (Claude Code, etc.) already sent, never a
	// replacement. Deliberately NOT Memo's own identity/persona: the
	// existing design (see injectGatewayMemory's doc comment) is that an
	// external tool supplies its own system prompt and shouldn't have
	// Memo's assistant identity forced into it. This is instead a place for
	// the *local user* to add their own standing instruction to every
	// gateway call (e.g. "always answer in Turkish", "prefer functional
	// style") — their choice, not Memo's.
	SystemPrompt string `yaml:"system_prompt" json:"system_prompt"`
	// Token is the dev gateway's own API key — deliberately independent of
	// RemoteAccess's token/device model below (Faz 2, yapacam.md). Before
	// Faz 2 this feature shared RemoteAccess.Token as a shortcut; that field
	// is now migrated away into hashed per-device records on Load (see
	// migrateLegacyRemoteToken) and would otherwise get regenerated and
	// immediately re-migrated-away on every restart, breaking this token's
	// stability. Deliberately still plaintext, unlike RemoteAccess's device
	// tokens: this key is checked by devGatewayAuthOK against arbitrary
	// local processes (Claude Code, other CLI tools), a lower-stakes,
	// same-machine-by-default threat model than remote network access.
	Token string `yaml:"token" json:"-"`

	// ClaudeCodeCLI tracks the "one-click connect" toggle in the Developer
	// screen (see internal/app/claudecodecli.go) — whether Memo has pointed
	// the Claude Code CLI's own ~/.claude/settings.json env block at this
	// gateway, and what was there before, so disconnecting restores it
	// exactly instead of just deleting the keys.
	ClaudeCodeCLI ClaudeCodeCLIState `yaml:"claude_code_cli" json:"claude_code_cli"`
}

// ClaudeCodeCLIState is internal bookkeeping for
// App.ConnectClaudeCodeCLI/DisconnectClaudeCodeCLI — only Connected is ever
// exposed to the frontend (json:"-" on the rest); the Prev* fields exist
// solely so a disconnect can restore whatever ANTHROPIC_BASE_URL/
// ANTHROPIC_API_KEY the user already had in their settings.json (if any)
// instead of just clearing them.
type ClaudeCodeCLIState struct {
	Connected      bool   `yaml:"connected" json:"connected"`
	PrevBaseURLSet bool   `yaml:"prev_base_url_set" json:"-"`
	PrevBaseURL    string `yaml:"prev_base_url" json:"-"`
	PrevAPIKeySet  bool   `yaml:"prev_api_key_set" json:"-"`
	PrevAPIKey     string `yaml:"prev_api_key" json:"-"`
	// Model is the "type/model-id" string (see gatewayModelsProvider on the
	// frontend) written to settings.json's top-level "model" field so Claude
	// Code sends a model name the gateway actually recognizes instead of its
	// own built-in default (which the gateway rejects — see the
	// "model must be 'type/model-id'" error surfaced in the Live Log).
	// Empty means "leave the model field alone" (no override configured).
	Model        string `yaml:"model" json:"model"`
	PrevModelSet bool   `yaml:"prev_model_set" json:"-"`
	PrevModel    string `yaml:"prev_model" json:"-"`
}

type RemoteAccessConfig struct {
	Enabled        bool   `yaml:"enabled" json:"enabled"`
	Port           int    `yaml:"port" json:"port"`
	NgrokMode      bool   `yaml:"ngrok_mode" json:"ngrok_mode"`
	NgrokToken     string `yaml:"ngrok_token" json:"ngrok_token"`
	NgrokAutoStart bool   `yaml:"ngrok_auto_start" json:"ngrok_auto_start"`

	// Tunnel mode selects how remote access is exposed:
	//   "lan"       — bind 0.0.0.0, reachable on the local network only
	//   "ngrok"     — ngrok tunnel (legacy, ephemeral URL)
	//   "tailscale" — embedded Tailscale (tsnet); stable URL, no extra binary
	TunnelMode        string `yaml:"tunnel_mode" json:"tunnel_mode"`
	TailscaleKey      string `yaml:"tailscale_key" json:"tailscale_key"`
	TailscaleHostname string `yaml:"tailscale_hostname" json:"tailscale_hostname"`
	TailscaleFunnel   bool   `yaml:"tailscale_funnel" json:"tailscale_funnel"`
	// TailscaleConnectedOnce is set the first time the embedded tunnel
	// connects successfully. Interactive-login users (TailscaleKey empty)
	// never have a key for startupTailscale's boot-time gate to check —
	// this is the signal that lets their tunnel auto-reconnect on the next
	// launch too, since tsnet can reauthenticate silently from the node
	// identity already persisted in StateDir without a fresh browser login.
	TailscaleConnectedOnce bool `yaml:"tailscale_connected_once" json:"tailscale_connected_once"`

	// AuthMode selects how a request against a 0.0.0.0-bound listener must
	// authenticate (see internal/webserver's remoteAuthOK):
	//   "none"           — no credential required at all (must still be
	//                      surfaced as a loud warning wherever this mode is
	//                      shown — see warnAuthDisabled)
	//   "token"          — a registered device token only (legacy behavior)
	//   "password"       — Username+PasswordHash login, session token only
	//   "token_password" — either a device token OR a valid session token
	//                      (OR logic — whichever the connecting client has)
	AuthMode string `yaml:"auth_mode" json:"auth_mode"`
	// Username/PasswordHash back "password"/"token_password" modes. There is
	// deliberately no multi-user model here (see yapacam.md Faz 2): this is
	// one person's own choice of how to authenticate to their own server,
	// not separate accounts with separate data. PasswordHash is always an
	// argon2id-encoded hash (internal/remoteauth.HashPassword) — the plain
	// password itself is never persisted anywhere.
	Username     string `yaml:"username" json:"username"`
	PasswordHash string `yaml:"password_hash" json:"-"`
	// Devices replaces the single shared Token below with one hashed record
	// per paired device/client, so one device's access can be revoked
	// without rotating every other device's credential. Token is kept only
	// so an existing pre-Faz-2 config.yaml still parses; Load() migrates any
	// plaintext value found there into a Device record and blanks it — see
	// migrateLegacyRemoteToken.
	Devices []RemoteDevice `yaml:"devices" json:"-"`
	Token   string         `yaml:"token" json:"-"`

	// Accounts is the multi-user/role model added in Faz 5.1 (yapacam.md) —
	// a deliberately separate concept from both Devices (per-client tokens,
	// no identity/role at all) and the legacy Username/PasswordHash pair
	// above (one person's own single credential, no role). Once Accounts is
	// non-empty, LoginRemotePassword/ValidateRemoteSession/SessionRole
	// authenticate exclusively against it and the legacy fields become
	// display-only leftovers of whichever config a pre-Faz-5.1 install had;
	// see migrateLegacyRemoteAccount for how an existing single-credential
	// setup becomes this list's first ("admin") entry automatically, with
	// no action required from the user and no loss of access.
	Accounts []Account `yaml:"accounts" json:"-"`

	// SetupBootstrapped is set once the token-only first-run path
	// (App.BootstrapTokenAuth, POST /api/setup/create-device) completes.
	// NeedsSetup() otherwise only ever looks at Accounts/Username, both of
	// which the token-only path deliberately never touches — without this
	// flag, a token-only install would report needs_setup=true forever and
	// every unauthenticated bootstrap endpoint gated on NeedsSetup() would
	// stay reachable indefinitely instead of closing after first use, the
	// same way create-admin's does via Accounts. The admin-account path
	// doesn't need this (Accounts alone already flips NeedsSetup() false),
	// but BootstrapTokenAuth sets it too for symmetry, so NeedsSetup() has a
	// single, complete definition of "first run already happened" no matter
	// which of the two bootstrap paths a client took.
	SetupBootstrapped bool `yaml:"setup_bootstrapped" json:"-"`
}

// RemoteDevice is one paired client's access record. Only TokenHash is ever
// persisted for the credential itself — the plaintext token is shown to the
// user exactly once, at the moment it's generated, and never again.
type RemoteDevice struct {
	ID         string    `yaml:"id" json:"id"`
	Name       string    `yaml:"name" json:"name"`
	TokenHash  string    `yaml:"token_hash" json:"-"`
	CreatedAt  time.Time `yaml:"created_at" json:"created_at"`
	LastSeenAt time.Time `yaml:"last_seen_at,omitempty" json:"last_seen_at,omitempty"`
}

// Account is one login identity under the Faz 5.1 multi-user model — a
// username+password pair with a Role ("admin" or "user") that gates
// server-management endpoints (see internal/webserver's admin-only
// handlers). PasswordHash is always argon2id (internal/remoteauth.
// HashPassword); the plain password is never persisted. The first account
// ever created (via POST /api/setup/create-admin, only reachable while the
// list is empty — see App.NeedsSetup) is always Role "admin"; every
// account after that is created by an existing admin, who chooses the role.
type Account struct {
	ID           string    `yaml:"id" json:"id"`
	Username     string    `yaml:"username" json:"username"`
	PasswordHash string    `yaml:"password_hash" json:"-"`
	Role         string    `yaml:"role" json:"role"`
	CreatedAt    time.Time `yaml:"created_at" json:"created_at"`
	// Permissions is the granular, per-capability allow-list (Faz 5.1.1)
	// layered on top of Role — only ever consulted for Role "user" (an
	// "admin" account always has every capability regardless of what this
	// struct holds; see internal/app/remote_auth.go's EffectivePermissions).
	// Every field defaults to zero value (false) when omitted, so a freshly
	// created "user" account with no boxes checked is maximally
	// restricted — chat only — rather than maximally permissive; an admin
	// has to deliberately opt a capability in, not opt it out.
	Permissions AccountPermissions `yaml:"permissions" json:"permissions"`
}

// AccountPermissions is the checkbox list an admin fills in when creating
// or editing a "user"-role Account (Faz 5.1.1, yapacam.md — "bol
// checkboxlu bir yetki sistemi"). Each field gates one feature area's
// mutating/management actions (internal/webserver's requirePermission) —
// plain chat itself is never gated by any of these, so an account with
// every box left unchecked can still talk to Memo, just nothing else.
// Memory is the one exception that also gates *reads*, not just writes
// (internal/webserver's requirePermissionStrict) — the point of denying it
// is that the account can't see the shared memory contents at all, not
// merely that it can't edit them.
type AccountPermissions struct {
	// Models gates switching/testing external providers and
	// searching/downloading/importing/starting/stopping local models —
	// in practice, entering the Providers Settings tab or the Model Store
	// screen in any way that changes something.
	Models bool `yaml:"models" json:"models"`
	// Memory gates the entire /api/memory/* surface, reads included — see
	// this type's own doc comment for why this one differs from the rest.
	Memory bool `yaml:"memory" json:"memory"`
	// Agent gates entering/using agent (tool-execution) chats and changing
	// agent-mode/auto-permission settings. Known limitation: agent-mode
	// itself is still a single global flag shared by the whole backend
	// process (Faz 5.2 in yapacam.md is what eventually gives each account
	// its own isolated state) — denying this stops the account from
	// turning agent mode on itself or opening an agent chat, but if agent
	// mode already happens to be on globally (left on by another session),
	// a plain chat message from this account still routes through it. Not
	// a silent gap: flagged here and in the yapacam.md/handoff entry that
	// shipped this.
	Agent bool `yaml:"agent" json:"agent"`
	// Calendar gates creating/editing/deleting calendar events and
	// changing calendar settings.
	Calendar bool `yaml:"calendar" json:"calendar"`
	// WhatsApp gates connecting/disconnecting the WhatsApp bridge, sending
	// messages, and the self-chat assistant toggle.
	WhatsApp bool `yaml:"whatsapp" json:"whatsapp"`
	// Telegram mirrors WhatsApp for the Telegram bot bridge.
	Telegram bool `yaml:"telegram" json:"telegram"`
	// Routines gates creating/editing/deleting scheduled routines.
	Routines bool `yaml:"routines" json:"routines"`
}

type APIConfig struct {
	BaseURL        string `yaml:"base_url"`
	EmbeddingModel string `yaml:"embedding_model"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
}

type LlamaConfig struct {
	EngineMode       string  `yaml:"engine_mode" json:"engine_mode"`               // "auto", "cpu", "nvidia", "amd", "metal"
	BinaryPath       string  `yaml:"binary_path" json:"binary_path"`               // path to llama-server, auto-detected if empty
	Port             int     `yaml:"port" json:"port"`                             // default 8081
	EmbeddingPort    int     `yaml:"embedding_port" json:"embedding_port"`         // default 8082
	CtxSize          int     `yaml:"ctx_size" json:"ctx_size"`                     // default 4096
	MaxHistory       int     `yaml:"max_history" json:"max_history"`               // default 20 (legacy, use MaxContextTokens instead)
	MaxContextTokens int     `yaml:"max_context_tokens" json:"max_context_tokens"` // default 0 = auto (128K for external, CtxSize for local)
	ModelsDir        string  `yaml:"models_dir" json:"models_dir"`                 // default "./data/models"
	Temperature      float64 `yaml:"temperature" json:"temperature"`               // default 0.7
	TopP             float64 `yaml:"top_p" json:"top_p"`                           // default 0.9
	MaxTokens        int     `yaml:"max_tokens" json:"max_tokens"`                 // default 0 (no limit)

	// EmbeddingGPULayers controls how many layers the dedicated embedding
	// llama-server (a.llamaEmbedServer, a separate process from the chat
	// model's own server) offloads to the GPU when auto-started
	// (autoStartEmbeddingModel / startupEmbeddingModel). Defaults to 0 (CPU
	// only) rather than llama.Server.Start's usual -1 ("auto-detect max
	// layers for this GPU"): that auto-detect only ever considers total
	// VRAM, with no idea the chat model's own server is already resident —
	// on any GPU without generous headroom, both servers independently
	// sizing themselves as if they were the only model running reliably
	// oversubscribes VRAM, forcing the chat model into partial CPU
	// fallback and tanking its generation speed for the entire session
	// (observed: ~10 tok/s with RAG off degrading to ~2-3 tok/s with it
	// on). Embedding models are tiny (typically 100-400M params) and fast
	// enough on CPU alone that this trade is a clear win by default. A
	// user with real VRAM headroom to spare can still set this explicitly
	// (config.yaml, or per-launch via the manual "start model" dialog,
	// which already accepts an arbitrary gpuLayers value uncoupled from
	// this default).
	EmbeddingGPULayers int `yaml:"embedding_gpu_layers" json:"embedding_gpu_layers"`
}

// SwarmConfig controls Memo Swarm — pooling multiple machines' compute via
// llama.cpp's RPC backend to run one model too large for any single one of
// them (see PLAN_memo_swarm.md). Gated behind AppConfig.Beta, same as the
// embedded Tailscale tunnel.
type SwarmConfig struct {
	// RPCPort is the port this machine's rpc-server binds to when acting as
	// a worker ("Join"). Default 50052, matching llama.cpp's own default.
	RPCPort int `yaml:"rpc_port" json:"rpc_port"`

	// LastRoomCode is the most recently generated (as host) or joined (as
	// worker) room code — persisted purely as a UX convenience (re-show on
	// the Host screen after a restart, prefill on Join after a reconnect
	// attempt). Never causes an auto-rejoin/auto-host on startup.
	LastRoomCode string `yaml:"last_room_code" json:"last_room_code"`

	// Role reflects the last-known role, "none" | "host" | "worker" — used
	// only to decide which of the two screens to default-show; never
	// auto-starts anything on launch.
	Role string `yaml:"role" json:"role"`

	// Workers is the host-side persisted worker list (address, label,
	// share%, order) so a host doesn't have to re-add machines after a
	// restart. A re-loaded worker shows as disconnected until it rejoins
	// (worker-initiated registration) — Memo never dials a worker
	// unprompted to "resume."
	Workers []SwarmWorkerConfig `yaml:"workers" json:"workers"`
}

// SwarmWorkerConfig is one host-side worker slot. Order in AppConfig.Swarm.Workers
// matters — it maps positionally to llama-server's --tensor-split ratio order.
type SwarmWorkerConfig struct {
	ID           string  `yaml:"id" json:"id"`
	Label        string  `yaml:"label" json:"label"`
	Address      string  `yaml:"address" json:"address"`
	SharePercent float64 `yaml:"share_percent" json:"share_percent"`
}

// SwarmConfigUpdate is a partial update DTO. RPCPort is a pointer (rather
// than the plain-value fields LlamaConfigUpdate itself mostly uses) so a
// request that omits it entirely leaves the stored port untouched, instead
// of an absent field decoding to 0 and getting silently clamped back to the
// default by validate() on every save.
type SwarmConfigUpdate struct {
	RPCPort *int `json:"rpc_port"`
}

// LlamaConfigUpdate is a partial update for llama.cpp settings, decoded
// directly from the PUT /api/models/config request body. Temperature/TopP/
// MaxTokens are pointers because 0 is a legitimate, meaningful value for
// each (greedy decoding, "no limit" respectively) and must be distinguishable
// from "the caller didn't include this field" — which a plain value field
// merged with a `!= 0` check cannot express: it would silently ignore an
// explicit request to set the field to zero. The other fields have no
// meaningful zero value (a port or ctx size of 0 is never intentional), so
// they stay plain values.
type LlamaConfigUpdate struct {
	EngineMode    string   `json:"engine_mode"`
	BinaryPath    string   `json:"binary_path"`
	Port          int      `json:"port"`
	EmbeddingPort int      `json:"embedding_port"`
	CtxSize       int      `json:"ctx_size"`
	MaxHistory    int      `json:"max_history"`
	ModelsDir     string   `json:"models_dir"`
	Temperature   *float64 `json:"temperature"`
	TopP          *float64 `json:"top_p"`
	MaxTokens     *int     `json:"max_tokens"`
}

type IdentityConfig struct {
	UserName        string `yaml:"user_name"`
	AssistantName   string `yaml:"assistant_name"`
	Style           string `yaml:"style"`
	SystemRole      string `yaml:"system_role"`
	IncognitoPrompt string `yaml:"incognito_prompt"`
	// MinimalMode strips identity/origin/style injection from the system
	// prompt entirely — only memory context (if memory is separately
	// enabled) is still included. For users who want zero prompt overhead
	// on a tight local-model context budget. Off by default (false), since
	// most users want the persona/identity behavior.
	MinimalMode bool `yaml:"minimal_mode"`
	// Granular Minimal Mode overrides — each only has any effect while
	// MinimalMode is true. They let a user keep Minimal Mode's overall
	// "strip everything" intent while selectively re-enabling one category,
	// instead of the only alternative being to turn Minimal Mode off
	// entirely and get everything back. Each defaults to false (off),
	// matching Minimal Mode's original all-or-nothing behavior for anyone
	// who never opens the per-category breakdown.
	MinimalModeKeepPersona      bool `yaml:"minimal_mode_keep_persona"`
	MinimalModeKeepCapabilities bool `yaml:"minimal_mode_keep_capabilities"`
	MinimalModeKeepPassive      bool `yaml:"minimal_mode_keep_passive"`
	MinimalModeKeepProactive    bool `yaml:"minimal_mode_keep_proactive"`
	// LearnedStyleNotes is a short paragraph describing the user's
	// communication style/personality, produced by the "import memory from
	// another AI" feature (internal/app/memory_import.go) — injected into
	// BuildSystemPrompt alongside the fixed Style instructions.
	LearnedStyleNotes string `yaml:"learned_style_notes"`
	// UILanguage is "tr" or "en" (empty means unset — the GUI's language
	// toggle has never written it, or this config predates the field). The
	// backend has otherwise never known the GUI's display language at all
	// (frontend/lib/core/l10n.dart's locale choice used to live purely in
	// Flutter's own SharedPreferences) — this field exists specifically so a
	// second client with no SharedPreferences of its own, the terminal REPL
	// (internal/replcli), can follow the same language the GUI is set to
	// instead of always defaulting to Turkish.
	UILanguage string `yaml:"ui_language"`
}

type MemoryConfig struct {
	PersistDir         string  `yaml:"persist_dir" json:"persist_dir"`
	TopK               int     `yaml:"top_k" json:"top_k"`
	MinSimilarity      float32 `yaml:"min_similarity" json:"min_similarity"`
	MemoryEnabled      bool    `yaml:"memory_enabled" json:"memory_enabled"`
	EmbeddingDimension int     `yaml:"embedding_dimension" json:"embedding_dimension"`
	EmbeddingModelRepo string  `yaml:"embedding_model_repo" json:"embedding_model_repo"`
	EmbeddingModelFile string  `yaml:"embedding_model_file" json:"embedding_model_file"`
	EmbeddingAutoStart bool    `yaml:"embedding_auto_start" json:"embedding_auto_start"`
	// AutoFactExtraction runs a narrow, background LLM call after each saved
	// interaction to pull out durable personal facts (name, birthday, pets,
	// etc.) and pin them so they're injected into every system prompt
	// unconditionally, instead of competing with routine chit-chat under RAG
	// ranking. See internal/app/memory.go's extractAndPinFacts.
	AutoFactExtraction bool `yaml:"auto_fact_extraction" json:"auto_fact_extraction"`
	// FactExtractionEveryNTurns batches the fact-extraction LLM call: it runs
	// once per N saved turns over the combined messages, instead of firing
	// after every turn (which the benchmark measured as roughly one extra LLM
	// request per turn). 1 = every turn, no batching. Default 3.
	FactExtractionEveryNTurns int `yaml:"fact_extraction_every_n_turns" json:"fact_extraction_every_n_turns"`

	// PinnedFactsPerTurn caps how many pinned facts are injected into a single
	// turn's system prompt. Pinned facts used to be dumped in unconditionally
	// (up to a hard 75) with no per-query ranking, which is most of why
	// retrieval returned a near-identical ~40+ memory set every turn
	// regardless of the question. They are now scored by relevance to the
	// current query and only the top N (plus a small always-on recent core)
	// survive — see internal/memory/store.go GetPinnedFactsRanked.
	PinnedFactsPerTurn int `yaml:"pinned_facts_per_turn" json:"pinned_facts_per_turn"`
	// RecencyHalfLifeDays weights retrieval toward newer memories: a memory's
	// similarity score is multiplied by 0.5 once it is this many days old
	// (halving again every further half-life). 0/unset disables recency
	// weighting. See internal/memory/store.go RetrieveContext.
	RecencyHalfLifeDays int `yaml:"recency_half_life_days" json:"recency_half_life_days"`
	// QueryHistoryTurns is how many prior user turns are prepended to the
	// current message when building the retrieval query. The old hardcoded 3
	// blended four turns into one near-static vector, so consecutive turns
	// retrieved almost the same set; 1 keeps enough context for "buna ne
	// demiştik?" follow-ups without smearing the query. See
	// internal/app/helpers.go buildMemoryQuery.
	QueryHistoryTurns int `yaml:"query_history_turns" json:"query_history_turns"`

	// Dream periodically rewrites the pinned-facts set as a whole, merging
	// facts about the same topic into fewer, denser ones (e.g. four separate
	// facts about one pet into a single sentence) instead of letting the set
	// only ever grow — see internal/memory/store.go's runDream. Independent
	// of AutoFactExtraction: that controls whether facts get pinned in the
	// first place, this controls whether the already-pinned set gets
	// periodically compressed.
	DreamEnabled bool `yaml:"dream_enabled" json:"dream_enabled"`
	// DreamInitialDelayMinutes is how long after startup the first Dream
	// pass can run (matches the general consolidation loop's existing
	// 5-minute warmup — see runImportanceDecay).
	DreamInitialDelayMinutes int `yaml:"dream_initial_delay_minutes" json:"dream_initial_delay_minutes"`
	// DreamIntervalHours is how often Dream re-checks after the first pass.
	// A changed value takes effect from the next scheduled check onward, not
	// retroactively — RunDreamNow (manual trigger) exists for "run it right
	// now" instead.
	DreamIntervalHours int `yaml:"dream_interval_hours" json:"dream_interval_hours"`
}

var (
	instance *AppConfig
	once     sync.Once
	mu       sync.RWMutex
	cfgPath  string
)

func Default() *AppConfig {
	return &AppConfig{
		API: APIConfig{
			BaseURL:        "http://127.0.0.1:8081/v1",
			EmbeddingModel: "nomic-embed-text-v1.5",
			TimeoutSeconds: 120,
		},
		Identity: IdentityConfig{
			UserName:        "User",
			AssistantName:   "Memo",
			Style:           "casual",
			SystemRole:      "",
			IncognitoPrompt: "You are Memo, in Incognito Mode. This is a secure session. Never refer to past events, because you have no memory here. Do your best to assist the user right now.",
		},
		Memory: MemoryConfig{
			PersistDir:                "./data/memory",
			TopK:                      8,
			MinSimilarity:             0.35,
			MemoryEnabled:             true,
			EmbeddingDimension:        768,
			EmbeddingModelRepo:        "nomic-ai/nomic-embed-text-v1.5-GGUF",
			EmbeddingModelFile:        "nomic-embed-text-v1.5.Q4_K_M.gguf",
			AutoFactExtraction:        true,
			FactExtractionEveryNTurns: 3,
			PinnedFactsPerTurn:        10,
			RecencyHalfLifeDays:       30,
			QueryHistoryTurns:         1,
			DreamEnabled:              true,
			DreamInitialDelayMinutes:  5,
			DreamIntervalHours:        24,
		},
		RemoteAccess: RemoteAccessConfig{
			Enabled:  false,
			Port:     8090,
			Token:    "",
			AuthMode: "token",
		},
		DevGateway: DevGatewayConfig{
			RequireAPIKey: false,
			UseMemory:     false,
		},
		Llama: LlamaConfig{
			EngineMode:         "auto",
			BinaryPath:         "",
			Port:               8081,
			EmbeddingPort:      8082,
			CtxSize:            8192,
			MaxHistory:         20,
			ModelsDir:          "./data/models",
			Temperature:        0.7,
			TopP:               0.9,
			MaxTokens:          0,
			EmbeddingGPULayers: 0, // CPU by default — see field doc comment
		},
		Whisper: WhisperConfig{
			Enabled:  false,
			Language: "auto",
			Port:     9877,
		},
		TTS: TTSConfig{
			Enabled: false,
		},
		LiveMode: LiveModeConfig{
			Enabled:               false,
			ActiveEngine:          "local",
			WorkMode:              "delegate",
			AgentPermissionPolicy: "voice_prompt",
			BargeInSensitivity:    "high",
		},
		Sync: SyncConfig{
			Enabled:          false,
			TokenPath:        "./data/sync_token.json",
			IntervalMessages: 50,
		},
		WhatsApp: WhatsAppConfig{
			Enabled:           true,
			DataDir:           "./data/whatsapp",
			AutoIndex:         true,
			MaxHistoryDays:    7,
			SelfChatAssistant: false,
		},
		Proactive: ProactiveConfig{
			Enabled: true,
			Level:   "subtle",
		},
		Learning: LearningConfig{
			SingleModelEnabled: false,
			ModelID:            "",
		},
		Calendar: CalendarConfig{
			ReminderLeadMinutes: 30,
		},
		Mood: MoodConfig{
			// Off by default: the mood engine injects directives that change
			// the assistant's tone on every message, which is a surprising
			// thing to find already running on a fresh install. Same
			// reasoning as WebSearch below — a feature that alters every
			// reply should be opted into, not opted out of. Existing
			// installs are unaffected: Load() overlays their config.yaml,
			// which already carries an explicit mood.enabled.
			Enabled:  false,
			Alpha:    0.95,
			Beta:     0.15,
			SigmaMin: 0.10,
			SigmaMax: 0.60,
		},
		WebSearch: WebSearchConfig{
			// On by default (changed from off, direct user request): the
			// "hits the network on every message" reasoning this default
			// once had is stale — web search was later redesigned around
			// real tool-calling (see routeStream), so the model itself
			// decides per-message whether a search is actually needed,
			// at zero cost on every turn it doesn't. There's no longer a
			// meaningful privacy/network cost to defaulting this on, and
			// an assistant that can't look anything up unless a user finds
			// a settings toggle first is a worse default experience.
			// Existing installs are unaffected: Load() overlays their
			// config.yaml, which already carries an explicit value.
			Enabled:    true,
			MaxResults: 5,
		},
		AgentMode: AgentModeConfig{
			// Off by default — a fresh install shouldn't start with
			// file/command tool access live. Existing installs that had
			// agent mode on when they last shut down now keep it on, since
			// SetAgentEnabled persists it and Load() overlays their
			// config.yaml.
			Enabled:             false,
			WorkingSetEnabled:   true,
			WorkingSetMaxTokens: 600,
		},
		Browser: BrowserConfig{
			// Off by default — a fresh browser per fetch, closed
			// immediately after, matches Memo's own positioning of being
			// lighter than competitors that keep one resident.
			KeepAlive: false,
		},
		Swarm: SwarmConfig{
			RPCPort: 50052,
			Role:    "none",
		},
		TaskLoop: TaskLoopConfig{
			PlanningSelfConfig:          true,
			SubAgents:                   true,
			StepGranularity:             "hybrid",
			AutoApprovePlan:             false,
			TaskMemory:                  false,
			MaxExecutorAttempts:         3,
			MaxParallelSteps:            3,
			HandoffStateMaxTokens:       2000,
			StreamIdleTimeoutSec:        300,
			AcceptanceCommandTimeoutSec: 300,
		},
	}
}

func Load(path string) (*AppConfig, error) {
	mu.Lock()
	defer mu.Unlock()

	cfgPath = path
	cfg := Default()

	seeded := false
	data, err := os.ReadFile(path)
	if err != nil && os.IsNotExist(err) {
		// On first run at a writable location (e.g. the Windows installer's
		// %ProgramData%\Memo\config), seed from a config.yaml shipped next to the
		// executable / working dir so packaged defaults aren't lost.
		// On Windows/macOS packaged builds the CWD is not the install dir.
		// Resolve the seed path relative to the executable instead.
		seedPath := filepath.Join("config", "config.yaml")
		if exe, exeErr := os.Executable(); exeErr == nil {
			seedPath = filepath.Join(filepath.Dir(exe), "config", "config.yaml")
		}
		if seed, serr := os.ReadFile(seedPath); serr == nil {
			abs, _ := filepath.Abs(path)
			seedAbs, _ := filepath.Abs(seedPath)
			if abs != seedAbs {
				data, err = seed, nil
				seeded = true
			}
		}
	}
	if err != nil {
		if os.IsNotExist(err) {
			cfg.validate() // normalize data paths (e.g. rebase onto DataDir) before first save
			if saveErr := saveToFile(cfg, path); saveErr != nil {
				return nil, fmt.Errorf("config.Load: failed to create default config: %w", saveErr)
			}
			instance = cfg
			return cfg, nil
		}
		return nil, fmt.Errorf("config.Load: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("config.Load: failed to parse YAML: %w", err)
	}

	if fixes := cfg.validate(); len(fixes) > 0 {
		logx.Printf("config: applied defaults for: %v", fixes)
	}
	migrated := migrateLegacyRemoteToken(cfg)
	migrated = migrateLegacyRemoteAccount(cfg) || migrated
	if seeded || migrated {
		// Persist the seeded/migrated config to its writable home so later
		// runs read it directly (and, for migration, so the plaintext token
		// this replaced never touches disk again after this one rewrite).
		if saveErr := saveToFile(cfg, path); saveErr != nil {
			logx.Printf("config: failed to persist seeded/migrated config: %v", saveErr)
		}
	}
	instance = cfg
	return cfg, nil
}

// migrateLegacyRemoteToken moves a pre-Faz-2 plaintext RemoteAccess.Token
// into a hashed RemoteDevice record and blanks the plaintext field, so an
// existing user's config.yaml never has to be touched by hand and no
// existing paired client loses access — the same secret value still
// authenticates, just checked against its hash instead of compared as
// plaintext. Deliberately only called from Load() (once per process
// start), never from validate() (which Save() also runs): validate() runs
// on every save, including the one immediately after generating a *new*
// token, and would otherwise migrate-and-blank a token before it was ever
// used.
func migrateLegacyRemoteToken(cfg *AppConfig) bool {
	if cfg.RemoteAccess.Token == "" {
		return false
	}
	cfg.RemoteAccess.Devices = append(cfg.RemoteAccess.Devices, RemoteDevice{
		ID:        remoteauth.GenerateDeviceID(),
		Name:      "Legacy",
		TokenHash: remoteauth.HashToken(cfg.RemoteAccess.Token),
		CreatedAt: time.Now(),
	})
	cfg.RemoteAccess.Token = ""
	return true
}

// migrateLegacyRemoteAccount turns a pre-Faz-5.1 config's single
// Username+PasswordHash password credential into that same person's first
// Accounts entry, with Role "admin" — so an existing install that already
// had password auth configured is treated as already "set up" (App.
// NeedsSetup returns false) rather than being shown the first-run bootstrap
// screen, and so LoginRemotePassword's account-list path (see its doc
// comment) has something to authenticate against immediately. Unlike
// migrateLegacyRemoteToken, this deliberately does NOT blank the legacy
// fields afterward — GetRemoteAccessStatus and the desktop Settings tab's
// existing SetRemoteAuthConfig flow still read/write them directly, and
// leaving them in place costs nothing once Accounts is authoritative.
// No-ops (and is safe to call every Load()) once Accounts is non-empty,
// so this never re-runs after the one-time migration.
func migrateLegacyRemoteAccount(cfg *AppConfig) bool {
	if len(cfg.RemoteAccess.Accounts) > 0 {
		return false
	}
	if cfg.RemoteAccess.Username == "" || cfg.RemoteAccess.PasswordHash == "" {
		return false
	}
	cfg.RemoteAccess.Accounts = append(cfg.RemoteAccess.Accounts, Account{
		ID:           remoteauth.GenerateDeviceID(),
		Username:     cfg.RemoteAccess.Username,
		PasswordHash: cfg.RemoteAccess.PasswordHash,
		Role:         "admin",
		CreatedAt:    time.Now(),
	})
	return true
}

func Get() *AppConfig {
	mu.RLock()
	defer mu.RUnlock()

	if instance == nil {
		return Default()
	}
	return instance
}

func Save(cfg *AppConfig) error {
	mu.Lock()
	defer mu.Unlock()

	if cfgPath == "" {
		cfgPath = ConfigFilePath()
	}

	cfg.validate()
	instance = cfg
	return saveToFile(cfg, cfgPath)
}

func saveToFile(cfg *AppConfig, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("config.saveToFile: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("config.saveToFile: %w", err)
	}

	return fileutil.AtomicWrite(path, data, 0600)
}

func (c *AppConfig) validate() []string {
	var fixes []string
	if c.API.BaseURL == "" {
		c.API.BaseURL = "http://127.0.0.1:8081/v1"
		fixes = append(fixes, "API.BaseURL")
	}
	if c.API.TimeoutSeconds <= 0 {
		c.API.TimeoutSeconds = 120
		fixes = append(fixes, "API.TimeoutSeconds")
	}
	if c.Identity.AssistantName == "" {
		c.Identity.AssistantName = "Memo"
		fixes = append(fixes, "Identity.AssistantName")
	}
	if c.Identity.Style == "" {
		c.Identity.Style = "casual"
		fixes = append(fixes, "Identity.Style")
	}
	if c.Memory.PersistDir == "" {
		c.Memory.PersistDir = "./data/memory"
		fixes = append(fixes, "Memory.PersistDir")
	}
	if c.Memory.TopK <= 0 {
		c.Memory.TopK = 8
		fixes = append(fixes, "Memory.TopK")
	}
	if c.Memory.MinSimilarity < 0.15 {
		// Below ~0.15 the vector gate admits almost everything (sim = 1 -
		// dist/2, so 0.1 lets cosine distance up to 1.8/2.0 through) — that
		// was the old default and it meant no filtering at all. Anything a
		// user deliberately set at or above 0.15 is left alone.
		c.Memory.MinSimilarity = 0.35
		fixes = append(fixes, "Memory.MinSimilarity")
	}
	if c.Memory.PinnedFactsPerTurn <= 0 {
		c.Memory.PinnedFactsPerTurn = 10
		fixes = append(fixes, "Memory.PinnedFactsPerTurn")
	}
	if c.Memory.RecencyHalfLifeDays <= 0 {
		c.Memory.RecencyHalfLifeDays = 30
		fixes = append(fixes, "Memory.RecencyHalfLifeDays")
	}
	if c.Memory.QueryHistoryTurns <= 0 {
		c.Memory.QueryHistoryTurns = 1
		fixes = append(fixes, "Memory.QueryHistoryTurns")
	}
	if c.Memory.FactExtractionEveryNTurns <= 0 {
		c.Memory.FactExtractionEveryNTurns = 3
		fixes = append(fixes, "Memory.FactExtractionEveryNTurns")
	}
	if c.AgentMode.WorkingSetMaxTokens <= 0 {
		// Absent from an older config.yaml — adopt the new defaults (on, 600)
		// rather than leaving the feature silently disabled on upgrade.
		c.AgentMode.WorkingSetMaxTokens = 600
		c.AgentMode.WorkingSetEnabled = true
		fixes = append(fixes, "AgentMode.WorkingSet")
	}
	if c.Memory.EmbeddingDimension <= 0 {
		c.Memory.EmbeddingDimension = 768
		fixes = append(fixes, "Memory.EmbeddingDimension")
	}
	if c.Memory.DreamInitialDelayMinutes <= 0 {
		c.Memory.DreamInitialDelayMinutes = 5
		fixes = append(fixes, "Memory.DreamInitialDelayMinutes")
	}
	if c.Memory.DreamIntervalHours <= 0 {
		c.Memory.DreamIntervalHours = 24
		fixes = append(fixes, "Memory.DreamIntervalHours")
	}
	if c.RemoteAccess.Port <= 0 || c.RemoteAccess.Port > 65535 {
		c.RemoteAccess.Port = 8080
		fixes = append(fixes, "RemoteAccess.Port")
	}
	if c.RemoteAccess.TailscaleHostname == "" {
		c.RemoteAccess.TailscaleHostname = "memo"
		fixes = append(fixes, "RemoteAccess.TailscaleHostname")
	}
	if c.RemoteAccess.AuthMode == "" {
		// Pre-Faz-2 configs (and any RemoteAccess block that predates
		// AuthMode entirely) implicitly relied on the single shared token —
		// defaulting to "token" here preserves exactly that behavior rather
		// than silently falling back to "none" (would deauthenticate every
		// remote client) or "password" (no username/password exists yet).
		c.RemoteAccess.AuthMode = "token"
		fixes = append(fixes, "RemoteAccess.AuthMode")
	}
	if c.RemoteAccess.TunnelMode == "" {
		// Preserve legacy behaviour: if ngrok was on, default to ngrok mode.
		if c.RemoteAccess.NgrokMode {
			c.RemoteAccess.TunnelMode = "ngrok"
		} else {
			c.RemoteAccess.TunnelMode = "lan"
		}
		fixes = append(fixes, "RemoteAccess.TunnelMode")
	}
	if c.Llama.Port <= 0 || c.Llama.Port > 65535 {
		c.Llama.Port = 8081
		fixes = append(fixes, "Llama.Port")
	}
	if c.Llama.EmbeddingPort <= 0 || c.Llama.EmbeddingPort > 65535 {
		c.Llama.EmbeddingPort = 8082
		fixes = append(fixes, "Llama.EmbeddingPort")
	}
	if c.Llama.CtxSize <= 0 {
		c.Llama.CtxSize = 8192
		fixes = append(fixes, "Llama.CtxSize")
	}
	if c.Llama.MaxHistory <= 0 {
		c.Llama.MaxHistory = 20
		fixes = append(fixes, "Llama.MaxHistory")
	}
	if c.Llama.ModelsDir == "" {
		c.Llama.ModelsDir = "./data/models"
		fixes = append(fixes, "Llama.ModelsDir")
	}
	// < 0 (not <= 0): 0 is a legitimate, intentional value for both — greedy
	// decoding — and must survive validate(), which Save() runs on every
	// config write, not just on initial load from a possibly-empty/corrupt
	// file. An <= 0 check here silently reset any user request to set
	// either field to exactly 0 straight back to the default (see
	// BUG-QM2). Only a genuinely invalid negative value gets defaulted.
	if c.Llama.Temperature < 0 {
		c.Llama.Temperature = 0.7
		fixes = append(fixes, "Llama.Temperature")
	}
	if c.Llama.TopP < 0 {
		c.Llama.TopP = 0.9
		fixes = append(fixes, "Llama.TopP")
	}
	if c.Llama.MaxTokens < 0 {
		c.Llama.MaxTokens = 0
		fixes = append(fixes, "Llama.MaxTokens")
	}
	if c.Swarm.RPCPort <= 0 || c.Swarm.RPCPort > 65535 {
		c.Swarm.RPCPort = 50052
		fixes = append(fixes, "Swarm.RPCPort")
	}
	if c.Swarm.Role == "" {
		c.Swarm.Role = "none"
		fixes = append(fixes, "Swarm.Role")
	}
	for i := range c.Swarm.Workers {
		if c.Swarm.Workers[i].SharePercent < 0 || c.Swarm.Workers[i].SharePercent > 100 {
			w := c.Swarm.Workers[i].SharePercent
			if w < 0 {
				c.Swarm.Workers[i].SharePercent = 0
			} else {
				c.Swarm.Workers[i].SharePercent = 100
			}
			fixes = append(fixes, fmt.Sprintf("Swarm.Workers[%d].SharePercent", i))
		}
	}
	// Dedupe worker IDs — keep the first occurrence, drop later ones. An ID
	// collision would otherwise let two distinct WorkerSlot entries silently
	// alias the same --tensor-split position when internal/swarm looks one
	// up by ID (add/remove/reorder/share-set), corrupting the coordinator's
	// worker list.
	if len(c.Swarm.Workers) > 1 {
		seen := make(map[string]bool, len(c.Swarm.Workers))
		deduped := c.Swarm.Workers[:0]
		dropped := false
		for _, w := range c.Swarm.Workers {
			if w.ID != "" && seen[w.ID] {
				dropped = true
				continue
			}
			seen[w.ID] = true
			deduped = append(deduped, w)
		}
		c.Swarm.Workers = deduped
		if dropped {
			fixes = append(fixes, "Swarm.Workers[dedup]")
		}
	}
	if c.Whisper.Language == "" {
		c.Whisper.Language = "auto"
		fixes = append(fixes, "Whisper.Language")
	}
	if c.Whisper.Port <= 0 || c.Whisper.Port > 65535 {
		c.Whisper.Port = 9877
		fixes = append(fixes, "Whisper.Port")
	}
	if c.Sync.TokenPath == "" {
		c.Sync.TokenPath = "./data/sync_token.json"
		fixes = append(fixes, "Sync.TokenPath")
	}
	if c.Sync.IntervalMessages <= 0 {
		c.Sync.IntervalMessages = 50
		fixes = append(fixes, "Sync.IntervalMessages")
	}

	if c.Calendar.ReminderLeadMinutes <= 0 {
		c.Calendar.ReminderLeadMinutes = 30
		fixes = append(fixes, "Calendar.ReminderLeadMinutes")
	}

	// Rebase process-relative "data/..." paths onto the resolved DataDir so that
	// shipped/old configs keep working on Windows, where the install directory is
	// read-only. No-op on Linux/macOS (DataDir is the relative "data").
	c.Memory.PersistDir = rebaseDataPath(c.Memory.PersistDir)
	c.Llama.ModelsDir = rebaseDataPath(c.Llama.ModelsDir)
	c.Sync.TokenPath = rebaseDataPath(c.Sync.TokenPath)
	c.WhatsApp.DataDir = rebaseDataPath(c.WhatsApp.DataDir)

	return fixes
}

// rebaseDataPath rewrites a process-relative path under "data/" (optionally
// prefixed with "./") onto the resolved DataDir. Absolute paths and paths not
// under "data/" are returned unchanged.
func rebaseDataPath(p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	norm := strings.TrimPrefix(filepath.ToSlash(p), "./")
	if norm == "data" {
		return DataDir()
	}
	if rest, ok := strings.CutPrefix(norm, "data/"); ok {
		return DataPath(filepath.FromSlash(rest))
	}
	return p
}
