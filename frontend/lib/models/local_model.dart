/// Local model on disk — mirrors Go `modelstore.LocalModel`
class LocalModel {
  final String repoId;
  final String filename;
  final int size;
  final String path;
  final bool isEmbedding;
  final bool isVision;
  /// Confirmed via HF `.meta.json` sidecar — populated after download.
  final bool supportsTools;
  final bool supportsVision;
  final bool supportsCode;
  final List<String> tags;

  const LocalModel({
    required this.repoId,
    required this.filename,
    required this.size,
    required this.path,
    required this.isEmbedding,
    this.isVision = false,
    this.supportsTools = false,
    this.supportsVision = false,
    this.supportsCode = false,
    this.tags = const [],
  });

  factory LocalModel.fromJson(Map<String, dynamic> json) => LocalModel(
        repoId: json['repo_id'] as String? ?? '',
        filename: json['filename'] as String? ?? '',
        size: json['size'] as int? ?? 0,
        path: json['path'] as String? ?? '',
        isEmbedding: json['is_embedding'] as bool? ?? false,
        isVision: (json['mmproj_path'] as String?)?.isNotEmpty ?? false,
        supportsTools: json['supports_tools'] as bool? ?? false,
        supportsVision: json['supports_vision'] as bool? ?? false,
        supportsCode: json['supports_code'] as bool? ?? false,
        tags: (json['tags'] as List<dynamic>?)?.map((e) => e.toString()).toList() ?? [],
      );

  /// Whether this model likely supports tool/function calling.
  /// Uses confirmed HF metadata when available, falls back to filename heuristic.
  bool get likelySupportsTools {
    if (supportsTools) return true;
    if (isEmbedding) return false;
    final lower = '${filename.toLowerCase()} ${repoId.toLowerCase()}';
    const toolFamilies = [
      'llama-3', 'llama3', 'qwen2', 'qwen2.5', 'mistral', 'mixtral',
      'hermes', 'functionary', 'nexusraven', 'gorilla', 'phi-3', 'phi-4',
    ];
    final hasFamily = toolFamilies.any((f) => lower.contains(f));
    final isInstruct = lower.contains('instruct') || lower.contains('chat');
    return hasFamily && isInstruct;
  }

  /// Human-readable file size
  String get sizeFormatted {
    if (size >= 1024 * 1024 * 1024) {
      return '${(size / (1024 * 1024 * 1024)).toStringAsFixed(1)} GB';
    } else if (size >= 1024 * 1024) {
      return '${(size / (1024 * 1024)).toStringAsFixed(1)} MB';
    } else {
      return '${(size / 1024).toStringAsFixed(0)} KB';
    }
  }
}

/// Hugging Face search result — mirrors Go `modelstore.HFModelResult`
class HFModelResult {
  final String id;
  final String author;
  final int downloads;
  final int likes;
  final List<String> tags;
  final String lastModified;

  const HFModelResult({
    required this.id,
    required this.author,
    required this.downloads,
    required this.likes,
    required this.tags,
    this.lastModified = '',
  });

  factory HFModelResult.fromJson(Map<String, dynamic> json) => HFModelResult(
        id: json['id'] as String? ?? '',
        author: json['author'] as String? ?? '',
        downloads: json['downloads'] as int? ?? 0,
        likes: json['likes'] as int? ?? 0,
        tags: (json['tags'] as List<dynamic>?)
                ?.map((e) => e.toString())
                .toList() ??
            [],
        lastModified: json['lastModified'] as String? ?? '',
      );

  static const _toolTags = {'function-calling', 'tool-use', 'tool-calling', 'tools'};
  static const _visionTags = {
    'image-to-text', 'visual-question-answering', 'image-text-to-text', 'vision', 'multimodal',
  };
  static const _codeTags = {'code', 'code-generation', 'coding'};

  bool get supportsTools => tags.any((t) => _toolTags.contains(t.toLowerCase()));
  bool get supportsVision => tags.any((t) => _visionTags.contains(t.toLowerCase()));
  bool get supportsCode => tags.any((t) => _codeTags.contains(t.toLowerCase()));
}

/// GGUF file within a HF repo — mirrors Go `modelstore.GGUFFile`
class GGUFFile {
  final String filename;
  final int size;

  const GGUFFile({required this.filename, required this.size});

  factory GGUFFile.fromJson(Map<String, dynamic> json) => GGUFFile(
        filename: json['filename'] as String? ?? '',
        size: json['size'] as int? ?? 0,
      );

  String get sizeFormatted {
    if (size >= 1024 * 1024 * 1024) {
      return '${(size / (1024 * 1024 * 1024)).toStringAsFixed(1)} GB';
    } else if (size >= 1024 * 1024) {
      return '${(size / (1024 * 1024)).toStringAsFixed(1)} MB';
    }
    return '${(size / 1024).toStringAsFixed(0)} KB';
  }
}

/// Download progress — mirrors Go `modelstore.DownloadProgress`
class DownloadProgress {
  final bool active;
  final String repoId;
  final String filename;
  final int totalBytes;
  final int downloaded;
  final double percent;
  final String speed;
  final String? error;

  const DownloadProgress({
    this.active = false,
    this.repoId = '',
    this.filename = '',
    this.totalBytes = 0,
    this.downloaded = 0,
    this.percent = 0,
    this.speed = '',
    this.error,
  });

  factory DownloadProgress.fromJson(Map<String, dynamic> json) =>
      DownloadProgress(
        active: json['active'] as bool? ?? false,
        repoId: json['repo_id'] as String? ?? '',
        filename: json['filename'] as String? ?? '',
        totalBytes: json['total_bytes'] as int? ?? 0,
        downloaded: json['downloaded'] as int? ?? 0,
        percent: (json['percent'] as num?)?.toDouble() ?? 0,
        speed: json['speed'] as String? ?? '',
        error: json['error'] as String?,
      );
}
