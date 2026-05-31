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
        ChatListNotifier.new);

class ChatListNotifier extends AsyncNotifier<List<ChatSession>> {
  @override
  Future<List<ChatSession>> build() async {
    final api = ref.read(apiClientProvider);
    return api.listChats();
  }

  Future<void> refresh() async {
    state = const AsyncLoading();
    state = await AsyncValue.guard(
        () => ref.read(apiClientProvider).listChats());
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
        ActiveChatIdNotifier.new);

class ActiveChatIdNotifier extends AsyncNotifier<String> {
  @override
  Future<String> build() async {
    return ref.read(apiClientProvider).getActiveChatId();
  }

  Future<void> switchTo(String id) async {
    final api = ref.read(apiClientProvider);
    await api.switchChat(id);
    state = AsyncData(id);
    ref.invalidate(messagesProvider);
  }
}

// ─── Messages ───────────────────────────────────────────────────

final messagesProvider =
    AsyncNotifierProvider<MessagesNotifier, List<ChatMessage>>(
        MessagesNotifier.new);

class MessagesNotifier extends AsyncNotifier<List<ChatMessage>> {
  @override
  Future<List<ChatMessage>> build() async {
    return ref.read(apiClientProvider).getMessages();
  }

  Future<void> refresh() async {
    state = await AsyncValue.guard(
        () => ref.read(apiClientProvider).getMessages());
  }

  Future<void> sendMessage(String message) async {
    final api = ref.read(apiClientProvider);

    // Signal sending state
    ref.read(isSendingProvider.notifier).state = true;

    // Optimistically add user message
    final userMsg = ChatMessage(
      role: 'user',
      content: message,
      timestamp: DateTime.now().toIso8601String().substring(11, 16),
    );

    // Add empty assistant message for streaming
    final assistantMsg = ChatMessage(
      role: 'assistant',
      content: '',
      timestamp: DateTime.now().toIso8601String().substring(11, 16),
    );

    final current = state.valueOrNull ?? [];
    state = AsyncData([...current, userMsg, assistantMsg]);

    try {
      final stream = api.sendMessageStream(message);
      String fullReply = '';

      await for (final token in stream) {
        fullReply += token;

        // Update the last message in the list
        final updatedMessages = [...state.valueOrNull ?? <ChatMessage>[]];
        if (updatedMessages.isNotEmpty) {
          updatedMessages[updatedMessages.length - 1] = ChatMessage(
            role: 'assistant',
            content: fullReply,
            timestamp: assistantMsg.timestamp,
          );
          state = AsyncData(updatedMessages);
        }
      }

      // Final refresh to ensure everything is synced (metadata, IDs, etc)
      await refresh();
      // Also refresh chat list (titles may have changed)
      ref.invalidate(chatListProvider);
    } catch (e) {
      // Revert if error? For now just refresh
      await refresh();
    } finally {
      ref.read(isSendingProvider.notifier).state = false;
    }
  }

  Future<String> sendFile(String message, String filePath) async {
    final api = ref.read(apiClientProvider);

    ref.read(isSendingProvider.notifier).state = true;

    final fileName = filePath.split('/').last;
    final displayMsg = message.isEmpty ? '*(Dosya gönderildi: $fileName)*' : '$message\n*(Dosya: $fileName)*';

    final userMsg = ChatMessage(
      role: 'user',
      content: displayMsg,
      timestamp: DateTime.now().toIso8601String().substring(11, 16),
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

// ─── Incognito Mode ─────────────────────────────────────────────

final incognitoProvider = StateNotifierProvider<IncognitoNotifier, bool>(
    (ref) => IncognitoNotifier(ref));

class IncognitoNotifier extends StateNotifier<bool> {
  final Ref _ref;
  IncognitoNotifier(this._ref) : super(false);

  Future<void> toggle() async {
    state = !state;
    await _ref.read(apiClientProvider).toggleIncognito(state);
  }
}

// ─── Sending State ──────────────────────────────────────────────

/// True when a message is being sent and we're waiting for a reply.
final isSendingProvider = StateProvider<bool>((ref) => false);

// ─── Connection Status ──────────────────────────────────────────

final connectionStatusProvider = FutureProvider<bool>((ref) async {
  return ref.read(apiClientProvider).isAlive();
});
