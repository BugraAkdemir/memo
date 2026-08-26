/// One non-local Live Mode engine's configuration — mirrors Go
/// `livemode.EngineConfig` (internal/livemode/engine.go). "local" has no
/// entry of its own (see the Go type's doc comment); this model only ever
/// represents one of the other four engine types.
class LiveModeEngineConfig {
  final String type; // "google_live" | "openai_realtime" | "elevenlabs" | "custom"
  final String? apiKey;
  final String model; // live-fetched model ID (Phase 4+) — free text for now
  final String voice; // ElevenLabs only
  final String baseURL; // Custom only
  final bool enabled;

  const LiveModeEngineConfig({
    required this.type,
    this.apiKey,
    this.model = '',
    this.voice = '',
    this.baseURL = '',
    this.enabled = false,
  });

  factory LiveModeEngineConfig.fromJson(Map<String, dynamic> json) {
    return LiveModeEngineConfig(
      type: json['type'] as String? ?? '',
      apiKey: json['api_key'] as String?,
      model: json['model'] as String? ?? '',
      voice: json['voice'] as String? ?? '',
      baseURL: json['base_url'] as String? ?? '',
      enabled: json['enabled'] as bool? ?? false,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'type': type,
      'api_key': apiKey ?? '',
      'model': model,
      'voice': voice,
      'base_url': baseURL,
      'enabled': enabled,
    };
  }

  LiveModeEngineConfig copyWith({
    String? apiKey,
    String? model,
    String? voice,
    String? baseURL,
    bool? enabled,
  }) {
    return LiveModeEngineConfig(
      type: type,
      apiKey: apiKey ?? this.apiKey,
      model: model ?? this.model,
      voice: voice ?? this.voice,
      baseURL: baseURL ?? this.baseURL,
      enabled: enabled ?? this.enabled,
    );
  }
}
