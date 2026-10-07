import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/core/theme.dart';
import 'package:memo_flutter/models/quota_notice.dart';
import 'package:memo_flutter/providers/auth_gate_provider.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/quota_notice_provider.dart';
import 'package:memo_flutter/providers/settings_provider.dart';
import 'package:memo_flutter/widgets/quota_notice_card.dart';

class _FixedChat extends ActiveChatIdNotifier {
  _FixedChat(this.id);
  final String id;
  @override
  Future<String> build() async => id;
}

class _Rig {
  final sent = <String>[];
  DateTime clock = DateTime.now();
  bool autoEnabled = true;
  late final QuotaNoticeNotifier notifier = QuotaNoticeNotifier(
    now: () => clock,
    activeChatId: () => 'chat-1',
    isSending: () => false,
    autoEnabled: () => autoEnabled,
    send: (m) async => sent.add(m),
  );
}

Future<_Rig> _pump(
  WidgetTester tester, {
  QuotaNotice? notice,
  String chatId = 'chat-1',
  MemoLocale locale = MemoLocale.en,
  Size size = const Size(900, 700),
  bool? autoPref, // null = nothing stored at all, the first-run state
}) async {
  SharedPreferences.setMockInitialValues({QuotaAutoContinueNotifier.prefKey: ?autoPref});
  final prefs = await SharedPreferences.getInstance();
  L10n.setLocale(locale);
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);
  final rig = _Rig();
  if (notice != null) rig.notifier.show('chat-1', notice);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        prefsProvider.overrideWithValue(prefs),
        apiClientProvider.overrideWithValue(MemoApiClient(baseUrl: 'http://127.0.0.1:1')),
        authGateProvider.overrideWith((ref) => Stream.value(const AuthGateInfo(AuthGateState.ok))),
        activeChatIdProvider.overrideWith(() => _FixedChat(chatId)),
        quotaNoticeProvider.overrideWith((ref) => rig.notifier),
      ],
      child: MaterialApp(
        theme: ThemeData(extensions: [MemoTheme.dark]),
        home: Scaffold(body: Align(alignment: Alignment.bottomCenter, child: QuotaNoticeCard(clock: () => rig.clock))),
      ),
    ),
  );
  await tester.pump();
  await tester.pump();
  return rig;
}

QuotaNotice _exhausted({Duration? resetIn, String model = 'claude-opus-4-6-thinking'}) {
  final now = DateTime.now();
  return QuotaNotice(
    kind: 'exhausted',
    model: model,
    remainingPercent: 0,
    resetAt: resetIn == null ? null : now.add(resetIn),
    receivedAt: now,
  );
}

