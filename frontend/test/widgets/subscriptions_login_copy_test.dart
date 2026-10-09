import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/models/subscriptions.dart';
import 'package:memo_flutter/providers/chat_provider.dart' show apiClientProvider;
import 'package:memo_flutter/providers/settings_provider.dart';
import 'package:memo_flutter/widgets/settings/tabs/subscriptions_tab.dart';

/// Records what was written to the platform clipboard, standing in for a real
/// clipboard.
class _FakeClipboard {
  final writes = <String>[];
  int failures = 0;
}

/// Every callback URL the tab handed to the backend, in order.
final List<String> submitted = [];

class _FakeSubscriptions extends SubscriptionsNotifier {
  _FakeSubscriptions([this.initial = const SubscriptionsState(
        bundled: true,
        running: true,
        providers: ['antigravity', 'claude', 'codex'],
      )]);
  // Not `state`: that is AsyncNotifierBase's own getter.
  final SubscriptionsState initial;

  @override
  Future<SubscriptionsState> build() async => initial;

  @override
  Future<void> reload() async {}
}

/// Answers every request with the state a sign-in returns, including the vendor
/// URL. Real sockets aren't usable under `flutter test` (see
/// api_client_test.dart for why), so this hooks Dio's adapter layer.
class _FakeSubscriptionsAdapter implements HttpClientAdapter {
  _FakeSubscriptionsAdapter(this.authUrl, {this.login = const {}});
  final String authUrl;
  final Map<String, dynamic> login;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    // The hand-back of the callback URL is what this test watches.
    if ((options.data as Map?)?['action'] == 'submit_callback') {
      submitted.add((options.data as Map)['callback_url'] as String? ?? '');
    }
    return ResponseBody.fromString(
      jsonEncode({
        ...login,
        'auth_url': authUrl,
        'bundled': true,
        'running': true,
        'providers': ['antigravity', 'claude', 'codex'],
        'accounts': [],
        'models': [],
      }),
      200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

/// The reported bug, end to end through the real widget: a sign-in starts, the
/// vendor URL appears, and pressing the copy button must actually put THAT url
/// on the clipboard — and say so. The old code called `Clipboard.setData`
/// without awaiting it and always showed "Copied", so on a browser over plain
/// HTTP (no `navigator.clipboard`) the link silently never reached the paste
/// buffer.
void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  const authUrl =
      'https://auth.openai.com/oauth/authorize?client_id=x&state=SECRET&'
      'redirect_uri=http%3A%2F%2F192.168.1.50%3A8090%2Fapi%2Fsubscriptions%2Foauth%2Fcallback';

  late _FakeClipboard clip;

  setUp(() {
    clip = _FakeClipboard();
    submitted.clear();
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(SystemChannels.platform,
            (MethodCall call) async {
      if (call.method == 'Clipboard.setData') {
        if (clip.failures > 0) {
          clip.failures--;
          throw PlatformException(code: 'copy_fail');
        }
        clip.writes.add((call.arguments as Map)['text'] as String);
      }
      return null;
    });
  });

  tearDown(() {
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(SystemChannels.platform, null);
  });

  Future<void> pumpLogin(
    WidgetTester tester, {
    MemoLocale locale = MemoLocale.en,
    Map<String, dynamic> login = const {},
  }) async {
    L10n.setLocale(locale);
    tester.view.physicalSize = const Size(1000, 1800);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    final client = MemoApiClient(baseUrl: 'http://memo.test');
    client.dio.httpClientAdapter = _FakeSubscriptionsAdapter(authUrl, login: login);

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          apiClientProvider.overrideWithValue(client),
          subscriptionsProvider.overrideWith(() => _FakeSubscriptions(
                login.isEmpty
                    ? const SubscriptionsState(
                        bundled: true, running: true, providers: ['antigravity', 'claude', 'codex'])
                    : SubscriptionsState.fromJson({
                        'bundled': true,
                        'running': true,
                        'providers': ['antigravity', 'claude', 'codex'],
                        ...login,
                      }),
              )),
        ],
        child: const MaterialApp(home: Scaffold(body: SubscriptionsTab())),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text(L10n.t('subs_sign_in')).first);
    // One pump for the tap, one for the awaited API call to land, one for the
    // setState that reveals the URL. `pumpAndSettle` never returns here: the
    // tab polls the login every 2 s, so there is always a frame pending.
    await tester.pump();
    await tester.pump(const Duration(seconds: 1));
    await tester.pump();
  }

  testWidgets('the copy button puts the whole sign-in URL on the clipboard',
      (tester) async {
    await pumpLogin(tester);

    expect(find.text(authUrl), findsOneWidget,
        reason: 'the vendor URL is shown for copying');

    await tester.tap(find.byTooltip(L10n.t('copy')));
    // pumpAndSettle never returns here (the tab polls the login every 2 s, so
    // a frame is always pending); pump long enough for the SnackBar to show.
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 400));
    await tester.pump(const Duration(milliseconds: 400));

    expect(clip.writes, hasLength(1));
    // Byte for byte: a truncated URL is an unusable login link, and the state
    // parameter is what makes it valid at all.
    expect(clip.writes.single, authUrl);
    expect(find.text(L10n.t('copied')), findsOneWidget);
  });

  testWidgets('a failed copy is reported instead of claiming success',
      (tester) async {
    await pumpLogin(tester);
    clip.failures = 1;

    await tester.tap(find.byTooltip(L10n.t('copy')));
    // pumpAndSettle never returns here (the tab polls the login every 2 s, so
    // a frame is always pending); pump long enough for the SnackBar to show.
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 400));
    await tester.pump(const Duration(milliseconds: 400));

    expect(clip.writes, isEmpty);
    expect(find.text(L10n.t('copied')), findsNothing,
        reason: 'must not claim a copy that did not happen');
    expect(find.text(L10n.t('copy_failed')), findsOneWidget);
  });

  testWidgets('the copy button works in Turkish too', (tester) async {
    await pumpLogin(tester, locale: MemoLocale.tr);

    await tester.tap(find.byTooltip(L10n.t('copy')));
    // pumpAndSettle never returns here (the tab polls the login every 2 s, so
    // a frame is always pending); pump long enough for the SnackBar to show.
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 400));
    await tester.pump(const Duration(milliseconds: 400));

    expect(clip.writes.single, authUrl);
  });

  /// The reported bug: Memo runs on a Raspberry Pi / VDS and is driven from a
  /// laptop. Every vendor registers a loopback `redirect_uri` and refuses any
  /// other, so the browser ends up on a localhost page on the WRONG machine and
  /// the code never reaches the sidecar. The sidecar gives up waiting and asks
  /// for the callback URL on stdin — these drive that hand-back from the UI.
  Future<void> pumpRemoteSignIn(
    WidgetTester tester, {
    MemoLocale locale = MemoLocale.en,
  }) =>
      pumpLogin(tester, locale: locale, login: const {
        'login': {
          'provider': 'codex',
          'running': true,
          'url': 'https://auth.openai.com/oauth/authorize?client_id=x&state=SECRET',
          'needs_paste': true,
        },
      });

  testWidgets('a plain desktop sign-in shows no paste box at all',
      (tester) async {
    await pumpLogin(tester);
    expect(find.text(L10n.t('subs_paste_title')), findsNothing,
        reason: 'nothing to hand back when the browser IS on this machine');
  });

  testWidgets('the paste box appears once the sidecar asks for the URL',
      (tester) async {
    await pumpRemoteSignIn(tester);
    expect(find.text(L10n.t('subs_paste_title')), findsOneWidget,
        reason: 'the only way in from a machine that is not the browser\'s');
    expect(find.text(L10n.t('subs_paste_field')), findsOneWidget);
  });

  testWidgets('pasting the address bar URL hands it to the backend verbatim',
      (tester) async {
    await pumpRemoteSignIn(tester);

    const pasted =
        'http://localhost:1455/auth/callback?code=REALCODE&state=REALSTATE';
    await tester.enterText(find.byType(TextField).last, pasted);
    await tester.tap(find.text(L10n.t('subs_paste_send')));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 400));

    expect(submitted, [pasted],
        reason: 'a trimmed or rewritten URL would exchange nothing');
  });

  testWidgets('the paste box works in Turkish too', (tester) async {
    await pumpRemoteSignIn(tester, locale: MemoLocale.tr);

    expect(find.text(L10n.t('subs_paste_title')), findsOneWidget);
    expect(tester.takeException(), isNull);
  });
}
