/// External TTS provider configuration — mirrors Go `tts.ProviderConfig`
/// (internal/tts/provider.go). Deliberately smaller than [ProviderConfig]
/// (chat providers): no base URL/model/temperature/top_p/max_tokens, and
/// has a voice field instead — see PLAN_voice_live_mode_faz2.md's 2.1 note.
class TTSProviderConfig {
  final String type;
  final String name;
  final String? apiKey;
  final String voice;
  final bool enabled;
  final int priority;
  final bool connected;
  final String? error;

  const TTSProviderConfig({
    required this.type,
    required this.name,
    this.apiKey,
    required this.voice,
    this.enabled = false,
    this.priority = 0,
    this.connected = false,
    this.error,
  });

  factory TTSProviderConfig.fromJson(Map<String, dynamic> json) {
    return TTSProviderConfig(
      type: json['type'] as String? ?? '',
      name: json['name'] as String? ?? '',
      apiKey: json['api_key'] as String?,
      voice: json['voice'] as String? ?? '',
      enabled: json['enabled'] as bool? ?? false,
      priority: json['priority'] as int? ?? 0,
      connected: json['connected'] as bool? ?? false,
      error: json['error'] as String?,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'type': type,
      'name': name,
      'api_key': apiKey ?? '',
      'voice': voice,
      'enabled': enabled,
      'priority': priority,
    };
  }

  TTSProviderConfig copyWith({
    String? type,
    String? name,
    String? apiKey,
    String? voice,
    bool? enabled,
    int? priority,
  }) {
    return TTSProviderConfig(
      type: type ?? this.type,
      name: name ?? this.name,
      apiKey: apiKey ?? this.apiKey,
      voice: voice ?? this.voice,
      enabled: enabled ?? this.enabled,
      priority: priority ?? this.priority,
      connected: connected,
      error: error,
    );
  }
}

/// TTS provider types the backend knows about (internal/tts.ProviderType).
/// Only "openai" has a real implementation (Faz 2.2) — "elevenlabs" is
/// declared here for future use but selecting it fails server-side with a
/// clear "not yet supported" error.
class TTSProviderDefaults {
  static const Map<String, String> displayNames = {
    'openai': 'OpenAI',
    'elevenlabs': 'ElevenLabs',
  };

  static const List<String> implementedTypes = ['openai'];
}
