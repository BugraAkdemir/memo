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

  Future<String> sendMessage(String message) async {
    final api = ref.read(apiClientProvider);

    // Optimistically add user message
    final userMsg = ChatMessage(
      role: 'user',
      content: message,
      timestamp: DateTime.now().toIso8601String().substring(11, 16),
    );

    final current = state.valueOrNull ?? [];
    state = AsyncData([...current, userMsg]);

    // Send and get reply
    final reply = await api.sendMessage(message);

    // Refresh full message list from backend
    await refresh();
    // Also refresh chat list (titles may have changed)
    ref.invalidate(chatListProvider);

    return reply;
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
