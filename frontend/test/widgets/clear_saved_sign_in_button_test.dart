import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/core/local_session_state.dart';
import 'package:memo_flutter/providers/auth_gate_provider.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/settings_provider.dart';
import 'package:memo_flutter/widgets/clear_saved_sign_in_button.dart';

/// The manual escape hatch a user reaches for when their saved sign-in
/// belongs to a backend that no longer exists — the situation that, on
/// 2026-08-13, could only be resolved by clearing site data by hand in
/// DevTools.
void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  Future<SharedPreferences> pump(
    WidgetTester tester, {
    required Map<String, Object> prefs,
  }) async {
    SharedPreferences.setMockInitialValues(prefs);
    final store = await SharedPreferences.getInstance();
    final client = MemoApiClient(baseUrl: 'http://memo.test');
    client.setSessionToken('stale-token');
    await tester.pumpWidget(ProviderScope(
      overrides: [
        prefsProvider.overrideWithValue(store),
        apiClientProvider.overrideWithValue(client),
      ],
      child: const MaterialApp(
        home: Scaffold(body: Center(child: ClearSavedSignInButton())),
      ),
    ));
    return store;
  }

  testWidgets('confirming clears server-coupled state but keeps device prefs',
      (tester) async {
    final store = await pump(tester, prefs: {
      serverInstallIdKey: 'install-a',
      authSetupDoneKey: true,
      'memo_remote_access_token': 'stale-token',
      'memo_session_username': 'bugra',
      'memo_session_role': 'admin',
      'memo_tour_seen': true,
      // Device preferences and the address of the backend itself.
      'memo_locale': 'en',
      'memo_theme_mode': 'dark',
      'memo_api_base_url': 'http://memo.test',
    });

    await tester.tap(find.text(L10n.t('clear_sign_in_button')));
    await tester.pumpAndSettle();
    await tester.tap(find.text(L10n.t('clear_sign_in_confirm')));
    await tester.pumpAndSettle();

    for (final key in serverCoupledPrefsKeys) {
      expect(store.get(key), isNull, reason: '$key must be cleared');
    }
    expect(store.getString(serverInstallIdKey), isNull);

    expect(store.getString('memo_locale'), 'en');
    expect(store.getString('memo_theme_mode'), 'dark');
    expect(store.getString('memo_api_base_url'), 'http://memo.test',
        reason: 'clearing the backend address would strand the client — '
            're-pointing at another server is a separate, explicit action');
  });

  testWidgets('cancelling changes nothing', (tester) async {
    final store = await pump(tester, prefs: {
      authSetupDoneKey: true,
      'memo_remote_access_token': 'stale-token',
    });

    await tester.tap(find.text(L10n.t('clear_sign_in_button')));
    await tester.pumpAndSettle();
    await tester.tap(find.text(L10n.t('cancel')));
    await tester.pumpAndSettle();

    expect(store.getBool(authSetupDoneKey), isTrue);
    expect(store.getString('memo_remote_access_token'), 'stale-token');
  });

  testWidgets('the confirmation says server data is not affected',
      (tester) async {
    await pump(tester, prefs: {});

    await tester.tap(find.text(L10n.t('clear_sign_in_button')));
    await tester.pumpAndSettle();

    // The whole reason this copy was written the way it was: a user
    // pressing this while already confused must not be left wondering
    // whether their chats and memory are about to be deleted.
    expect(find.text(L10n.t('clear_sign_in_body')), findsOneWidget);
  });
}
