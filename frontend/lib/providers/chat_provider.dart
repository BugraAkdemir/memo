import 'dart:async';
import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:path/path.dart' as p;

import '../core/api_client.dart';
import '../core/l10n.dart';
import '../models/activity_step.dart';
import '../models/agent.dart';
import '../models/chat.dart';
import '../models/token_usage.dart';
import 'agent_provider.dart';
import 'settings_provider.dart';

/// Global API client instance. Reads the backend URL from SharedPreferences
/// so users can configure a custom address. Falls back to 127.0.0.1:8090.
final apiClientProvider = Provider<MemoApiClient>((ref) {
  final prefs = ref.read(prefsProvider);
  final savedUrl = prefs.getString('memo_api_base_url');
  final baseUrl = (savedUrl != null && savedUrl.isNotEmpty)
      ? savedUrl
      : 'http://127.0.0.1:8090';
  return MemoApiClient(baseUrl: baseUrl);
});

// ─── Chat List ──────────────────────────────────────────────────

final chatListProvider =
    AsyncNotifierProvider<ChatListNotifier, List<ChatSession>>(
      ChatListNotifier.new,
    );

class ChatListNotifier extends AsyncNotifier<List<ChatSession>> {
  @override
  Future<List<ChatSession>> build() async {
    final api = ref.read(apiClientProvider);
    return api.listChats();
  }

  Future<void> refresh() async {
    state = const AsyncLoading();
    state = await AsyncValue.guard(
      () => ref.read(apiClientProvider).listChats(),
    );
  }

  Future<String> createNew() async {
    final api = ref.read(apiClientProvider);
    final id = await api.newChat();
    await refresh();
    return id;
  }

  Future<void> delete(String id) async {
    final api = ref.read(apiClientProvider);
    await api.deleteChat(id);
    await refresh();
    // If the deleted chat was active, invalidate activeChatId so it reloads
    final activeId = ref.read(activeChatIdProvider).valueOrNull;
    if (activeId == id) {
      ref.invalidate(activeChatIdProvider);
    }
  }

  Future<void> rename(String id, String title) async {
    final api = ref.read(apiClientProvider);
    await api.renameChat(id, title);
    await refresh();
  }
}

// ─── Active Chat ────────────────────────────────────────────────

final activeChatIdProvider =
    AsyncNotifierProvider<ActiveChatIdNotifier, String>(
      ActiveChatIdNotifier.new,
    );

class ActiveChatIdNotifier extends AsyncNotifier<String> {
  @override
  Future<String> build() async {
    return ref.read(apiClientProvider).getActiveChatId();
  }

  Future<void> switchTo(String id) async {
    final api = ref.read(apiClientProvider);
    // Cancel any in-flight stream
    ref.read(messagesProvider.notifier).stopStreaming();
    await api.switchChat(id);
    state = AsyncData(id);
    ref.invalidate(messagesProvider);

    // Auto-enable/disable agent mode based on chat type
    final chats = ref.read(chatListProvider).valueOrNull ?? [];
    final chat = chats.where((c) => c.id == id).firstOrNull;
    final isAgent = chat?.isAgentChat ?? false;
    if (isAgent) {
      if (!ref.read(agentEnabledProvider)) {
        ref.read(agentEnabledProvider.notifier).setEnabled(true);
      }
    }
  }
}

// ─── Streaming Content ──────────────────────────────────────────

/// Holds the currently streaming assistant content (token by token).
/// Resets to '' when streaming ends or on error.
final streamingContentProvider = StateProvider<String>((ref) => '');

/// Holds the currently streaming thinking content (token by token).
final streamingThinkingProvider = StateProvider<String>((ref) => '');

/// Holds the agent events while streaming.
final streamingAgentEventsProvider = StateProvider<List<AgentEvent>>(
  (ref) => [],
);

/// Transient pre-token status (e.g. 'web_search') shown in the typing indicator
/// while the backend works before the first content token arrives.
final streamingStatusProvider = StateProvider<String>((ref) => '');

