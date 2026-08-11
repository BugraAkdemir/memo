import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/settings_provider.dart';
import 'package:memo_flutter/widgets/settings/tabs/accounts_tab.dart';

/// Answers every request from a response map, recording each RequestOptions
/// as it goes (simple: no auth-mode state machine needed here — the tab
/// only ever calls the accounts endpoints, which the map covers).
class _RecordingAdapter implements HttpClientAdapter {
  _RecordingAdapter(this.responses, {this.onRequest});
  final Map<String, (int, Object?)> responses;
  final void Function(RequestOptions options)? onRequest;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    onRequest?.call(options);
    final (status, body) = responses[options.path] ?? (500, null);
    return ResponseBody.fromString(
      body == null ? '' : jsonEncode(body),
      status,
      headers: {Headers.contentTypeHeader: [Headers.jsonContentType]},
    );
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  Future<ProviderContainer> pump(
    WidgetTester tester,
    _RecordingAdapter adapter,
    Map<String, Object> prefs,
  ) async {
    SharedPreferences.setMockInitialValues(prefs);
    final prefsHandle = await SharedPreferences.getInstance();
    final client = MemoApiClient(baseUrl: 'http://memo.test');
    client.dio.httpClientAdapter = adapter;
    final container = ProviderContainer(
      overrides: [
        apiClientProvider.overrideWithValue(client),
        prefsProvider.overrideWithValue(prefsHandle),
      ],
    );
    await tester.pumpWidget(UncontrolledProviderScope(
      container: container,
      child: MaterialApp(
        home: Scaffold(body: AccountsTab()),
      ),
    ));
    await tester.pumpAndSettle();
    return container;
  }

  testWidgets('admin session: lists accounts and roles', (tester) async {
    final adapter = _RecordingAdapter({
      '/api/accounts': (200, [
        {'id': 'a1', 'username': 'admin', 'role': 'admin'},
        {'id': 'a2', 'username': 'kaya', 'role': 'user'},
      ]),
    });
    final container =
        await pump(tester, adapter, {'memo_session_role': 'admin'});
    addTearDown(container.dispose);

    expect(find.text('admin'), findsOneWidget);
    expect(find.text('kaya'), findsOneWidget);
    expect(find.text(L10n.t('accounts_role_admin')), findsOneWidget);
    expect(find.text(L10n.t('accounts_role_user')), findsOneWidget);
    expect(find.text(L10n.t('accounts_add')), findsOneWidget);
    // Admin sees delete + password-change actions on each row.
    expect(find.byIcon(Icons.delete_outline), findsNWidgets(2));
    expect(find.byIcon(Icons.lock_outline), findsNWidgets(2));
  });

  testWidgets('admin session: add account posts and refreshes the list',
      (tester) async {
    final requests = <RequestOptions>[];
    final accounts = [
      {'id': 'a1', 'username': 'admin', 'role': 'admin'},
    ];
    final adapter = _RecordingAdapter(
      {
        '/api/accounts': (200, accounts),
      },
      onRequest: (o) {
        requests.add(o);
        if (o.method == 'POST') {
          accounts.add({
            'id': 'a2',
            'username': o.data['username'],
            'role': o.data['role'],
          });
        }
      },
    );
    final container =
        await pump(tester, adapter, {'memo_session_role': 'admin'});
    addTearDown(container.dispose);

    await tester.tap(find.text(L10n.t('accounts_add')));
    await tester.pumpAndSettle();
    await tester.enterText(
        find.widgetWithText(TextField, L10n.t('accounts_add_username')),
        'yeni');
    await tester.enterText(
        find.widgetWithText(TextField, L10n.t('accounts_add_password')),
        'sifre123');
    await tester.tap(find.text(L10n.t('accounts_add_submit')));
    await tester.pumpAndSettle();

    expect(
      requests.where((r) => r.method == 'POST').length,
      1,
      reason: 'add should POST exactly once',
    );
    final post = requests.firstWhere((r) => r.method == 'POST');
    expect(post.data['username'], 'yeni');
    expect(post.data['role'], 'user');
    // Refreshed list shows the new account.
    expect(find.text('yeni'), findsOneWidget);
  });

