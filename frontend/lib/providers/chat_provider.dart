import 'dart:async';
import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:path/path.dart' as p;

import '../core/api_client.dart';
import '../core/backend_url.dart';
import '../core/l10n.dart';
import '../models/agent.dart';
import '../models/chat.dart';
import '../models/token_usage.dart';
import 'agent_provider.dart';
import 'auth_gate_provider.dart';
import 'gate_guard.dart';
import 'settings_provider.dart';
import '../core/friendly_error.dart';

/// Global API client instance. Reads the backend URL from SharedPreferences
/// so users can configure a custom address. Falls back to 127.0.0.1:8090.
///
/// Also seeds the client with whatever remote-access token was last learned
/// and persists any newly learned one — see MemoApiClient's
/// savedRemoteToken doc comment for why (BUG-L5).
const _remoteTokenPrefsKey = 'memo_remote_access_token';

final apiClientProvider = Provider<MemoApiClient>((ref) {
  final prefs = ref.read(prefsProvider);
  // On web the page is served by the very backend it must talk to, so the
  // effective URL is resolved through the page's own origin — a stale
  // saved localhost/127.0.0.1 value (from a previous localhost session) is
  // ignored when the page itself was loaded from a LAN address, which is
  // exactly the cross-origin/127.0.0.1 trap that produced a wall of CORS
  // errors. Desktop keeps its 127.0.0.1 default + explicit change-server.
  final saved = prefs.getString('memo_api_base_url') ?? '';
  final baseUrl = kIsWeb
      ? webBackendUrl(saved, Uri.base.origin)
      : normalizeBackendUrl(saved);
  return MemoApiClient(
    baseUrl: baseUrl,
    savedRemoteToken: prefs.getString(_remoteTokenPrefsKey),
    onRemoteTokenLearned: (token) => prefs.setString(_remoteTokenPrefsKey, token),
  );
});

// Chat ids with a CLI-backed background stream currently in flight — polled
// for the chat sidebar's "processing" indicator. autoDispose: only worth
// polling while the sidebar (or anything else) is actually watching it.
final runningCLIChatsProvider = StreamProvider.autoDispose<Set<String>>((ref) async* {
  final api = ref.watch(apiClientProvider);
  while (true) {
    // BUG-ONB4: no click on a token-gated backend while the gate is up.
    if (authGateBlocked(ref.read(authGateProvider).valueOrNull)) {
      yield <String>{};
      await cancellablePause(ref, const Duration(seconds: 3));
      continue;
    }
    try {
      final ids = await api.getRunningCLIChats();
      yield ids.toSet();
    } catch (_) {
      yield <String>{};
    }
    await cancellablePause(ref, const Duration(seconds: 3));
  }
});

/// The currently active chat's own CLI provider (empty string if it isn't
/// CLI-backed). Watched by chat_input.dart's "is there anywhere to send
/// this message" check — without this, a chat using ONLY a CLI provider
/// (no app-wide provider, no local model) looked like it had nowhere to
/// send to and blocked sending entirely.
final activeChatCLIProviderProvider = FutureProvider.autoDispose<String>((ref) async {
  final chatId = await ref.watch(activeChatIdProvider.future);
  if (chatId.isEmpty) return '';
  return ref.watch(apiClientProvider).getChatCLIProvider(chatId);
});

/// The currently active chat's own CLI working directory (empty if unset).
/// Shown next to the model picker so it's always visible which folder a CLI
/// provider is actually operating in.
final activeChatCLIWorkdirProvider = FutureProvider.autoDispose<String>((ref) async {
  final chatId = await ref.watch(activeChatIdProvider.future);
  if (chatId.isEmpty) return '';
  return ref.watch(apiClientProvider).getChatCLIWorkdir(chatId);
});

/// The currently active chat's CLI model override (empty = CLI's own
/// default). Drives the top bar's model picker while a chat is in CLI mode.
final activeChatCLIModelProvider = FutureProvider.autoDispose<String>((ref) async {
  final chatId = await ref.watch(activeChatIdProvider.future);
  if (chatId.isEmpty) return '';
  return ref.watch(apiClientProvider).getChatCLIModel(chatId);
});

