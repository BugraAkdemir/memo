/// A downloadable Piper voice from the curated catalog — mirrors Go
/// `tts.Voice` (internal/tts/voice_store.go). Fully offline: downloading
/// one requires no API key, just a one-time fetch from Hugging Face.
class TTSVoice {
  final String locale; // e.g. "tr_TR", "en_US"
  final String name; // e.g. "fahrettin", "lessac"
  final String quality; // e.g. "medium"

  const TTSVoice({required this.locale, required this.name, required this.quality});

  String get id => '$locale-$name-$quality';
  String get language => locale.contains('_') ? locale.split('_').first : locale;

  factory TTSVoice.fromJson(Map<String, dynamic> json) {
    return TTSVoice(
      locale: json['locale'] as String? ?? '',
      name: json['name'] as String? ?? '',
      quality: json['quality'] as String? ?? '',
    );
  }
}

/// A voice already downloaded to disk — mirrors Go `tts.LocalVoice`.
class TTSLocalVoice extends TTSVoice {
  final String path;
  final int size;

  const TTSLocalVoice({
    required super.locale,
    required super.name,
    required super.quality,
    required this.path,
    required this.size,
  });

  factory TTSLocalVoice.fromJson(Map<String, dynamic> json) {
    return TTSLocalVoice(
      locale: json['locale'] as String? ?? '',
      name: json['name'] as String? ?? '',
      quality: json['quality'] as String? ?? '',
      path: json['path'] as String? ?? '',
      size: json['size'] as int? ?? 0,
    );
  }
}

/// Mirrors Go `tts.VoiceDownloadProgress`.
class TTSVoiceDownloadProgress {
  final bool active;
  final String voiceId;
  final int totalBytes;
  final int downloaded;
  final double percent;
  final String? error;

  const TTSVoiceDownloadProgress({
    required this.active,
    required this.voiceId,
    required this.totalBytes,
    required this.downloaded,
    required this.percent,
    this.error,
  });

  factory TTSVoiceDownloadProgress.fromJson(Map<String, dynamic> json) {
    return TTSVoiceDownloadProgress(
      active: json['active'] as bool? ?? false,
      voiceId: json['voice_id'] as String? ?? '',
      totalBytes: json['total_bytes'] as int? ?? 0,
      downloaded: json['downloaded'] as int? ?? 0,
      percent: (json['percent'] as num?)?.toDouble() ?? 0,
      error: json['error'] as String?,
    );
  }
}

/// Combined response from GET /api/tts/voices.
class TTSVoiceStoreState {
  final List<TTSVoice> catalog;
  final List<TTSLocalVoice> local;
  final List<TTSVoiceDownloadProgress> downloads;

  /// config.TTS.ModelPath — the .onnx path the local Piper synthesizer is
  /// actually configured with right now, if any. Compare a TTSLocalVoice's
  /// [TTSLocalVoice.path] against this to know which downloaded voice (if
  /// any) is the one currently in use.
  final String selectedPath;

  const TTSVoiceStoreState({
    required this.catalog,
    required this.local,
    required this.downloads,
    required this.selectedPath,
  });

  factory TTSVoiceStoreState.fromJson(Map<String, dynamic> json) {
    return TTSVoiceStoreState(
      catalog: (json['catalog'] as List? ?? [])
          .map((e) => TTSVoice.fromJson(e as Map<String, dynamic>))
          .toList(),
      local: (json['local'] as List? ?? [])
          .map((e) => TTSLocalVoice.fromJson(e as Map<String, dynamic>))
          .toList(),
      downloads: (json['downloads'] as List? ?? [])
          .map((e) => TTSVoiceDownloadProgress.fromJson(e as Map<String, dynamic>))
          .toList(),
      selectedPath: json['selected_path'] as String? ?? '',
    );
  }
}

/// Display names for known languages in the curated catalog — the backend
/// only sends a two-letter code (tts.Voice.Language()), the human-readable
/// label lives on the Flutter side like every other display-name map in
/// this codebase (see ProviderDefaults.displayNames).
class TTSVoiceLanguageNames {
  static const Map<String, String> names = {
    'tr': 'Türkçe',
    'en': 'English',
  };

  static String of(String code) => names[code] ?? code;
}
