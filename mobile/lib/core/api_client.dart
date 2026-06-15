import 'dart:async';
import 'dart:convert';

import 'package:dio/dio.dart';

class StreamChunk {
  final String content;
  final String? thinking;
  final String? finishReason;

  const StreamChunk({
    required this.content,
    this.thinking,
    this.finishReason,
  });

  bool get isAgentEvent => finishReason == 'agent_event';
}

/// An agent event received during streaming (tool execution info).
class AgentEvent {
  final String type;
  final String? requestId;
  final String? toolName;
  final String? error;
  final String? dangerLevel;
  final int? durationMs;

  const AgentEvent({
    required this.type,
    this.requestId,
    this.toolName,
    this.error,
    this.dangerLevel,
    this.durationMs,
  });

  factory AgentEvent.fromJson(Map<String, dynamic> json) => AgentEvent(
        type: json['type'] as String? ?? 'unknown',
        requestId: json['request_id'] as String?,
        toolName: json['tool'] as String?,
        error: json['error'] as String?,
        dangerLevel: json['danger_level'] as String?,
        durationMs: json['duration_ms'] as int?,
      );

  bool get isPermissionRequest => type == 'permission_request';
  bool get isToolExecuting => type == 'tool_executing';
  bool get isToolResult => type == 'tool_result';
  bool get isToolError => type == 'tool_error';
}

class ChatMessage {
  final String role;
  final String content;
  final String? thinking;
  final String timestamp;

  const ChatMessage({
    required this.role,
    required this.content,
    this.thinking,
    required this.timestamp,
  });

  factory ChatMessage.fromJson(Map<String, dynamic> json) => ChatMessage(
        role: json['role'] as String? ?? '',
        content: json['content'] as String? ?? '',
        thinking: json['thinking'] as String?,
        timestamp: json['timestamp'] as String? ?? '',
      );

  bool get isUser => role == 'user';
  bool get isAssistant => role == 'assistant';
}

class ChatSession {
  final String id;
  final String title;
  final int msgCount;
  final String updatedAt;

  const ChatSession({
    required this.id,
    required this.title,
    required this.msgCount,
    required this.updatedAt,
  });

  factory ChatSession.fromJson(Map<String, dynamic> json) => ChatSession(
        id: json['id'] as String? ?? '',
        title: json['title'] as String? ?? 'New Chat',
        msgCount: json['msg_count'] as int? ?? 0,
        updatedAt: json['updated_at'] as String? ?? '',
      );
}

class MemoApiClient {
  late Dio _dio;
  String _baseUrl;
  String _token = '';