/// Model ids the given CLI type can be switched to — see
/// MemoApiClient.getCLIModelOptions for why this can legitimately be empty.
final cliModelOptionsProvider =
    FutureProvider.autoDispose.family<List<String>, String>((ref, cliType) {
  return ref.watch(apiClientProvider).getCLIModelOptions(cliType);
});

/// Chat ids whose CLI job just finished while the user wasn't looking at
/// that chat — cleared once they open it. Notification-badge behavior for
/// the sidebar's CLI indicator (yapacam.md §2.5).
final cliJustFinishedChatsProvider =
    StateNotifierProvider<CLIJustFinishedNotifier, Set<String>>((ref) {
  return CLIJustFinishedNotifier(ref);
});

class CLIJustFinishedNotifier extends StateNotifier<Set<String>> {
  final Ref _ref;
  Set<String> _lastRunning = {};

  CLIJustFinishedNotifier(this._ref) : super({}) {
    _ref.listen(runningCLIChatsProvider, (previous, next) {
      final current = next.valueOrNull ?? {};
      final finished = _lastRunning.difference(current);
      if (finished.isNotEmpty) {
        final activeId = _ref.read(activeChatIdProvider).valueOrNull;
        state = {...state, ...finished}..remove(activeId);
      }
      _lastRunning = current;
    });
  }

