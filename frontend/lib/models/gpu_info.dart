/// GPU info — mirrors Go `llama.GPUInfo`
class GPUInfo {
  final String type; // "nvidia", "amd", "cpu"
  final String name;
  final int vramMb;
  final int recommendedLayers;
  final String description;
  final int ramTotalMb; // total system RAM in MB (0 if unknown)

  const GPUInfo({
    this.type = 'cpu',
    this.name = 'CPU',
    this.vramMb = 0,
    this.recommendedLayers = 0,
    this.description = '',
    this.ramTotalMb = 0,
  });

  factory GPUInfo.fromJson(Map<String, dynamic> json) => GPUInfo(
        type: json['type'] as String? ?? 'cpu',
        name: json['name'] as String? ?? 'CPU',
        vramMb: json['vram_mb'] as int? ?? 0,
        recommendedLayers: json['recommended_layers'] as int? ?? 0,
        description: json['description'] as String? ?? '',
        ramTotalMb: json['ram_total_mb'] as int? ?? 0,
      );

  bool get hasGpu => type != 'cpu';
  bool get isNvidia => type == 'nvidia';
  bool get isAmd => type == 'amd';

  String get vramFormatted {
    if (vramMb >= 1024) {
      return '${(vramMb / 1024).toStringAsFixed(1)} GB';
    }
    return '$vramMb MB';
  }

  String get ramFormatted {
    if (ramTotalMb >= 1024) {
      return '${(ramTotalMb / 1024).toStringAsFixed(0)} GB';
    }
    return '$ramTotalMb MB';
  }
}

/// Server status — mirrors Go `llama.ServerStatus`
class ServerStatus {
  final bool running;
  final String modelPath;
  final String modelName;
  final int port;
  final int pid;
  final GPUInfo gpu;

  const ServerStatus({
    this.running = false,
    this.modelPath = '',
    this.modelName = '',
    this.port = 0,
    this.pid = 0,
    this.gpu = const GPUInfo(),
  });

  factory ServerStatus.fromJson(Map<String, dynamic> json) => ServerStatus(
        running: json['running'] as bool? ?? false,
        modelPath: json['model_path'] as String? ?? '',
        modelName: json['model_name'] as String? ?? '',
        port: json['port'] as int? ?? 0,
        pid: json['pid'] as int? ?? 0,
        gpu: json['gpu'] != null
            ? GPUInfo.fromJson(json['gpu'] as Map<String, dynamic>)
            : const GPUInfo(),
      );
}

/// Memory file info — mirrors Go `memory.GobFileInfo`
class MemoryFileInfo {
  final String path;
  final String name;
  final int sizeKb;
  final String modified;

  const MemoryFileInfo({
    required this.path,
    required this.name,
    required this.sizeKb,
    required this.modified,
  });

  factory MemoryFileInfo.fromJson(Map<String, dynamic> json) => MemoryFileInfo(
        path: json['path'] as String? ?? '',
        name: json['name'] as String? ?? '',
        sizeKb: json['size_kb'] as int? ?? 0,
        modified: json['modified'] as String? ?? '',
      );
}

/// Memory search result — mirrors Go `memory.DebugSearchResult`
class MemorySearchResult {
  final String id;
  final String content;
  final double similarity;
  final String timestamp;
  final String userMsg;
  final String assistMsg;
  final String matchType;
  final int importance;
  final String source;
  final String tags;
  final int retrieveCount;

  const MemorySearchResult({
    required this.id,
    required this.content,
    required this.similarity,
    required this.timestamp,
    required this.userMsg,
    required this.assistMsg,
    required this.matchType,
    this.importance = 3,
    this.source = 'conversation',
    this.tags = '',
    this.retrieveCount = 0,
  });

  factory MemorySearchResult.fromJson(Map<String, dynamic> json) =>
      MemorySearchResult(
        id: json['id'] as String? ?? '',
        content: json['content'] as String? ?? '',
        similarity: (json['similarity'] as num?)?.toDouble() ?? 0.0,
        timestamp: json['timestamp'] as String? ?? '',
        userMsg: json['user_msg'] as String? ?? '',
        assistMsg: json['assist_msg'] as String? ?? '',
        matchType: json['match_type'] as String? ?? '',
        importance: json['importance'] as int? ?? 3,
        source: json['source'] as String? ?? 'conversation',
        tags: json['tags'] as String? ?? '',
        retrieveCount: json['retrieve_count'] as int? ?? 0,
      );
}

class MemoryStats {
  final int count;
  final int explicitCount;
  final int addedThisWeek;
  final int pendingDeletion;
  final List<MemorySearchResult> topRetrieved;

  const MemoryStats({
    required this.count,
    required this.explicitCount,
    required this.addedThisWeek,
    required this.pendingDeletion,
    required this.topRetrieved,
  });

  factory MemoryStats.fromJson(Map<String, dynamic> json) => MemoryStats(
        count: json['count'] as int? ?? 0,
        explicitCount: json['explicit_count'] as int? ?? 0,
        addedThisWeek: json['added_this_week'] as int? ?? 0,
        pendingDeletion: json['pending_deletion'] as int? ?? 0,
        topRetrieved: json['top_retrieved'] is List
            ? (json['top_retrieved'] as List)
                .map((e) => MemorySearchResult.fromJson(e as Map<String, dynamic>))
                .toList()
            : [],
      );
}
