import 'dart:async';
import 'dart:convert';

import 'package:dio/dio.dart';

import '../models/chat.dart';
import '../models/gpu_info.dart';
import '../models/local_model.dart';
import '../models/orchestra_config.dart';
import '../models/provider_config.dart';

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

  /// Send a message with streaming SSE. Yields [StreamChunk] with content and/or thinking.
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
            if (data['done'] == true) {
              break;
            }
          } catch (e) {
            // ignore malformed chunks
          }
        }
      }
    } on DioException catch (e) {
      if (e.type == DioExceptionType.cancel) return;
      throw Exception('Bağlantı hatası: $e');
    } catch (e) {
      throw Exception('Bağlantı hatası: $e');
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

  /// Create an agent chat with a project directory, returns the new chat ID.
  Future<String> createAgentChat(String projectPath) async {
    final res = await _dio.post(
      '/api/agent/chat',
      data: {'project_path': projectPath},
    );
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

  /// Rename a chat by ID.
  Future<void> renameChat(String id, String title) async {
    await _dio.post('/api/chats/rename', data: {'id': id, 'title': title});
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

  /// Update a message's content by index.
  Future<void> updateMessage(int index, String content) async {
    await _dio.post(
      '/api/messages/update',
      data: {'index': index, 'content': content},
    );
  }

  /// Delete a message by index.
  Future<void> deleteMessage(int index) async {
    await _dio.post('/api/messages/delete', data: {'index': index});
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

  // ─── OpenRouter ───────────────────────────────────────────────

  /// Save OpenRouter API key and activate provider.
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

  /// Fetch available models from OpenRouter for the given API key.
  Future<Map<String, dynamic>> fetchOpenRouterModels(String apiKey) async {
    final res = await _dio.post(
      '/api/openrouter/models',
      data: {'api_key': apiKey},
    );
    return res.data as Map<String, dynamic>;
  }

  // ─── Status ─────────────────────────────────────────────────────

  /// Check backend connection status.
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
    await _dio.put(
      '/api/memory/settings',
      data: {'top_k': topK, 'min_similarity': minSimilarity},
    );
  }

  Future<bool> getMemoryEnabled() async {
    final res = await _dio.get('/api/memory/enabled');
    return res.data['enabled'] as bool? ?? true;
  }

  Future<void> setMemoryEnabled(bool enabled) async {
    await _dio.put('/api/memory/enabled', data: {'enabled': enabled});
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
        return e.response?.data['error'].toString() ??
            e.message ??
            'Unknown error';
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

  // ─── Backup / Restore (.memo) ────────────────────────────────────

  /// Export all user data as .memo bytes.
  Future<List<int>> exportData({bool includeModels = false}) async {
    final res = await _dio.get('/api/export',
        queryParameters: {'include_models': includeModels.toString()},
        options: Options(responseType: ResponseType.bytes));
    return res.data as List<int>;
  }

  /// Import from .memo bytes.
  Future<void> importData(List<int> data) async {
    await _dio.post('/api/import',
        data: Stream.fromIterable([data]),
        options: Options(
          contentType: 'application/octet-stream',
          headers: {'Content-Length': data.length.toString()},
        ));
  }

  /// Wipe all user data.
  Future<void> wipeData() async {
    await _dio.post('/api/wipe');
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
    } catch (e) {
      return false;
    }
  }

  // ─── Provider Management ─────────────────────────────────────────

  /// Get all provider configs.
  Future<List<ProviderConfig>> getProviders() async {
    final res = await _dio.get('/api/providers');
    if (res.data is List) {
      return (res.data as List)
          .map((e) => ProviderConfig.fromJson(e as Map<String, dynamic>))
          .toList();
    }
    return [];
  }

  /// Update a provider config.
  Future<void> updateProvider(ProviderConfig config) async {
    await _dio.put('/api/providers', data: config.toJson());
  }

  /// Delete a provider config.
  Future<void> deleteProvider(String type) async {
    await _dio.delete('/api/providers', data: {'type': type});
  }

  /// Test a provider connection.
  Future<Map<String, dynamic>> testProvider(ProviderConfig config) async {
    final res = await _dio.post('/api/providers/test', data: config.toJson());
    return res.data as Map<String, dynamic>;
  }

  /// Get active provider.
  Future<String> getActiveProvider() async {
    final res = await _dio.get('/api/providers/active');
    return res.data['provider'] as String? ?? '';
  }

  /// Set active provider.
  Future<void> setActiveProvider(String type) async {
    await _dio.put('/api/providers/active', data: {'provider': type});
  }

  // ─── Orchestra Mode ───────────────────────────────────────────────

  /// Get orchestra config.
  Future<OrchestraConfig> getOrchestraConfig() async {
    final res = await _dio.get('/api/orchestra/config');
    return OrchestraConfig.fromJson(res.data as Map<String, dynamic>);
  }

  /// Update orchestra config.
  Future<void> updateOrchestraConfig(OrchestraConfig config) async {
    await _dio.put('/api/orchestra/config', data: config.toJson());
  }

  // ─── Agent Mode ───────────────────────────────────────────────────

  /// Get agent enabled status.
  Future<bool> getAgentEnabled() async {
    final res = await _dio.get('/api/agent/enabled');
    return res.data['enabled'] as bool? ?? false;
  }

  /// Set agent enabled status.
  Future<void> setAgentEnabled(bool enabled) async {
    await _dio.put('/api/agent/enabled', data: {'enabled': enabled});
  }

  /// Send permission response.
  Future<void> handleAgentPermission(String requestId, String policy) async {
    await _dio.post(
      '/api/agent/permission',
      data: {'request_id': requestId, 'policy': policy},
    );
  }

  /// Get persistent agent permissions.
  Future<List<Map<String, dynamic>>> getAgentPermissions() async {
    final res = await _dio.get('/api/agent/permissions');
    if (res.data is List) {
      return (res.data as List).cast<Map<String, dynamic>>();
    }
    return [];
  }

  /// Revoke a specific agent permission by ID.
  Future<void> revokeAgentPermission(String id) async {
    await _dio.delete('/api/agent/permissions', queryParameters: {'id': id});
  }

  /// Clear all agent permissions.
  Future<void> clearAgentPermissions() async {
    await _dio.delete('/api/agent/permissions');
  }

  /// Undo the last agent file edit.
  Future<void> undoAgentEdit() async {
    await _dio.post('/api/agent/undo');
  }

  // ─── Version Check ───────────────────────────────────────────

  /// Get current app version from backend.
  Future<String> getVersion() async {
    final res = await _dio.get('/api/version');
    return res.data['version'] as String? ?? 'unknown';
  }

  /// Check if a newer version is available.
  /// Returns a map with current, latest, and error fields.
  Future<Map<String, dynamic>> checkVersion() async {
    try {
      final res = await _dio.get('/api/version/check');
      return res.data as Map<String, dynamic>;
    } catch (e) {
      return {'current': 'unknown', 'latest': null, 'error': e.toString()};
    }
  }

  // ─── WhatsApp ──────────────────────────────────────────────────

  /// Get WhatsApp connection status.
  Future<Map<String, dynamic>> getWhatsAppStatus() async {
    final res = await _dio.get('/api/whatsapp/status');
    return res.data as Map<String, dynamic>;
  }

  /// Start WhatsApp connection (triggers QR pairing if not logged in).
  Future<Map<String, dynamic>> startWhatsApp() async {
    final res = await _dio.post('/api/whatsapp/start');
    return res.data as Map<String, dynamic>;
  }

  /// Stop/disconnect WhatsApp.
  Future<void> stopWhatsApp() async {
    await _dio.post('/api/whatsapp/stop');
  }

  /// Send a WhatsApp message.
  Future<Map<String, dynamic>> sendWhatsApp(String jid, String text) async {
    final res = await _dio.post('/api/whatsapp/send', data: {'jid': jid, 'text': text});
    return res.data as Map<String, dynamic>;
  }

  /// Search WhatsApp messages.
  Future<List<dynamic>> searchWhatsApp(String query) async {
    final res = await _dio.get('/api/whatsapp/search', queryParameters: {'q': query});
    return (res.data as List<dynamic>?) ?? [];
  }

  /// Get WhatsApp chat list.
  Future<List<dynamic>> getWhatsAppChats() async {
    final res = await _dio.get('/api/whatsapp/chats');
    return (res.data as List<dynamic>?) ?? [];
  }

  /// Get messages for a specific WhatsApp chat.
  Future<List<dynamic>> getWhatsAppMessages(String jid) async {
    final res = await _dio.get('/api/whatsapp/messages', queryParameters: {'jid': jid});
    return (res.data as List<dynamic>?) ?? [];
  }

  /// Get WhatsApp message statistics.
  Future<Map<String, dynamic>> getWhatsAppStats() async {
    final res = await _dio.get('/api/whatsapp/stats');
    return res.data as Map<String, dynamic>;
  }

  /// Get WhatsApp chat mode state.
  Future<bool> getWhatsAppChatMode() async {
    final res = await _dio.get('/api/whatsapp/chat-mode');
    return (res.data as Map<String, dynamic>?)?['enabled'] == true;
  }

  /// Set WhatsApp chat mode.
  Future<void> setWhatsAppChatMode(bool enabled) async {
    await _dio.post('/api/whatsapp/chat-mode', data: {'enabled': enabled});
  }

  /// Send a message in WhatsApp chat mode (streaming SSE).
  Stream<String> sendWhatsAppChatStream(String message) async* {
    final response = await _dio.post(
      '/api/whatsapp/chat-stream',
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
        final content = line.substring(6);
        if (content == '[DONE]') return;
        yield content;
      }
    }
  }
}
