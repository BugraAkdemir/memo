/// Chat message model — mirrors Go `sessions.ChatMessage`
class ChatMessage {
  final String role;
  final String content;
  final String? thinking;
  final String? imagePath;
  final String? filePath;
  final String timestamp;

  const ChatMessage({
    required this.role,
    required this.content,
    this.thinking,
    this.imagePath,
    this.filePath,
    required this.timestamp,
  });

  factory ChatMessage.fromJson(Map<String, dynamic> json) => ChatMessage(
        role: json['role'] as String? ?? '',
        content: json['content'] as String? ?? '',
        thinking: json['thinking'] as String?,
        imagePath: json['image_path'] as String?,
        filePath: json['file_path'] as String?,
        timestamp: json['timestamp'] as String? ?? '',
      );

  Map<String, dynamic> toJson() => {
        'role': role,
        'content': content,
        if (thinking != null) 'thinking': thinking,
        if (imagePath != null) 'image_path': imagePath,
        if (filePath != null) 'file_path': filePath,
        'timestamp': timestamp,
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

  const StreamChunk({required this.content, this.thinking});

  factory StreamChunk.fromJson(Map<String, dynamic> json) => StreamChunk(
        content: json['content'] as String? ?? '',
        thinking: json['thinking'] as String?,
      );
}

/// Chat session info — mirrors Go `sessions.SessionInfo`
class ChatSession {
  final String id;
  final String title;
  final String createdAt;
  final String updatedAt;
  final int msgCount;

  const ChatSession({
    required this.id,
    required this.title,
    required this.createdAt,
    required this.updatedAt,
    required this.msgCount,
  });

  factory ChatSession.fromJson(Map<String, dynamic> json) => ChatSession(
        id: json['id'] as String? ?? '',
        title: json['title'] as String? ?? 'New Chat',
        createdAt: json['created_at'] as String? ?? '',
        updatedAt: json['updated_at'] as String? ?? '',
        msgCount: json['msg_count'] as int? ?? 0,
      );
}