/// Live, unified activity timeline for the right-side panel: orchestra plan
/// tasks + agent tool steps, in the order they happen. Updated during a turn,
/// kept until the next send so the user can review what just happened.
final activityStepsProvider = StateProvider<List<ActivityStep>>((ref) => []);

/// Live token usage for the current/last turn (Claude-Code-style counter).
final tokenUsageProvider = StateProvider<TokenUsage?>((ref) => null);

/// Web-search mode: when on, every message is enriched with live web results
/// (no keyword detection). Persisted server-side.
final webSearchModeProvider =
    StateNotifierProvider<WebSearchModeNotifier, bool>(
        (ref) => WebSearchModeNotifier(ref.read(apiClientProvider), ref));

class WebSearchModeNotifier extends StateNotifier<bool> {
  final MemoApiClient _api;
  final Ref _ref;
  WebSearchModeNotifier(this._api, this._ref) : super(false) {
    _init();
  }

  Future<void> _init() async {
    try {
      state = await _api.getWebSearchEnabled();
    } catch (e) {
      debugPrint('chat: web search init error: $e');
      // leave default (off) on error
    }
  }

  Future<void> toggle() async {
    final next = !state;
    try {
      await _api.setWebSearchEnabled(next);
      state = next;
    } catch (e) {
      _ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: Web arama modu değiştirilemedi ($e)';
    }
  }
}

// ─── Messages ───────────────────────────────────────────────────

final messagesProvider =
    AsyncNotifierProvider<MessagesNotifier, List<ChatMessage>>(
      MessagesNotifier.new,
    );

class MessagesNotifier extends AsyncNotifier<List<ChatMessage>> {
  CancelToken? _cancelToken;
  bool _stopped = false;
  bool _disposed = false;
  Timer? _delayedRefreshTimer;
  int _toolSeq = 0; // sequence for synthesising agent tool-step ids

  /// Inserts or updates a step in the activity timeline, keyed by id.
  void _upsertActivity(ActivityStep step) {
    final list = [...ref.read(activityStepsProvider)];
    final idx = list.indexWhere((s) => s.id == step.id);
    if (idx >= 0) {
      list[idx] = list[idx].copyWith(
        title: step.title.isNotEmpty ? step.title : null,
        subtitle: step.subtitle.isNotEmpty ? step.subtitle : null,
        status: step.status,
        durationMs: step.durationMs > 0 ? step.durationMs : null,
        detail: step.detail,
      );
    } else {
      list.add(step);
    }
    ref.read(activityStepsProvider.notifier).state = list;
  }

  /// Marks any still-running activity step as errored. Called when a turn is
  /// cancelled or fails so the panel doesn't leave a spinner running forever.
  void _settleRunningSteps(String detail) {
    final list = ref.read(activityStepsProvider);
    if (!list.any((s) => s.status == StepStatus.running)) return;
    ref.read(activityStepsProvider.notifier).state = [
      for (final s in list)
        s.status == StepStatus.running
            ? s.copyWith(status: StepStatus.error, detail: detail)
            : s,
    ];
  }

  /// Maps an agent tool event into the unified activity timeline.
  void _toolEventToActivity(AgentEvent ev) {
    switch (ev.type) {
      case 'tool_executing':
        _toolSeq++;
        _upsertActivity(ActivityStep(
          id: 'tool-$_toolSeq',
          kind: 'tool',
          title: ev.toolName ?? 'Araç',
          status: StepStatus.running,
        ));
        break;
      case 'tool_result':
      case 'tool_error':
      case 'permission_denied':
        final id = _toolSeq > 0 ? 'tool-$_toolSeq' : 'tool-1';
        _upsertActivity(ActivityStep(
          id: id,
          kind: 'tool',
          title: ev.toolName ?? 'Araç',
          status: ev.type == 'tool_result'
              ? StepStatus.done
              : StepStatus.error,
          durationMs: ev.durationMs ?? 0,
          detail: ev.error,
        ));
        break;
    }
  }

