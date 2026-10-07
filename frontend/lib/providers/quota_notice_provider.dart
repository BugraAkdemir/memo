import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../models/quota_notice.dart';
import 'chat_provider.dart';
import 'settings_provider.dart';

/// Whether Memo continues a chat by itself once an exhausted allowance has
/// refilled. On by default: a long coding session that hit the limit should not
/// sit stopped until someone notices. Device preference, not tied to a backend.
final quotaAutoContinueProvider = StateNotifierProvider<QuotaAutoContinueNotifier, bool>(
  (ref) => QuotaAutoContinueNotifier(ref.read(prefsProvider)),
);

class QuotaAutoContinueNotifier extends StateNotifier<bool> {
  static const prefKey = 'memo_quota_auto_continue';
  final SharedPreferences _prefs;

  QuotaAutoContinueNotifier(this._prefs) : super(_prefs.getBool(prefKey) ?? true);

  Future<void> set(bool value) async {
    state = value;
    await _prefs.setBool(prefKey, value);
  }
}

/// The quota notice each chat is currently showing (one per chat), and the
/// timer that types "continue" into a chat once its allowance is back.
final quotaNoticeProvider = StateNotifierProvider<QuotaNoticeNotifier, Map<String, QuotaNotice>>(
  (ref) => QuotaNoticeNotifier(
    now: DateTime.now,
    activeChatId: () => ref.read(activeChatIdProvider).valueOrNull ?? '',
    isSending: () => ref.read(isSendingProvider),
    autoEnabled: () => ref.read(quotaAutoContinueProvider),
    send: (message) => ref.read(messagesProvider.notifier).sendMessage(message),
  ),
);

class QuotaNoticeNotifier extends StateNotifier<Map<String, QuotaNotice>> {
  QuotaNoticeNotifier({
    required this.now,
    required this.activeChatId,
    required this.isSending,
    required this.autoEnabled,
    required this.send,
    this.tickEvery = const Duration(seconds: 1),
  }) : super(const {});

  final DateTime Function() now;
  final String Function() activeChatId;
  final bool Function() isSending;
  final bool Function() autoEnabled;
  final Future<void> Function(String message) send;
  final Duration tickEvery;

  Timer? _timer;

  // Last automatic "continue" per chat: lets a refusal that follows straight
  // after count as the next attempt (longer fallback wait), instead of starting
  // over at five minutes every time.
  final Map<String, ({DateTime at, int attempt})> _lastAuto = {};

  /// Records what a marker chunk said for [chatId]. Malformed content is ignored.
  void showFromMarker(String chatId, String content) {
    if (chatId.isEmpty) return;
    final n = QuotaNotice.tryParse(content, now: now());
    if (n != null) show(chatId, n);
  }

  void show(String chatId, QuotaNotice notice) {
    var n = notice;
    final prev = _lastAuto[chatId];
    if (n.exhausted && prev != null && now().difference(prev.at) < const Duration(minutes: 10)) {
      n = n.withAttempt(prev.attempt + 1);
    }
    state = {...state, chatId: n};
    _syncTimer();
  }

  /// Drops the notice for [chatId] — the user dismissed it, or sent something
  /// themselves, so the conversation moved on.
  void dismiss(String chatId) {
    if (!state.containsKey(chatId)) return;
    state = {...state}..remove(chatId);
    _syncTimer();
  }

  /// [dismiss] for the chat on screen. Does nothing — and reads no other
  /// provider — while there is no notice at all, which is almost always: a send
  /// must not start building the active-chat provider chain just to find that out.
  void dismissActive() {
    if (state.isEmpty) return;
    dismiss(activeChatId());
  }

  /// The user pressed "Continue now".
  Future<void> continueNow(String chatId) async {
    final n = state[chatId];
    if (n == null || isSending()) return;
    await _fire(chatId, n);
  }

  Future<void> _fire(String chatId, QuotaNotice n) async {
    _lastAuto[chatId] = (at: now(), attempt: n.attempt);
    dismiss(chatId);
    await send(kQuotaContinueMessage);
  }

  void _tick() {
    final chat = activeChatId();
    if (chat.isEmpty) return;
    final n = state[chat];
    if (n == null || !n.exhausted || !autoEnabled() || isSending()) return;
    if (now().isBefore(n.resumeAt)) return;
    unawaited(_fire(chat, n).catchError((Object e) {
      debugPrint('quota_notice: automatic continue failed: $e');
    }));
  }

  /// The ticker runs only while some chat is waiting for a refill.
  void _syncTimer() {
    final waiting = state.values.any((n) => n.exhausted);
    if (waiting && _timer == null) {
      _timer = Timer.periodic(tickEvery, (_) => _tick());
    } else if (!waiting && _timer != null) {
      _timer!.cancel();
      _timer = null;
    }
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }
}
