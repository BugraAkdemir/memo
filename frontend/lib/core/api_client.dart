import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

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

  Dio get dio => _dio;

  MemoApiClient({required this.baseUrl}) {
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
        if (!line.startsWith('data: ')) continue;
        final jsonStr = line.substring(6);

        // Decode in its own guard: only genuine JSON parse failures should be
        // skipped here. A backend error field must NOT be swallowed — that was
        // the bug where a failed Orchestra/agent run silently committed only its
        // "Şef planlıyor..." preamble and looked frozen.
        Map<String, dynamic> data;
        try {
          data = json.decode(jsonStr) as Map<String, dynamic>;
        } catch (_) {
          continue; // malformed chunk — skip
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

  Future<void> setRemoteAccessAutoStart(bool autoStart) async {
    await _dio.put(
      '/api/remote-access',
      data: {
        'ngrok_auto_start': autoStart,
      },
    );
  }

  /// Toggles experimental (beta) features.
  Future<void> setBeta(bool enabled) async {
    await _dio.put('/api/remote-access', data: {'beta': enabled});
  }

  /// Configures the embedded Tailscale tunnel (stable URL, no extra binary).
  Future<void> setTailscaleMode(
    bool enabled,
    int port, {
    String authKey = '',
    String hostname = '',
    bool funnel = false,
  }) async {
    try {
      await _dio.put(
        '/api/remote-access',
        data: {
          'enabled': enabled,
          'port': port,
          'tunnel_mode': 'tailscale',
          'tailscale_key': authKey,
          'tailscale_hostname': hostname,
          'tailscale_funnel': funnel,
        },
      );
    } on DioException catch (e) {
      // Surface the backend's actual error message instead of a generic 500.
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

  /// Send a file (image or document) with SSE streaming.
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

        // Decode in its own guard: only genuine JSON parse failures should be
        // skipped here. A backend error field must NOT be swallowed — that was
        // the bug where a failed Orchestra/agent run silently committed only its
        // "Şef planlıyor..." preamble and looked frozen.
        Map<String, dynamic> data;
        try {
          data = json.decode(jsonStr) as Map<String, dynamic>;
        } catch (_) {
          continue; // malformed chunk — skip
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

  /// Delete a provider config. [name] disambiguates when multiple providers
  /// share the same type (the backend deletes by name when given).
  Future<void> deleteProvider(String type, {String? name}) async {
    await _dio.delete('/api/providers', data: {
      'type': type,
      if (name != null && name.isNotEmpty) 'name': name,
    });
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

  /// Get auto-permission mode (Shift+Tab).
  Future<bool> getAgentAutoPermission() async {
    final res = await _dio.get('/api/agent/auto-permission');
    return res.data['enabled'] as bool? ?? false;
  }

  /// Set auto-permission mode (Shift+Tab).
  Future<void> setAgentAutoPermission(bool enabled) async {
    await _dio.put('/api/agent/auto-permission', data: {'enabled': enabled});
  }

  // ─── Skills ──────────────────────────────────────────────────

  /// List all installed skills.
  Future<List<Map<String, dynamic>>> listSkills() async {
    final res = await _dio.get('/api/skills/list');
    if (res.data is List) {
      return (res.data as List).cast<Map<String, dynamic>>();
    }
    return [];
  }

  /// Get a specific skill by name.
  Future<Map<String, dynamic>> getSkill(String name) async {
    final res = await _dio.get('/api/skills/get/$name');
    return res.data as Map<String, dynamic>;
  }

  /// Install a skill from a local path.
  Future<Map<String, dynamic>> installSkill(String path) async {
    final res = await _dio.post('/api/skills/install', data: {'path': path});
    return res.data as Map<String, dynamic>;
  }

  /// Remove an installed skill.
  Future<void> removeSkill(String name) async {
    await _dio.delete('/api/skills/remove/$name');
  }

  /// Set which skills are active.
  Future<void> setActiveSkills(List<String> names) async {
    await _dio.put('/api/skills/active', data: {'names': names});
  }

  /// Get list of currently active skill names.
  Future<List<String>> getActiveSkills() async {
    final res = await _dio.get('/api/skills/active-list');
    if (res.data is Map && res.data['names'] is List) {
      return (res.data['names'] as List).cast<String>();
    }
    return [];
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

  /// Stop/disconnect WhatsApp (keeps session).
  Future<void> stopWhatsApp() async {
    await _dio.post('/api/whatsapp/stop');
  }

  /// Logout WhatsApp (removes session — next connect will show QR again).
  Future<void> logoutWhatsApp() async {
    await _dio.post('/api/whatsapp/logout');
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

  /// Full URL for a chat's profile picture (served and cached by the backend).
  /// Used directly with Image.network; falls back to a letter avatar on 404.
  /// [full] requests the full-resolution photo instead of the list thumbnail.
  String whatsAppAvatarUrl(String jid, {bool full = false}) =>
      '$baseUrl/api/whatsapp/avatar?jid=${Uri.encodeComponent(jid)}'
      '${full ? '&full=1' : ''}';

  /// Downloads a chat's profile picture bytes (full-res by default) — used to
  /// save the photo to disk from the enlarged preview.
  Future<Uint8List> fetchWhatsAppAvatarBytes(String jid,
      {bool full = true}) async {
    final res = await _dio.get(
      '/api/whatsapp/avatar',
      queryParameters: {'jid': jid, if (full) 'full': '1'},
      options: Options(responseType: ResponseType.bytes),
    );
    return Uint8List.fromList(res.data as List<int>);
  }

  /// Get WhatsApp message statistics.
  Future<Map<String, dynamic>> getWhatsAppStats() async {
    final res = await _dio.get('/api/whatsapp/stats');
    return res.data as Map<String, dynamic>;
  }

  // ─── Web Search ────────────────────────────────────────────────

  /// Whether web-search mode is on (every message enriched with live results).
  Future<bool> getWebSearchEnabled() async {
    final res = await _dio.get('/api/websearch');
    return (res.data as Map<String, dynamic>?)?['enabled'] == true;
  }

  /// Enable/disable web-search mode.
  Future<void> setWebSearchEnabled(bool enabled) async {
    await _dio.post('/api/websearch', data: {'enabled': enabled});
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
  /// Yields parsed [StreamChunk]s — agent tool events arrive with
  /// finishReason == 'agent_event' (their content is JSON), real reply text
  /// arrives as plain content chunks. The caller must distinguish the two so
  /// agent events render as status badges instead of raw JSON in the bubble.
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

  /// Get current proactive settings.
  Future<Map<String, dynamic>> getProactiveSettings() async {
    final res = await _dio.get('/api/proactive/settings');
    return res.data as Map<String, dynamic>;
  }

  /// Update proactive settings (enabled + level).
  Future<void> setProactiveSettings(bool enabled, String level) async {
    await _dio.post('/api/proactive/settings', data: {
      'enabled': enabled,
      'level': level,
    });
  }

  /// List learned patterns.
  Future<List<Map<String, dynamic>>> getProactivePatterns() async {
    final res = await _dio.get('/api/proactive/patterns');
    return (res.data['patterns'] as List?)?.cast<Map<String, dynamic>>() ?? [];
  }

  /// Forget a specific pattern by ID.
  Future<void> forgetPattern(String id) async {
    await _dio.post('/api/proactive/patterns/forget', data: {'id': id});
  }

  /// Clear all learning data (observations + patterns).
  Future<void> clearLearningData() async {
    await _dio.post('/api/proactive/clear');
  }

  /// Get pending proactive suggestion (for mobile polling).
  Future<Map<String, dynamic>?> getPendingSuggestion() async {
    final res = await _dio.get('/api/proactive/pending');
    return res.data['pending'] as Map<String, dynamic>?;
  }

  /// Respond to a pending suggestion (yes / no / stop).
  Future<void> respondToSuggestion(String id, String response) async {
    await _dio.post('/api/proactive/respond', data: {
      'id': id,
      'response': response,
    });
  }

  // ─── Learning model routing ─────────────────────────────────────

  /// Get learning settings (single model mode + model id).
  Future<Map<String, dynamic>> getLearningSettings() async {
    final res = await _dio.get('/api/learning/settings');
    return Map<String, dynamic>.from(res.data as Map);
  }

  /// Update learning settings (single model mode + model id).
  Future<void> updateLearningSettings(bool singleModelEnabled, String modelId) async {
    await _dio.put('/api/learning/settings', data: {
      'single_model_enabled': singleModelEnabled,
      'model_id': modelId,
    });
  }

  // ─── Calendar ───────────────────────────────────────────────────

  /// List calendar events in [from, to].
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

  /// Add a manual calendar event.
  Future<Map<String, dynamic>> addCalendarEvent(String title, DateTime startTime, String description) async {
    final res = await _dio.post('/api/calendar/events', data: {
      'title': title,
      'start_time': startTime.toUtc().toIso8601String(),
      'description': description,
    });
    return Map<String, dynamic>.from(res.data as Map);
  }

  /// Delete a calendar event.
  Future<void> deleteCalendarEvent(String id) async {
    await _dio.delete('/api/calendar/events/$id');
  }

  /// Get calendar settings (reminder lead minutes).
  Future<Map<String, dynamic>> getCalendarSettings() async {
    final res = await _dio.get('/api/calendar/settings');
    return Map<String, dynamic>.from(res.data as Map);
  }

  /// Update calendar settings (reminder lead minutes + time-guess toggle).
  Future<void> updateCalendarSettings(int reminderLeadMinutes,
      {bool disableTimeGuess = false}) async {
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

  /// Gracefully shuts down the backend.  The backend may restart the process
  /// (exit code 42 triggers run_memo.sh), so do not assume this call returns.
  Future<void> shutdown() async {
    try {
      await _dio.post('/api/shutdown');
    } catch (_) {
      // The server closes mid-request — ignore transport errors.
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