  @override
  Future<List<ChatMessage>> build() async {
    ref.onDispose(() {
      _disposed = true;
      _delayedRefreshTimer?.cancel();
    });
    return ref.read(apiClientProvider).getMessages();
  }

  void addMessage(ChatMessage msg) {
    final current = [...(state.valueOrNull ?? <ChatMessage>[])];
    state = AsyncData([...current, msg]);
  }

  void stopStreaming() {
    _stopped = true;
    _cancelToken?.cancel();
    _cancelToken = null;
    ref.read(isSendingProvider.notifier).state = false;
      ref.read(streamingContentProvider.notifier).state = '';
      ref.read(streamingThinkingProvider.notifier).state = '';
      ref.read(streamingAgentEventsProvider.notifier).state = [];
      ref.read(streamingStatusProvider.notifier).state = '';
    ref.read(streamingAgentEventsProvider.notifier).state = [];
    _settleRunningSteps('durduruldu');
  }

  Future<void> refresh() async {
    state = await AsyncValue.guard(
      () => ref.read(apiClientProvider).getMessages(),
    );
  }

  Future<void> updateMessage(int index, String newContent) async {
    final api = ref.read(apiClientProvider);
    try {
      await api.updateMessage(index, newContent);
      final current = [...(state.valueOrNull ?? <ChatMessage>[])];
      if (index >= 0 && index < current.length) {
        current[index] = ChatMessage(
          role: current[index].role,
          content: newContent,
          thinking: current[index].thinking,
          imagePath: current[index].imagePath,
          filePath: current[index].filePath,
          timestamp: current[index].timestamp,
          agentEvents: current[index].agentEvents,
        );
        state = AsyncData(current);
      }
    } catch (e) {
      ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: Mesaj güncellenemedi ($e)';
    }
  }

  Future<void> deleteMessage(int index) async {
    final api = ref.read(apiClientProvider);
    try {
      await api.deleteMessage(index);
      final current = [...(state.valueOrNull ?? <ChatMessage>[])];
      if (index >= 0 && index < current.length) {
        current.removeAt(index);
        state = AsyncData(current);
      }
    } catch (e) {
      ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: Mesaj silinemedi ($e)';
    }
  }

  /// Returns true if the message was a memory command (/remember, /forget)
  /// and was handled locally — caller should NOT send to AI.
  Future<bool> _handleMemoryCommand(String message, MemoApiClient api) async {
    final trimmed = message.trim();
    if (trimmed.startsWith('/remember ')) {
      final content = trimmed.substring('/remember '.length).trim();
      if (content.isEmpty) return false;
      try {
        await api.saveExplicitMemory(content);
        final ts = DateTime.now().toIso8601String().substring(11, 19);
        final current = state.valueOrNull ?? [];
        state = AsyncData([
          ...current,
          ChatMessage(role: 'user', content: trimmed, timestamp: ts),
          ChatMessage(
            role: 'assistant',
            content: '✓ Remembered: "$content"',
            timestamp: ts,
          ),
        ]);
      } catch (e) {
        ref.read(errorMessageProvider.notifier).state = 'Memory save failed: $e';
      }
      return true;
    }
    if (trimmed.startsWith('/forget ')) {
      final pattern = trimmed.substring('/forget '.length).trim();
      if (pattern.isEmpty) return false;
      try {
        final deleted = await api.deleteExplicitMemory(pattern);
        final ts = DateTime.now().toIso8601String().substring(11, 19);
        final current = state.valueOrNull ?? [];
        state = AsyncData([
          ...current,
          ChatMessage(role: 'user', content: trimmed, timestamp: ts),
          ChatMessage(
            role: 'assistant',
            content: deleted > 0
                ? '✓ Forgot $deleted memory entry(ies) matching "$pattern"'
                : 'No memories found matching "$pattern"',
            timestamp: ts,
          ),
        ]);
      } catch (e) {
        ref.read(errorMessageProvider.notifier).state = 'Memory delete failed: $e';
      }
      return true;
    }
    return false;
  }

