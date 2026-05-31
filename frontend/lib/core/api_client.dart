import 'dart:async';
import 'dart:convert';

import 'package:dio/dio.dart';

import '../models/chat.dart';
import '../models/gpu_info.dart';
import '../models/local_model.dart';

/// Memo Go backend REST API client.
/// Connects to headless Go server on localhost (plain HTTP, no TLS).
class MemoApiClient {
  late final Dio _dio;
  final String baseUrl;

  MemoApiClient({this.baseUrl = 'http://127.0.0.1:8090'}) {
    _dio = Dio(
      BaseOptions(
        baseUrl: baseUrl,
        connectTimeout: const Duration(seconds: 5),
        receiveTimeout: const Duration(seconds: 120),
        headers: {'Content-Type': 'application/json'},
      ),
    );
  }

  // ─── Chat ───────────────────────────────────────────────────────

  /// Send a message and get the full reply (non-streaming).
  Future<String> sendMessage(String message) async {
    final res = await _dio.post('/api/send', data: {'message': message});
    return res.data['reply'] as String? ?? '';
  }

  /// Send a message with streaming SSE.
  Stream<String> sendMessageStream(String message) async* {
    try {
      final response = await _dio.post(
        '/api/send/stream',
        data: {'message': message},
        options: Options(responseType: ResponseType.stream),
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
              yield '⚠️ Hata: ${data['error']}';
              break;
            }
            if (data['content'] != null) {
              yield data['content'] as String;
            }
            if (data['done'] == true) {
              break;
            }
          } catch (_) {
            // ignore malformed chunks
          }
        }
      }
    } catch (e) {
      yield '⚠️ Bağlantı hatası: $e';
    }
  }

  /// Get all chats list.
  Future<List<ChatSession>> listChats() async {
    final res = await _dio.get('/api/chats');
    if (res.data is List) {
      return (res.data as List)
          .map((e) => ChatSession.fromJson(e as Map<String, dynamic>))
          .toList();
    }
    // Backend wraps it in an object sometimes
    if (res.data is Map && res.data['chats'] != null) {
      return (res.data['chats'] as List)
          .map((e) => ChatSession.fromJson(e as Map<String, dynamic>))
          .toList();
    }
    return [];
  }

  /// Create new chat, returns the new chat ID.
  Future<String> newChat() async {
    final res = await _dio.post('/api/chats/new');
    return res.data['id'] as String? ?? '';
  }

  /// Switch to a chat by ID.
  Future<void> switchChat(String id) async {
    await _dio.post('/api/chats/switch', data: {'id': id});
  }

  /// Delete a chat by ID.
  Future<void> deleteChat(String id) async {
    await _dio.post('/api/chats/delete', data: {'id': id});
  }

  /// Get active chat ID.
  Future<String> getActiveChatId() async {
    final res = await _dio.get('/api/chats/active');
    return res.data['id'] as String? ?? '';
  }

  /// Get messages of the active chat.
  Future<List<ChatMessage>> getMessages() async {
    final res = await _dio.get('/api/messages');
    if (res.data is List) {
      return (res.data as List)
          .map((e) => ChatMessage.fromJson(e as Map<String, dynamic>))
          .toList();
    }
    return [];
  }

  /// Export current chat as markdown.
  Future<String> exportChat() async {
    final res = await _dio.post('/api/chat/export');
    return res.data['markdown'] as String? ?? '';
  }

  /// Generate AI title for current chat.
  Future<String> generateTitle() async {
    final res = await _dio.post('/api/chat/title');
    return res.data['title'] as String? ?? '';
  }

  // ─── Status ─────────────────────────────────────────────────────

  /// Check backend connection status.
  Future<Map<String, dynamic>> getStatus() async {
    final res = await _dio.get('/api/status');
    return res.data as Map<String, dynamic>;
  }

  /// Get app version.
  Future<String> getVersion() async {
    final res = await _dio.get('/api/version');
    return res.data['version'] as String? ?? 'unknown';
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
    await _dio.put(
      '/api/memory/settings',
      data: {'top_k': topK, 'min_similarity': minSimilarity},
    );
  }

  // ─── Models (Local) ─────────────────────────────────────────────

  Future<List<LocalModel>> listLocalModels() async {
    final res = await _dio.get('/api/models/local');
    if (res.data is List) {
      return (res.data as List)
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

  String _extractErrorMessage(DioException e) {
    if (e.response?.data != null) {
      // If server returned a plain string error or a JSON with error field
      if (e.response?.data is String) return e.response!.data as String;
      if (e.response?.data is Map && e.response?.data['error'] != null) {
        return e.response?.data['error'].toString() ?? e.message ?? 'Unknown error';
      }
    }
    return e.message ?? 'Unknown error';
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
  }) async {
    final data = <String, dynamic>{};
    if (engineMode != null) data['engine_mode'] = engineMode;
    if (binaryPath != null) data['binary_path'] = binaryPath;
    if (port != null) data['port'] = port;
    if (ctxSize != null) data['ctx_size'] = ctxSize;

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

  Future<void> downloadModel(String repoId, String filename) async {
    await _dio.post(
      '/api/models/download',
      data: {'repo_id': repoId, 'filename': filename},
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
    // Installation can take 5-10+ minutes when compiling from source.
    // Override the default 120s timeout with 15 minutes for this call only.
    await _dio.post(
      '/api/models/llama/install',
      options: Options(receiveTimeout: const Duration(minutes: 15)),
    );
  }

  Future<void> skipLlamaGPUInstall() async {
    await _dio.post('/api/models/llama/skip');
  }

  // ─── Remote Access ──────────────────────────────────────────────

  Future<Map<String, dynamic>> getRemoteAccess() async {
    final res = await _dio.get('/api/remote-access');
    return res.data as Map<String, dynamic>;
  }

  Future<void> setRemoteAccess(bool enabled, int port) async {
    await _dio.put(
      '/api/remote-access',
      data: {'enabled': enabled, 'port': port},
    );
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
    await _dio.put(
      '/api/sync/settings',
      data: {
        'enabled': enabled,
        'client_id': clientId,
        'client_secret': clientSecret,
        'passphrase': passphrase,
        'token_path': tokenPath,
        'interval_messages': intervalMessages,
      },
    );
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

  // ─── Recording ──────────────────────────────────────────────────

  Future<void> startRecording() async {
    await _dio.post('/api/recording/start');
  }

  Future<String> stopRecording() async {
    final res = await _dio.post('/api/recording/stop');
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

  // ─── Health check ───────────────────────────────────────────────

  /// Returns true if the Go backend is reachable.
  Future<bool> isAlive() async {
    try {
      await _dio.get('/api/version');
      return true;
    } catch (_) {
      return false;
    }
  }
}
