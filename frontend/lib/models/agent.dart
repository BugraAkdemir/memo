class AgentEvent {
  final String type;
  final String? requestId;
  final String? toolName;
  final dynamic args;
  final String? result;
  final String? error;
  final String? dangerLevel;
  final int? durationMs;
  final String? content;
  final String? preview;

  const AgentEvent({
    required this.type,
    this.requestId,
    this.toolName,
    this.args,
    this.result,
    this.error,
    this.dangerLevel,
    this.durationMs,
    this.content,
    this.preview,
  });

  factory AgentEvent.fromJson(Map<String, dynamic> json) => AgentEvent(
        type: json['type'] as String? ?? 'unknown',
        requestId: json['request_id'] as String?,
        toolName: json['tool'] as String?,
        args: json['args'],
        result: json['result'] as String?,
        error: json['error'] as String?,
        dangerLevel: json['danger_level'] as String?,
        durationMs: json['duration_ms'] as int?,
        content: json['content'] as String?,
        preview: json['preview'] as String?,
      );

  Map<String, dynamic> toJson() => {
        'type': type,
        if (requestId != null) 'request_id': requestId,
        if (toolName != null) 'tool': toolName,
        if (args != null) 'args': args,
        if (result != null) 'result': result,
        if (error != null) 'error': error,
        if (dangerLevel != null) 'danger_level': dangerLevel,
        if (durationMs != null) 'duration_ms': durationMs,
        if (content != null) 'content': content,
        if (preview != null) 'preview': preview,
      };
}

class AgentPermission {
  final String id;
  final String toolName;
  final String argsHash;
  final String policy;
  final String createdAt;
  final String updatedAt;

  const AgentPermission({
    required this.id,
    required this.toolName,
    required this.argsHash,
    required this.policy,
    required this.createdAt,
    required this.updatedAt,
  });

  factory AgentPermission.fromJson(Map<String, dynamic> json) => AgentPermission(
        id: json['id'] as String? ?? '',
        toolName: json['tool_name'] as String? ?? '',
        argsHash: json['args_hash'] as String? ?? '',
        policy: json['policy'] as String? ?? '',
        createdAt: json['created_at'] as String? ?? '',
        updatedAt: json['updated_at'] as String? ?? '',
      );

  Map<String, dynamic> toJson() => {
        'id': id,
        'tool_name': toolName,
        'args_hash': argsHash,
        'policy': policy,
        'created_at': createdAt,
        'updated_at': updatedAt,
      };
}
