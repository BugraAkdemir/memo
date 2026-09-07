/// External speech-to-text provider configuration — mirrors Go
/// `stt.ProviderConfig` (internal/stt/provider.go). Shaped like
/// [TTSProviderConfig] but with no `voice` field. [baseUrl] is only
/// meaningful for the `custom` type (a user-supplied OpenAI-Whisper-
/// compatible `POST {base_url}/audio/transcriptions` endpoint).
class STTProviderConfig {
  final String type;
  final String name;
  final String? apiKey;
  final String? baseUrl;
  final bool enabled;
  final int priority;
  final bool connected;
  final String? error;

  const STTProviderConfig({
    required this.type,
    required this.name,
    this.apiKey,
    this.baseUrl,
    this.enabled = false,
    this.priority = 0,
    this.connected = false,
    this.error,
  });

  factory STTProviderConfig.fromJson(Map<String, dynamic> json) {
    return STTProviderConfig(
      type: json['type'] as String? ?? '',
      name: json['name'] as String? ?? '',
      apiKey: json['api_key'] as String?,
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
      if (baseUrl != null && baseUrl!.isNotEmpty) 'base_url': baseUrl,
      'enabled': enabled,
      'priority': priority,
    };
  }
}

/// STT provider types the backend implements (internal/stt.NewProvider):
/// `elevenlabs` (POST /v1/speech-to-text) and `custom` (a user-supplied
/// OpenAI-Whisper-compatible endpoint). `custom` requires
/// [STTProviderConfig.baseUrl].
class STTProviderDefaults {
  static const Map<String, String> displayNames = {
    'elevenlabs': 'ElevenLabs',
    'custom': 'Custom (Whisper-compatible)',
  };

  static const List<String> implementedTypes = ['elevenlabs', 'custom'];

  /// Types that need a base URL entered.
  static const List<String> needsBaseUrl = ['custom'];
}
