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

  /// Model context-window size (tokens). Budgets how much chat history is sent.
  /// 0 = use a sensible default for the provider.
  final int contextTokens;
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
    this.contextTokens = 0,
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
      contextTokens: json['context_tokens'] as int? ?? 0,
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
      'context_tokens': contextTokens,
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
    int? contextTokens,
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
      contextTokens: contextTokens ?? this.contextTokens,
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
    'custom': 'Özel (OpenAI uyumlu)',
  };

  /// Where the user gets an API key for each provider. Shown as a tappable
  /// "Anahtar al →" link so adding a provider doesn't require hunting the web.
  static const Map<String, String> apiKeyUrls = {
    'openai': 'https://platform.openai.com/api-keys',
    'gemini': 'https://aistudio.google.com/app/apikey',
    'grok': 'https://console.x.ai',
    'groq': 'https://console.groq.com/keys',
    'claude': 'https://console.anthropic.com/settings/keys',
    'openrouter': 'https://openrouter.ai/keys',
  };

  /// One-line, plain-language hint about each provider shown under the picker.
  static const Map<String, String> hints = {
    'openai': 'ChatGPT\'nin arkasındaki modeller (GPT-4o).',
    'gemini': 'Google\'ın modelleri — cömert ücretsiz kota.',
    'grok': 'xAI\'nin Grok modelleri (X Premium).',
    'groq': 'Çok hızlı, açık kaynak modeller — ücretsiz başla.',
    'claude': 'Anthropic Claude — uzun bağlam ve kod için güçlü.',
    'openrouter': 'Tek anahtarla yüzlerce modele eriş.',
    'ollama': 'Bilgisayarında yerel model çalıştır — anahtar gerekmez.',
    'custom': 'Herhangi bir OpenAI uyumlu endpoint. Base URL\'i sen girersin.',
  };

  /// Providers that need no API key (local). Custom endpoints often need one,
  /// but not always (local proxies), so it's treated as optional there.
  static bool needsApiKey(String type) => type != 'ollama' && type != 'custom';

  /// Providers that require the user to supply a Base URL (no sensible default).
  static bool needsBaseUrl(String type) => type == 'custom';
}
