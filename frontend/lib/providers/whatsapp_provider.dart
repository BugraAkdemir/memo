import 'dart:async';
import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../core/api_client.dart';
import '../models/whatsapp.dart';
import 'chat_provider.dart';

final whatsAppStatusProvider =
    StateNotifierProvider<WhatsAppStatusNotifier, AsyncValue<WhatsAppStatus>>(
        (ref) => WhatsAppStatusNotifier(ref.read(apiClientProvider)));

final whatsAppChatsProvider = FutureProvider<List<WhatsAppChatSummary>>((ref) async {
  final data = await ref.read(apiClientProvider).getWhatsAppChats();
  return data.map((e) => WhatsAppChatSummary.fromJson(e as Map<String, dynamic>)).toList();
});

final whatsAppMessagesProvider =
    FutureProvider.family<List<WhatsAppMessage>, String>((ref, jid) async {
  final data = await ref.read(apiClientProvider).getWhatsAppMessages(jid);
  return data.map((e) => WhatsAppMessage.fromJson(e as Map<String, dynamic>)).toList();
});

final whatsAppSearchProvider =
    FutureProvider.family<List<WhatsAppMessage>, String>((ref, query) async {
  final data = await ref.read(apiClientProvider).searchWhatsApp(query);
  return data.map((e) => WhatsAppMessage.fromJson(e as Map<String, dynamic>)).toList();
});

final whatsAppChatModeProvider =
    StateNotifierProvider<WhatsAppChatModeNotifier, bool>(
        (ref) => WhatsAppChatModeNotifier(ref.read(apiClientProvider)));

class WhatsAppChatModeNotifier extends StateNotifier<bool> {
  final MemoApiClient _api;
  WhatsAppChatModeNotifier(this._api) : super(false);

  Future<void> init() async {
    try {
      state = await _api.getWhatsAppChatMode();
    } catch (e) {
      debugPrint('whatsapp: chat mode init error: $e');
    }
  }

  Future<void> toggle() async {
    final next = !state;
    try {
      await _api.setWhatsAppChatMode(next);
      state = next;
    } catch (e) {
      debugPrint('whatsapp: toggle error: $e');
    }
  }
}

class WhatsAppStatusNotifier extends StateNotifier<AsyncValue<WhatsAppStatus>> {
  final MemoApiClient _api;
  Timer? _pollTimer;

  WhatsAppStatusNotifier(this._api) : super(const AsyncValue.loading()) {
    _fetch();
  }

  Future<void> _fetch() async {
    try {
      final data = await _api.getWhatsAppStatus();
      if (mounted) state = AsyncValue.data(WhatsAppStatus.fromJson(data));
    } catch (e) {
      if (mounted) state = AsyncValue.error(e, StackTrace.current);
    }
  }

  /// Refresh and reschedule poll at the interval appropriate for the current state.
  Future<void> refresh() => _fetch();

  /// Start adaptive polling: fast when waiting for QR scan, slow when connected.
  void startPolling() {
    _pollTimer?.cancel();
    _schedule();
  }

  void _schedule() {
    final status = state.valueOrNull;
    Duration interval;
    if (status == null || !status.initialized) {
      // Not yet started — check every 3s in case backend auto-starts.
      interval = const Duration(seconds: 3);
    } else if (!status.loggedIn) {
      // Waiting for QR to arrive OR QR displayed — poll fast either way.
      interval = const Duration(seconds: 2);
    } else if (!status.connected) {
      // Logged in but disconnected — reconnect in progress.
      interval = const Duration(seconds: 4);
    } else {
      // Connected and logged in — heartbeat only.
      interval = const Duration(seconds: 15);
    }

    _pollTimer = Timer(interval, () async {
      await _fetch();
      if (mounted) _schedule();
    });
  }

  void stopPolling() {
    _pollTimer?.cancel();
    _pollTimer = null;
  }

  Future<void> connect() async {
    try {
      final data = await _api.startWhatsApp();
      if (mounted) {
        state = AsyncValue.data(WhatsAppStatus.fromJson(data));
        startPolling();
      }
    } catch (e) {
      if (mounted) state = AsyncValue.error(e, StackTrace.current);
    }
  }

  Future<void> disconnect() async {
    try {
      await _api.stopWhatsApp();
      await _fetch();
    } catch (e) {
      if (mounted) state = AsyncValue.error(e, StackTrace.current);
    }
  }

  Future<void> logout() async {
    try {
      await _api.logoutWhatsApp();
      await _fetch();
    } catch (e) {
      if (mounted) state = AsyncValue.error(e, StackTrace.current);
    }
  }

  @override
  void dispose() {
    stopPolling();
    super.dispose();
  }
}