  Future<void> sendMessage(String message) async {
    if (ref.read(isSendingProvider)) return;

    _stopped = false;
    _cancelToken = CancelToken();
    final api = ref.read(apiClientProvider);

    // Intercept /remember and /forget before sending to AI
    if (await _handleMemoryCommand(message, api)) return;

    // Signal sending state
    ref.read(isSendingProvider.notifier).state = true;

    // Reset the activity timeline + token counter for this new turn.
    _toolSeq = 0;
    ref.read(activityStepsProvider.notifier).state = [];
    ref.read(tokenUsageProvider.notifier).state = null;

    // Optimistically add user message only (assistant handled via streamingContent)
    final timestamp = DateTime.now().toIso8601String().substring(11, 19);
    final userMsg = ChatMessage(
      role: 'user',
      content: message,
      timestamp: timestamp,
    );

    final current = state.valueOrNull ?? [];
    state = AsyncData([...current, userMsg]);

    try {
      final streamingEnabled = ref.read(streamingEnabledProvider);
      String fullReply = '';
      String fullThinking = '';
      List<AgentEvent> finalAgentEvents = [];

      if (streamingEnabled) {
        final stream = api.sendMessageStream(
          message,
          cancelToken: _cancelToken,
        );

        // Backend budgets up to 5 minutes for a single generation (local model
        // prefill on slow/CPU hardware, or a long agent tool run), and sends no
        // heartbeat while waiting for the first chunk. A shorter client-side
        // timeout here would abort — and lose — a response the backend was
        // still legitimately producing, so both modes must match that budget.
        final isAgentMode = ref.read(agentEnabledProvider);
        const streamTimeout = Duration(seconds: 300);

        await for (final chunk in stream.timeout(
          streamTimeout,
          onTimeout: (sink) => sink.addError(
            Exception(isAgentMode
                ? L10n.t('error_agent_timeout')
                : L10n.t('error_server_timeout')),
          ),
        )) {
              if (chunk.finishReason == 'status') {
            // Pre-token status (e.g. web_search) — show in the typing line.
            ref.read(streamingStatusProvider.notifier).state = chunk.content;
          } else if (chunk.finishReason == 'activity') {
            // Structured orchestra task update → right-side activity panel.
            try {
              final decoded = json.decode(chunk.content);
              if (decoded is Map<String, dynamic>) {
                _upsertActivity(ActivityStep.fromActivityJson(decoded));
              }
            } catch (_) {
              // ignore malformed activity payloads
            }
          } else if (chunk.finishReason == 'usage') {
            try {
              final decoded = json.decode(chunk.content);
              if (decoded is Map<String, dynamic>) {
                ref.read(tokenUsageProvider.notifier).state =
                    TokenUsage.fromJson(decoded);
              }
            } catch (_) {/* ignore */}
          } else if (chunk.finishReason == 'agent_event') {
            try {
              final ev = AgentEvent.fromJson(json.decode(chunk.content));
              _toolEventToActivity(ev); // mirror into the activity timeline
              final currentEvents = [...ref.read(streamingAgentEventsProvider)];

              // Only keep permission_request for dialogs and final results/errors.
              // tool_executing events are transient — replace the last event with
              // this one so the UI shows a single "dusunuyor..." line instead of
              // accumulating one per tool call.
              if (ev.type == 'tool_executing') {
                // Keep only this as the current executing event (replaces previous)
                if (currentEvents.isNotEmpty && currentEvents.last.type == 'tool_executing') {
                  currentEvents[currentEvents.length - 1] = ev;
                } else {
                  currentEvents.add(ev);
                }
              } else if (ev.type == 'tool_result' || ev.type == 'tool_error' || ev.type == 'permission_denied') {
                // Check if we can replace the last executing event with this result
                if (currentEvents.isNotEmpty && currentEvents.last.type == 'tool_executing') {
                  currentEvents[currentEvents.length - 1] = ev;
                } else {
                  currentEvents.add(ev);
                }
              } else if (ev.type == 'permission_request') {
                currentEvents.add(ev);
                ref.read(agentEventBusProvider).emit(ev);
              }

              ref.read(streamingAgentEventsProvider.notifier).state =
                  currentEvents;
              finalAgentEvents = currentEvents;
            } catch (e) {
              // ignore parse errors
            }
          } else {
            // First real content — clear any pre-token status (e.g. web_search).
            if (ref.read(streamingStatusProvider).isNotEmpty) {
              ref.read(streamingStatusProvider.notifier).state = '';
            }
            fullReply += chunk.content;
            fullThinking += chunk.thinking ?? '';

            // Update streaming providers instead of copying the entire message list
            ref.read(streamingContentProvider.notifier).state = fullReply;
            ref.read(streamingThinkingProvider.notifier).state = fullThinking;
          }
        }

        if (_stopped) {
          _stopped = false;
          ref.read(streamingContentProvider.notifier).state = '';
          ref.read(streamingThinkingProvider.notifier).state = '';
          ref.read(streamingAgentEventsProvider.notifier).state = [];
          ref.read(streamingStatusProvider.notifier).state = '';
          return;
        }
      } else {
        fullReply = await api.sendMessage(message);
      }

      // Append final assistant message to the list
      final list = [...(state.valueOrNull ?? <ChatMessage>[])];
      if (fullReply.isNotEmpty ||
          fullThinking.isNotEmpty ||
          finalAgentEvents.isNotEmpty) {
        list.add(
          ChatMessage(
            role: 'assistant',
            content: fullReply,
            thinking: fullThinking.isNotEmpty ? fullThinking : null,
            timestamp: timestamp,
            agentEvents: finalAgentEvents.isNotEmpty ? finalAgentEvents : null,
          ),
        );
      }
      state = AsyncData(list);

      // Clear streaming state before refresh to avoid stale UI
      ref.read(streamingContentProvider.notifier).state = '';
      ref.read(streamingThinkingProvider.notifier).state = '';
      ref.read(streamingAgentEventsProvider.notifier).state = [];
      ref.read(streamingStatusProvider.notifier).state = '';

      // Refresh chat metadata — wait a beat so async title generation finishes
      ref.invalidate(chatListProvider);
      _delayedRefreshTimer?.cancel();
      _delayedRefreshTimer = Timer(const Duration(seconds: 2), () {
        if (!_disposed) ref.invalidate(chatListProvider);
      });
    } catch (e) {
      _stopped = false;
      ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: Mesaj gönderilemedi ($e)';
      ref.read(streamingContentProvider.notifier).state = '';
      ref.read(streamingThinkingProvider.notifier).state = '';
      ref.read(streamingAgentEventsProvider.notifier).state = [];
      ref.read(streamingStatusProvider.notifier).state = '';
      _settleRunningSteps('hata'); // stop any perpetual spinner in the panel
      await refresh();
    } finally {
      _cancelToken = null;
      ref.read(isSendingProvider.notifier).state = false;
    }
  }

