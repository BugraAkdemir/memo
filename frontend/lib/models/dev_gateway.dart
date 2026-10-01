/// Developer screen: the local Anthropic/OpenAI-compatible API gateway that
/// lets external tools (Claude Code via ANTHROPIC_BASE_URL, or anything
/// OpenAI-compatible) use whichever model/provider Memo has configured.
/// Mirrors the backend's DevGatewayConfig (internal/config/config.go) +
/// GatewayModel/GatewayLogEntry (internal/models/devgateway.go).
class DevGatewayConfig {
  final bool requireAPIKey;
  final bool useMemory;
  final String systemPrompt;
  final String token;

  const DevGatewayConfig({
    this.requireAPIKey = false,
    this.useMemory = false,
    this.systemPrompt = '',
    this.token = '',
  });

  factory DevGatewayConfig.fromJson(Map<String, dynamic> json) {
    return DevGatewayConfig(
      requireAPIKey: json['require_api_key'] as bool? ?? false,
      useMemory: json['use_memory'] as bool? ?? false,
      systemPrompt: json['system_prompt'] as String? ?? '',
      token: json['token'] as String? ?? '',
    );
  }
}

/// State of the Developer screen's one-click "connect Claude Code CLI"
/// toggle plus the model override written alongside it — mirrors the
/// backend's {"connected", "model"} response from
/// GET/POST /api/dev-gateway/claude-code-cli.
class ClaudeCodeCLIState {
  final bool connected;
  final String model;

  const ClaudeCodeCLIState({this.connected = false, this.model = ''});

  factory ClaudeCodeCLIState.fromJson(Map<String, dynamic> json) {
    return ClaudeCodeCLIState(
      connected: json['connected'] as bool? ?? false,
      model: json['model'] as String? ?? '',
    );
  }
}

/// State of the Developer screen's "connect Google account" flow for the
/// gemini-sub subscription provider — mirrors the {"connected", "email"}
/// response from GET/POST /api/dev-gateway/google-account.
class GoogleAccountState {
  final bool connected;
  final String email;
  final String model;

  const GoogleAccountState({this.connected = false, this.email = '', this.model = ''});

  factory GoogleAccountState.fromJson(Map<String, dynamic> json) {
    return GoogleAccountState(
      connected: json['connected'] as bool? ?? false,
      email: json['email'] as String? ?? '',
      model: json['model'] as String? ?? '',
    );
  }
}

/// State of the Settings → Claude Subscription "connect Claude account" flow
/// for the claude-sub provider — mirrors the {"connected", "account",
/// "source", "model"} response from GET/POST
/// /api/dev-gateway/claude-account.
///
/// Unlike [GoogleAccountState] there is no email to show: the OAuth scope set
/// claude-sub uses has no userinfo endpoint. `source` says how the connection
/// happened instead ("env", "claude-code-file", "macos-keychain", "browser",
/// "adopted"), which is the only thing that explains to a user why clicking
/// Connect did or did not open a browser.
class ClaudeAccountState {
  final bool connected;
  final String account;
  final String source;
  final String model;

  const ClaudeAccountState({
    this.connected = false,
    this.account = '',
    this.source = '',
    this.model = '',
  });

  factory ClaudeAccountState.fromJson(Map<String, dynamic> json) {
    return ClaudeAccountState(
      connected: json['connected'] as bool? ?? false,
      account: json['account'] as String? ?? '',
      source: json['source'] as String? ?? '',
      model: json['model'] as String? ?? '',
    );
  }

  /// True when the connection came from a Claude Code login already on this
  /// machine rather than from the browser flow — the case where no browser
  /// opened, and the UI should say so rather than look broken.
  bool get adoptedLocally => source == 'env' || source == 'claude-code-file' || source == 'macos-keychain';
}

/// Result of one POST {"connect": true} to /api/dev-gateway/claude-account.
///
/// Exactly one of the two outcomes is meaningful: either `connected` is true
/// (a local Claude Code login was adopted and nothing else has to happen), or
/// `authUrl` + `state` are set and the user has to complete the hosted flow by
/// hand. `source` is informational either way.
class ClaudeConnectAttempt {
  final bool connected;
  final String source;
  final String authUrl;
  final String state;

  const ClaudeConnectAttempt({
    this.connected = false,
    this.source = '',
    this.authUrl = '',
    this.state = '',
  });
}

class GatewayModel {
  final String id;
  final String type;

  const GatewayModel({required this.id, required this.type});

  factory GatewayModel.fromJson(Map<String, dynamic> json) {
    return GatewayModel(
      id: json['id'] as String? ?? '',
      type: json['type'] as String? ?? '',
    );
  }
}

/// One recorded /v1/messages request/response, for the Developer screen's
/// live log view.
class GatewayLogEntry {
  final int seq;
  final String timestamp;
  final String model;
  final bool stream;
  final bool hasTools;
  final String requestPreview;
  final String responsePreview;
  final String error;
  final int durationMs;

  const GatewayLogEntry({
    required this.seq,
    required this.timestamp,
    required this.model,
    required this.stream,
    required this.hasTools,
    required this.requestPreview,
    required this.responsePreview,
    required this.error,
    required this.durationMs,
  });

  bool get isError => error.isNotEmpty;

  factory GatewayLogEntry.fromJson(Map<String, dynamic> json) {
    return GatewayLogEntry(
      seq: (json['seq'] as num?)?.toInt() ?? 0,
      timestamp: json['timestamp'] as String? ?? '',
      model: json['model'] as String? ?? '',
      stream: json['stream'] as bool? ?? false,
      hasTools: json['has_tools'] as bool? ?? false,
      requestPreview: json['request_preview'] as String? ?? '',
      responsePreview: json['response_preview'] as String? ?? '',
      error: json['error'] as String? ?? '',
      durationMs: (json['duration_ms'] as num?)?.toInt() ?? 0,
    );
  }
}
