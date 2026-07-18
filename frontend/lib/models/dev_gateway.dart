/// Developer screen: the local Anthropic/OpenAI-compatible API gateway that
/// lets external tools (Claude Code via ANTHROPIC_BASE_URL, or anything
/// OpenAI-compatible) use whichever model/provider Memo has configured.
/// Mirrors the backend's DevGatewayConfig (internal/config/config.go) +
/// GatewayModel/GatewayLogEntry (internal/models/devgateway.go).
class DevGatewayConfig {
  final bool requireAPIKey;
  final bool useMemory;
  final String token;

  const DevGatewayConfig({
    this.requireAPIKey = false,
    this.useMemory = false,
    this.token = '',
  });

  factory DevGatewayConfig.fromJson(Map<String, dynamic> json) {
    return DevGatewayConfig(
      requireAPIKey: json['require_api_key'] as bool? ?? false,
      useMemory: json['use_memory'] as bool? ?? false,
      token: json['token'] as String? ?? '',
    );
  }
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
