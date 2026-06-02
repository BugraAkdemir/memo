import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/api_client.dart';
import '../models/chat.dart';

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
    // Clear any in-flight streaming state
    ref.read(streamingContentProvider.notifier).state = '';
    ref.read(streamingThinkingProvider.notifier).state = '';
    await api.switchChat(id);
    state = AsyncData(id);
    ref.invalidate(messagesProvider);
  }
}

// ─── Streaming Content ──────────────────────────────────────────

/// Holds the currently streaming assistant content (token by token).
/// Resets to '' when streaming ends or on error.
final streamingContentProvider = StateProvider<String>((ref) => '');

/// Holds the currently streaming thinking content (token by token).
final streamingThinkingProvider = StateProvider<String>((ref) => '');

// ─── Messages ───────────────────────────────────────────────────

final messagesProvider =
    AsyncNotifierProvider<MessagesNotifier, List<ChatMessage>>(
      MessagesNotifier.new,
    );

class MessagesNotifier extends AsyncNotifier<List<ChatMessage>> {
  @override
  Future<List<ChatMessage>> build() async {
    return ref.read(apiClientProvider).getMessages();
  }

  Future<void> refresh() async {
    state = await AsyncValue.guard(
      () => ref.read(apiClientProvider).getMessages(),
    );
  }

  Future<void> sendMessage(String message) async {
    if (ref.read(isSendingProvider)) return;

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
      final stream = api.sendMessageStream(message);
      String fullReply = '';
      String fullThinking = '';

      await for (final chunk in stream) {
        fullReply += chunk.content;
        fullThinking += chunk.thinking ?? '';

        // Update streaming providers instead of copying the entire message list
        ref.read(streamingContentProvider.notifier).state = fullReply;
        ref.read(streamingThinkingProvider.notifier).state = fullThinking;
      }

      // Append final assistant message to the list
      final list = [...(state.valueOrNull ?? <ChatMessage>[])];
      if (fullReply.isNotEmpty || fullThinking.isNotEmpty) {
        list.add(
          ChatMessage(
            role: 'assistant',
            content: fullReply,
            thinking: fullThinking.isNotEmpty ? fullThinking : null,
            timestamp: timestamp,
          ),
        );
      }
      state = AsyncData(list);

      // Clear streaming state before refresh to avoid stale UI
      ref.read(streamingContentProvider.notifier).state = '';
      ref.read(streamingThinkingProvider.notifier).state = '';

      // Refresh only chat metadata; refreshing messages here can race with the
      // backend session write and make the just-streamed reply disappear.
      ref.invalidate(chatListProvider);
    } catch (e) {
      ref.read(errorMessageProvider.notifier).state = e.toString();
      // Clear streaming state on error
      ref.read(streamingContentProvider.notifier).state = '';
      ref.read(streamingThinkingProvider.notifier).state = '';
      await refresh();
    } finally {
      ref.read(isSendingProvider.notifier).state = false;
    }
  }

  Future<String> sendFile(String message, String filePath) async {
    if (ref.read(isSendingProvider)) return '';

    final api = ref.read(apiClientProvider);

    ref.read(isSendingProvider.notifier).state = true;

    final fileName = filePath.split('/').last;
    final displayMsg = message.isEmpty
        ? '*(Dosya gönderildi: $fileName)*'
        : '$message\n*(Dosya: $fileName)*';

    final userMsg = ChatMessage(
      role: 'user',
      content: displayMsg,
      timestamp: DateTime.now().toIso8601String().substring(11, 19),
    );

    final current = state.valueOrNull ?? [];
    state = AsyncData([...current, userMsg]);

    try {
      final reply = await api.sendFile(message, filePath);
      await refresh();
      ref.invalidate(chatListProvider);
      return reply;
    } finally {
      ref.read(isSendingProvider.notifier).state = false;
    }
  }
}

// ─── Error Message ──────────────────────────────────────────────

/// Holds a one-shot error message to display as a snackbar.
/// Cleared after being read.
final errorMessageProvider = StateProvider<String>((ref) => '');

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
    } catch (_) {
      state = previous;
    }
  }
}

// ─── Sending State ──────────────────────────────────────────────

/// True when a message is being sent and we're waiting for a reply.
final isSendingProvider = StateProvider<bool>((ref) => false);

// ─── Connection Status ──────────────────────────────────────────

final connectionStatusProvider = FutureProvider<bool>((ref) async {
  return ref.read(apiClientProvider).isAlive();
});