  Future<String> sendFile(String message, String filePath) async {
    if (ref.read(isSendingProvider)) return '';

    _stopped = false;
    _cancelToken = CancelToken();
    final api = ref.read(apiClientProvider);

    ref.read(isSendingProvider.notifier).state = true;

    final fileName = p.basename(filePath);
    final ext = filePath.split('.').last.toLowerCase();
    final isImage = ['png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp'].contains(ext);
    final displayMsg = message.isEmpty
        ? '*(Dosya gönderildi: $fileName)*'
        : '$message\n*(Dosya: $fileName)*';

    final userMsg = ChatMessage(
      role: 'user',
      content: displayMsg,
      imagePath: isImage ? filePath : null,
      timestamp: DateTime.now().toIso8601String().substring(11, 19),
    );

    final current = state.valueOrNull ?? [];
    state = AsyncData([...current, userMsg]);

    try {
      final streamingEnabled = ref.read(streamingEnabledProvider);
      String fullReply = '';
      String fullThinking = '';

      if (streamingEnabled) {
        final stream = api.sendFileStream(
          message,
          filePath,
          cancelToken: _cancelToken,
        );

        // Matches the backend's 300s generation budget — see sendMessage's
        // stream.timeout above for why this can't be shorter.
        await for (final chunk in stream.timeout(
          const Duration(seconds: 300),
          onTimeout: (sink) => sink.addError(
            Exception(L10n.t('error_server_timeout')),
          ),
        )) {
          fullReply += chunk.content;
          fullThinking += chunk.thinking ?? '';

          ref.read(streamingContentProvider.notifier).state = fullReply;
          ref.read(streamingThinkingProvider.notifier).state = fullThinking;
        }

        if (_stopped) {
          _stopped = false;
          return '';
        }
      } else {
        fullReply = await api.sendFile(message, filePath);
      }

      final list = [...(state.valueOrNull ?? <ChatMessage>[])];
      if (fullReply.isNotEmpty || fullThinking.isNotEmpty) {
        list.add(
          ChatMessage(
            role: 'assistant',
            content: fullReply,
            thinking: fullThinking.isNotEmpty ? fullThinking : null,
            timestamp: DateTime.now().toIso8601String().substring(11, 19),
          ),
        );
      }
      state = AsyncData(list);

      ref.read(streamingContentProvider.notifier).state = '';
      ref.read(streamingThinkingProvider.notifier).state = '';

      await refresh();
      ref.invalidate(chatListProvider);
      return fullReply;
    } catch (e) {
      ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: Dosya gönderilemedi ($e)';
      ref.read(streamingContentProvider.notifier).state = '';
      ref.read(streamingThinkingProvider.notifier).state = '';
      await refresh();
      return '';
    } finally {
      _cancelToken = null;
      ref.read(isSendingProvider.notifier).state = false;
    }
  }
}