  MemoApiClient({String this._baseUrl = 'http://192.168.1.100:8090'}) {
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

  String get baseUrl => _baseUrl;

  void updateBaseUrl(String url) {
    _baseUrl = url;
    _initDio();
  }

  void setToken(String token) {
    _token = token;
    _initDio();
  }

  Future<bool> isAlive() async {
    try {
      await _dio.get('/api/version');
      return true;
    } catch (_) {
      return false;
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

  Future<void> switchChat(String id) async {
    await _dio.post('/api/chats/switch', data: {'id': id});
  }

  Future<void> deleteChat(String id) async {
    await _dio.post('/api/chats/delete', data: {'id': id});
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

  Future<Map<String, dynamic>> getStatus() async {
    final res = await _dio.get('/api/status');
    return res.data as Map<String, dynamic>;
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
        if (line.startsWith('data: ')) {
          final jsonStr = line.substring(6);
          try {
            final data = json.decode(jsonStr);
            if (data['error'] != null && (data['error'] as String).isNotEmpty) {
              throw Exception(data['error'] as String);
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
            if (data['done'] == true) break;
          } catch (_) {}
        }
      }
    } on DioException catch (e) {
      if (e.type == DioExceptionType.cancel) return;
      throw Exception('Connection error: $e');
    } catch (e) {
      throw Exception('Connection error: $e');
    }
  }

  Future<String> getActiveProvider() async {
    final res = await _dio.get('/api/providers/active');
    return res.data['provider'] as String? ?? '';
  }

  Future<void> setActiveProvider(String type) async {
    await _dio.put('/api/providers/active', data: {'provider': type});
  }

  Future<bool> getAgentEnabled() async {
    final res = await _dio.get('/api/agent/enabled');
    return res.data['enabled'] as bool? ?? false;
  }

  Future<void> setAgentEnabled(bool enabled) async {
    await _dio.put('/api/agent/enabled', data: {'enabled': enabled});
  }

  /// Send permission response for agent tool execution.
  Future<void> handleAgentPermission(String requestId, String policy) async {
    await _dio.post('/api/agent/permission', data: {
      'request_id': requestId,
      'policy': policy,
    });
  }

  /// Get list of active skill names.
  Future<List<String>> getActiveSkills() async {
    final res = await _dio.get('/api/skills/active-list');
    if (res.data is Map && res.data['names'] is List) {
      return (res.data['names'] as List).cast<String>();
    }
    return [];
  }

  /// Set active skills.
  Future<void> setActiveSkills(List<String> names) async {
    await _dio.put('/api/skills/active', data: {'names': names});
  }

  /// List all installed skills.
  Future<List<Map<String, dynamic>>> listSkills() async {
    final res = await _dio.get('/api/skills/list');
    if (res.data is List) {
      return (res.data as List).cast<Map<String, dynamic>>();
    }
    return [];
  }

  Future<void> startModel({
    required String path,
    int ctxSize = 4096,
    int port = 8081,
    int gpuLayers = -1,
  }) async {
    await _dio.post(
      '/api/models/start',
      data: {
        'path': path,
        'ctx_size': ctxSize,
        'port': port,
        'gpu_layers': gpuLayers,
      },
    );
  }

  Future<void> stopModel() async {
    await _dio.post('/api/models/stop');
  }

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

  Future<void> deleteProvider(String type) async {
    await _dio.delete('/api/providers', data: {'type': type});
  }

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

  Future<void> downloadModel(String repoId, String filename) async {
    await _dio.post('/api/models/download', data: {
      'repo_id': repoId,
      'filename': filename,
    });
  }

  Future<Map<String, dynamic>> getModelStatus() async {
    final res = await _dio.get('/api/models/status');
    return res.data as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> getDownloadProgress() async {
    final res = await _dio.get('/api/models/download/progress');
    return res.data as Map<String, dynamic>;
  }

  // ─── Remote Access ──────────────────────────────────────────────

  Future<RemoteAccessStatus> getRemoteAccess() async {
    final res = await _dio.get('/api/remote-access');
    return RemoteAccessStatus.fromJson(res.data as Map<String, dynamic>);
  }

  Future<void> setRemoteAccess(bool enabled, int port, {bool ngrokMode = false, String ngrokToken = ''}) async {
    await _dio.put(
      '/api/remote-access',
      data: {
        'enabled': enabled,
        'port': port,
        'ngrok_mode': ngrokMode,
        'ngrok_token': ngrokToken,
      },
    );
  }

  Future<void> setRemoteAccessAutoStart(bool autoStart) async {
    await _dio.put(
      '/api/remote-access',
      data: {
        'ngrok_auto_start': autoStart,
      },
    );
  }

  // ── Backend events ────────────────────────────────────────────────────────

  /// Polls /api/events and returns the list of recent AppEvent objects.
  Future<List<Map<String, dynamic>>> getAppEvents() async {
    try {
      final res = await _dio.get('/api/events');
      if (res.data is List) {
        return List<Map<String, dynamic>>.from(res.data as List);
      }
    } catch (_) {}
    return [];
  }

  // ── Calendar ──────────────────────────────────────────────────────────────

  Future<List<Map<String, dynamic>>> getCalendarEvents({
    DateTime? from,
    DateTime? to,
  }) async {
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
    return res.data as Map<String, dynamic>;
  }

  Future<void> deleteCalendarEvent(String id) async {
    await _dio.delete('/api/calendar/events/$id');
  }

  Future<Map<String, dynamic>> getCalendarSettings() async {
    final res = await _dio.get('/api/calendar/settings');
    return res.data as Map<String, dynamic>;
  }

  Future<void> updateCalendarSettings({required int reminderLeadMinutes}) async {
    await _dio.put('/api/calendar/settings', data: {
      'reminder_lead_minutes': reminderLeadMinutes,
    });
  }

  // ── Learning settings ─────────────────────────────────────────────────────

  Future<Map<String, dynamic>> getLearningSettings() async {
    final res = await _dio.get('/api/learning/settings');
    return res.data as Map<String, dynamic>;
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
  });

  factory RemoteAccessStatus.fromJson(Map<String, dynamic> json) {
    return RemoteAccessStatus(
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
    );
  }

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
  final bool connected;

  const ProviderConfig({
    required this.type,
    required this.name,
    this.apiKey,
    this.baseUrl,
    required this.model,
    this.enabled = true,
    this.connected = false,
  });

  ProviderConfig copyWith({
    String? type,
    String? name,
    String? apiKey,
    String? baseUrl,
    String? model,
    bool? enabled,
    bool? connected,
  }) {
    return ProviderConfig(
      type: type ?? this.type,
      name: name ?? this.name,
      apiKey: apiKey ?? this.apiKey,
      baseUrl: baseUrl ?? this.baseUrl,
      model: model ?? this.model,
      enabled: enabled ?? this.enabled,
      connected: connected ?? this.connected,
    );
  }

  factory ProviderConfig.fromJson(Map<String, dynamic> json) => ProviderConfig(
        type: json['type'] as String? ?? '',
        name: json['name'] as String? ?? '',
        apiKey: json['api_key'] as String?,
        baseUrl: json['base_url'] as String?,
        model: json['model'] as String? ?? '',
        enabled: json['enabled'] as bool? ?? true,
        connected: json['connected'] as bool? ?? false,
      );

  Map<String, dynamic> toJson() => {
        'type': type,
        'name': name,
        if (apiKey != null) 'api_key': apiKey,
        if (baseUrl != null) 'base_url': baseUrl,
        'model': model,
        'enabled': enabled,
      };
}

class LocalModel {
  final String path;
  final String filename;
  final int sizeBytes;
  final bool isEmbedding;

  const LocalModel({
    required this.path,
    required this.filename,
    required this.sizeBytes,
    this.isEmbedding = false,
  });

  factory LocalModel.fromJson(Map<String, dynamic> json) => LocalModel(
        path: json['path'] as String? ?? '',
        filename: json['filename'] as String? ?? json['name'] as String? ?? '',
        sizeBytes: json['size_bytes'] as int? ?? 0,
        isEmbedding: json['is_embedding'] as bool? ?? false,
      );
}
