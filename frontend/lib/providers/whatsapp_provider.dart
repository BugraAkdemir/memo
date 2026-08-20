import 'dart:async';
import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../core/api_client.dart';
import '../core/l10n.dart';
import '../models/whatsapp.dart';
import 'chat_provider.dart';
import 'auth_gate_provider.dart';
import 'gate_guard.dart';
import '../core/friendly_error.dart';

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
      debugPrint('whatsapp: chat mode init error: ${FriendlyError.describeGeneric(e)}');
    }
  }

  Future<void> toggle() async {
    final next = !state;
    try {
      await _api.setWhatsAppChatMode(next);
      state = next;
    } catch (e) {
      debugPrint('whatsapp: toggle error: ${FriendlyError.describeGeneric(e)}');
      _ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: WhatsApp sohbet modu değiştirilemedi (${FriendlyError.describeGeneric(e)})';
    }
  }
}

// ─── Self-Chat Assistant ───────────────────────────────────────
//
// Whether Memo auto-replies to messages in the user's own WhatsApp
// "Message Yourself" chat — a persisted (config.yaml) setting, distinct
// from WhatsAppChatModeNotifier above (that one is ephemeral and means
// "chat with Memo about your WhatsApp from inside Memo's own UI"; this one
// means "your WhatsApp self-chat becomes another way to talk to Memo").
// Follows the same AsyncNotifierProvider + auth-gate-guard + optimistic-
// toggle shape as memoryEnabledProvider (settings_provider.dart), since
// it's the same kind of thing: a real config value that must be fetched on
// load, not just an in-memory flag that always starts at a known default.

final whatsAppSelfChatAssistantProvider =
    AsyncNotifierProvider<WhatsAppSelfChatAssistantNotifier, bool>(
      WhatsAppSelfChatAssistantNotifier.new,
    );

class WhatsAppSelfChatAssistantNotifier extends AsyncNotifier<bool> {
  // See MemoryEnabledNotifier's identical guard for why this exists: without
  // it, two toggle() calls fired before the first resolves both negate the
  // same stale starting value and land on the same end state instead of
  // toggling twice.
  bool _toggling = false;

  @override
  Future<bool> build() async {
    if (authGateBlocked(ref.read(authGateProvider).valueOrNull)) return false;
    return ref.read(apiClientProvider).getWhatsAppSelfChatAssistant();
  }

  Future<void> toggle() async {
    if (_toggling) return;
    _toggling = true;
    final current = state.valueOrNull ?? false;
    final next = !current;
    state = AsyncData(next);
    try {
      await ref.read(apiClientProvider).setWhatsAppSelfChatAssistant(next);
    } catch (e) {
      state = AsyncData(current);
      ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: ${L10n.t('whatsapp_self_chat_assistant_toggle_failed')} (${FriendlyError.describeGeneric(e)})';
    } finally {
      _toggling = false;
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
    // BUG-ONB4: the WhatsApp screen stays mounted under the gate overlay
    // (IndexedStack) and its poll kept 401-ing the token-gated backend every
    // 2-15s. Skip the request while the gate is up; the adaptive schedule
    // keeps checking back until it opens.
    if (authGateBlocked(_ref.read(authGateProvider).valueOrNull)) return;
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
      try {
        await _fetch();
      } catch (e) {
        // _fetch already catches its own network/API errors — this only
        // catches the rare case where notifying a listener throws (e.g. a
        // widget's Element already defunct from a fast navigate-in/
        // navigate-out, seen live: "Cannot use 'ref' after the widget was
        // disposed" / Element.markNeedsBuild on a defunct Element). Without
        // this, that exception propagates out of this Timer callback and
        // permanently kills the recursive _schedule() chain below — no
        // crash report, just polling silently stopping forever and the UI
        // stuck wherever it last was (reported live as "Preparing QR
        // code..." never resolving).
        debugPrint('whatsapp: status poll listener error (ignored): $e');
      }
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
          '${L10n.t('error')}: WhatsApp bağlantısı başlatılamadı (${FriendlyError.describeGeneric(e)})';
    }
  }

  Future<void> disconnect() async {
    try {
      await _api.stopWhatsApp();
      await _fetch();
    } catch (e) {
      if (mounted) state = AsyncValue.error(e, StackTrace.current);
      _ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: WhatsApp bağlantısı kesilemedi (${FriendlyError.describeGeneric(e)})';
    }
  }

  Future<void> logout() async {
    try {
      await _api.logoutWhatsApp();
      await _fetch();
    } catch (e) {
      if (mounted) state = AsyncValue.error(e, StackTrace.current);
      _ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: WhatsApp oturumu kapatılamadı (${FriendlyError.describeGeneric(e)})';
    }
  }

  @override
  void dispose() {
    stopPolling();
    super.dispose();
  }
}
