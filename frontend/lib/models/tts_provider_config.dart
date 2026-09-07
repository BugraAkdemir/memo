/// External TTS provider configuration — mirrors Go `tts.ProviderConfig`
/// (internal/tts/provider.go). Deliberately smaller than [ProviderConfig]
/// (chat providers): no model/temperature/top_p/max_tokens, and has a voice
/// field instead — see PLAN_voice_live_mode_faz2.md's 2.1 note. [baseUrl] is
/// only meaningful for the `custom` type (a user-supplied OpenAI-compatible
/// TTS endpoint).
class TTSProviderConfig {
  final String type;
  final String name;
  final String? apiKey;
  final String voice;

  /// Empty = the provider's built-in default model. Populated from the live
  /// model list (`POST /api/tts/providers/models`) for providers that expose
  /// one.
  final String? model;
  final String? baseUrl;
  final bool enabled;
  final int priority;
  final bool connected;
  final String? error;

  const TTSProviderConfig({
    required this.type,
    required this.name,
    this.apiKey,
    required this.voice,
    this.model,
    this.baseUrl,
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
      model: json['model'] as String?,
      baseUrl: json['base_url'] as String?,
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
      if (model != null && model!.isNotEmpty) 'model': model,
      if (baseUrl != null && baseUrl!.isNotEmpty) 'base_url': baseUrl,
      'enabled': enabled,
      'priority': priority,
    };
  }

  TTSProviderConfig copyWith({
    String? type,
    String? name,
    String? apiKey,
    String? voice,
    String? model,
    String? baseUrl,
    bool? enabled,
    int? priority,
  }) {
    return TTSProviderConfig(
      type: type ?? this.type,
      name: name ?? this.name,
      apiKey: apiKey ?? this.apiKey,
      voice: voice ?? this.voice,
      model: model ?? this.model,
      baseUrl: baseUrl ?? this.baseUrl,
      enabled: enabled ?? this.enabled,
      priority: priority ?? this.priority,
      connected: connected,
      error: error,
    );
  }
}

/// One entry from an external TTS provider's live voice list
/// (`POST /api/tts/providers/voices` → ElevenLabs `GET /v1/voices`).
class TTSProviderVoice {
  final String id;
  final String name;

  const TTSProviderVoice({required this.id, required this.name});

  factory TTSProviderVoice.fromJson(Map<String, dynamic> json) => TTSProviderVoice(
        id: json['voice_id'] as String? ?? '',
        name: json['name'] as String? ?? '',
      );
}

/// TTS provider types the backend implements (internal/tts.NewProvider):
/// `openai`, `elevenlabs` and `custom` all build real providers. `custom`
/// additionally requires [TTSProviderConfig.baseUrl]. Only `elevenlabs`
/// exposes a live voice-discovery endpoint; the others return a clear
/// "not supported" and the voice must be entered by hand.
class TTSProviderDefaults {
  static const Map<String, String> displayNames = {
    'openai': 'OpenAI',
    'elevenlabs': 'ElevenLabs',
    'custom': 'Custom (OpenAI-compatible)',
  };

  static const List<String> implementedTypes = ['openai', 'elevenlabs', 'custom'];

  /// Types that need a base URL entered.
  static const List<String> needsBaseUrl = ['custom'];

  /// Types with a live voice-discovery endpoint.
  static const List<String> hasVoiceDiscovery = ['elevenlabs'];

  /// Types with a live model-discovery endpoint.
  static const List<String> hasModelDiscovery = ['elevenlabs'];
}
