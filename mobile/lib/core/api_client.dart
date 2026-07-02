import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';

// ─── Models ───────────────────────────────────────────────────────────

class StreamChunk {
  final String content;
  final String? thinking;
  final String? finishReason;

  const StreamChunk({required this.content, this.thinking, this.finishReason});

  factory StreamChunk.fromJson(Map<String, dynamic> json) => StreamChunk(
        content: json['content'] as String? ?? '',
        thinking: json['thinking'] as String?,
        finishReason: json['finish_reason'] as String?,
      );

  bool get isAgentEvent => finishReason == 'agent_event';
}

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

  bool get isPermissionRequest => type == 'permission_request';
  bool get isToolExecuting => type == 'tool_executing';
  bool get isToolResult => type == 'tool_result';
  bool get isToolError => type == 'tool_error';
}

class ChatMessage {
  final String role;
  final String content;
  final String? thinking;
  final String? imagePath;
  final String? filePath;
  final String timestamp;
  final List<dynamic>? agentEvents;

  const ChatMessage({
    required this.role,
    required this.content,
    this.thinking,
    this.imagePath,
    this.filePath,
    required this.timestamp,
    this.agentEvents,
  });

  factory ChatMessage.fromJson(Map<String, dynamic> json) => ChatMessage(
        role: json['role'] as String? ?? '',
        content: json['content'] as String? ?? '',
        thinking: json['thinking'] as String?,
        imagePath: json['image_path'] as String?,
        filePath: json['file_path'] as String?,
        timestamp: json['timestamp'] as String? ?? '',
        agentEvents: json['agent_events'] as List<dynamic>?,
      );

  bool get isUser => role == 'user';
  bool get isAssistant => role == 'assistant';
}

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

  factory ChatSession.fromJson(Map<String, dynamic> json) => ChatSession(
        id: json['id'] as String? ?? '',
        title: json['title'] as String? ?? 'New Chat',
        createdAt: json['created_at'] as String? ?? '',
        updatedAt: json['updated_at'] as String? ?? '',
        msgCount: json['msg_count'] as int? ?? 0,
        projectPath: json['project_path'] as String?,
      );

  bool get isAgentChat => projectPath != null && projectPath!.isNotEmpty;
}

class RemoteAccessStatus {
  final bool enabled;
  final int port;
  final bool running;
  final List<String> addresses;
  final String token;
  final bool ngrokMode;
  final String ngrokToken;
  final String ngrokUrl;
  final String ngrokError;
  final bool ngrokAutoStart;
  final bool beta;

  const RemoteAccessStatus({
    this.enabled = false,
    this.port = 8090,
    this.running = false,
    this.addresses = const [],
    this.token = '',
    this.ngrokMode = false,
    this.ngrokToken = '',
    this.ngrokUrl = '',
    this.ngrokError = '',
    this.ngrokAutoStart = false,
    this.beta = false,
  });

  factory RemoteAccessStatus.fromJson(Map<String, dynamic> json) =>
      RemoteAccessStatus(
        enabled: json['enabled'] as bool? ?? false,
        port: json['port'] as int? ?? 8090,
        running: json['running'] as bool? ?? false,
        addresses: (json['addresses'] as List?)?.cast<String>() ?? [],
        token: json['token'] as String? ?? '',
        ngrokMode: json['ngrok_mode'] as bool? ?? false,
        ngrokToken: json['ngrok_token'] as String? ?? '',
        ngrokUrl: json['ngrok_url'] as String? ?? '',
        ngrokError: json['ngrok_error'] as String? ?? '',
        ngrokAutoStart: json['ngrok_auto_start'] as bool? ?? false,
        beta: json['beta'] as bool? ?? false,
      );

