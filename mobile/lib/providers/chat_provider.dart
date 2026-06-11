import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/api_client.dart';
import 'connection_provider.dart';

final chatProvider =
    StateNotifierProvider<ChatNotifier, ChatState>((ref) {
  final api = ref.read(apiClientProvider);
  return ChatNotifier(api);
});

class ChatState {
  final List<ChatSession> sessions;
  final String? activeSessionId;
  final List<ChatMessage> messages;
  final bool loading;
  final String? error;
  final bool streaming;
  final String currentStreamContent;

  const ChatState({
    this.sessions = const [],
    this.activeSessionId,
    this.messages = const [],
    this.loading = false,
    this.error,
    this.streaming = false,
    this.currentStreamContent = '',
  });

  ChatState copyWith({
    List<ChatSession>? sessions,
    String? activeSessionId,
    List<ChatMessage>? messages,
    bool? loading,
    String? error,
    bool? streaming,
    String? currentStreamContent,
  }) {
    return ChatState(
      sessions: sessions ?? this.sessions,
      activeSessionId: activeSessionId ?? this.activeSessionId,
      messages: messages ?? this.messages,
      loading: loading ?? this.loading,
      error: error,
      streaming: streaming ?? this.streaming,
      currentStreamContent: currentStreamContent ?? this.currentStreamContent,
    );
  }
}

class ChatNotifier extends StateNotifier<ChatState> {
  final MemoApiClient _api;
  CancelToken? _cancelToken;

  ChatNotifier(this._api) : super(const ChatState());

  Future<void> loadSessions() async {
    state = state.copyWith(loading: true, error: null);
    try {
      final sessions = await _api.listChats();
      state = state.copyWith(sessions: sessions, loading: false);

      if (sessions.isNotEmpty && state.activeSessionId == null) {
        await switchChat(sessions.first.id);
      }
    } catch (e) {
      state = state.copyWith(loading: false, error: e.toString());
    }
  }

  Future<void> switchChat(String id) async {
    state = state.copyWith(loading: true, error: null);
    try {
      await _api.switchChat(id);
      final messages = await _api.getMessages();
      state = state.copyWith(
        activeSessionId: id,
        messages: messages,
        loading: false,
      );
    } catch (e) {
      state = state.copyWith(loading: false, error: e.toString());
    }
  }

  Future<void> newChat() async {
    state = state.copyWith(loading: true, error: null);
    try {
      final id = await _api.newChat();
      await switchChat(id);
      await loadSessions();
    } catch (e) {
      state = state.copyWith(loading: false, error: e.toString());
    }
  }

  Future<void> deleteChat(String id) async {
    try {
      await _api.deleteChat(id);
      if (id == state.activeSessionId) {
        state = state.copyWith(activeSessionId: null, messages: []);
      }
      await loadSessions();
    } catch (e) {
      state = state.copyWith(error: e.toString());
    }
  }

  void sendMessage(String message) {
    _cancelToken = CancelToken();

    final userMsg = ChatMessage(
      role: 'user',
      content: message,
      timestamp: DateTime.now().toIso8601String(),
    );

    state = state.copyWith(
      messages: [...state.messages, userMsg],
      streaming: true,
      currentStreamContent: '',
      error: null,
    );

    _api
        .sendMessageStream(message, cancelToken: _cancelToken)
        .listen(
      (chunk) {
        state = state.copyWith(
          currentStreamContent: state.currentStreamContent + chunk.content,
        );
      },
      onError: (e) {
        final fullContent = state.currentStreamContent;
        if (fullContent.isNotEmpty) {
          final assistantMsg = ChatMessage(
            role: 'assistant',
            content: fullContent,
            timestamp: DateTime.now().toIso8601String(),
          );
          state = state.copyWith(
            messages: [...state.messages, assistantMsg],
            streaming: false,
            currentStreamContent: '',
          );
        } else {
          state = state.copyWith(
            streaming: false,
            currentStreamContent: '',
            error: e.toString(),
          );
        }
      },
      onDone: () {
        final fullContent = state.currentStreamContent;
        if (fullContent.isNotEmpty) {
          final assistantMsg = ChatMessage(
            role: 'assistant',
            content: fullContent,
            timestamp: DateTime.now().toIso8601String(),
          );
          state = state.copyWith(
            messages: [...state.messages, assistantMsg],
            streaming: false,
            currentStreamContent: '',
          );
        } else {
          state = state.copyWith(streaming: false, currentStreamContent: '');
        }
      },
      cancelOnError: false,
    );
  }

  void cancelStream() {
    _cancelToken?.cancel();
    final fullContent = state.currentStreamContent;
    if (fullContent.isNotEmpty) {
      final assistantMsg = ChatMessage(
        role: 'assistant',
        content: fullContent,
        timestamp: DateTime.now().toIso8601String(),
      );
      state = state.copyWith(
        messages: [...state.messages, assistantMsg],
        streaming: false,
        currentStreamContent: '',
      );
    } else {
      state = state.copyWith(streaming: false, currentStreamContent: '');
    }
  }

  void clearError() {
    state = state.copyWith(error: null);
  }
}
