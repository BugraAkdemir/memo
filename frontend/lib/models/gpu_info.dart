/// GPU info — mirrors Go `llama.GPUInfo`
class GPUInfo {
  final String type; // "nvidia", "amd", "cpu"
  final String name;
  final int vramMb;
  final int recommendedLayers;
  final String description;

  const GPUInfo({
    this.type = 'cpu',
    this.name = 'CPU',
    this.vramMb = 0,
    this.recommendedLayers = 0,
    this.description = '',
  });

  factory GPUInfo.fromJson(Map<String, dynamic> json) => GPUInfo(
        type: json['type'] as String? ?? 'cpu',
        name: json['name'] as String? ?? 'CPU',
        vramMb: json['vram_mb'] as int? ?? 0,
        recommendedLayers: json['recommended_layers'] as int? ?? 0,
        description: json['description'] as String? ?? '',
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
