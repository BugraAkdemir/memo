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

  MemoApiClient({String this._baseUrl = ''}) {
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

  Future<void> startEmbeddingModel({required String path, int gpuLayers = -1}) async {
    await _dio.post(
      '/api/models/embedding/start',
      data: {'path': path, 'gpu_layers': gpuLayers},
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

  Future<void> updateCalendarSettings({
    required int reminderLeadMinutes,
    bool disableTimeGuess = false,
  }) async {
    await _dio.put('/api/calendar/settings', data: {
      'reminder_lead_minutes': reminderLeadMinutes,
      'disable_time_guess': disableTimeGuess,
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

  // ── Chat extras ───────────────────────────────────────────────────────────

  Future<void> renameChat(String id, String title) async {
    await _dio.put('/api/chats/rename', data: {'id': id, 'title': title});
  }

  Future<String> getActiveChatId() async {
    final res = await _dio.get('/api/chats/active');
    return res.data['id'] as String? ?? '';
  }

  Future<String> generateTitle() async {
    final res = await _dio.post('/api/chats/generate-title');
    return res.data['title'] as String? ?? '';
  }

  Future<String> exportChat() async {
    final res = await _dio.get('/api/chats/export');
    return res.data['markdown'] as String? ?? '';
  }

  Future<void> deleteMessage(int index) async {
    await _dio.delete('/api/messages/$index');
  }

  Future<void> updateMessage(int index, String content) async {
    await _dio.put('/api/messages/$index', data: {'content': content});
  }

  Future<String> createAgentChat(String projectPath) async {
    final res = await _dio.post('/api/chats/agent', data: {'project_path': projectPath});
    return res.data['id'] as String? ?? '';
  }

  // ── Memory ────────────────────────────────────────────────────────────────

  Future<bool> getMemoryEnabled() async {
    final res = await _dio.get('/api/memory/enabled');
    return res.data['enabled'] as bool? ?? false;
  }

  Future<void> setMemoryEnabled(bool enabled) async {
    await _dio.put('/api/memory/enabled', data: {'enabled': enabled});
  }

  Future<Map<String, dynamic>> getMemorySettings() async {
    final res = await _dio.get('/api/memory/settings');
    return res.data as Map<String, dynamic>;
  }

  Future<void> updateMemorySettings({
    required int topK,
    required double minSimilarity,
    required bool autoForget,
    required int autoForgetDays,
  }) async {
    await _dio.put('/api/memory/settings', data: {
      'top_k': topK,
      'min_similarity': minSimilarity,
      'auto_forget': autoForget,
      'auto_forget_days': autoForgetDays,
    });
  }

  Future<void> saveExplicitMemory(String content, {String tags = ''}) async {
    await _dio.post('/api/memory/explicit', data: {
      'content': content,
      if (tags.isNotEmpty) 'tags': tags,
    });
  }

  Future<int> deleteExplicitMemory(String pattern) async {
    final res = await _dio.delete('/api/memory/explicit', data: {'pattern': pattern});
    return res.data['deleted'] as int? ?? 0;
  }

  Future<String> exportMemories() async {
    final res = await _dio.get('/api/memory/export');
    if (res.data is Map) return (res.data as Map<String, dynamic>).toString();
    return res.data.toString();
  }

  Future<int> importMemories(String jsonData) async {
    final res = await _dio.post('/api/memory/import', data: {'data': jsonData});
    return res.data['imported'] as int? ?? 0;
  }

  Future<MemoryStats> getMemoryStats() async {
    final res = await _dio.get('/api/memory/stats');
    return MemoryStats.fromJson(res.data as Map<String, dynamic>);
  }

  Future<List<MemorySearchResult>> debugMemorySearch(String query) async {
    final res = await _dio.get('/api/memory/search', queryParameters: {'q': query});
    if (res.data is List) {
      return (res.data as List)
          .map((e) => MemorySearchResult.fromJson(e as Map<String, dynamic>))
          .toList();
    }
    return [];
  }

  // ── System prompt / incognito ─────────────────────────────────────────────

  Future<String> getSystemPrompt() async {
    final res = await _dio.get('/api/system-prompt');
    return res.data['prompt'] as String? ?? '';
  }

  Future<void> setSystemPrompt(String prompt) async {
    await _dio.put('/api/system-prompt', data: {'prompt': prompt});
  }

  Future<void> resetSystemPrompt() async {
    await _dio.delete('/api/system-prompt');
  }

  Future<void> toggleIncognito(bool enabled) async {
    await _dio.put('/api/incognito', data: {'enabled': enabled});
  }

  Future<String> getIncognitoPrompt() async {
    final res = await _dio.get('/api/incognito/prompt');
    return res.data['prompt'] as String? ?? '';
  }

  Future<void> setIncognitoPrompt(String prompt) async {
    await _dio.put('/api/incognito/prompt', data: {'prompt': prompt});
  }

  // ── Mood ──────────────────────────────────────────────────────────────────

  Future<double> getMoodScore() async {
    final res = await _dio.get('/api/mood/score');
    return (res.data['score'] as num?)?.toDouble() ?? 0.0;
  }

  Future<bool> getMoodEnabled() async {
    final res = await _dio.get('/api/mood/enabled');
    return res.data['enabled'] as bool? ?? false;
  }

  Future<void> setMoodEnabled(bool enabled) async {
    await _dio.put('/api/mood/enabled', data: {'enabled': enabled});
  }

  // ── Version / shutdown ────────────────────────────────────────────────────

  Future<String> getVersion() async {
    final res = await _dio.get('/api/version');
    return res.data['version'] as String? ?? '';
  }

  Future<void> shutdown() async {
    try {
      await _dio.post('/api/shutdown');
    } catch (_) {}
  }

  // ── Agent extras ──────────────────────────────────────────────────────────

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

  Future<List<Map<String, dynamic>>> getAgentPermissions() async {
    final res = await _dio.get('/api/agent/permissions');
    if (res.data is List) {
      return List<Map<String, dynamic>>.from(res.data as List);
    }
    return [];
  }

  Future<void> revokeAgentPermission(String id) async {
    await _dio.delete('/api/agent/permissions/$id');
  }

  Future<void> clearAgentPermissions() async {
    await _dio.delete('/api/agent/permissions');
  }

  // ── WhatsApp ──────────────────────────────────────────────────────────────

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
    if (res.data is List) return res.data as List;
    return [];
  }

  Future<List<dynamic>> getWhatsAppChats() async {
    final res = await _dio.get('/api/whatsapp/chats');
    if (res.data is List) return res.data as List;
    return [];
  }

  Future<List<dynamic>> getWhatsAppMessages(String jid) async {
    final res = await _dio.get('/api/whatsapp/messages', queryParameters: {'jid': jid});
    if (res.data is List) return res.data as List;
    return [];
  }

  String whatsAppAvatarUrl(String jid) =>
      '$_baseUrl/api/whatsapp/avatar?jid=${Uri.encodeComponent(jid)}';

  Future<Map<String, dynamic>> getWhatsAppStats() async {
    final res = await _dio.get('/api/whatsapp/stats');
    return res.data as Map<String, dynamic>;
  }

  Future<bool> getWhatsAppChatMode() async {
    final res = await _dio.get('/api/whatsapp/chat-mode');
    return res.data['enabled'] as bool? ?? false;
  }

  Future<void> setWhatsAppChatMode(bool enabled) async {
    await _dio.put('/api/whatsapp/chat-mode', data: {'enabled': enabled});
  }

  Stream<StreamChunk> sendWhatsAppChatStream(
    String message, {
    CancelToken? cancelToken,
  }) async* {
    try {
      final response = await _dio.post(
        '/api/whatsapp/chat/stream',
        data: {'message': message},
        options: Options(responseType: ResponseType.stream),
        cancelToken: cancelToken,
      );

      final lineStream = (response.data.stream as Stream<List<int>>)
          .transform(utf8.decoder)
          .transform(const LineSplitter());

      await for (final line in lineStream) {
        if (line.startsWith('data: ')) {
          final jsonStr = line.substring(6);
          try {
            final data = json.decode(jsonStr) as Map<String, dynamic>;
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
    }
  }

  // ── Proactive / learning ──────────────────────────────────────────────────

  Future<Map<String, dynamic>> getProactiveSettings() async {
    final res = await _dio.get('/api/proactive/settings');
    return res.data as Map<String, dynamic>;
  }

  Future<void> setProactiveSettings(bool enabled, String level) async {
    await _dio.put('/api/proactive/settings', data: {'enabled': enabled, 'level': level});
  }

  Future<List<Map<String, dynamic>>> getProactivePatterns() async {
    final res = await _dio.get('/api/proactive/patterns');
    if (res.data is List) {
      return List<Map<String, dynamic>>.from(res.data as List);
    }
    return [];
  }

  Future<void> forgetPattern(String id) async {
    await _dio.delete('/api/proactive/patterns/$id');
  }

  Future<void> clearLearningData() async {
    await _dio.delete('/api/learning/data');
  }

  Future<Map<String, dynamic>?> getPendingSuggestion() async {
    try {
      final res = await _dio.get('/api/proactive/suggestion');
      if (res.data is Map && res.data['id'] != null) {
        return res.data as Map<String, dynamic>;
      }
    } catch (_) {}
    return null;
  }

  Future<void> respondToSuggestion(String id, String response) async {
    await _dio.post('/api/proactive/suggestion/respond', data: {'id': id, 'response': response});
  }

  // ── Self-interest / system management ────────────────────────────────────

  Future<bool> getSelfInterestEnabled() async {
    final res = await _dio.get('/api/self-interest/enabled');
    return res.data['enabled'] as bool? ?? false;
  }

  Future<void> setSelfInterestEnabled(bool enabled) async {
    await _dio.put('/api/self-interest/enabled', data: {'enabled': enabled});
  }

  Future<bool> getSystemManagementEnabled() async {
    final res = await _dio.get('/api/system-management/enabled');
    return res.data['enabled'] as bool? ?? false;
  }

  Future<void> setSystemManagementEnabled(bool enabled) async {
    await _dio.put('/api/system-management/enabled', data: {'enabled': enabled});
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
      beta: json['beta'] as bool? ?? false,
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

class MemoryStats {
  final int total;
  final int explicit;
  final int automatic;
  final int pendingDeletion;
  final double avgImportance;

  const MemoryStats({
    this.total = 0,
    this.explicit = 0,
    this.automatic = 0,
    this.pendingDeletion = 0,
    this.avgImportance = 0,
  });

  factory MemoryStats.fromJson(Map<String, dynamic> json) => MemoryStats(
        total: json['total'] as int? ?? 0,
        explicit: json['explicit'] as int? ?? 0,
        automatic: json['automatic'] as int? ?? 0,
        pendingDeletion: json['pending_deletion'] as int? ?? 0,
        avgImportance: (json['avg_importance'] as num?)?.toDouble() ?? 0,
      );
}

class MemorySearchResult {
  final String id;
  final String content;
  final double score;
  final String matchType;
  final int importance;
  final String source;
  final String createdAt;

  const MemorySearchResult({
    required this.id,
    required this.content,
    this.score = 0,
    this.matchType = '',
    this.importance = 3,
    this.source = '',
    this.createdAt = '',
  });

  factory MemorySearchResult.fromJson(Map<String, dynamic> json) =>
      MemorySearchResult(
        id: json['id'] as String? ?? '',
        content: json['content'] as String? ?? '',
        score: (json['score'] as num?)?.toDouble() ?? 0,
        matchType: json['match_type'] as String? ?? '',
        importance: json['importance'] as int? ?? 3,
        source: json['source'] as String? ?? '',
        createdAt: json['created_at'] as String? ?? '',
      );
}
