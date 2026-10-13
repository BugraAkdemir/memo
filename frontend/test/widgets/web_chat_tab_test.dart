import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/models/provider_config.dart';
import 'package:memo_flutter/providers/auth_gate_provider.dart';
import 'package:memo_flutter/providers/chat_provider.dart' show apiClientProvider;
import 'package:memo_flutter/widgets/settings/tabs/web_chat_tab.dart';

/// Records the web chat calls; the state it serves is whatever [consented]
/// currently says, like the backend would.
class _Api extends MemoApiClient {
  _Api({this.beta = true}) : super(baseUrl: 'http://127.0.0.1:1');
  final bool beta;
  bool consented = false;
  final actions = <String>[];

  Map<String, dynamic> _state() => {
        'beta': beta,
        'notice_version': 1,
        'sites': [
          {
            'site': 'chatgpt',
            'name': 'ChatGPT',
            'running': false,
            'state': 'idle',
            'verified': false,
            'installed': true,
            'connecting': false,
            'consented': consented,
          },
          {
            'site': 'claude',
            'name': 'Claude',
            'running': false,
            'state': 'idle',
            'verified': false,
            'installed': true,
            'connecting': false,
            'consented': false,
          },
        ],
      };

  @override
  Future<Map<String, dynamic>> getWebChat() async => _state();

  @override
  Future<Map<String, dynamic>> webChatAction(String action, String site, {bool forget = false}) async {
    actions.add('$action:$site');
    if (action == 'consent' && site == 'chatgpt') consented = true;
    if (action == 'revoke' && site == 'chatgpt') consented = false;
    return _state();
  }

  @override
  Future<List<ProviderConfig>> getProviders() async => const [];
}

Future<_Api> _pump(WidgetTester tester, {bool beta = true, MemoLocale locale = MemoLocale.en, Size? size}) async {
  L10n.setLocale(locale);
  tester.view.physicalSize = size ?? const Size(1000, 1600);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);
  final api = _Api(beta: beta);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        apiClientProvider.overrideWithValue(api),
        authGateProvider.overrideWith((ref) => Stream.value(const AuthGateInfo(AuthGateState.ok))),
      ],
      child: const MaterialApp(home: Scaffold(body: WebChatTab())),
    ),
  );
  await tester.pumpAndSettle();
  // Leave nothing running: the tab polls on a timer.
  addTearDown(() async {
    await tester.pumpWidget(const SizedBox());
  });
  return api;
}

void main() {
  providerTypeTests();
  testWidgets('shows both sites, the risk line, and the enable button before consent', (tester) async {
    await _pump(tester);
    expect(find.text('ChatGPT'), findsOneWidget);
    expect(find.text('Claude'), findsOneWidget);
    expect(find.text(L10n.t('web_chat_risk_short')), findsOneWidget);
    expect(find.text(L10n.t('web_chat_enable')), findsNWidgets(2));
    // Nothing that spends the account is offered before the notice is confirmed.
    expect(find.text(L10n.t('web_chat_connect')), findsNothing);
    expect(find.text(L10n.t('web_chat_add_model')), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets('beta off: only the explanation, no site cards', (tester) async {
    await _pump(tester, beta: false);
    expect(find.text(L10n.t('web_chat_beta_off')), findsOneWidget);
    expect(find.text(L10n.t('web_chat_enable')), findsNothing);
  });

  testWidgets('the notice cannot be confirmed without ticking the box', (tester) async {
    final api = await _pump(tester);
    await tester.tap(find.text(L10n.t('web_chat_enable')).first);
    await tester.pumpAndSettle();
    expect(find.text(L10n.t('web_chat_notice_title', {'site': 'ChatGPT'})), findsOneWidget);

    final confirm = find.widgetWithText(FilledButton, L10n.t('web_chat_notice_confirm'));
    expect(tester.widget<FilledButton>(confirm).onPressed, isNull, reason: 'no default consent');

    await tester.tap(find.text(L10n.t('web_chat_notice_accept')));
    await tester.pump();
    expect(tester.widget<FilledButton>(confirm).onPressed, isNotNull);
    await tester.tap(confirm);
    await tester.pumpAndSettle();

    expect(api.actions, ['consent:chatgpt']);
    // After consent that site offers Connect; the other still asks first.
    expect(find.text(L10n.t('web_chat_connect')), findsOneWidget);
    expect(find.text(L10n.t('web_chat_enable')), findsOneWidget);
  });

  testWidgets('cancelling the notice records nothing', (tester) async {
    final api = await _pump(tester);
    await tester.tap(find.text(L10n.t('web_chat_enable')).first);
    await tester.pumpAndSettle();
    await tester.tap(find.text(L10n.t('cancel')));
    await tester.pumpAndSettle();
    expect(api.actions, isEmpty);
  });

  testWidgets('Turkish at phone width: no overflow', (tester) async {
    await _pump(tester, locale: MemoLocale.tr, size: const Size(360, 1400));
    expect(tester.takeException(), isNull);
    await tester.tap(find.text(L10n.t('web_chat_enable')).first);
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
  });

  test('every web chat string exists in both languages', () {
    const keys = [
      'tab_web_chat', 'web_chat_desc', 'web_chat_risk_short', 'web_chat_beta_off', 'web_chat_enable',
      'web_chat_connect', 'web_chat_disconnect', 'web_chat_forget', 'web_chat_revoke', 'web_chat_add_model',
      'web_chat_added', 'web_chat_experimental', 'web_chat_experimental_tip', 'web_chat_state_not_enabled',
      'web_chat_state_idle', 'web_chat_state_connecting', 'web_chat_state_ready', 'web_chat_state_login',
      'web_chat_state_challenge', 'web_chat_state_unknown', 'web_chat_state_no_browser',
      'web_chat_notice_title', 'web_chat_notice_body', 'web_chat_notice_accept', 'web_chat_notice_confirm',
    ];
    for (final loc in [MemoLocale.en, MemoLocale.tr]) {
      L10n.setLocale(loc);
      for (final k in keys) {
        expect(L10n.t(k), isNot(k), reason: '$k missing for $loc');
      }
    }
    L10n.setLocale(MemoLocale.en);
    final en = L10n.t('web_chat_notice_body', {'site': 'ChatGPT'});
    L10n.setLocale(MemoLocale.tr);
    final tr = L10n.t('web_chat_notice_body', {'site': 'ChatGPT'});
    expect(en, isNot(tr), reason: 'the Turkish map must not hold the English text');
    expect(tr.contains('\${site}'), isFalse);
  });
}

void providerTypeTests() {
  test('web chat provider types need no API key and are not offered by the picker', () {
    for (final t in ['web-chatgpt', 'web-claude']) {
      expect(ProviderDefaults.isWebChatType(t), isTrue);
      expect(ProviderDefaults.needsApiKey(t), isFalse,
          reason: 'editing the entry must not demand a key it can never have');
      expect(ProviderDefaults.displayNames[t], isNotNull);
      expect(ProviderDefaults.defaultModels[t], isNotEmpty);
    }
    expect(ProviderDefaults.isWebChatType('openai'), isFalse);
    expect(ProviderDefaults.needsApiKey('openai'), isTrue);
  });
}