  String? get publicUrl =>
      ngrokUrl.isNotEmpty ? ngrokUrl : (addresses.isNotEmpty ? addresses.first : null);
}

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

  factory ProviderConfig.fromJson(Map<String, dynamic> json) => ProviderConfig(
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

  Map<String, dynamic> toJson() => {
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

class LocalModel {
  final String repoId;
  final String filename;
  final int size;
  final String path;
  final bool isEmbedding;
  final bool isVision;
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

  int get sizeBytes => size;

  String get sizeFormatted {
    if (size >= 1024 * 1024 * 1024) {
      return '${(size / (1024 * 1024 * 1024)).toStringAsFixed(1)} GB';
    } else if (size >= 1024 * 1024) {
      return '${(size / (1024 * 1024)).toStringAsFixed(1)} MB';
    }
    return '${(size / 1024).toStringAsFixed(0)} KB';
  }
}

class GPUInfo {
  final String type;
  final String name;
  final int vramMb;
  final int recommendedLayers;
  final String description;
  final int ramTotalMb;

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
}

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
}

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

class OrchestraConfig {
  final bool enabled;
  final String chiefType;
  final String chiefModel;
  final List<RoleConfig> roles;

  const OrchestraConfig({
    this.enabled = false,
    this.chiefType = 'claude',
    this.chiefModel = 'claude-sonnet-4-20250514',
    this.roles = const [],
  });

  factory OrchestraConfig.fromJson(Map<String, dynamic> json) => OrchestraConfig(
        enabled: json['enabled'] as bool? ?? false,
        chiefType: json['chief_type'] as String? ?? 'claude',
        chiefModel: json['chief_model'] as String? ?? 'claude-sonnet-4-20250514',
        roles: (json['roles'] as List? ?? [])
            .map((e) => RoleConfig.fromJson(e as Map<String, dynamic>))
            .toList(),
      );

  Map<String, dynamic> toJson() => {
        'enabled': enabled,
        'chief_type': chiefType,
        'chief_model': chiefModel,
        'roles': roles.map((r) => r.toJson()).toList(),
      };
}

class RoleConfig {
  final String role;
  final bool enabled;
  final String modelType;
  final String modelName;
  final String systemPrompt;

  const RoleConfig({
    required this.role,
    this.enabled = false,
    this.modelType = '',
    this.modelName = '',
    this.systemPrompt = '',
  });

  factory RoleConfig.fromJson(Map<String, dynamic> json) => RoleConfig(
        role: json['role'] as String? ?? '',
        enabled: json['enabled'] as bool? ?? false,
        modelType: json['model_type'] as String? ?? '',
        modelName: json['model_name'] as String? ?? '',
        systemPrompt: json['system_prompt'] as String? ?? '',
      );

  Map<String, dynamic> toJson() => {
        'role': role,
        'enabled': enabled,
        'model_type': modelType,
        'model_name': modelName,
        'system_prompt': systemPrompt,
      };
}

// ─── API Client ───────────────────────────────────────────────────────

class MemoApiClient {
  late final Dio _dio;
  String _baseUrl;
  String _token = '';

  Dio get dio => _dio;
  String get baseUrl => _baseUrl;

  // ignore: prefer_initializing_formals
  MemoApiClient({String baseUrl = ''}) : _baseUrl = baseUrl {
    _initDio();
  }

  void _initDio() {
    final headers = <String, dynamic>{'Content-Type': 'application/json'};
    if (_token.isNotEmpty) {
      headers['X-Memo-Token'] = _token;
    }
    _dio = Dio(
      BaseOptions(
        baseUrl: _baseUrl,
        connectTimeout: const Duration(seconds: 5),
        receiveTimeout: const Duration(seconds: 120),
        headers: headers,
      ),
    );
  }

  void updateBaseUrl(String url) {
    _baseUrl = url;
    _initDio();
  }

  void setToken(String token) {
    _token = token;
    _initDio();
  }

  String _extractErrorMessage(DioException e) {
    if (e.response?.data != null) {
      if (e.response?.data is String) return e.response!.data as String;
      if (e.response?.data is Map && e.response?.data['error'] != null) {
        return e.response?.data['error'].toString() ?? e.message ?? 'Unknown error';
      }
    }
    return e.message ?? 'Unknown error';
  }

  // ─── Chat ───────────────────────────────────────────────────────

  Future<String> sendMessage(String message) async {
    final res = await _dio.post('/api/send', data: {'message': message});
    return res.data['reply'] as String? ?? '';
  }

  Stream<StreamChunk> sendMessageStream(
    String message, {
    CancelToken? cancelToken,
  }) async* {
    try {
      final response = await _dio.post(
        '/api/send/stream',
        data: {'message': message},
        options: Options(responseType: ResponseType.stream),
        cancelToken: cancelToken,
      );

      final stream = response.data.stream;
      final lineStream = stream
          .cast<List<int>>()
          .transform(utf8.decoder)
          .transform(const LineSplitter());

      await for (final line in lineStream) {
        if (!line.startsWith('data: ')) continue;
        final jsonStr = line.substring(6);

        Map<String, dynamic> data;
        try {
          data = json.decode(jsonStr) as Map<String, dynamic>;
        } catch (_) {
          continue;
        }

        final err = data['error'];
        if (err is String && err.isNotEmpty) {
          throw Exception(err);
        }
        if (data['content'] != null ||
            data['thinking'] != null ||
            data['finish_reason'] != null) {
          yield StreamChunk(
            content: data['content'] as String? ?? '',
            thinking: data['thinking'] as String?,
            finishReason: data['finish_reason'] as String?,
          );
        }
        if (data['done'] == true) {
          break;
        }
      }
    } on DioException catch (e) {
      if (e.type == DioExceptionType.cancel) return;
      throw Exception('Bağlantı hatası: $e');
    }
  }

  Future<List<ChatSession>> listChats() async {
    final res = await _dio.get('/api/chats');
    if (res.data is List) {
      return (res.data as List)
          .map((e) => ChatSession.fromJson(e as Map<String, dynamic>))
          .toList();
    }
    if (res.data is Map && res.data['chats'] != null) {
      return (res.data['chats'] as List)
          .map((e) => ChatSession.fromJson(e as Map<String, dynamic>))
          .toList();
    }
    return [];
  }

  Future<String> newChat() async {
    final res = await _dio.post('/api/chats/new');
    return res.data['id'] as String? ?? '';
  }

  Future<String> createAgentChat(String projectPath) async {
    final res = await _dio.post(
      '/api/agent/chat',
      data: {'project_path': projectPath},
    );
    return res.data['id'] as String? ?? '';
  }

  Future<void> switchChat(String id) async {
    await _dio.post('/api/chats/switch', data: {'id': id});
  }

  Future<void> deleteChat(String id) async {
    await _dio.post('/api/chats/delete', data: {'id': id});
  }

  Future<void> renameChat(String id, String title) async {
    await _dio.post('/api/chats/rename', data: {'id': id, 'title': title});
  }

  Future<String> getActiveChatId() async {
    final res = await _dio.get('/api/chats/active');
    return res.data['id'] as String? ?? '';
  }

  Future<List<ChatMessage>> getMessages() async {
    final res = await _dio.get('/api/messages');
    if (res.data is List) {
      return (res.data as List)
          .map((e) => ChatMessage.fromJson(e as Map<String, dynamic>))
          .toList();
    }
    return [];
  }

  Future<void> updateMessage(int index, String content) async {
    await _dio.post('/api/messages/update', data: {'index': index, 'content': content});
  }

  Future<void> deleteMessage(int index) async {
    await _dio.post('/api/messages/delete', data: {'index': index});
  }

  Future<String> exportChat() async {
    final res = await _dio.post('/api/chat/export');
    return res.data['markdown'] as String? ?? '';
  }

  Future<String> generateTitle() async {
    final res = await _dio.post('/api/chat/title');
    return res.data['title'] as String? ?? '';
  }

  // ─── OpenRouter ───────────────────────────────────────────────

  Future<Map<String, dynamic>> connectOpenRouter({
    required String apiKey,
    String model = 'openai/gpt-4o',
  }) async {
    final res = await _dio.post(
      '/api/openrouter/connect',
      data: {'api_key': apiKey, 'model': model},
    );
    return res.data as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> fetchOpenRouterModels(String apiKey) async {
    final res = await _dio.post(
      '/api/openrouter/models',
      data: {'api_key': apiKey},
    );
    return res.data as Map<String, dynamic>;
  }

  // ─── Status ─────────────────────────────────────────────────────

  Future<Map<String, dynamic>> getStatus() async {
    final res = await _dio.get('/api/status');
    return res.data as Map<String, dynamic>;
  }

  // ─── Incognito ──────────────────────────────────────────────────

  Future<void> toggleIncognito(bool enabled) async {
    await _dio.post('/api/incognito', data: {'enabled': enabled});
  }

  // ─── System Prompt ──────────────────────────────────────────────

  Future<String> getSystemPrompt() async {
    final res = await _dio.get('/api/system-prompt');
    return res.data['prompt'] as String? ?? '';
  }

  Future<void> setSystemPrompt(String prompt) async {
    await _dio.put('/api/system-prompt', data: {'prompt': prompt});
  }

  Future<void> resetSystemPrompt() async {
    await _dio.post('/api/system-prompt/reset');
  }

  // ─── Incognito Prompt ───────────────────────────────────────────

  Future<String> getIncognitoPrompt() async {
    final res = await _dio.get('/api/incognito-prompt');
    return res.data['prompt'] as String? ?? '';
  }

  Future<void> setIncognitoPrompt(String prompt) async {
    await _dio.put('/api/incognito-prompt', data: {'prompt': prompt});
  }

  // ─── Memory ─────────────────────────────────────────────────────

  Future<List<MemoryFileInfo>> listMemoryFiles() async {
    final res = await _dio.get('/api/memory/files');
    if (res.data is List) {
      return (res.data as List)
          .map((e) => MemoryFileInfo.fromJson(e as Map<String, dynamic>))
          .toList();
    }
    return [];
  }

  Future<void> deleteMemoryFile(String path) async {
    await _dio.delete('/api/memory/files', data: {'path': path});
  }

  Future<void> clearAllMemory() async {
    await _dio.post('/api/memory/clear');
  }

  Future<Map<String, dynamic>> getMemorySettings() async {
    final res = await _dio.get('/api/memory/settings');
    return res.data as Map<String, dynamic>;
  }

  Future<void> updateMemorySettings({
    required int topK,
    required double minSimilarity,
  }) async {
    await _dio.put('/api/memory/settings', data: {
      'top_k': topK,
      'min_similarity': minSimilarity,
    });
  }

  Future<bool> getMemoryEnabled() async {
    final res = await _dio.get('/api/memory/enabled');
    return res.data['enabled'] as bool? ?? false;
  }

  Future<void> setMemoryEnabled(bool enabled) async {
    await _dio.put('/api/memory/enabled', data: {'enabled': enabled});
  }

  Future<List<MemorySearchResult>> debugMemorySearch(String query) async {
    final res = await _dio.get(
      '/api/memory/debug-search',
      queryParameters: {'q': query},
    );
    if (res.data is List) {
      return (res.data as List)
          .map((e) => MemorySearchResult.fromJson(e as Map<String, dynamic>))
          .toList();
    }
    return [];
  }

  Future<void> saveExplicitMemory(String content, {String tags = ''}) async {
    await _dio.post('/api/memory/explicit/save', data: {'content': content, 'tags': tags});
  }

  Future<int> deleteExplicitMemory(String pattern) async {
    final res = await _dio.post('/api/memory/explicit/delete', data: {'pattern': pattern});
    return (res.data['deleted'] as int?) ?? 0;
  }

  Future<String> exportMemories() async {
    final res = await _dio.get('/api/memory/export');
    if (res.data is String) return res.data as String;
    return '';
  }

  Future<int> importMemories(String jsonData) async {
    final res = await _dio.post('/api/memory/import', data: jsonData,
        options: Options(headers: {'Content-Type': 'application/json'}));
    return (res.data['imported'] as int?) ?? 0;
  }

  Future<MemoryStats> getMemoryStats() async {
    final res = await _dio.get('/api/memory/stats');
    return MemoryStats.fromJson(res.data as Map<String, dynamic>);
  }

  Future<List<MemorySearchResult>> filteredMemorySearch(
      String query, {String? since, String? tag}) async {
    final params = <String, dynamic>{'q': query};
    if (since != null) params['since'] = since;
    if (tag != null && tag.isNotEmpty) params['tag'] = tag;
    final res = await _dio.get('/api/memory/search', queryParameters: params);
    if (res.data is List) {
      return (res.data as List)
          .map((e) => MemorySearchResult.fromJson(e as Map<String, dynamic>))
          .toList();
    }
    return [];
  }

  // ─── Models (Local) ─────────────────────────────────────────────

  Future<List<LocalModel>> listLocalModels() async {
    final res = await _dio.get('/api/models/local');
    if (res.data is List) {
      return (res.data as List)
          .map((e) => LocalModel.fromJson(e as Map<String, dynamic>))
          .toList();
    }
    if (res.data is Map && res.data['models'] != null) {
      return (res.data['models'] as List)
          .map((e) => LocalModel.fromJson(e as Map<String, dynamic>))
          .toList();
    }
    return [];
  }

  Future<void> deleteLocalModel(String path) async {
    await _dio.delete('/api/models/local', data: {'path': path});
  }

  Future<void> importModel(String path) async {
    await _dio.post('/api/models/import', data: {'path': path});
  }

  Future<void> startModel({
    required String path,
    int ctxSize = 4096,
    int port = 8081,
    int gpuLayers = -1,
  }) async {
    try {
      await _dio.post(
        '/api/models/start',
        data: {
          'path': path,
          'ctx_size': ctxSize,
          'port': port,
          'gpu_layers': gpuLayers,
        },
      );
    } on DioException catch (e) {
      throw Exception(_extractErrorMessage(e));
    }
  }

  Future<void> stopModel() async {
    await _dio.post('/api/models/stop');
  }

  Future<ServerStatus> getModelStatus() async {
    final res = await _dio.get('/api/models/status');
    return ServerStatus.fromJson(res.data as Map<String, dynamic>);
  }

  Future<Map<String, dynamic>> getLlamaConfig() async {
    final res = await _dio.get('/api/models/config');
    return res.data as Map<String, dynamic>;
  }

  Future<void> updateLlamaConfig({
    String? engineMode,
    String? binaryPath,
    int? port,
    int? ctxSize,
    double? temperature,
    double? topP,
    int? maxTokens,
  }) async {
    final data = <String, dynamic>{};
    if (engineMode != null) data['engine_mode'] = engineMode;
    if (binaryPath != null) data['binary_path'] = binaryPath;
    if (port != null) data['port'] = port;
    if (ctxSize != null) data['ctx_size'] = ctxSize;
    if (temperature != null) data['temperature'] = temperature;
    if (topP != null) data['top_p'] = topP;
    if (maxTokens != null) data['max_tokens'] = maxTokens;
    await _dio.put('/api/models/config', data: data);
  }

  // ─── Embedding Model ───────────────────────────────────────────

  Future<void> startEmbeddingModel({
    required String path,
    int gpuLayers = -1,
  }) async {
    try {
      await _dio.post(
        '/api/models/embedding/start',
        data: {'path': path, 'gpu_layers': gpuLayers},
      );
    } on DioException catch (e) {
      throw Exception(_extractErrorMessage(e));
    }
  }

  Future<void> stopEmbeddingModel() async {
    await _dio.post('/api/models/embedding/stop');
  }

  Future<ServerStatus> getEmbeddingStatus() async {
    final res = await _dio.get('/api/models/embedding/status');
    return ServerStatus.fromJson(res.data as Map<String, dynamic>);
  }

  // ─── GPU ────────────────────────────────────────────────────────

  Future<GPUInfo> getGpuInfo() async {
    final res = await _dio.get('/api/gpu');
    return GPUInfo.fromJson(res.data as Map<String, dynamic>);
  }

  // ─── Model Store (HF) ──────────────────────────────────────────

  Future<List<HFModelResult>> searchModels(String query) async {
    final res = await _dio.post('/api/models/search', data: {'query': query});
    if (res.data is List) {
      return (res.data as List)
          .map((e) => HFModelResult.fromJson(e as Map<String, dynamic>))
          .toList();
    }
    return [];
  }

  Future<List<GGUFFile>> getModelFiles(String repoId) async {
    final res = await _dio.get(
      '/api/models/files',
      queryParameters: {'repo': repoId},
    );
    if (res.data is List) {
      return (res.data as List)
          .map((e) => GGUFFile.fromJson(e as Map<String, dynamic>))
          .toList();
    }
    return [];
  }

  Future<void> downloadModel(String repoId, String filename,
      {int expectedSize = 0}) async {
    await _dio.post(
      '/api/models/download',
      data: {
        'repo_id': repoId,
        'filename': filename,
        'expected_size': expectedSize,
      },
    );
  }

  Future<DownloadProgress> getDownloadProgress() async {
    final res = await _dio.get('/api/models/download/progress');
    return DownloadProgress.fromJson(res.data as Map<String, dynamic>);
  }

  Future<void> cancelDownload() async {
    await _dio.post('/api/models/download/cancel');
  }

  // ─── Llama Installation ─────────────────────────────────────────

  Future<bool> checkLlamaInstallation() async {
    final res = await _dio.get('/api/models/llama/check');
    return res.data['installed'] as bool? ?? false;
  }

  Future<void> installLlamaServer() async {
    await _dio.post(
      '/api/models/llama/install',
      options: Options(receiveTimeout: const Duration(minutes: 15)),
    );
  }

  Future<void> skipLlamaGPUInstall() async {
    await _dio.post('/api/models/llama/skip');
  }

  // ─── Remote Access ──────────────────────────────────────────────

  Future<Map<String, dynamic>> getRemoteAccessRaw() async {
    final res = await _dio.get('/api/remote-access');
    return res.data as Map<String, dynamic>;
  }

  Future<RemoteAccessStatus> getRemoteAccess() async {
    final res = await _dio.get('/api/remote-access');
    return RemoteAccessStatus.fromJson(res.data as Map<String, dynamic>);
  }

  Future<void> setRemoteAccess(bool enabled, int port, {bool ngrokMode = false, String ngrokToken = ''}) async {
    await _dio.put('/api/remote-access', data: {
      'enabled': enabled,
      'port': port,
      'ngrok_mode': ngrokMode,
      'ngrok_token': ngrokToken,
    });
  }

  Future<void> setRemoteAccessAutoStart(bool autoStart) async {
    await _dio.put('/api/remote-access', data: {'ngrok_auto_start': autoStart});
  }

  Future<void> setBeta(bool enabled) async {
    await _dio.put('/api/remote-access', data: {'beta': enabled});
  }

  Future<void> setTailscaleMode(
    bool enabled,
    int port, {
    String authKey = '',
    String hostname = '',
    bool funnel = false,
  }) async {
    try {
      await _dio.put('/api/remote-access', data: {
        'enabled': enabled,
        'port': port,
        'tunnel_mode': 'tailscale',
        'tailscale_key': authKey,
        'tailscale_hostname': hostname,
        'tailscale_funnel': funnel,
      });
    } on DioException catch (e) {
      final body = e.response?.data;
      final msg = (body is String && body.trim().isNotEmpty)
          ? body.trim()
          : (e.message ?? 'bilinmeyen hata');
      throw Exception(msg);
    }
  }

  // ─── Sync ───────────────────────────────────────────────────────

  Future<bool> checkSyncAuth() async {
    final res = await _dio.get('/api/sync/auth');
    return res.data['authenticated'] as bool? ?? false;
  }

  Future<String> startSyncAuth() async {
    final res = await _dio.post('/api/sync/auth');
    return res.data['url'] as String? ?? '';
  }

  Future<Map<String, dynamic>> getSyncAccount() async {
    final res = await _dio.get('/api/sync/account');
    return res.data as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> getSyncSettings() async {
    final res = await _dio.get('/api/sync/settings');
    return res.data as Map<String, dynamic>;
  }

  Future<void> updateSyncSettings({
    required bool enabled,
    required String clientId,
    required String clientSecret,
    required String passphrase,
    required String tokenPath,
    required int intervalMessages,
  }) async {
    await _dio.put('/api/sync/settings', data: {
      'enabled': enabled,
      'client_id': clientId,
      'client_secret': clientSecret,
      'passphrase': passphrase,
      'token_path': tokenPath,
      'interval_messages': intervalMessages,
    });
  }

  Future<void> triggerSync() async {
    await _dio.post('/api/sync/trigger');
  }

  Future<void> pullSync() async {
    await _dio.post('/api/sync/pull');
  }

  Future<void> syncNow() async {
    await _dio.post('/api/sync/now');
  }

  Future<void> disconnectSync() async {
    await _dio.post('/api/sync/disconnect');
  }

  // ─── Backup / Restore (.memo) ────────────────────────────────────

  Future<List<int>> exportData({bool includeModels = false}) async {
    final res = await _dio.get('/api/export',
        queryParameters: {'include_models': includeModels.toString()},
        options: Options(responseType: ResponseType.bytes));
    return res.data as List<int>;
  }

  Future<void> importData(List<int> data) async {
    await _dio.post('/api/import',
        data: Stream.fromIterable([data]),
        options: Options(
          contentType: 'application/octet-stream',
          headers: {'Content-Length': data.length.toString()},
        ));
  }

  Future<void> wipeData() async {
    await _dio.post('/api/wipe');
  }

  // ─── Transcription ───────────────────────────────────────────────

  Future<String> transcribeAudio(Uint8List audioData) async {
    final res = await _dio.post(
      '/api/transcribe',
      data: audioData,
      options: Options(contentType: 'application/octet-stream'),
    );
    return res.data['text'] as String? ?? '';
  }

  // ─── Image ──────────────────────────────────────────────────────

  Future<String> getImageBase64(String path) async {
    final res = await _dio.get('/api/image', queryParameters: {'path': path});
    return res.data['data'] as String? ?? '';
  }

  // ─── File Upload ────────────────────────────────────────────────

  Future<String> sendFile(String message, String filePath) async {
    final formData = FormData.fromMap({
      'message': message,
      'file': await MultipartFile.fromFile(filePath),
    });
    final res = await _dio.post('/api/send_file', data: formData);
    return res.data['reply'] as String? ?? '';
  }

  Stream<StreamChunk> sendFileStream(
    String message,
    String filePath, {
    CancelToken? cancelToken,
  }) async* {
    try {
      final formData = FormData.fromMap({
        'message': message,
        'file': await MultipartFile.fromFile(filePath),
      });
      final response = await _dio.post(
        '/api/send_file/stream',
        data: formData,
        options: Options(responseType: ResponseType.stream),
        cancelToken: cancelToken,
      );

      final stream = response.data.stream;
      final lineStream = stream
          .cast<List<int>>()
          .transform(utf8.decoder)
          .transform(const LineSplitter());

      await for (final line in lineStream) {
        if (!line.startsWith('data: ')) continue;
        final jsonStr = line.substring(6);

        Map<String, dynamic> data;
        try {
          data = json.decode(jsonStr) as Map<String, dynamic>;
        } catch (_) {
          continue;
        }

        final err = data['error'];
        if (err is String && err.isNotEmpty) {
          throw Exception(err);
        }
        if (data['content'] != null ||
            data['thinking'] != null ||
            data['finish_reason'] != null) {
          yield StreamChunk(
            content: data['content'] as String? ?? '',
            thinking: data['thinking'] as String?,
            finishReason: data['finish_reason'] as String?,
          );
        }
        if (data['done'] == true) {
          break;
        }
      }
    } on DioException catch (e) {
      if (e.type == DioExceptionType.cancel) return;
      throw Exception('Bağlantı hatası: $e');
    }
  }

  // ─── Health check ───────────────────────────────────────────────

  Future<bool> isAlive() async {
    try {
      await _dio.get('/api/version');
      return true;
    } catch (e) {
      return false;
    }
  }

  // ─── Provider Management ─────────────────────────────────────────

  Future<List<ProviderConfig>> getProviders() async {
    final res = await _dio.get('/api/providers');
    if (res.data is List) {
      return (res.data as List)
          .map((e) => ProviderConfig.fromJson(e as Map<String, dynamic>))
          .toList();
    }
    return [];
  }

  Future<void> updateProvider(ProviderConfig config) async {
    await _dio.put('/api/providers', data: config.toJson());
  }

  Future<void> deleteProvider(String type, {String? name}) async {
    await _dio.delete('/api/providers', data: {
      'type': type,
      if (name != null && name.isNotEmpty) 'name': name,
    });
  }

  Future<Map<String, dynamic>> testProvider(ProviderConfig config) async {
    final res = await _dio.post('/api/providers/test', data: config.toJson());
    return res.data as Map<String, dynamic>;
  }

  Future<String> getActiveProvider() async {
    final res = await _dio.get('/api/providers/active');
    return res.data['provider'] as String? ?? '';
  }

  Future<void> setActiveProvider(String type) async {
    await _dio.put('/api/providers/active', data: {'provider': type});
  }

  // ─── Orchestra Mode ───────────────────────────────────────────────

  Future<OrchestraConfig> getOrchestraConfig() async {
    final res = await _dio.get('/api/orchestra/config');
    return OrchestraConfig.fromJson(res.data as Map<String, dynamic>);
  }

  Future<void> updateOrchestraConfig(OrchestraConfig config) async {
    await _dio.put('/api/orchestra/config', data: config.toJson());
  }

  // ─── Agent Mode ───────────────────────────────────────────────────

  Future<bool> getAgentEnabled() async {
    final res = await _dio.get('/api/agent/enabled');
    return res.data['enabled'] as bool? ?? false;
  }

  Future<void> setAgentEnabled(bool enabled) async {
    await _dio.put('/api/agent/enabled', data: {'enabled': enabled});
  }

  Future<void> handleAgentPermission(String requestId, String policy) async {
    await _dio.post(
      '/api/agent/permission',
      data: {'request_id': requestId, 'policy': policy},
    );
  }

  Future<List<Map<String, dynamic>>> getAgentPermissions() async {
    final res = await _dio.get('/api/agent/permissions');
    if (res.data is List) {
      return List<Map<String, dynamic>>.from(res.data as List);
    }
    return [];
  }

  Future<void> revokeAgentPermission(String id) async {
    await _dio.delete('/api/agent/permissions', queryParameters: {'id': id});
  }

  Future<void> clearAgentPermissions() async {
    await _dio.delete('/api/agent/permissions');
  }

  Future<void> undoAgentEdit() async {
    await _dio.post('/api/agent/undo');
  }

  Future<bool> getAgentAutoPermission() async {
    final res = await _dio.get('/api/agent/auto-permission');
    return res.data['enabled'] as bool? ?? false;
  }

  Future<void> setAgentAutoPermission(bool enabled) async {
    await _dio.put('/api/agent/auto-permission', data: {'enabled': enabled});
  }

  // ─── Skills ──────────────────────────────────────────────────

  Future<List<Map<String, dynamic>>> listSkills() async {
    final res = await _dio.get('/api/skills/list');
    if (res.data is List) {
      return List<Map<String, dynamic>>.from(res.data as List);
    }
    return [];
  }

  Future<Map<String, dynamic>> getSkill(String name) async {
    final res = await _dio.get('/api/skills/get/$name');
    return res.data as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> installSkill(String path) async {
    final res = await _dio.post('/api/skills/install', data: {'path': path});
    return res.data as Map<String, dynamic>;
  }

  Future<void> removeSkill(String name) async {
    await _dio.delete('/api/skills/remove/$name');
  }

  Future<void> setActiveSkills(List<String> names) async {
    await _dio.put('/api/skills/active', data: {'names': names});
  }

  Future<List<String>> getActiveSkills() async {
    final res = await _dio.get('/api/skills/active-list');
    if (res.data is Map && res.data['names'] is List) {
      return (res.data['names'] as List).cast<String>();
    }
    return [];
  }

  // ─── Version Check ───────────────────────────────────────────

  Future<String> getVersion() async {
    final res = await _dio.get('/api/version');
    return res.data['version'] as String? ?? 'unknown';
  }

  Future<Map<String, dynamic>> checkVersion() async {
    try {
      final res = await _dio.get('/api/version/check');
      return res.data as Map<String, dynamic>;
    } catch (e) {
      return {'current': 'unknown', 'latest': null, 'error': e.toString()};
    }
  }

  // ─── WhatsApp ──────────────────────────────────────────────────

  Future<Map<String, dynamic>> getWhatsAppStatus() async {
    final res = await _dio.get('/api/whatsapp/status');
    return res.data as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> startWhatsApp() async {
    final res = await _dio.post('/api/whatsapp/start');
    return res.data as Map<String, dynamic>;
  }

  Future<void> stopWhatsApp() async {
    await _dio.post('/api/whatsapp/stop');
  }

  Future<void> logoutWhatsApp() async {
    await _dio.post('/api/whatsapp/logout');
  }

  Future<Map<String, dynamic>> sendWhatsApp(String jid, String text) async {
    final res = await _dio.post('/api/whatsapp/send', data: {'jid': jid, 'text': text});
    return res.data as Map<String, dynamic>;
  }

  Future<List<dynamic>> searchWhatsApp(String query) async {
    final res = await _dio.get('/api/whatsapp/search', queryParameters: {'q': query});
    return (res.data as List<dynamic>?) ?? [];
  }

  Future<List<dynamic>> getWhatsAppChats() async {
    final res = await _dio.get('/api/whatsapp/chats');
    return (res.data as List<dynamic>?) ?? [];
  }

  Future<List<dynamic>> getWhatsAppMessages(String jid) async {
    final res = await _dio.get('/api/whatsapp/messages', queryParameters: {'jid': jid});
    return (res.data as List<dynamic>?) ?? [];
  }

  String whatsAppAvatarUrl(String jid, {bool full = false}) =>
      '$baseUrl/api/whatsapp/avatar?jid=${Uri.encodeComponent(jid)}'
      '${full ? '&full=1' : ''}';

  Future<Uint8List> fetchWhatsAppAvatarBytes(String jid,
      {bool full = true}) async {
    final res = await _dio.get(
      '/api/whatsapp/avatar',
      queryParameters: {'jid': jid, if (full) 'full': '1'},
      options: Options(responseType: ResponseType.bytes),
    );
    return Uint8List.fromList(res.data as List<int>);
  }

  Future<Map<String, dynamic>> getWhatsAppStats() async {
    final res = await _dio.get('/api/whatsapp/stats');
    return res.data as Map<String, dynamic>;
  }

  // ─── Web Search ────────────────────────────────────────────────

  Future<bool> getWebSearchEnabled() async {
    final res = await _dio.get('/api/websearch');
    return (res.data as Map<String, dynamic>?)?['enabled'] == true;
  }

  Future<void> setWebSearchEnabled(bool enabled) async {
    await _dio.post('/api/websearch', data: {'enabled': enabled});
  }

  // ─── WhatsApp Chat Mode ──────────────────────────────────────────

  Future<bool> getWhatsAppChatMode() async {
    final res = await _dio.get('/api/whatsapp/chat-mode');
    return (res.data as Map<String, dynamic>?)?['enabled'] == true;
  }

  Future<void> setWhatsAppChatMode(bool enabled) async {
    await _dio.post('/api/whatsapp/chat-mode', data: {'enabled': enabled});
  }

  Stream<StreamChunk> sendWhatsAppChatStream(String message,
      {CancelToken? cancelToken}) async* {
    try {
      final response = await _dio.post(
        '/api/whatsapp/chat-stream',
        data: {'message': message},
        options: Options(responseType: ResponseType.stream),
        cancelToken: cancelToken,
      );
      final stream = response.data.stream;
      final lineStream = stream
          .cast<List<int>>()
          .transform(utf8.decoder)
          .transform(const LineSplitter());
      await for (final line in lineStream) {
        if (cancelToken?.isCancelled == true) return;
        if (!line.startsWith('data: ')) continue;
        final jsonStr = line.substring(6);
        if (jsonStr == '[DONE]') return;
        try {
          final data = json.decode(jsonStr) as Map<String, dynamic>;
          final err = data['error'] as String?;
          if (err != null && err.isNotEmpty) {
            throw Exception(err);
          }
          yield StreamChunk.fromJson(data);
        } on FormatException {
          // Ignore malformed/non-JSON keep-alive lines.
        }
      }
    } catch (e) {
      throw Exception('WhatsApp stream error: $e');
    }
  }

  // ─── Proactive Learning ─────────────────────────────────────────

  Future<Map<String, dynamic>> getProactiveSettings() async {
    final res = await _dio.get('/api/proactive/settings');
    return res.data as Map<String, dynamic>;
  }

  Future<void> setProactiveSettings(bool enabled, String level) async {
    await _dio.post('/api/proactive/settings', data: {
      'enabled': enabled,
      'level': level,
    });
  }

  Future<List<Map<String, dynamic>>> getProactivePatterns() async {
    final res = await _dio.get('/api/proactive/patterns');
    return (res.data['patterns'] as List?)?.cast<Map<String, dynamic>>() ?? [];
  }

  Future<void> forgetPattern(String id) async {
    await _dio.post('/api/proactive/patterns/forget', data: {'id': id});
  }

  Future<void> clearLearningData() async {
    await _dio.post('/api/proactive/clear');
  }

  Future<Map<String, dynamic>?> getPendingSuggestion() async {
    final res = await _dio.get('/api/proactive/pending');
    return res.data['pending'] as Map<String, dynamic>?;
  }

  Future<void> respondToSuggestion(String id, String response) async {
    await _dio.post('/api/proactive/respond', data: {
      'id': id,
      'response': response,
    });
  }

  // ─── Learning model routing ─────────────────────────────────────

  Future<Map<String, dynamic>> getLearningSettings() async {
    final res = await _dio.get('/api/learning/settings');
    return Map<String, dynamic>.from(res.data as Map);
  }

  Future<void> updateLearningSettings({
    required bool singleModelEnabled,
    required String modelId,
  }) async {
    await _dio.put('/api/learning/settings', data: {
      'single_model_enabled': singleModelEnabled,
      'model_id': modelId,
    });
  }

  // ─── Calendar ───────────────────────────────────────────────────

  Future<List<Map<String, dynamic>>> getCalendarEvents({DateTime? from, DateTime? to}) async {
    final params = <String, String>{};
    if (from != null) params['from'] = from.toUtc().toIso8601String();
    if (to != null) params['to'] = to.toUtc().toIso8601String();
    final res = await _dio.get('/api/calendar/events', queryParameters: params);
    if (res.data is List) {
      return List<Map<String, dynamic>>.from(res.data as List);
    }
    return [];
  }

  Future<Map<String, dynamic>> addCalendarEvent({
    required String title,
    required DateTime startTime,
    String description = '',
  }) async {
    final res = await _dio.post('/api/calendar/events', data: {
      'title': title,
      'start_time': startTime.toUtc().toIso8601String(),
      'description': description,
    });
    return Map<String, dynamic>.from(res.data as Map);
  }

  Future<void> deleteCalendarEvent(String id) async {
    await _dio.delete('/api/calendar/events/$id');
  }

  Future<Map<String, dynamic>> getCalendarSettings() async {
    final res = await _dio.get('/api/calendar/settings');
    return Map<String, dynamic>.from(res.data as Map);
  }

  Future<void> updateCalendarSettings({
    required int reminderLeadMinutes,
    bool disableTimeGuess = false,
  }) async {
    await _dio.put('/api/calendar/settings', data: {
      'reminder_lead_minutes': reminderLeadMinutes,
      'disable_time_guess': disableTimeGuess,
    });
  }

  // ─── Mood ───────────────────────────────────────────────────────

  Future<double> getMoodScore() async {
    try {
      final res = await _dio.get('/api/mood/score');
      final v = res.data['score'];
      return (v as num?)?.toDouble() ?? 0.0;
    } catch (_) {
      return 0.0;
    }
  }

  Future<bool> getMoodEnabled() async {
    try {
      final res = await _dio.get('/api/mood/settings');
      return (res.data['enabled'] as bool?) ?? false;
    } catch (_) {
      return false;
    }
  }

  Future<void> setMoodEnabled(bool enabled) async {
    await _dio.put('/api/mood/settings', data: {'enabled': enabled});
  }

  Future<bool> getSelfInterestEnabled() async {
    try {
      final res = await _dio.get('/api/mood/self-interest');
      return (res.data['enabled'] as bool?) ?? false;
    } catch (_) {
      return false;
    }
  }

  Future<void> setSelfInterestEnabled(bool enabled) async {
    await _dio.put('/api/mood/self-interest', data: {'enabled': enabled});
  }

  Future<bool> getSystemManagementEnabled() async {
    try {
      final res = await _dio.get('/api/mood/system-management');
      return (res.data['enabled'] as bool?) ?? false;
    } catch (_) {
      return false;
    }
  }

  Future<void> setSystemManagementEnabled(bool enabled) async {
    await _dio.put('/api/mood/system-management', data: {'enabled': enabled});
  }

  // ─── Events ─────────────────────────────────────────────────────

  Future<List<Map<String, dynamic>>> getAppEvents() async {
    try {
      final res = await _dio.get('/api/events');
      if (res.data is List) {
        return List<Map<String, dynamic>>.from(res.data as List);
      }
    } catch (_) {}
    return [];
  }

  // ─── Shutdown ───────────────────────────────────────────────────

  Future<void> shutdown() async {
    try {
      await _dio.post('/api/shutdown');
    } catch (_) {
      // Server closes mid-request — ignore transport errors.
    }
  }

  /// Returns the backend's listen port (e.g. 8090).
  Future<int> getListenPort() async {
    try {
      final res = await _dio.get('/api/status');
      return (res.data['port'] as num?)?.toInt() ?? 8090;
    } catch (_) {
      return 8090;
    }
  }
}
