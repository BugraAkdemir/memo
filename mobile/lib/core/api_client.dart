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

  MemoApiClient({String baseUrl = 'http://192.168.1.100:8090'})
      : _baseUrl = baseUrl {
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