void main() {
  testWidgets('shows nothing when the open chat has no notice', (tester) async {
    await _pump(tester);
    expect(find.byKey(const Key('quotaNoticeCard')), findsNothing);
  });

  testWidgets('a notice for a different chat is not shown here', (tester) async {
    await _pump(tester, notice: _exhausted(resetIn: const Duration(hours: 1)), chatId: 'chat-2');
    expect(find.byKey(const Key('quotaNoticeCard')), findsNothing);
  });

  testWidgets('exhausted: says so, names the model, shows the countdown, and automatic continue is ticked by default',
      (tester) async {
    await _pump(tester, notice: _exhausted(resetIn: const Duration(hours: 3, minutes: 12)));
    expect(find.text(L10n.t('quota_exhausted_title')), findsOneWidget);
    expect(find.textContaining('Claude Opus 4.6 · Thinking'), findsOneWidget);
    expect(find.textContaining('3h 11m'), findsOneWidget, reason: 'countdown to the refill time');
    expect(find.text(L10n.t('quota_auto_continue')), findsOneWidget);
    expect(tester.widget<Checkbox>(find.byType(Checkbox)).value, isTrue, reason: 'on by default');
    expect(find.text(L10n.t('quota_auto_hint')), findsOneWidget);
    expect(find.text(L10n.t('quota_continue_now')), findsOneWidget);
  });

  testWidgets('without a known refill time it says it will try again later instead of inventing a time',
      (tester) async {
    await _pump(tester, notice: _exhausted());
    expect(find.textContaining('about 4m'), findsOneWidget, reason: 'the five-minute back-off, counted down');
    expect(find.textContaining('Refills in'), findsNothing);
  });

  testWidgets('unticking the box stores the choice and hides the "will write continue" hint', (tester) async {
    await _pump(tester, notice: _exhausted(resetIn: const Duration(hours: 1)));
    await tester.tap(find.byType(Checkbox));
    await tester.pump();
    expect(tester.widget<Checkbox>(find.byType(Checkbox)).value, isFalse);
    expect(find.text(L10n.t('quota_auto_hint')), findsNothing);
    final prefs = await SharedPreferences.getInstance();
    expect(prefs.getBool(QuotaAutoContinueNotifier.prefKey), isFalse, reason: 'remembered for next time');
  });

  testWidgets('with nothing stored (first run) automatic continue is ON', (tester) async {
    await _pump(tester, notice: _exhausted(resetIn: const Duration(hours: 1)));
    expect(tester.widget<Checkbox>(find.byType(Checkbox)).value, isTrue);
    final prefs = await SharedPreferences.getInstance();
    expect(prefs.containsKey(QuotaAutoContinueNotifier.prefKey), isFalse, reason: 'a default is not a stored choice');
  });

  testWidgets('a stored "off" preference is honoured when the card appears', (tester) async {
    await _pump(tester, notice: _exhausted(resetIn: const Duration(hours: 1)), autoPref: false);
    expect(tester.widget<Checkbox>(find.byType(Checkbox)).value, isFalse);
  });

  testWidgets('"Continue now" types the continue message and takes the card away', (tester) async {
    final rig = await _pump(tester, notice: _exhausted(resetIn: const Duration(hours: 1)));
    await tester.tap(find.byKey(const Key('quotaContinueNow')));
    await tester.pump();
    expect(rig.sent, [kQuotaContinueMessage]);
    expect(find.byKey(const Key('quotaNoticeCard')), findsNothing);
  });

  testWidgets('the close button dismisses it without sending anything', (tester) async {
    final rig = await _pump(tester, notice: _exhausted(resetIn: const Duration(hours: 1)));
    await tester.tap(find.byKey(const Key('quotaNoticeDismiss')));
    await tester.pump();
    expect(rig.sent, isEmpty);
    expect(find.byKey(const Key('quotaNoticeCard')), findsNothing);
  });

  testWidgets('low allowance: a warning with the percentage and refill time, no checkbox, no continue button',
      (tester) async {
    await _pump(
      tester,
      notice: QuotaNotice(
        kind: 'low',
        model: 'claude-sonnet-4-6',
        remainingPercent: 8,
        resetAt: DateTime.now().add(const Duration(days: 2, hours: 4, minutes: 1)),
        receivedAt: DateTime.now(),
      ),
    );
    expect(find.text(L10n.t('quota_low_title')), findsOneWidget);
    expect(find.textContaining('Claude Sonnet 4.6: 8% left'), findsOneWidget);
    expect(find.textContaining('2d 4h'), findsOneWidget);
    expect(find.byType(Checkbox), findsNothing);
    expect(find.byKey(const Key('quotaContinueNow')), findsNothing);
  });

  testWidgets('low allowance with no model name still reads sensibly', (tester) async {
    await _pump(tester, notice: QuotaNotice(kind: 'low', remainingPercent: 7, receivedAt: DateTime.now()));
    expect(find.textContaining('7% of your allowance is left'), findsOneWidget);
  });

  testWidgets('Turkish at phone width: no overflow, strings come from L10n', (tester) async {
    await _pump(
      tester,
      notice: _exhausted(resetIn: const Duration(days: 6, hours: 2)),
      locale: MemoLocale.tr,
      size: const Size(360, 640),
    );
    expect(find.text(L10n.t('quota_exhausted_title')), findsOneWidget);
    expect(find.text(L10n.t('quota_auto_continue')), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('the countdown moves on its own', (tester) async {
    final rig = await _pump(tester, notice: _exhausted(resetIn: const Duration(minutes: 5)));
    expect(find.textContaining('4m'), findsWidgets);
    rig.clock = rig.clock.add(const Duration(seconds: 61));
    await tester.pump(const Duration(seconds: 1)); // the card's own one-second repaint
    expect(find.textContaining('3m'), findsWidgets);
    expect(find.textContaining('4m'), findsNothing);
  });
}
