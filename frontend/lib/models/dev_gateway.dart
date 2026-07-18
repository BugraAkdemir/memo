/// Settings > Developer tab: the local Anthropic/OpenAI-compatible API
/// gateway that lets external tools (Claude Code via ANTHROPIC_BASE_URL,
/// or anything OpenAI-compatible) use whichever model/provider Memo has
/// configured. Mirrors the backend's DevGatewayConfig
/// (internal/config/config.go) + GatewayModel (internal/models/devgateway.go).
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