// ─── Error Message ──────────────────────────────────────────────

/// Holds a one-shot error message to display as a snackbar.
/// Cleared after being read.
final errorMessageProvider = StateProvider<String>((ref) => '');

// ─── Composer Draft ─────────────────────────────────────────────

/// One-shot starter text pushed into the chat input (e.g. from welcome-screen
/// suggestions). The input consumes it, fills the field, then clears it back
/// to null.
final composerDraftProvider = StateProvider<String?>((ref) => null);

// ─── Incognito Mode ─────────────────────────────────────────────

final incognitoProvider = StateNotifierProvider<IncognitoNotifier, bool>(
  (ref) => IncognitoNotifier(ref),
);

class IncognitoNotifier extends StateNotifier<bool> {
  final Ref _ref;
  IncognitoNotifier(this._ref) : super(false);

  Future<void> toggle() async {
    final previous = state;
    state = !state;
    try {
      await _ref.read(apiClientProvider).toggleIncognito(state);
    } catch (e) {
      debugPrint('chat: incognito toggle error: $e');
      state = previous;
      _ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: Gizli mod değiştirilemedi ($e)';
    }
  }
}

// ─── Sending State ──────────────────────────────────────────────

/// True when a message is being sent and we're waiting for a reply.
final isSendingProvider = StateProvider<bool>((ref) => false);

// ─── Connection Status (polls every 30s) ────────────────────────

final connectionStatusProvider = StreamProvider.autoDispose<bool>((ref) async* {
  var alive = true;
  ref.onDispose(() => alive = false);
  final api = ref.read(apiClientProvider);
  while (alive) {
    try {
      yield await api.isAlive();
    } catch (e) {
      debugPrint('chat: connectionStatus error: $e');
      yield false;
    }
    if (!alive) break;
    await Future.delayed(const Duration(seconds: 30));
  }
});
