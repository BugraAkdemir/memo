import 'dart:async';
import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../core/api_client.dart';
import '../core/l10n.dart';
import '../models/telegram.dart';
import 'chat_provider.dart';
import 'auth_gate_provider.dart';
import 'gate_guard.dart';
import '../core/friendly_error.dart';

/// Telegram bot connection state — same StateNotifier + adaptive-polling
/// shape as WhatsAppStatusNotifier (whatsapp_provider.dart), simplified for
/// a surface with no QR pairing step: "connect" here means saving a bot
/// token from @BotFather, not scanning anything.
final telegramStatusProvider = StateNotifierProvider.autoDispose<
    TelegramStatusNotifier, AsyncValue<TelegramStatus>>((ref) {
  final notifier = TelegramStatusNotifier(ref.read(apiClientProvider), ref);
  ref.onDispose(notifier.stopPolling);
  return notifier;
});

class TelegramStatusNotifier extends StateNotifier<AsyncValue<TelegramStatus>> {
  final MemoApiClient _api;
  final Ref _ref;
  Timer? _pollTimer;

  TelegramStatusNotifier(this._api, this._ref) : super(const AsyncValue.loading()) {
    _fetch();
  }

  Future<void> _fetch() async {
    // Same BUG-ONB4 guard as WhatsAppStatusNotifier: don't poll a
    // token-gated backend while the auth gate is still up.
    if (authGateBlocked(_ref.read(authGateProvider).valueOrNull)) return;
    try {
      final data = await _api.getTelegramStatus();
      if (mounted) state = AsyncValue.data(TelegramStatus.fromJson(data));
    } catch (e) {
      if (mounted) state = AsyncValue.error(e, StackTrace.current);
    }
  }

  Future<void> refresh() => _fetch();

  /// Start adaptive polling: faster while waiting for the owner's first
  /// message to link, slow heartbeat once fully connected.
  void startPolling() {
    _pollTimer?.cancel();
    _schedule();
  }

  void _schedule() {
    final status = state.valueOrNull;
    Duration interval;
    if (status == null || !status.configured) {
      interval = const Duration(seconds: 5);
    } else if (status.reconnecting) {
      interval = const Duration(seconds: 4);
    } else if (!status.ownerLinked) {
      interval = const Duration(seconds: 3);
    } else {
      interval = const Duration(seconds: 15);
    }

    _pollTimer = Timer(interval, () async {
      try {
        await _fetch();
      } catch (e) {
        // Same defensive catch as WhatsAppStatusNotifier._schedule: a
        // listener throwing on a since-defunct widget must not kill the
        // recursive polling chain.
        debugPrint('telegram: status poll listener error (ignored): $e');
      }
      if (mounted) _schedule();
    });
  }

  void stopPolling() {
    _pollTimer?.cancel();
    _pollTimer = null;
  }

  Future<void> connect(String botToken) async {
    try {
      final data = await _api.connectTelegram(botToken);
      if (mounted) {
        state = AsyncValue.data(TelegramStatus.fromJson(data));
        startPolling();
      }
    } catch (e) {
      _ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: ${L10n.t('telegram_connect_failed')} (${FriendlyError.describeGeneric(e)})';
    }
  }

  /// Re-establish the connection with the token already on the backend —
  /// for the dead-state "reconnect" button (the plain "retry" only re-polls
  /// status).
  Future<void> reconnect() async {
    try {
      final data = await _api.reconnectTelegram();
      if (mounted) {
        state = AsyncValue.data(TelegramStatus.fromJson(data));
        startPolling();
      }
    } catch (e) {
      _ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: ${L10n.t('telegram_connect_failed')} (${FriendlyError.describeGeneric(e)})';
    }
  }

  Future<void> stop() async {
    try {
      await _api.stopTelegram();
      await _fetch();
    } catch (e) {
      if (mounted) state = AsyncValue.error(e, StackTrace.current);
      _ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: ${L10n.t('telegram_stop_failed')} (${FriendlyError.describeGeneric(e)})';
    }
  }

  Future<void> disconnect() async {
    try {
      await _api.disconnectTelegram();
      await _fetch();
    } catch (e) {
      if (mounted) state = AsyncValue.error(e, StackTrace.current);
      _ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: ${L10n.t('telegram_disconnect_failed')} (${FriendlyError.describeGeneric(e)})';
    }
  }

  @override
  void dispose() {
    stopPolling();
    super.dispose();
  }
}
