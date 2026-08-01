import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';

import 'l10n.dart';
import '../models/chat.dart';
import '../models/gpu_info.dart';
import '../models/local_model.dart';
import '../models/minimal_mode_overrides.dart';
import '../models/orchestra_config.dart';
import '../models/provider_config.dart';
import '../models/dev_gateway.dart';
import '../models/swarm.dart';
import '../models/task_list.dart';
import '../models/tts_provider_config.dart';
import '../models/tts_voice.dart';
import '../models/usage_stats.dart';

/// Memo Go backend REST API client.
/// Connects to headless Go server on localhost (plain HTTP, no TLS).
class MemoApiClient {
  late final Dio _dio;
  final String baseUrl;

  /// Called whenever _applyRemoteToken learns a (possibly new) token, so
  /// the caller can persist it for the next app launch — see the
  /// savedRemoteToken constructor parameter for why this closes BUG-L5.
  final void Function(String token)? onRemoteTokenLearned;

  Dio get dio => _dio;

  /// savedRemoteToken is whatever token this client learned and persisted
  /// on a previous run (BUG-L5 fix): if ngrok auto-start is on,
  /// Startup() rebinds the backend straight to 0.0.0.0 (token-gated)
  /// before this client ever gets a chance to call
  /// getRemoteAccess()/setRemoteAccess() and learn the token fresh — so
  /// its very first request after a restart would otherwise always 401.
  /// The token is stable across restarts (internal/app/remote.go only
  /// generates one if RemoteAccess.Token is still empty), so applying the
  /// last-known value immediately, before any request is made, closes
  /// that window in the common case (remote access left configured the
  /// same way between launches). A stale/rotated token still just 401s,
  /// exactly like before this fix — no worse, and self-corrects the next
  /// time getRemoteAccess()/setRemoteAccess() is called.
  MemoApiClient({
    required this.baseUrl,
    String? savedRemoteToken,
    this.onRemoteTokenLearned,
  }) {
    _dio = Dio(
      BaseOptions(
        baseUrl: baseUrl,
        connectTimeout: const Duration(seconds: 5),
        receiveTimeout: const Duration(seconds: 120),
        headers: {'Content-Type': 'application/json'},
      ),
    );
    if (savedRemoteToken != null && savedRemoteToken.isNotEmpty) {
      _dio.options.headers['X-Memo-Token'] = savedRemoteToken;
    }
  }

  /// The backend now requires X-Memo-Token on every request once remote
  /// access is enabled (LAN mode or ngrok both rebind the server to
  /// 0.0.0.0) — this desktop client talks to the same single port, so it
  /// needs the token too, exactly like the mobile client already does.
  /// Captured from getRemoteAccess()/setRemoteAccess() responses, which
  /// include it (see _applyRemoteToken call sites below).
  void _applyRemoteToken(dynamic data) {
    if (data is Map && data['token'] is String && (data['token'] as String).isNotEmpty) {
      final token = data['token'] as String;
      _dio.options.headers['X-Memo-Token'] = token;
      onRemoteTokenLearned?.call(token);
    }
  }

  /// Runtime type guard — throws [Exception] instead of [TypeError] when
  /// the backend returns an unexpected response type.
  static T _guard<T>(dynamic data) {
    if (data is T) return data;
    throw Exception('Expected $T, got ${data.runtimeType}');
  }

