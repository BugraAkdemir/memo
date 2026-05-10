/// Local model on disk — mirrors Go `modelstore.LocalModel`
class LocalModel {
  final String repoId;
  final String filename;
  final int size;
  final String path;
  final bool isEmbedding;

  const LocalModel({
    required this.repoId,
    required this.filename,
    required this.size,
    required this.path,
    required this.isEmbedding,
  });

  factory LocalModel.fromJson(Map<String, dynamic> json) => LocalModel(
        repoId: json['repo_id'] as String? ?? '',
        filename: json['filename'] as String? ?? '',
        size: json['size'] as int? ?? 0,
        path: json['path'] as String? ?? '',
        isEmbedding: json['is_embedding'] as bool? ?? false,
      );

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

  const HFModelResult({
    required this.id,
    required this.author,
    required this.downloads,
    required this.likes,
    required this.tags,
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
      );
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

  const DownloadProgress({
    this.active = false,
    this.repoId = '',
    this.filename = '',
    this.totalBytes = 0,
    this.downloaded = 0,
    this.percent = 0,
    this.speed = '',
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
      );
}
