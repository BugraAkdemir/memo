import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/models/quota_notice.dart';
import 'package:memo_flutter/providers/quota_notice_provider.dart';

/// A notifier on a clock the test moves by hand; every message it types into the
/// chat is recorded.
class _Rig {
  DateTime clock = DateTime.utc(2026, 10, 7, 12);
  String active = 'chat-1';
  bool sending = false;
  bool auto = true;
  final sent = <String>[];

  late final QuotaNoticeNotifier n = QuotaNoticeNotifier(
    now: () => clock,
    activeChatId: () => active,
    isSending: () => sending,
    autoEnabled: () => auto,
    send: (m) async => sent.add(m),
  );
}

QuotaNotice _exhausted(_Rig r, {Duration? resetIn, int attempt = 0}) => QuotaNotice(
      kind: 'exhausted',
      model: 'gpt-5.5',
      resetAt: resetIn == null ? null : r.clock.add(resetIn),
      receivedAt: r.clock,
      attempt: attempt,
    );

/// One second of wall time for the ticker, mirrored on the injected clock.
Future<void> _advance(WidgetTester t, _Rig r, Duration d) async {
  for (var s = 0; s < d.inSeconds; s++) {
    r.clock = r.clock.add(const Duration(seconds: 1));
    await t.pump(const Duration(seconds: 1));
  }
}

void main() {
  testWidgets('continues by itself once the refill time (plus margin) has passed — and not a second sooner',
      (t) async {
    final r = _Rig();
    r.n.show('chat-1', _exhausted(r, resetIn: const Duration(seconds: 60)));
    await _advance(t, r, const Duration(seconds: 79));
    expect(r.sent, isEmpty, reason: 'the refill is at 60s and the margin 20s: not before 80s');
    await _advance(t, r, const Duration(seconds: 2));
    expect(r.sent, [kQuotaContinueMessage]);
    expect(r.n.state.containsKey('chat-1'), isFalse, reason: 'the card goes away once it has acted');
    r.n.dispose();
  });

  testWidgets('with automatic continue switched off it never types, but "Continue now" still works', (t) async {
    final r = _Rig()..auto = false;
    r.n.show('chat-1', _exhausted(r, resetIn: const Duration(seconds: 5)));
    await _advance(t, r, const Duration(minutes: 5));
    expect(r.sent, isEmpty);
    expect(r.n.state.containsKey('chat-1'), isTrue, reason: 'the card stays so the user can act');
    await r.n.continueNow('chat-1');
    expect(r.sent, [kQuotaContinueMessage]);
    r.n.dispose();
  });

  testWidgets('waits while a reply is still streaming, then goes', (t) async {
    final r = _Rig()..sending = true;
    r.n.show('chat-1', _exhausted(r, resetIn: const Duration(seconds: 1)));
    await _advance(t, r, const Duration(seconds: 60));
    expect(r.sent, isEmpty);
    r.sending = false;
    await _advance(t, r, const Duration(seconds: 2));
    expect(r.sent, [kQuotaContinueMessage]);
    r.n.dispose();
  });

  testWidgets('only types into the chat that is on screen; the other waits for the user to come back', (t) async {
    final r = _Rig()..active = 'chat-2';
    r.n.show('chat-1', _exhausted(r, resetIn: const Duration(seconds: 1)));
    await _advance(t, r, const Duration(seconds: 60));
    expect(r.sent, isEmpty, reason: 'sendMessage writes to the active chat; chat-1 is not it');
    r.active = 'chat-1';
    await _advance(t, r, const Duration(seconds: 2));
    expect(r.sent, [kQuotaContinueMessage]);
    r.n.dispose();
  });

  testWidgets('a low-allowance warning never continues anything and never starts the ticker', (t) async {
    final r = _Rig();
    r.n.show('chat-1', QuotaNotice(kind: 'low', remainingPercent: 8, receivedAt: r.clock));
    await _advance(t, r, const Duration(minutes: 30));
    expect(r.sent, isEmpty);
    expect(r.n.state['chat-1']!.low, isTrue);
    r.n.dispose();
  });

  testWidgets('dismissing stops the ticker (no timer outlives the card)', (t) async {
    final r = _Rig();
    r.n.show('chat-1', _exhausted(r, resetIn: const Duration(hours: 1)));
    r.n.dismiss('chat-1');
    await _advance(t, r, const Duration(hours: 2));
    expect(r.sent, isEmpty);
    r.n.dispose();
  });

  testWidgets('with no refill time it backs off: five minutes first, then ten if it refused again', (t) async {
    final r = _Rig();
    r.n.show('chat-1', _exhausted(r));
    await _advance(t, r, const Duration(minutes: 4, seconds: 59));
    expect(r.sent, isEmpty);
    await _advance(t, r, const Duration(seconds: 2));
    expect(r.sent, hasLength(1));

    // The vendor refused again right after: the next wait is the longer one.
    r.n.show('chat-1', _exhausted(r));
    expect(r.n.state['chat-1']!.attempt, 1);
    await _advance(t, r, const Duration(minutes: 9, seconds: 59));
    expect(r.sent, hasLength(1));
    await _advance(t, r, const Duration(seconds: 2));
    expect(r.sent, hasLength(2));
    r.n.dispose();
  });

  testWidgets('an old automatic attempt is forgotten: a much later exhaustion starts over at five minutes', (t) async {
    final r = _Rig();
    r.n.show('chat-1', _exhausted(r));
    await _advance(t, r, const Duration(minutes: 6));
    expect(r.sent, hasLength(1));
    r.clock = r.clock.add(const Duration(hours: 3));
    r.n.show('chat-1', _exhausted(r));
    expect(r.n.state['chat-1']!.attempt, 0);
    r.n.dispose();
  });

  testWidgets('a marker chunk with bad content is ignored; a good one is shown', (t) async {
    final r = _Rig();
    r.n.showFromMarker('chat-1', 'not json');
    r.n.showFromMarker('', json.encode({'kind': 'exhausted'}));
    expect(r.n.state, isEmpty);
    r.n.showFromMarker('chat-1', json.encode({'kind': 'low', 'model': 'm', 'remaining_percent': 9}));
    expect(r.n.state['chat-1']!.remainingPercent, 9);
    r.n.dispose();
  });

  testWidgets('one card per chat: a newer notice replaces the older', (t) async {
    final r = _Rig();
    r.n.show('chat-1', QuotaNotice(kind: 'low', remainingPercent: 9, receivedAt: r.clock));
    r.n.show('chat-1', _exhausted(r, resetIn: const Duration(hours: 1)));
    expect(r.n.state['chat-1']!.exhausted, isTrue);
    expect(r.n.state, hasLength(1));
    r.n.dispose();
  });

  testWidgets('dismissActive reads nothing while no card exists, and clears only the chat on screen', (t) async {
    var reads = 0;
    final r = _Rig();
    final n = QuotaNoticeNotifier(
      now: () => r.clock,
      activeChatId: () {
        reads++;
        return r.active;
      },
      isSending: () => false,
      autoEnabled: () => true,
      send: (m) async => r.sent.add(m),
    );
    n.dismissActive();
    expect(reads, 0, reason: 'a send must not start building the active-chat provider chain for nothing');

    n.show('chat-1', _exhausted(r, resetIn: const Duration(hours: 1)));
    n.show('chat-2', _exhausted(r, resetIn: const Duration(hours: 1)));
    n.dismissActive(); // chat-1 is on screen
    expect(n.state.keys, ['chat-2']);
    n.dispose();
  });
}
