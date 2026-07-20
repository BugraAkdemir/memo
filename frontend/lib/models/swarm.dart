/// One worker machine registered with a Memo Swarm host.
/// Mirrors Go `swarm.WorkerSlot` JSON.
class SwarmWorker {
  final String id;
  final String label;
  final String address;
  final double sharePercent;
  final bool connected;
  final DateTime? lastSeen;

  const SwarmWorker({
    required this.id,
    required this.label,
    required this.address,
    required this.sharePercent,
    required this.connected,
    this.lastSeen,
  });

  factory SwarmWorker.fromJson(Map<String, dynamic> json) {
    DateTime? lastSeen;
    final raw = json['last_seen'];
    if (raw != null) {
      lastSeen = DateTime.tryParse(raw.toString());
    }
    return SwarmWorker(
      id: json['id'] as String? ?? '',
      label: json['label'] as String? ?? '',
      address: json['address'] as String? ?? '',
      sharePercent: (json['share_percent'] as num?)?.toDouble() ?? 0,
      connected: json['connected'] == true,
      lastSeen: lastSeen,
    );
  }

  Map<String, dynamic> toJson() => {
        'id': id,
        'label': label,
        'address': address,
        'share_percent': sharePercent,
        'connected': connected,
        if (lastSeen != null) 'last_seen': lastSeen!.toIso8601String(),
      };

  SwarmWorker copyWith({
    String? id,
    String? label,
    String? address,
    double? sharePercent,
    bool? connected,
    DateTime? lastSeen,
  }) =>
      SwarmWorker(
        id: id ?? this.id,
        label: label ?? this.label,
        address: address ?? this.address,
        sharePercent: sharePercent ?? this.sharePercent,
        connected: connected ?? this.connected,
        lastSeen: lastSeen ?? this.lastSeen,
      );
}

/// Combined host + worker view from GET /api/swarm/status.
/// Mirrors Go `app.SwarmStatus` JSON.
class SwarmStatus {
  /// "none" | "host" | "worker"
  final String role;
  final String roomCode;
  final bool running;
  final double hostShare;
  final String modelPath;
  final List<SwarmWorker> workers;
  final String hostAddr;
  final bool connected;
  final int rpcPort;
  final bool beta;

  const SwarmStatus({
    this.role = 'none',
    this.roomCode = '',
    this.running = false,
    this.hostShare = 100,
    this.modelPath = '',
    this.workers = const [],
    this.hostAddr = '',
    this.connected = false,
    this.rpcPort = 50052,
    this.beta = false,
  });

  bool get isHost => role == 'host';
  bool get isWorker => role == 'worker';
  bool get isIdle => role == 'none' || role.isEmpty;

  factory SwarmStatus.fromJson(Map<String, dynamic> json) {
    final rawWorkers = json['workers'];
    final workers = <SwarmWorker>[];
    if (rawWorkers is List) {
      for (final w in rawWorkers) {
        if (w is Map<String, dynamic>) {
          workers.add(SwarmWorker.fromJson(w));
        } else if (w is Map) {
          workers.add(SwarmWorker.fromJson(Map<String, dynamic>.from(w)));
        }
      }
    }
    return SwarmStatus(
      role: json['role'] as String? ?? 'none',
      roomCode: json['room_code'] as String? ?? '',
      running: json['running'] == true,
      hostShare: (json['host_share'] as num?)?.toDouble() ?? 100,
      modelPath: json['model_path'] as String? ?? '',
      workers: workers,
      hostAddr: json['host_addr'] as String? ?? '',
      connected: json['connected'] == true,
      rpcPort: json['rpc_port'] as int? ?? 50052,
      beta: json['beta'] == true,
    );
  }

  Map<String, dynamic> toJson() => {
        'role': role,
        'room_code': roomCode,
        'running': running,
        'host_share': hostShare,
        'model_path': modelPath,
        'workers': workers.map((w) => w.toJson()).toList(),
        'host_addr': hostAddr,
        'connected': connected,
        'rpc_port': rpcPort,
        'beta': beta,
      };
}
