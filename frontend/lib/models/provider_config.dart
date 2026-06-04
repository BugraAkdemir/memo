/// Provider configuration model — mirrors Go `provider.ProviderConfig`
class ProviderConfig {
  final String type;
  final String name;
  final String? apiKey;
  final String? baseUrl;
  final String model;
  final bool enabled;
  final int priority;
  final double temperature;
  final double topP;
  final int maxTokens;
  final bool connected;
  final String? error;

  const ProviderConfig({
    required this.type,
    required this.name,
    this.apiKey,
    this.baseUrl,
    required this.model,
    this.enabled = false,
    this.priority = 0,
    this.temperature = 0.7,
    this.topP = 0.9,
    this.maxTokens = 0,
    this.connected = false,
    this.error,
  });

  factory ProviderConfig.fromJson(Map<String, dynamic> json) {
    return ProviderConfig(
      type: json['type'] as String? ?? '',
      name: json['name'] as String? ?? '',
      apiKey: json['api_key'] as String?,
      baseUrl: json['base_url'] as String?,
      model: json['model'] as String? ?? '',
      enabled: json['enabled'] as bool? ?? false,
      priority: json['priority'] as int? ?? 0,
      temperature: (json['temperature'] as num?)?.toDouble() ?? 0.7,
      topP: (json['top_p'] as num?)?.toDouble() ?? 0.9,
      maxTokens: json['max_tokens'] as int? ?? 0,
      connected: json['connected'] as bool? ?? false,
      error: json['error'] as String?,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'type': type,
      'name': name,
      'api_key': apiKey ?? '',
      'base_url': baseUrl ?? '',
      'model': model,
      'enabled': enabled,
      'priority': priority,
      'temperature': temperature,
      'top_p': topP,
      'max_tokens': maxTokens,
    };
  }

  ProviderConfig copyWith({
    String? type,
    String? name,
    String? apiKey,
    String? baseUrl,
    String? model,
    bool? enabled,
    int? priority,
    double? temperature,
    double? topP,
    int? maxTokens,
    bool? connected,
    String? error,
  }) {
    return ProviderConfig(
      type: type ?? this.type,
      name: name ?? this.name,
      apiKey: apiKey ?? this.apiKey,
      baseUrl: baseUrl ?? this.baseUrl,
      model: model ?? this.model,
      enabled: enabled ?? this.enabled,
      priority: priority ?? this.priority,
      temperature: temperature ?? this.temperature,
      topP: topP ?? this.topP,
      maxTokens: maxTokens ?? this.maxTokens,
      connected: connected ?? this.connected,
      error: error ?? this.error,
    );
  }
}

/// Default provider models
class ProviderDefaults {
  static const Map<String, String> defaultModels = {
    'openai': 'gpt-4o',
    'gemini': 'gemini-2.0-flash',
    'grok': 'grok-2',
    'groq': 'openai/gpt-oss-20b',
    'claude': 'claude-sonnet-4-20250514',
    'openrouter': 'openai/gpt-4o',
    'ollama': 'llama3',
  };

  static const Map<String, String> defaultBaseUrls = {
    'openai': 'https://api.openai.com/v1',
    'gemini': 'https://generativelanguage.googleapis.com/v1beta',
    'grok': 'https://api.x.ai/v1',
    'groq': 'https://api.groq.com/openai/v1',
    'claude': 'https://api.anthropic.com/v1',
    'openrouter': 'https://openrouter.ai/api/v1',
    'ollama': 'http://127.0.0.1:11434/v1',
  };

  static const Map<String, String> displayNames = {
    'openai': 'OpenAI',
    'gemini': 'Google Gemini',
    'grok': 'xAI Grok',
    'groq': 'Groq',
    'claude': 'Anthropic Claude',
    'openrouter': 'OpenRouter',
    'ollama': 'Ollama',
  };
}