  testWidgets('user session: admin-only note, no management actions',
      (tester) async {
    final adapter = _RecordingAdapter({
      '/api/accounts': (200, [
        {'id': 'a1', 'username': 'kaya', 'role': 'user'},
      ]),
    });
    final container = await pump(tester, adapter, {'memo_session_role': 'user'});
    addTearDown(container.dispose);

    expect(find.text(L10n.t('accounts_admin_only_note')), findsOneWidget);
    expect(find.text(L10n.t('accounts_add')), findsNothing);
    expect(find.byIcon(Icons.delete_outline), findsNothing);
  });

  testWidgets('user session: own password change posts current + new password',
      (tester) async {
    final requests = <RequestOptions>[];
    final adapter = _RecordingAdapter(
      {
        '/api/accounts': (200, [
          {'id': 'a1', 'username': 'kaya', 'role': 'user'},
        ]),
        '/api/accounts/a1/password': (200, {'ok': true}),
      },
      onRequest: requests.add,
    );
    final container = await pump(tester, adapter, {
      'memo_session_role': 'user',
      'memo_session_username': 'kaya',
    });
    addTearDown(container.dispose);

    await tester.tap(find.byIcon(Icons.lock_outline));
    await tester.pumpAndSettle();
    expect(find.text(L10n.t('accounts_current_password')), findsOneWidget,
        reason: 'self password change requires the current password');
    await tester.enterText(
        find.widgetWithText(TextField, L10n.t('accounts_current_password')),
        'eski');
    await tester.enterText(
        find.widgetWithText(TextField, L10n.t('accounts_new_password')),
        'yeni');
    await tester.tap(find.text(L10n.t('accounts_password_submit')));
    await tester.pump(); // pop animasyonu başlar
    await tester.pump(const Duration(milliseconds: 400)); // pop biter
    await tester.pump(const Duration(milliseconds: 300)); // SnackBar görünür

    final req =
        requests.firstWhere((r) => r.path == '/api/accounts/a1/password');
    expect(req.data['current_password'], 'eski');
    expect(req.data['new_password'], 'yeni');
    expect(find.byType(SnackBar), findsOneWidget);
    expect(find.text(L10n.t('accounts_password_changed')), findsOneWidget);
  });

  testWidgets('sign out clears the session and appears only when logged in',
      (tester) async {
    final adapter = _RecordingAdapter({
      '/api/accounts': (200, [
        {'id': 'a1', 'username': 'kaya', 'role': 'user'},
      ]),
    });
    SharedPreferences.setMockInitialValues({
      'memo_session_role': 'user',
      'memo_session_username': 'kaya',
      'memo_remote_access_token': 'tok',
    });
    final prefs = await SharedPreferences.getInstance();
    final client = MemoApiClient(baseUrl: 'http://memo.test');
    client.dio.httpClientAdapter = adapter;
    final container = ProviderContainer(
      overrides: [
        apiClientProvider.overrideWithValue(client),
        prefsProvider.overrideWithValue(prefs),
      ],
    );
    addTearDown(container.dispose);
    await tester.pumpWidget(UncontrolledProviderScope(
      container: container,
      child: MaterialApp(
        home: Scaffold(body: AccountsTab()),
      ),
    ));
    await tester.pumpAndSettle();

    expect(find.text(L10n.t('accounts_sign_out')), findsOneWidget);

    await tester.tap(find.text(L10n.t('accounts_sign_out')));
    await tester.pumpAndSettle();
    // Confirm: the dialog's "Sign out" button is the one in the dialog (the
    // tab's own button is still in the tree underneath it).
    await tester.tap(find.text(L10n.t('accounts_sign_out')).last);
    await tester.pumpAndSettle();

    expect(prefs.getString('memo_session_role'), isNull);
    expect(prefs.getString('memo_session_username'), isNull);
    expect(prefs.getString('memo_remote_access_token'), isNull);
    // No session anymore: sign-out button disappears.
    expect(find.text(L10n.t('accounts_sign_out')), findsNothing);
  });
}
