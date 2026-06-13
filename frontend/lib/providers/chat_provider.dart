import 'dart:async';
import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/api_client.dart';
import '../models/agent.dart';
import '../models/chat.dart';
import 'agent_provider.dart';
import 'settings_provider.dart';

/// Global API client instance.
final apiClientProvider = Provider<MemoApiClient>((ref) {
  return MemoApiClient();
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

// ─── Messages ───────────────────────────────────────────────────

final messagesProvider =
    AsyncNotifierProvider<MessagesNotifier, List<ChatMessage>>(
      MessagesNotifier.new,
    );

class MessagesNotifier extends AsyncNotifier<List<ChatMessage>> {
  CancelToken? _cancelToken;
  bool _stopped = false;
  Timer? _delayedRefreshTimer;

  @override
  Future<List<ChatMessage>> build() async {
    ref.onDispose(() => _delayedRefreshTimer?.cancel());
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
      ref.read(errorMessageProvider.notifier).state = e.toString();
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
      ref.read(errorMessageProvider.notifier).state = e.toString();
    }
  }

  Future<void> sendMessage(String message) async {
    if (ref.read(isSendingProvider)) return;

    _stopped = false;
    _cancelToken = CancelToken();
    final api = ref.read(apiClientProvider);

    // Signal sending state
    ref.read(isSendingProvider.notifier).state = true;

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

        await for (final chunk in stream.timeout(
          const Duration(seconds: 60),
          onTimeout: (sink) => sink.addError(
            Exception('Sunucu yanıt vermiyor (60s zaman aşımı)'),
          ),
        )) {
          if (chunk.finishReason == 'agent_event') {
            try {
              final ev = AgentEvent.fromJson(json.decode(chunk.content));
              final currentEvents = [...ref.read(streamingAgentEventsProvider)];

              // Handle update or add logic (ToolExecuting vs ToolResult vs ToolError)
              final existingIdx = currentEvents.lastIndexWhere(
                (e) =>
                    e.toolName == ev.toolName &&
                    e.type != 'permission_request' &&
                    ev.type != 'permission_request',
              );
              if (existingIdx != -1 &&
                  (ev.type == 'tool_result' || ev.type == 'tool_error')) {
                // Replace executing state with result
                currentEvents[existingIdx] = ev;
              } else {
                currentEvents.add(ev);
              }

              ref.read(streamingAgentEventsProvider.notifier).state =
                  currentEvents;
              finalAgentEvents = currentEvents;

              if (ev.type == 'permission_request') {
                ref.read(agentEventBusProvider).emit(ev);
              }
            } catch (e) {
              // ignore parse errors
            }
          } else {
            fullReply += chunk.content;
            fullThinking += chunk.thinking ?? '';

            // Update streaming providers instead of copying the entire message list
            ref.read(streamingContentProvider.notifier).state = fullReply;
            ref.read(streamingThinkingProvider.notifier).state = fullThinking;
          }
        }

        if (_stopped) {
          _stopped = false;
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

      // Refresh chat metadata — wait a beat so async title generation finishes
      ref.invalidate(chatListProvider);
      _delayedRefreshTimer?.cancel();
      _delayedRefreshTimer = Timer(const Duration(seconds: 2), () {
        ref.invalidate(chatListProvider);
      });
    } catch (e) {
      _stopped = false;
      ref.read(errorMessageProvider.notifier).state = e.toString();
      ref.read(streamingContentProvider.notifier).state = '';
      ref.read(streamingThinkingProvider.notifier).state = '';
      ref.read(streamingAgentEventsProvider.notifier).state = [];
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

    final fileName = filePath.split('/').last;
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

        await for (final chunk in stream.timeout(
          const Duration(seconds: 60),
          onTimeout: (sink) => sink.addError(
            Exception('Sunucu yanıt vermiyor (60s zaman aşımı)'),
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
      ref.read(errorMessageProvider.notifier).state = e.toString();
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
    }
  }
}

// ─── Sending State ──────────────────────────────────────────────

/// True when a message is being sent and we're waiting for a reply.
final isSendingProvider = StateProvider<bool>((ref) => false);

// ─── Connection Status (polls every 30s) ────────────────────────

final connectionStatusProvider = StreamProvider<bool>((ref) async* {
  final api = ref.read(apiClientProvider);
  while (true) {
    try {
      yield await api.isAlive();
    } catch (e) {
      debugPrint('chat: connectionStatus error: $e');
      yield false;
    }
    await Future.delayed(const Duration(seconds: 30));
  }
});
