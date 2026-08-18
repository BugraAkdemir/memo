/// Chat message model — mirrors Go `sessions.ChatMessage`
class ChatMessage {
  final String role;
  final String content;
  final String? thinking;
  final String? imagePath;
  final String? filePath;
  final String timestamp;
  final List<dynamic>? agentEvents;
  // How many memories were retrieved and injected into the prompt for the
  // turn that produced this (assistant) message — 0 means either memory
  // was off or nothing relevant enough came back. Mirrors Go
  // sessions.ChatMessage.MemoryUsed.
  final int memoryUsed;

  const ChatMessage({
    required this.role,
    required this.content,
    this.thinking,
    this.imagePath,
    this.filePath,
    required this.timestamp,
    this.agentEvents,
    this.memoryUsed = 0,
  });

  factory ChatMessage.fromJson(Map<String, dynamic> json) => ChatMessage(
        role: json['role'] as String? ?? '',
        content: json['content'] as String? ?? '',
        thinking: json['thinking'] as String?,
        imagePath: json['image_path'] as String?,
        filePath: json['file_path'] as String?,
        timestamp: json['timestamp'] as String? ?? '',
        agentEvents: json['agent_events'] as List<dynamic>?,
        memoryUsed: json['memory_used'] as int? ?? 0,
      );

  Map<String, dynamic> toJson() => {
        'role': role,
        'content': content,
        if (thinking != null) 'thinking': thinking,
        if (imagePath != null) 'image_path': imagePath,
        if (filePath != null) 'file_path': filePath,
        'timestamp': timestamp,
        if (agentEvents != null) 'agent_events': agentEvents,
        if (memoryUsed > 0) 'memory_used': memoryUsed,
      };

  bool get isUser => role == 'user';
  bool get isAssistant => role == 'assistant';
  bool get hasImage => imagePath != null && imagePath!.isNotEmpty;
  bool get hasFile => filePath != null && filePath!.isNotEmpty;
  bool get hasThinking => thinking != null && thinking!.isNotEmpty;
}

/// A single chunk from the SSE stream, may contain content and/or thinking.
class StreamChunk {
  final String content;
  final String? thinking;
  final String? finishReason;

  const StreamChunk({
    required this.content,
    this.thinking,
    this.finishReason,
  });

  factory StreamChunk.fromJson(Map<String, dynamic> json) => StreamChunk(
        content: json['content'] as String? ?? '',
        thinking: json['thinking'] as String?,
        finishReason: json['finish_reason'] as String?,
      );
}

/// Chat session info — mirrors Go `sessions.SessionInfo`
class ChatSession {
  final String id;
  final String title;
  final String createdAt;
  final String updatedAt;
  final int msgCount;
  final String? projectPath;

  const ChatSession({
    required this.id,
    required this.title,
    required this.createdAt,
    required this.updatedAt,
    required this.msgCount,
    this.projectPath,
  });

  bool get isAgentChat => projectPath != null && projectPath!.isNotEmpty;

  factory ChatSession.fromJson(Map<String, dynamic> json) => ChatSession(
        id: json['id'] as String? ?? '',
        title: json['title'] as String? ?? 'New Chat',
        createdAt: json['created_at'] as String? ?? '',
        updatedAt: json['updated_at'] as String? ?? '',
        msgCount: json['msg_count'] as int? ?? 0,
        projectPath: json['project_path'] as String?,
      );
}
