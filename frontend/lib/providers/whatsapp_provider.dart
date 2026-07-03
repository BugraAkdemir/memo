import 'dart:async';
import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../core/api_client.dart';
import '../core/l10n.dart';
import '../models/whatsapp.dart';
import 'chat_provider.dart';

final whatsAppStatusProvider = StateNotifierProvider.autoDispose<
    WhatsAppStatusNotifier, AsyncValue<WhatsAppStatus>>((ref) {
  final notifier = WhatsAppStatusNotifier(ref.read(apiClientProvider), ref);
  ref.onDispose(notifier.stopPolling);
  return notifier;
});

final whatsAppChatsProvider = FutureProvider<List<WhatsAppChatSummary>>((ref) async {
  final data = await ref.read(apiClientProvider).getWhatsAppChats();
  return data.whereType<Map<String, dynamic>>().map((e) => WhatsAppChatSummary.fromJson(e)).toList();
});

final whatsAppMessagesProvider =
    FutureProvider.family<List<WhatsAppMessage>, String>((ref, jid) async {
  final data = await ref.read(apiClientProvider).getWhatsAppMessages(jid);
  return data.whereType<Map<String, dynamic>>().map((e) => WhatsAppMessage.fromJson(e)).toList();
});

final whatsAppSearchProvider =
    FutureProvider.family<List<WhatsAppMessage>, String>((ref, query) async {
  final data = await ref.read(apiClientProvider).searchWhatsApp(query);
  return data.whereType<Map<String, dynamic>>().map((e) => WhatsAppMessage.fromJson(e)).toList();
});

final whatsAppChatModeProvider =
    StateNotifierProvider<WhatsAppChatModeNotifier, bool>(
        (ref) => WhatsAppChatModeNotifier(ref.read(apiClientProvider), ref));

class WhatsAppChatModeNotifier extends StateNotifier<bool> {
  final MemoApiClient _api;
  final Ref _ref;
  WhatsAppChatModeNotifier(this._api, this._ref) : super(false);

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
      _ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: WhatsApp sohbet modu değiştirilemedi ($e)';
    }
  }
}

class WhatsAppStatusNotifier extends StateNotifier<AsyncValue<WhatsAppStatus>> {
  final MemoApiClient _api;
  final Ref _ref;
  Timer? _pollTimer;

  WhatsAppStatusNotifier(this._api, this._ref) : super(const AsyncValue.loading()) {
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
      _ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: WhatsApp bağlantısı başlatılamadı ($e)';
    }
  }

  Future<void> disconnect() async {
    try {
      await _api.stopWhatsApp();
      await _fetch();
    } catch (e) {
      if (mounted) state = AsyncValue.error(e, StackTrace.current);
      _ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: WhatsApp bağlantısı kesilemedi ($e)';
    }
  }

  Future<void> logout() async {
    try {
      await _api.logoutWhatsApp();
      await _fetch();
    } catch (e) {
      if (mounted) state = AsyncValue.error(e, StackTrace.current);
      _ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: WhatsApp oturumu kapatılamadı ($e)';
    }
  }

  @override
  void dispose() {
    stopPolling();
    super.dispose();
  }
}