  /// Runtime type guard for lists of typed elements.
  ///
  /// `data is List<E>` is unreliable for generic element types (Dart's
  /// generic reification means a `List<dynamic>` from JSON decoding
  /// structurally "is" almost anything), so a naive `.cast<E>()` on an
  /// unchecked list is *lazy*: a malformed element only throws a
  /// [TypeError] later, at whatever unrelated call site happens to
  /// iterate the list (a `.map()`/`.forEach()` far from here). This
  /// walks every element eagerly and throws a descriptive [Exception]
  /// right here, at the guard call site, if the shape doesn't match.
  static List<E> _guardList<E>(dynamic data) {
    if (data is! List) {
      throw Exception('Expected List, got ${data.runtimeType}');
    }
    for (var i = 0; i < data.length; i++) {
      if (data[i] is! E) {
        throw Exception(
          'Expected List<$E>, but element at index $i is '
          '${data[i].runtimeType}',
        );
      }
    }
    return data.cast<E>();
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

  /// Get recent background events (chat:done, memory:saved, memory:error,
  /// etc.) from the backend's ring buffer — used by the bug-report tab to
  /// optionally attach the last few errors to a report.
  Future<List<Map<String, dynamic>>> getEvents() async {
    final res = await _dio.get('/api/events');
    if (res.data is List) {
      return (_guard<List>(res.data)).cast<Map<String, dynamic>>();
    }
    return [];
  }

  /// Get all chats list.
  Future<List<ChatSession>> listChats() async {
    final res = await _dio.get('/api/chats');
    if (res.data is List) {
      return (_guard<List>(res.data))
          .map((e) => ChatSession.fromJson(e as Map<String, dynamic>))
          .toList();
    }
    // Backend wraps it in an object sometimes
    if (res.data is Map && res.data['chats'] != null) {
      return (_guard<List>(res.data['chats']))
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
      return (_guard<List>(res.data))
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
    return _guard<Map<String, dynamic>>(res.data);
  }

  /// Fetch available models from OpenRouter for the given API key.
  Future<Map<String, dynamic>> fetchOpenRouterModels(String apiKey) async {
    final res = await _dio.post(
      '/api/openrouter/models',
      data: {'api_key': apiKey},
    );
    return _guard<Map<String, dynamic>>(res.data);
  }

  // ─── Status ─────────────────────────────────────────────────────

  /// Check backend connection status.
  Future<Map<String, dynamic>> getStatus() async {
    final res = await _dio.get('/api/status');
    return _guard<Map<String, dynamic>>(res.data);
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

  /// Persists the GUI's current display language ("tr"/"en") backend-side.
  /// The backend has no display language of its own — this exists purely so
  /// a second client with no SharedPreferences of its own (the terminal
  /// REPL, internal/replcli) can follow whatever L10n.locale the GUI is
  /// currently set to, instead of always defaulting to Turkish.
  Future<void> setUILanguage(String lang) async {
    await _dio.put('/api/system-prompt/ui-language', data: {'language': lang});
  }

  /// Whether identity/persona/mood/web-search prompt injection is disabled
  /// — only memory context (if separately enabled) still reaches the model.
  Future<bool> getMinimalMode() async {
    final res = await _dio.get('/api/system-prompt/minimal-mode');
    return res.data['enabled'] as bool? ?? false;
  }

  Future<void> setMinimalMode(bool enabled) async {
    await _dio.put('/api/system-prompt/minimal-mode', data: {'enabled': enabled});
  }

  /// Granular Minimal Mode overrides — each only has any effect while
  /// Minimal Mode itself is on. Lets a user keep one category (e.g. the
  /// persona/system prompt, or ambient proactive nudging) active without
  /// having to turn Minimal Mode off entirely and get everything back.
  Future<MinimalModeOverrides> getMinimalModeOverrides() async {
    final res = await _dio.get('/api/system-prompt/minimal-mode/overrides');
    return MinimalModeOverrides.fromJson(res.data as Map<String, dynamic>);
  }

  Future<void> setMinimalModeOverrides(MinimalModeOverrides overrides) async {
    await _dio.put('/api/system-prompt/minimal-mode/overrides', data: overrides.toJson());
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
      return (_guard<List>(res.data))
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
    return _guard<Map<String, dynamic>>(res.data);
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
      return (_guard<List>(res.data))
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
    return MemoryStats.fromJson(_guard<Map<String, dynamic>>(res.data));
  }

  Future<UsageStatsSummary> getUsageStats({int days = 30}) async {
    final res = await _dio.get('/api/stats/usage', queryParameters: {'days': days});
    return UsageStatsSummary.fromJson(_guard<Map<String, dynamic>>(res.data));
  }

  Future<DevGatewayConfig> getDevGatewayConfig() async {
    final res = await _dio.get('/api/dev-gateway/config');
    return DevGatewayConfig.fromJson(_guard<Map<String, dynamic>>(res.data));
  }

  Future<void> setDevGatewayConfig({required bool requireAPIKey, required bool useMemory}) async {
    await _dio.put('/api/dev-gateway/config', data: {
      'require_api_key': requireAPIKey,
      'use_memory': useMemory,
    });
  }

  Future<List<GatewayModel>> getGatewayModels() async {
    final res = await _dio.get('/api/dev-gateway/models');
    final list = res.data;
    if (list is! List) return [];
    return list
        .map((e) => GatewayModel.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  Future<List<GatewayLogEntry>> getGatewayLogs() async {
    final res = await _dio.get('/api/dev-gateway/logs');
    final list = res.data;
    if (list is! List) return [];
    return list
        .map((e) => GatewayLogEntry.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  /// Synthesizes a short "what did you notice about yourself" reflection
  /// from the user's own conversation memory + mood trend over the last
  /// [windowDays] days (0 = backend default). [lang] is the UI locale
  /// ("tr"/"en"), same convention routines already use.
  Future<String> generateSelfInsight({int windowDays = 0, String lang = 'tr'}) async {
    final res = await _dio.post('/api/memory/insight',
        data: {'window_days': windowDays, 'lang': lang});
    final data = _guard<Map<String, dynamic>>(res.data);
    return (data['insight'] as String?) ?? '';
  }

  Future<List<MemorySearchResult>> filteredMemorySearch(
      String query, {String? since, String? tag}) async {
    final params = <String, dynamic>{'q': query};
    if (since != null) params['since'] = since;
    if (tag != null && tag.isNotEmpty) params['tag'] = tag;
    final res = await _dio.get('/api/memory/search', queryParameters: params);
    if (res.data is List) {
      return (_guard<List>(res.data))
          .map((e) => MemorySearchResult.fromJson(e as Map<String, dynamic>))
          .toList();
    }
    return [];
  }

  /// Sends free-form text (typically another AI's answer describing what it
  /// knows about the user) to be structured by the active model into atomic
  /// memory facts (+ an optional learned communication-style summary) and
  /// saved. Returns `{'factsSaved': int, 'styleUpdated': bool}`.
  Future<Map<String, dynamic>> importMemoryFromText(String content) async {
    final res = await _dio.post('/api/memory/import-text', data: {'content': content});
    final data = _guard<Map<String, dynamic>>(res.data);
    return {
      'factsSaved': (data['facts_saved'] as int?) ?? 0,
      'styleUpdated': (data['style_updated'] as bool?) ?? false,
    };
  }

  // ─── Models (Local) ─────────────────────────────────────────────

  Future<List<LocalModel>> listLocalModels() async {
    final res = await _dio.get('/api/models/local');
    if (res.data is List) {
      return (_guard<List>(res.data))
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
        // The backend blocks this whole request until llama-server reports
        // ready (StartLocalModel's WaitReady budget is 180s — a large model
        // on a slow disk can genuinely take close to that on its first,
        // OS-file-cache-cold load). The default 120s receiveTimeout would
        // fire first and throw a "failed to start" DioException while the
        // backend is still successfully loading it in the background —
        // override with margin above the backend's own budget.
        options: Options(receiveTimeout: const Duration(seconds: 200)),
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
    return ServerStatus.fromJson(_guard<Map<String, dynamic>>(res.data));
  }

  Future<Map<String, dynamic>> getLlamaConfig() async {
    final res = await _dio.get('/api/models/config');
    return _guard<Map<String, dynamic>>(res.data);
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
        // StartEmbeddingModel retries up to 3 times, each with its own 120s
        // WaitReady budget (a fresh embedding server occasionally dies within
        // its first second or two, which this retry recovers from) — worst
        // case close to 6 minutes. Same reasoning as startModel above: the
        // default 120s receiveTimeout would fire well before that and report
        // a spurious failure while the backend is still retrying.
        options: Options(receiveTimeout: const Duration(minutes: 7)),
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
    return ServerStatus.fromJson(_guard<Map<String, dynamic>>(res.data));
  }

  // ─── GPU ────────────────────────────────────────────────────────

  Future<GPUInfo> getGpuInfo() async {
    final res = await _dio.get('/api/gpu');
    return GPUInfo.fromJson(_guard<Map<String, dynamic>>(res.data));
  }

  // ─── Model Store (HF) ──────────────────────────────────────────

  Future<List<HFModelResult>> searchModels(String query) async {
    final res = await _dio.post('/api/models/search', data: {'query': query});
    if (res.data is List) {
      return (_guard<List>(res.data))
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
      return (_guard<List>(res.data))
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

  Future<List<DownloadProgress>> getDownloadProgress() async {
    final res = await _dio.get('/api/models/download/progress');
    if (res.data is List) {
      return (_guard<List>(res.data))
          .map((e) => DownloadProgress.fromJson(e as Map<String, dynamic>))
          .toList();
    }
    return [];
  }

  Future<void> cancelDownload(String repoId, String filename) async {
    await _dio.post(
      '/api/models/download/cancel',
      data: {'repo_id': repoId, 'filename': filename},
    );
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
    _applyRemoteToken(res.data);
    return _guard<Map<String, dynamic>>(res.data);
  }

  Future<void> setRemoteAccess(bool enabled, int port, {bool ngrokMode = false, String ngrokToken = ''}) async {
    final res = await _dio.put(
      '/api/remote-access',
      data: {
        'enabled': enabled,
        'port': port,
        'ngrok_mode': ngrokMode,
        'ngrok_token': ngrokToken,
      },
    );
    // This request may have just switched the server from 127.0.0.1
    // (unauthenticated) to 0.0.0.0 (token-gated) — it's sent while the
    // listener is still the old, unauthenticated one, so it's the one
    // chance to learn the token before every later request needs it.
    _applyRemoteToken(res.data);
  }

  Future<void> setRemoteAccessAutoStart(bool autoStart) async {
    final res = await _dio.put(
      '/api/remote-access',
      data: {
        'ngrok_auto_start': autoStart,
      },
    );
    _applyRemoteToken(res.data);
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
    return _guard<Map<String, dynamic>>(res.data);
  }

  Future<Map<String, dynamic>> getSyncSettings() async {
    final res = await _dio.get('/api/sync/settings');
    return _guard<Map<String, dynamic>>(res.data);
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
    return _guard<List<int>>(res.data);
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

  /// Remove the terminal `memo` CLI entry points, leaving config/data intact.
  Future<void> removeCli() async {
    await _dio.post('/api/cli/remove');
  }

  /// Re-copy the currently running binary onto the CLI's install location
  /// and rewrite its PATH wrapper. Fixes a stale/broken `memo` command.
  Future<void> reinstallCli() async {
    await _dio.post('/api/cli/reinstall');
  }

  /// Fully remove Memo: CLI entry points plus everything under the data
  /// directory (config, sessions, engine binaries), optionally preserving
  /// the memory database.
  Future<void> uninstallMemo({required bool keepMemory}) async {
    await _dio.post('/api/uninstall', data: {'keep_memory': keepMemory});
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

  // ─── TTS (Voice Live Mode, Faz 1 — see docs/plans/PLAN_voice_live_mode_faz1.md) ──

  /// Synthesizes [text] to WAV-encoded audio bytes via the backend's Piper
  /// synthesizer (POST /api/tts/synthesize). Mirrors transcribeAudio's shape
  /// in reverse: raw bytes response instead of raw bytes request, since the
  /// backend replies with `Content-Type: audio/wav` directly rather than
  /// base64-in-JSON — see internal/webserver's handleTTSSynthesize doc.
  Future<Uint8List> synthesizeSpeech(String text) async {
    final res = await _dio.post(
      '/api/tts/synthesize',
      data: {'text': text},
      options: Options(responseType: ResponseType.bytes),
    );
    return Uint8List.fromList(_guard<List<int>>(res.data));
  }

  /// Fetches one cached local "thinking" filler sound (GET
  /// /api/tts/filler, Faz 3) — same raw-bytes response shape as
  /// [synthesizeSpeech], no request body since there's nothing to
  /// configure.
  Future<Uint8List> getTTSFiller() async {
    final res = await _dio.get(
      '/api/tts/filler',
      options: Options(responseType: ResponseType.bytes),
    );
    return Uint8List.fromList(_guard<List<int>>(res.data));
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

  // ─── Client tracking ──────────────────────────────────────────────
  //
  // Lets a backend spawned on demand for a terminal session (see
  // internal/app/clients.go, internal/replcli's own heartbeat loop) know
  // this GUI is attached, so it doesn't shut itself down just because the
  // terminal that started it exits — and, since there's no reliable
  // window-close hook on desktop today, lets a backend notice this GUI
  // going away by simply no longer hearing from it (the backend prunes a
  // client that misses ~3 heartbeats). A standalone backend that never had
  // auto-shutdown armed just tracks this harmlessly.

  /// Registers this GUI instance as a client and returns its ID, to pass to
  /// [heartbeatClient]. Returns null if the backend doesn't support this
  /// (older build) or isn't reachable — callers should just skip
  /// heartbeating in that case, nothing else depends on it.
  Future<String?> registerClient() async {
    try {
      final resp = await _dio.post('/api/clients/register');
      return resp.data['client_id'] as String?;
    } catch (e) {
      return null;
    }
  }

  /// Refreshes clientId's last-seen time on the backend. Errors are
  /// swallowed — a missed heartbeat is exactly what the backend's
  /// staleness window tolerates.
  Future<void> heartbeatClient(String clientId) async {
    try {
      await _dio.post('/api/clients/heartbeat', data: {'client_id': clientId});
    } catch (e) {
      // best-effort — see class doc above
    }
  }

  /// Reports this client's current wall-clock UTC offset so every routine's
  /// stored Schedule.UTCOffsetMinutes (see its doc comment, BUG_REPORT TD-1)
  /// gets corrected to match. UTCOffsetMinutes freezes at routine-creation
  /// time and is never otherwise touched again, so without this a DST
  /// transition (or relocating to a new timezone) leaves every existing
  /// routine firing at the wrong wall-clock time forever. Called once per
  /// fresh client registration (see connectionStatusProvider in
  /// chat_provider.dart) rather than every heartbeat — cheap either way
  /// (the backend no-ops routines whose offset already matches), but a
  /// DST/relocation correction only actually needs to happen once per
  /// reconnect, not every 30s. Best-effort like the client-tracking calls
  /// above: nothing else depends on this succeeding on any given tick.
  Future<void> syncRoutineUtcOffset() async {
    try {
      await _dio.post(
        '/api/routines/sync-offset',
        data: {'utc_offset_minutes': DateTime.now().timeZoneOffset.inMinutes},
      );
    } catch (e) {
      // best-effort — see class doc above
    }
  }

  // ─── Provider Management ─────────────────────────────────────────

  /// Get all provider configs.
  Future<List<ProviderConfig>> getProviders() async {
    final res = await _dio.get('/api/providers');
    if (res.data is List) {
      return (_guard<List>(res.data))
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
    return _guard<Map<String, dynamic>>(res.data);
  }

  /// List available models for any OpenAI-compatible provider type (generic
  /// counterpart to [fetchOpenRouterModels] — used by OpenCode Zen/Go so the
  /// user picks from the API's real model list instead of typing one by hand).
  Future<Map<String, dynamic>> fetchProviderModels({
    required String type,
    required String apiKey,
    String? baseUrl,
  }) async {
    final res = await _dio.post(
      '/api/providers/models',
      data: {'type': type, 'api_key': apiKey, 'base_url': baseUrl ?? ''},
    );
    return _guard<Map<String, dynamic>>(res.data);
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

  // ─── TTS Provider Management (Faz 2) ───────────────────────────────

  /// Get all external TTS provider configs.
  Future<List<TTSProviderConfig>> getTTSProviders() async {
    final res = await _dio.get('/api/tts/providers');
    if (res.data is List) {
      return (_guard<List>(res.data))
          .map((e) => TTSProviderConfig.fromJson(e as Map<String, dynamic>))
          .toList();
    }
    return [];
  }

  /// Update (add or edit) a TTS provider config.
  Future<void> updateTTSProvider(TTSProviderConfig config) async {
    await _dio.put('/api/tts/providers', data: config.toJson());
  }

  /// Delete a TTS provider config by type, disambiguated by [name] when given.
  Future<void> deleteTTSProvider(String type, {String? name}) async {
    await _dio.delete('/api/tts/providers', data: {
      'type': type,
      if (name != null && name.isNotEmpty) 'name': name,
    });
  }

  /// Test a TTS provider connection (a real, short synthesis call).
  Future<Map<String, dynamic>> testTTSProvider(TTSProviderConfig config) async {
    final res = await _dio.post('/api/tts/providers/test', data: config.toJson());
    return _guard<Map<String, dynamic>>(res.data);
  }

  // ─── TTS Local Voice Store (Faz 2.6 — fully offline, no API key) ──

  /// Get the curated voice catalog + already-downloaded voices + any
  /// in-flight download progress, in one call.
  Future<TTSVoiceStoreState> getTTSVoices() async {
    final res = await _dio.get('/api/tts/voices');
    return TTSVoiceStoreState.fromJson(_guard<Map<String, dynamic>>(res.data));
  }

  /// Start downloading a curated voice in the background.
  Future<void> downloadTTSVoice(String locale, String name, String quality) async {
    await _dio.post('/api/tts/voices/download', data: {
      'locale': locale,
      'name': name,
      'quality': quality,
    });
  }

  /// Delete a downloaded voice's files.
  Future<void> deleteTTSVoice(String id) async {
    await _dio.delete('/api/tts/voices', data: {'id': id});
  }

  /// Point the local Piper synthesizer at a downloaded voice.
  Future<void> selectTTSVoice(String id) async {
    await _dio.post('/api/tts/voices/select', data: {'id': id});
  }

  // ─── Orchestra Mode ───────────────────────────────────────────────

  /// Get orchestra config.
  Future<OrchestraConfig> getOrchestraConfig() async {
    final res = await _dio.get('/api/orchestra/config');
    return OrchestraConfig.fromJson(_guard<Map<String, dynamic>>(res.data));
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
      return _guardList<Map<String, dynamic>>(res.data);
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
      return _guardList<Map<String, dynamic>>(res.data);
    }
    return [];
  }

  /// Get a specific skill by name.
  Future<Map<String, dynamic>> getSkill(String name) async {
    final res = await _dio.get('/api/skills/get/$name');
    return _guard<Map<String, dynamic>>(res.data);
  }

  /// Install a skill from a local path.
  Future<Map<String, dynamic>> installSkill(String path) async {
    final res = await _dio.post('/api/skills/install', data: {'path': path});
    return _guard<Map<String, dynamic>>(res.data);
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
      return _guard<Map<String, dynamic>>(res.data);
    } catch (e) {
      return {'current': 'unknown', 'latest': null, 'error': e.toString()};
    }
  }

  // ─── WhatsApp ──────────────────────────────────────────────────

  /// Get WhatsApp connection status.
  Future<Map<String, dynamic>> getWhatsAppStatus() async {
    final res = await _dio.get('/api/whatsapp/status');
    return _guard<Map<String, dynamic>>(res.data);
  }

  /// Start WhatsApp connection (triggers QR pairing if not logged in).
  Future<Map<String, dynamic>> startWhatsApp() async {
    final res = await _dio.post('/api/whatsapp/start');
    return _guard<Map<String, dynamic>>(res.data);
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
    return _guard<Map<String, dynamic>>(res.data);
  }

  /// Search WhatsApp messages.
  Future<List<dynamic>> searchWhatsApp(String query) async {
    final res = await _dio.get('/api/whatsapp/search', queryParameters: {'q': query});
    return (_guard<List<dynamic>?>(res.data)) ?? [];
  }

  /// Get WhatsApp chat list.
  Future<List<dynamic>> getWhatsAppChats() async {
    final res = await _dio.get('/api/whatsapp/chats');
    return (_guard<List<dynamic>?>(res.data)) ?? [];
  }

  /// Get messages for a specific WhatsApp chat.
  Future<List<dynamic>> getWhatsAppMessages(String jid) async {
    final res = await _dio.get('/api/whatsapp/messages', queryParameters: {'jid': jid});
    return (_guard<List<dynamic>?>(res.data)) ?? [];
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
    return Uint8List.fromList(_guard<List<int>>(res.data));
  }

  /// Get WhatsApp message statistics.
  Future<Map<String, dynamic>> getWhatsAppStats() async {
    final res = await _dio.get('/api/whatsapp/stats');
    return _guard<Map<String, dynamic>>(res.data);
  }

  // ─── Web Search ────────────────────────────────────────────────

  /// Whether web-search mode is on (every message enriched with live results).
  Future<bool> getWebSearchEnabled() async {
    final res = await _dio.get('/api/websearch');
    return _guard<Map<String, dynamic>>(res.data)['enabled'] == true;
  }

  /// Enable/disable web-search mode.
  Future<void> setWebSearchEnabled(bool enabled) async {
    await _dio.post('/api/websearch', data: {'enabled': enabled});
  }

  /// Get WhatsApp chat mode state.
  Future<bool> getWhatsAppChatMode() async {
    final res = await _dio.get('/api/whatsapp/chat-mode');
    return _guard<Map<String, dynamic>>(res.data)['enabled'] == true;
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
    } on DioException catch (e) {
      // A cancelled request (e.g. the user pressed Stop) is not an error —
      // the other two stream methods (sendMessageStream, sendFileStream)
      // already special-case this; this one didn't, so stopping a WhatsApp
      // reply mid-stream surfaced a spurious "WhatsApp stream error" to the
      // user instead of just ending quietly.
      if (e.type == DioExceptionType.cancel) return;
      throw Exception('WhatsApp stream error: $e');
    }
  }

  // ─── Swarm ──────────────────────────────────────────────────────

  /// Combined host + worker swarm status (Beta feature).
  Future<SwarmStatus> getSwarmStatus() async {
    final res = await _dio.get('/api/swarm/status');
    return SwarmStatus.fromJson(_guard<Map<String, dynamic>>(res.data));
  }

  /// Open a host room for the given GGUF path; returns the shareable room code.
  Future<String> swarmHostCreate(String modelPath) async {
    final res = await _dio.post('/api/swarm/host/create', data: {
      'model_path': modelPath,
    });
    final data = _guard<Map<String, dynamic>>(res.data);
    return data['room_code'] as String? ?? '';
  }

  /// Remove a registered worker from the host room.
  Future<void> swarmHostRemoveWorker(String workerId) async {
    await _dio.post('/api/swarm/host/workers/remove', data: {
      'worker_id': workerId,
    });
  }

  /// Reorder workers (order maps positionally to --tensor-split).
  Future<void> swarmHostReorderWorkers(int fromIndex, int toIndex) async {
    await _dio.post('/api/swarm/host/workers/reorder', data: {
      'from_index': fromIndex,
      'to_index': toIndex,
    });
  }

  /// Set one worker's percentage share of the tensor split.
  Future<void> swarmHostSetShare(String workerId, double sharePercent) async {
    await _dio.post('/api/swarm/host/workers/share', data: {
      'worker_id': workerId,
      'share_percent': sharePercent,
    });
  }

  /// Start the coordinator llama-server with --rpc to registered workers.
  Future<void> swarmHostStart({int ctxSize = 0}) async {
    await _dio.post('/api/swarm/host/start', data: {
      'ctx_size': ctxSize,
    });
  }

  /// Stop the coordinator llama-server without closing the room.
  Future<void> swarmHostStop() async {
    await _dio.post('/api/swarm/host/stop');
  }

  /// Stop the swarm (if running) and tear down the room.
  Future<void> swarmHostClose() async {
    await _dio.post('/api/swarm/host/close');
  }

  /// Join a host room by pasting its room code (starts local rpc-server).
  Future<void> swarmJoin(String code) async {
    await _dio.post('/api/swarm/join', data: {'code': code});
  }

  /// Leave the current swarm (stop local rpc-server).
  Future<void> swarmLeave() async {
    await _dio.post('/api/swarm/leave');
  }

  // ─── Proactive Learning ─────────────────────────────────────────

  /// Get current proactive settings.
  Future<Map<String, dynamic>> getProactiveSettings() async {
    final res = await _dio.get('/api/proactive/settings');
    return _guard<Map<String, dynamic>>(res.data);
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
    final patterns = res.data is Map ? res.data['patterns'] : null;
    if (patterns is! List) return [];
    return _guardList<Map<String, dynamic>>(patterns);
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
    return Map<String, dynamic>.from(_guard<Map>(res.data));
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
      return _guardList<Map<String, dynamic>>(res.data);
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
    return Map<String, dynamic>.from(_guard<Map>(res.data));
  }

  /// Delete a calendar event.
  Future<void> deleteCalendarEvent(String id) async {
    await _dio.delete('/api/calendar/events/$id');
  }

  /// Get calendar settings (reminder lead minutes).
  Future<Map<String, dynamic>> getCalendarSettings() async {
    final res = await _dio.get('/api/calendar/settings');
    return Map<String, dynamic>.from(_guard<Map>(res.data));
  }

  /// Update calendar settings (reminder lead minutes + time-guess toggle).
  Future<void> updateCalendarSettings(int reminderLeadMinutes,
      {bool disableTimeGuess = false}) async {
    await _dio.put('/api/calendar/settings', data: {
      'reminder_lead_minutes': reminderLeadMinutes,
      'disable_time_guess': disableTimeGuess,
    });
  }

  // ─── Routines (scheduled automations) ────────────────────────────

  /// List configured routines.
  Future<List<Map<String, dynamic>>> getRoutines() async {
    final res = await _dio.get('/api/routines');
    if (res.data is List) {
      return _guardList<Map<String, dynamic>>(res.data);
    }
    return [];
  }

  /// Parse a free-text routine description into a draft for confirmation.
  Future<Map<String, dynamic>> parseRoutineText(String text) async {
    final res = await _dio.post('/api/routines/parse', data: {'text': text});
    return Map<String, dynamic>.from(_guard<Map>(res.data));
  }

  /// Create a routine from a (possibly user-edited) draft.
  Future<Map<String, dynamic>> createRoutine({
    required String originalText,
    required Map<String, dynamic> draft,
    String whatsAppTargetJid = '',
    bool autoApproveTools = false,
  }) async {
    final res = await _dio.post('/api/routines', data: {
      'original_text': originalText,
      'draft': draft,
      'whatsapp_target_jid': whatsAppTargetJid,
      'auto_approve_tools': autoApproveTools,
      // Backend has no locale of its own — this is what lets it localize its
      // own generated text (notification title, context fillers, system
      // prompt) per routine (BUG-M1). See routine.Routine.Language.
      'language': L10n.locale == MemoLocale.en ? 'en' : 'tr',
      // Backend has no timezone of its own either — without this, "HH:MM"
      // is interpreted in whatever timezone the backend host happens to be
      // running in, not the user's (BUG-M4). See
      // routine.Schedule.UTCOffsetMinutes's doc comment for why this is a
      // fixed offset rather than a full IANA zone name.
      'utc_offset_minutes': DateTime.now().timeZoneOffset.inMinutes,
    });
    return Map<String, dynamic>.from(_guard<Map>(res.data));
  }

  /// Update a routine (e.g. toggling enabled).
  Future<Map<String, dynamic>> updateRoutine(Map<String, dynamic> routine) async {
    final id = routine['id'] as String;
    final res = await _dio.put('/api/routines/$id', data: routine);
    return Map<String, dynamic>.from(_guard<Map>(res.data));
  }

  /// Delete a routine.
  Future<void> deleteRoutine(String id) async {
    await _dio.delete('/api/routines/$id');
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

  // ─── Task Lists ─────────────────────────────────────────────────

  Future<List<TaskListInfo>> listTaskLists() async {
    final res = await _dio.get('/api/tasklists');
    if (res.data is! List) return [];
    return (res.data as List)
        .map((e) => TaskListInfo.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  Future<TaskList> createTaskList(
      String chatId, String title, List<String> items) async {
    final res = await _dio.post('/api/tasklists', data: {
      'chat_id': chatId,
      'title': title,
      'items': items,
    });
    return TaskList.fromJson(Map<String, dynamic>.from(_guard<Map>(res.data)));
  }

  Future<TaskList> getTaskList(String id) async {
    final res = await _dio.get('/api/tasklists/$id');
    return TaskList.fromJson(Map<String, dynamic>.from(_guard<Map>(res.data)));
  }

  Future<void> deleteTaskList(String id) async {
    await _dio.delete('/api/tasklists/$id');
  }

  Future<void> startTaskList(String id) async {
    await _dio.post('/api/tasklists/$id/start');
  }

  Future<void> stopTaskList(String id) async {
    await _dio.post('/api/tasklists/$id/stop');
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
