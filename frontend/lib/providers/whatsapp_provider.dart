import 'dart:async';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../core/api_client.dart';
import '../models/whatsapp.dart';
import 'chat_provider.dart';

final whatsAppStatusProvider = StateNotifierProvider<WhatsAppStatusNotifier, AsyncValue<WhatsAppStatus>>((ref) {
  return WhatsAppStatusNotifier(ref.read(apiClientProvider));
});

final whatsAppChatsProvider = FutureProvider<List<WhatsAppChatSummary>>((ref) async {
  final api = ref.read(apiClientProvider);
  final raw = await api.getWhatsAppChats();
  final data = raw ?? <dynamic>[];
  return data.map((e) => WhatsAppChatSummary.fromJson(e as Map<String, dynamic>)).toList();
});

final whatsAppMessagesProvider = FutureProvider.family<List<WhatsAppMessage>, String>((ref, jid) async {
  final api = ref.read(apiClientProvider);
  final raw = await api.getWhatsAppMessages(jid);
  final data = raw ?? <dynamic>[];
  return data.map((e) => WhatsAppMessage.fromJson(e as Map<String, dynamic>)).toList();
});

final whatsAppSearchProvider = FutureProvider.family<List<WhatsAppMessage>, String>((ref, query) async {
  final api = ref.read(apiClientProvider);
  final raw = await api.searchWhatsApp(query);
  final data = raw ?? <dynamic>[];
  return data.map((e) => WhatsAppMessage.fromJson(e as Map<String, dynamic>)).toList();
});

// WhatsApp chat mode toggle — separate from Agent mode
final whatsAppChatModeProvider = StateNotifierProvider<WhatsAppChatModeNotifier, bool>((ref) {
  return WhatsAppChatModeNotifier(ref.read(apiClientProvider));
});

class WhatsAppChatModeNotifier extends StateNotifier<bool> {
  final MemoApiClient _api;

  WhatsAppChatModeNotifier(this._api) : super(false);

  Future<void> init() async {
    try {
      final enabled = await _api.getWhatsAppChatMode();
      state = enabled;
    } catch (_) {}
  }

  Future<void> toggle() async {
    final next = !state;
    try {
      await _api.setWhatsAppChatMode(next);
      state = next;
    } catch (_) {}
  }
}

class WhatsAppStatusNotifier extends StateNotifier<AsyncValue<WhatsAppStatus>> {
  final MemoApiClient _api;
  Timer? _pollTimer;

  WhatsAppStatusNotifier(this._api) : super(const AsyncValue.loading()) {
    refresh();
  }

  void refresh() {
    _api.getWhatsAppStatus().then((data) {
      state = AsyncValue.data(WhatsAppStatus.fromJson(data));
    }).catchError((e) {
      state = AsyncValue.error(e, StackTrace.current);
    });
  }

  void startPolling() {
    _pollTimer?.cancel();
    _pollTimer = Timer.periodic(const Duration(seconds: 3), (_) => refresh());
  }

  void stopPolling() {
    _pollTimer?.cancel();
    _pollTimer = null;
  }

  Future<void> connect() async {
    state = const AsyncValue.loading();
    try {
      final data = await _api.startWhatsApp();
      state = AsyncValue.data(WhatsAppStatus.fromJson(data));
      startPolling();
    } catch (e) {
      state = AsyncValue.error(e, StackTrace.current);
    }
  }

  Future<void> disconnect() async {
    try {
      await _api.stopWhatsApp();
      stopPolling();
      refresh();
    } catch (e) {
      state = AsyncValue.error(e, StackTrace.current);
    }
  }

  @override
  void dispose() {
    stopPolling();
    super.dispose();
  }
}