  void markSeen(String chatId) {
    if (state.contains(chatId)) {
      state = {...state}..remove(chatId);
    }
  }
}

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
    } else {
      if (ref.read(agentEnabledProvider)) {
        ref.read(agentEnabledProvider.notifier).setEnabled(false);
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
      debugPrint('chat: web search init error: ${FriendlyError.describeGeneric(e)}');
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
          '${L10n.t('error')}: Web arama modu değiştirilemedi (${FriendlyError.describeGeneric(e)})';
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
  Timer? _delayedRefreshTimer;

  // Riverpod can rebuild this exact same MessagesNotifier *instance* (not a
  // fresh one) when messagesProvider is invalidated while something is still
  // watching it (e.g. a chat switch, or — confirmed via a captured
  // [SEND-DEBUG] log from a real session — something during ordinary app
  // startup, before the user ever sends anything): build() -> onDispose ->
  // build() again on the same object. A plain `bool _disposed` flag, reset
  // once in build(), breaks in *both* directions across that reuse:
  //   - left unreset, it stays permanently `true` after the *first* such
  //     cycle for the rest of the session, so every future sendMessage()'s
  //     `finally` block permanently skips resetting isSendingProvider (BUG:
  //     the send/stop button gets stuck on "stop" forever, on every turn).
  //   - reset unconditionally in build(), it *un-disposes* an old, abandoned
  //     stream that is still running on this same reused object (started
  //     under the previous generation, e.g. from a chat just switched away
  //     from) — reintroducing BUG-H2, where that abandoned stream clobbers
  //     isSendingProvider for whatever new send is legitimately in progress.
  //
  // A generation counter fixes both: each build() bumps it, and each
  // sendMessage()/sendFile() call captures the generation that was current
  // when *it* started. A call only touches shared state (isSendingProvider,
  // `state`, etc.) while its captured generation still matches the live one
  // — once build() runs again, every older in-flight call's captured value
  // is stale and it correctly no-ops, while a *new* call started after that
  // rebuild captures the new generation and works normally.
  int _generation = 0;

  @override
  Future<List<ChatMessage>> build() async {
    _generation++;
    _stopped = false;
    ref.onDispose(() {
      _delayedRefreshTimer?.cancel();
    });
    // BUG-ONB4: while the auth gate is up, the backend answers 401 — which
    // used to surface as a visible "Bir şeyler ters gitti" over the login
    // screen. The gate owns that state; mount as an empty chat instead.
    // (Notifier build may not `ref.watch` a stream provider — Riverpod
    // leaves the provider's future pending forever — and
    // `ref.listen`+`invalidateSelf` races the build's own completion, so
    // the "reload after login" half lives in chat_screen's widget listen,
    // alongside its existing errorMessageProvider listen.)
    final gate = ref.read(authGateProvider).valueOrNull;
    if (authGateBlocked(gate)) return const [];
    try {
      return await ref.read(apiClientProvider).getMessages();
    } on DioException catch (e) {
      // A 401 despite a closed gate means the saved token went stale — the
      // gate's own probe is about to flip back to loginNeeded, which re-runs
      // this build via the watch above. No error UI of our own: the gate
      // overlay is the correct response, not an error message.
      if (e.response?.statusCode == 401) return const [];
      rethrow;
    }
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
  }

  Future<void> refresh() async {
    final myGeneration = _generation;
    final result = await AsyncValue.guard(
      () => ref.read(apiClientProvider).getMessages(),
    );
    // The await above can outlive this generation (chat switched away from —
    // see the BUG-H2 guards in sendMessage/sendFile, which are refresh()'s
    // main callers). Writing `state` after a rebuild would touch state that
    // no longer belongs to this call.
    if (_generation != myGeneration) return;
    state = result;
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
          '${L10n.t('error')}: Mesaj güncellenemedi (${FriendlyError.describeGeneric(e)})';
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
          '${L10n.t('error')}: Mesaj silinemedi (${FriendlyError.describeGeneric(e)})';
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
        ref.read(errorMessageProvider.notifier).state = 'Memory save failed: ${FriendlyError.describeGeneric(e)}';
      }
      return true;
    }
    if (trimmed == '/insight') {
      final ts = DateTime.now().toIso8601String().substring(11, 19);
      final current = state.valueOrNull ?? [];
      state = AsyncData([
        ...current,
        ChatMessage(role: 'user', content: trimmed, timestamp: ts),
      ]);
      try {
        final lang = L10n.locale == MemoLocale.en ? 'en' : 'tr';
        final insight = await api.generateSelfInsight(lang: lang);
        state = AsyncData([
          ...state.valueOrNull ?? current,
          ChatMessage(role: 'assistant', content: insight, timestamp: ts),
        ]);
      } catch (e) {
        ref.read(errorMessageProvider.notifier).state = 'Insight generation failed: ${FriendlyError.describeGeneric(e)}';
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
        ref.read(errorMessageProvider.notifier).state = 'Memory delete failed: ${FriendlyError.describeGeneric(e)}';
      }
      return true;
    }
    return false;
  }

  Future<void> sendMessage(String message) async {
    if (ref.read(isSendingProvider)) return;
    final myGeneration = _generation;
    // Claim the sending state synchronously, right after the guard check,
    // with no `await` in between — otherwise two sendMessage() calls fired
    // back-to-back (double Enter press, OS key-repeat while Enter is held,
    // a double click on the send button) can both read isSendingProvider as
    // false and both proceed, since the claim used to happen after
    // `await _handleMemoryCommand` below. That let two overlapping requests
    // race through, clobbering the shared _cancelToken field and appending
    // two user-message bubbles for what was a single send.
    ref.read(isSendingProvider.notifier).state = true;

    _stopped = false;
    _cancelToken = CancelToken();
    final api = ref.read(apiClientProvider);

    // Intercept /remember and /forget before sending to AI
    if (await _handleMemoryCommand(message, api)) {
      ref.read(isSendingProvider.notifier).state = false;
      return;
    }

    // Reset the token counter for this new turn.
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

      final activeChatId = ref.read(activeChatIdProvider).valueOrNull;
      final cliProvider = activeChatId != null
          ? await api.getChatCLIProvider(activeChatId)
          : '';

      if (cliProvider.isNotEmpty && activeChatId != null) {
        // CLI-backed chat: no fixed timeout (the job can run far longer than
        // a normal reply and is tracked server-side independent of this
        // connection), and no agent_event/usage/activity chunks — the CLI
        // does its own tool work opaquely, Memo never sees it.
        final stream = api.sendCLIMessageStream(
          activeChatId,
          message,
          cancelToken: _cancelToken,
        );
        await for (final chunk in stream) {
          fullReply += chunk.content;
          ref.read(streamingContentProvider.notifier).state = fullReply;
        }
        if (_generation != myGeneration) return;
        if (_stopped) {
          _stopped = false;
          ref.read(streamingContentProvider.notifier).state = '';
          await refresh();
          return;
        }
      } else if (streamingEnabled) {
        final stream = api.sendMessageStream(
          message,
          chatId: activeChatId,
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
            // Orchestra plan/specialist progress — no UI consumes this
            // anymore (the old right-side activity panel was removed), so
            // just drop it.
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

        // A chat switch mid-stream (ActiveChatIdNotifier.switchTo) calls
        // stopStreaming() then ref.invalidate(messagesProvider) — this
        // bumps _generation (whether or not Riverpod reuses this same
        // object), but Riverpod doesn't cancel the coroutine, so execution
        // reaches here regardless. Touching a shared provider like
        // isSendingProvider below would clobber whatever the newly-active
        // chat's own send is doing (BUG-H2). Bail before touching anything
        // once that's happened.
        if (_generation != myGeneration) return;

        if (_stopped) {
          _stopped = false;
          ref.read(streamingContentProvider.notifier).state = '';
          ref.read(streamingThinkingProvider.notifier).state = '';
          ref.read(streamingAgentEventsProvider.notifier).state = [];
          ref.read(streamingStatusProvider.notifier).state = '';
          await refresh();
          return;
        }
      } else {
        fullReply = await api.sendMessage(message);
      }

      if (_generation != myGeneration) return;

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
        if (_generation == myGeneration) ref.invalidate(chatListProvider);
      });
    } catch (e) {
      if (_generation != myGeneration) return;
      _stopped = false;
      ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: Mesaj gönderilemedi (${FriendlyError.describeGeneric(e)})';
      ref.read(streamingContentProvider.notifier).state = '';
      ref.read(streamingThinkingProvider.notifier).state = '';
      ref.read(streamingAgentEventsProvider.notifier).state = [];
      ref.read(streamingStatusProvider.notifier).state = '';
    } finally {
      _cancelToken = null;
      if (_generation == myGeneration) {
        ref.read(isSendingProvider.notifier).state = false;
      }
    }
  }

  Future<String> sendFile(String message, String filePath) async {
    if (ref.read(isSendingProvider)) return '';
    final myGeneration = _generation;

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
      List<AgentEvent> finalAgentEvents = [];

      if (streamingEnabled) {
        final stream = api.sendFileStream(
          message,
          filePath,
          cancelToken: _cancelToken,
        );

        final isAgentMode = ref.read(agentEnabledProvider);

        await for (final chunk in stream.timeout(
          const Duration(seconds: 300),
          onTimeout: (sink) => sink.addError(
            Exception(isAgentMode
                ? L10n.t('error_agent_timeout')
                : L10n.t('error_server_timeout')),
          ),
        )) {
          if (chunk.finishReason == 'status') {
            ref.read(streamingStatusProvider.notifier).state = chunk.content;
          } else if (chunk.finishReason == 'activity' || chunk.finishReason == 'usage') {
            try {
              final decoded = json.decode(chunk.content);
              if (decoded is Map<String, dynamic> && chunk.finishReason == 'usage') {
                ref.read(tokenUsageProvider.notifier).state =
                    TokenUsage.fromJson(decoded);
              }
            } catch (_) {}
          } else if (chunk.finishReason == 'agent_event') {
            try {
              final ev = AgentEvent.fromJson(json.decode(chunk.content));
              final currentEvents = [...ref.read(streamingAgentEventsProvider)];
              if (ev.type == 'tool_executing') {
                if (currentEvents.isNotEmpty && currentEvents.last.type == 'tool_executing') {
                  currentEvents[currentEvents.length - 1] = ev;
                } else {
                  currentEvents.add(ev);
                }
              } else if (ev.type == 'tool_result' || ev.type == 'tool_error' || ev.type == 'permission_denied') {
                if (currentEvents.isNotEmpty && currentEvents.last.type == 'tool_executing') {
                  currentEvents[currentEvents.length - 1] = ev;
                } else {
                  currentEvents.add(ev);
                }
              } else if (ev.type == 'permission_request') {
                currentEvents.add(ev);
                ref.read(agentEventBusProvider).emit(ev);
              }
              ref.read(streamingAgentEventsProvider.notifier).state = currentEvents;
              finalAgentEvents = currentEvents;
            } catch (_) {}
          } else {
            if (ref.read(streamingStatusProvider).isNotEmpty) {
              ref.read(streamingStatusProvider.notifier).state = '';
            }
            fullReply += chunk.content;
            fullThinking += chunk.thinking ?? '';
            ref.read(streamingContentProvider.notifier).state = fullReply;
            ref.read(streamingThinkingProvider.notifier).state = fullThinking;
          }
        }

        // See the matching guard + comment in sendMessage() (BUG-H2): this
        // call's generation may already be stale by the time execution
        // resumes here (chat switched away from mid-stream).
        if (_generation != myGeneration) return '';

        if (_stopped) {
          _stopped = false;
          ref.read(streamingContentProvider.notifier).state = '';
          ref.read(streamingThinkingProvider.notifier).state = '';
          ref.read(streamingAgentEventsProvider.notifier).state = [];
          ref.read(streamingStatusProvider.notifier).state = '';
          await refresh();
          return '';
        }
      } else {
        fullReply = await api.sendFile(message, filePath);
      }

      if (_generation != myGeneration) return '';

      final list = [...(state.valueOrNull ?? <ChatMessage>[])];
      if (fullReply.isNotEmpty || fullThinking.isNotEmpty || finalAgentEvents.isNotEmpty) {
        list.add(
          ChatMessage(
            role: 'assistant',
            content: fullReply,
            thinking: fullThinking.isNotEmpty ? fullThinking : null,
            timestamp: DateTime.now().toIso8601String().substring(11, 19),
            agentEvents: finalAgentEvents.isNotEmpty ? finalAgentEvents : null,
          ),
        );
      }
      state = AsyncData(list);

      ref.read(streamingContentProvider.notifier).state = '';
      ref.read(streamingThinkingProvider.notifier).state = '';
      ref.read(streamingAgentEventsProvider.notifier).state = [];
      ref.read(streamingStatusProvider.notifier).state = '';

      await refresh();
      ref.invalidate(chatListProvider);
      return fullReply;
    } catch (e) {
      if (_generation != myGeneration) return '';
      ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: Dosya gönderilemedi (${FriendlyError.describeGeneric(e)})';
      ref.read(streamingContentProvider.notifier).state = '';
      ref.read(streamingThinkingProvider.notifier).state = '';
      ref.read(streamingAgentEventsProvider.notifier).state = [];
      ref.read(streamingStatusProvider.notifier).state = '';
      return '';
    } finally {
      _cancelToken = null;
      if (_generation == myGeneration) {
        ref.read(isSendingProvider.notifier).state = false;
      }
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
      debugPrint('chat: incognito toggle error: ${FriendlyError.describeGeneric(e)}');
      state = previous;
      _ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: Gizli mod değiştirilemedi (${FriendlyError.describeGeneric(e)})';
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
  // Registers this GUI with the backend's client registry the first tick
  // it's reachable, then just heartbeats on every tick after — see
  // MemoApiClient.registerClient's doc comment for why. There's no
  // reliable "app is closing" hook on desktop today, so this loop simply
  // stopping (ref.onDispose above) is how the backend eventually notices
  // the GUI is gone: it prunes a client that misses a few heartbeats.
  String? clientId;
  while (alive) {
    // BUG-ONB4: while the gate is up, the backend is reachable (the gate's
    // own poll is talking to it) but won't accept our heartbeat — every
    // register/heartbeat would 401. Report "reachable" (the gate decides
    // what the user must do) and skip the registry traffic entirely.
    if (authGateBlocked(ref.read(authGateProvider).valueOrNull)) {
      yield true;
      clientId = null;
      await cancellablePause(ref, const Duration(seconds: 5));
      continue;
    }
    try {
      final ok = await api.isAlive();
      yield ok;
      if (!ok) {
        // Unreachable this tick — any previous registration is moot (the
        // backend may have restarted); register fresh once it's back
        // instead of heartbeating a possibly-stale ID.
        clientId = null;
      } else if (clientId == null) {
        clientId = await api.registerClient();
        if (clientId != null) {
          // Fresh registration = a new/reconnected session — the moment a
          // DST transition or timezone relocation since the last connect
          // should be picked up. See MemoApiClient.syncRoutineUtcOffset's
          // doc comment; unawaited on purpose, this must never delay/break
          // the connection-status loop itself.
          unawaited(api.syncRoutineUtcOffset());
        }
      } else {
        await api.heartbeatClient(clientId);
      }
    } catch (e) {
      debugPrint('chat: connectionStatus error: ${FriendlyError.describeGeneric(e)}');
      yield false;
      clientId = null;
    }
    if (!alive) break;
    await cancellablePause(ref, const Duration(seconds: 30));
  }
});
