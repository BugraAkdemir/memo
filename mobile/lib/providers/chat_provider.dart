import 'dart:async';
import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/api_client.dart';
import 'connection_provider.dart';

final chatProvider =
    StateNotifierProvider<ChatNotifier, ChatState>((ref) {
  final api = ref.read(apiClientProvider);
  return ChatNotifier(api);
});

final agentEnabledProvider = StateNotifierProvider<AgentEnabledNotifier, bool>((ref) {
  return AgentEnabledNotifier(ref.read(apiClientProvider));
});

class AgentEnabledNotifier extends StateNotifier<bool> {
  final MemoApiClient _api;

  AgentEnabledNotifier(this._api) : super(false);

  Future<void> load() async {
    try {
      final enabled = await _api.getAgentEnabled();
      state = enabled;
    } catch (_) {}
  }

  Future<void> toggle() async {
    final next = !state;
    try {
      await _api.setAgentEnabled(next);
      state = next;
    } catch (_) {}
  }
}

final incognitoProvider = StateNotifierProvider<IncognitoNotifier, bool>((ref) {
  return IncognitoNotifier(ref.read(apiClientProvider));
});

class IncognitoNotifier extends StateNotifier<bool> {
  final MemoApiClient _api;

  IncognitoNotifier(this._api) : super(false);

  Future<void> load() async {
    try {
      final enabled = await _api.getIncognito();
      state = enabled;
    } catch (_) {}
  }

  Future<void> toggle() async {
    final next = !state;
    try {
      await _api.toggleIncognito(next);
      state = next;
    } catch (_) {}
  }
}

class ChatState {
  final List<ChatSession> sessions;
  final String? activeSessionId;
  final List<ChatMessage> messages;
  final bool loading;
  final String? error;
  final bool streaming;
  final String currentStreamContent;
  final List<AgentEvent> agentEvents;
  final bool agentEnabled;

  const ChatState({
    this.sessions = const [],
    this.activeSessionId,
    this.messages = const [],
    this.loading = false,
    this.error,
    this.streaming = false,
    this.currentStreamContent = '',
    this.agentEvents = const [],
    this.agentEnabled = false,
  });

  ChatState copyWith({
    List<ChatSession>? sessions,
    String? activeSessionId,
    List<ChatMessage>? messages,
    bool? loading,
    String? error,
    bool? streaming,
    String? currentStreamContent,
    List<AgentEvent>? agentEvents,
    bool? agentEnabled,
  }) {
    return ChatState(
      sessions: sessions ?? this.sessions,
      activeSessionId: activeSessionId ?? this.activeSessionId,
      messages: messages ?? this.messages,
      loading: loading ?? this.loading,
      error: error,
      streaming: streaming ?? this.streaming,
      currentStreamContent: currentStreamContent ?? this.currentStreamContent,
      agentEvents: agentEvents ?? this.agentEvents,
      agentEnabled: agentEnabled ?? this.agentEnabled,
    );
  }
}

class ChatNotifier extends StateNotifier<ChatState> {
  final MemoApiClient _api;
  CancelToken? _cancelToken;
  StreamSubscription? _streamSubscription;
  // Monotonic stream generation: each sendMessage/cancel bumps it so a
  // stale onDone/onError from an abandoned stream cannot clobber the live
  // one (BUG-M1; mirrors desktop MessagesNotifier._generation).
  int _streamGeneration = 0;

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
    // BUG-M1: refuse overlapping sends — previously two taps claimed two
    // streams, overwrote _cancelToken without cancelling the first, and
    // appended two user bubbles for one intent.
    if (state.streaming) return;
    final text = message.trim();
    if (text.isEmpty) return;

    _cancelToken?.cancel();
    _streamSubscription?.cancel();
    _cancelToken = CancelToken();
    final myGeneration = ++_streamGeneration;

    final userMsg = ChatMessage(
      role: 'user',
      content: text,
      timestamp: DateTime.now().toIso8601String(),
    );

    state = state.copyWith(
      messages: [...state.messages, userMsg],
      streaming: true,
      currentStreamContent: '',
      agentEvents: [],
      error: null,
    );

    _streamSubscription = _api
        .sendMessageStream(text, cancelToken: _cancelToken)
        .listen(
      (chunk) {
        if (_streamGeneration != myGeneration) return;
        if (chunk.isAgentEvent) {
          // Parse agent event from JSON content
          try {
            final ev = AgentEvent.fromJson(json.decode(chunk.content));
            final currentEvents = [...state.agentEvents];

            // Replace last executing event with this one to avoid stacking
            if (ev.isToolExecuting) {
              if (currentEvents.isNotEmpty && currentEvents.last.isToolExecuting) {
                currentEvents[currentEvents.length - 1] = ev;
              } else {
                currentEvents.add(ev);
              }
            } else if (ev.isToolResult || ev.isToolError) {
              if (currentEvents.isNotEmpty && currentEvents.last.isToolExecuting) {
                currentEvents[currentEvents.length - 1] = ev;
              } else {
                currentEvents.add(ev);
              }
            } else if (ev.isPermissionRequest) {
              currentEvents.add(ev);
              // Auto-show permission dialog via callback
              _onPermissionRequest?.call(ev);
            }

            state = state.copyWith(agentEvents: currentEvents);
          } catch (_) {}
        } else {
          state = state.copyWith(
            currentStreamContent: state.currentStreamContent + chunk.content,
          );
        }
      },
      onError: (e) {
        if (_streamGeneration != myGeneration) return;
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
            agentEvents: [],
          );
        } else {
          state = state.copyWith(
            streaming: false,
            currentStreamContent: '',
            agentEvents: [],
            error: e.toString(),
          );
        }
      },
      onDone: () {
        if (_streamGeneration != myGeneration) return;
        final fullContent = state.currentStreamContent;
        if (fullContent.isNotEmpty || state.agentEvents.isNotEmpty) {
          final assistantMsg = ChatMessage(
            role: 'assistant',
            content: fullContent,
            timestamp: DateTime.now().toIso8601String(),
          );
          state = state.copyWith(
            messages: [...state.messages, assistantMsg],
            streaming: false,
            currentStreamContent: '',
            agentEvents: [],
          );
        } else {
          state = state.copyWith(streaming: false, currentStreamContent: '', agentEvents: []);
        }
      },
      cancelOnError: false,
    );
  }

  // Callback for permission requests — set by chat screen for showing dialog.
  void Function(AgentEvent)? _onPermissionRequest;

  void setPermissionHandler(void Function(AgentEvent) handler) {
    _onPermissionRequest = handler;
  }

  void cancelStream() {
    _streamGeneration++;
    _cancelToken?.cancel();
    _streamSubscription?.cancel();
    _streamSubscription = null;
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
        agentEvents: [],
      );
    } else {
      state = state.copyWith(streaming: false, currentStreamContent: '', agentEvents: []);
    }
  }

  @override
  void dispose() {
    _streamGeneration++;
    _streamSubscription?.cancel();
    _cancelToken?.cancel();
    super.dispose();
  }

  void clearError() {
    state = state.copyWith(error: null);
  }
}
