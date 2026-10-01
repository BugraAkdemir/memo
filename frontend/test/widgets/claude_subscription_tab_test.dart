import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/models/dev_gateway.dart';
import 'package:memo_flutter/providers/chat_provider.dart' show apiClientProvider;
import 'package:memo_flutter/providers/settings_provider.dart';
import 'package:memo_flutter/widgets/settings/tabs/claude_subscription_tab.dart';

/// Direct widget coverage for the Claude Subscription tab. The equivalent tab
/// for gemini-sub has none at all — only the rail-visibility tests in
/// settings_dialog_test.dart, which never build the tab — so the whole connect
/// flow is untested there. What matters here is that the two very different
/// outcomes of one POST are rendered differently:
///
///   * a local Claude Code login (connected, no URL) must NOT show a paste box
///     — there is nothing to paste, and a paste box that cannot succeed is
///     worse than no paste box;
///   * no local login (not connected, URL + state) MUST show the paste box,
///     because the flow cannot finish any other way.
class _FakeClaudeAccountNotifier extends ClaudeAccountNotifier {
  _FakeClaudeAccountNotifier(this._st);
  // Underscore-prefixed: AsyncNotifierBase already has a `state` getter, and a
  // field of that name would be a return-type clash, not a shadow.
  final ClaudeAccountState _st;

  @override
  Future<ClaudeAccountState> build() async => _st;
}

/// Stands in for the account's live GET /v1/models. A plain FutureProvider has
/// no notifier class to subclass, so the override is a value.
const _liveModels = <String>['claude-sonnet-5', 'claude-haiku-4-5-20251001'];

ProviderContainer _containerWith(ClaudeAccountState st) {
  final container = ProviderContainer(overrides: [
    // Port 1 is a privileged port nothing listens on in a test sandbox, so any
    // real request through this client fails fast instead of reaching a
    // backend. Same trick settings_dialog_test.dart uses.
    apiClientProvider.overrideWithValue(MemoApiClient(baseUrl: 'http://127.0.0.1:1')),
    claudeAccountProvider.overrideWith(() => _FakeClaudeAccountNotifier(st)),
    claudeSubModelsProvider.overrideWith((ref) async => _liveModels),
  ]);
  addTearDown(container.dispose);
  return container;
}

Future<void> _pump(WidgetTester tester, ClaudeAccountState st) async {
  L10n.setLocale(MemoLocale.en);
  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: _containerWith(st),
      child: const MaterialApp(home: Scaffold(body: ClaudeSubscriptionTab())),
    ),
  );
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('a local login shows connected state and no paste box', (tester) async {
    await _pump(
      tester,
      const ClaudeAccountState(
        connected: true,
        source: 'claude-code-file',
        model: 'claude-sonnet-5',
      ),
    );

    expect(find.text(L10n.t('claude_account_connected')), findsOneWidget);
    // The whole point of the source field: explain why no browser opened.
    expect(find.text(L10n.t('claude_account_adopted')), findsWidgets);
    // No paste box, because there is nothing to paste.
    expect(find.byKey(const Key('claude_account_code_field')), findsNothing);
    expect(find.text(L10n.t('claude_account_paste_cta')), findsNothing);
    expect(find.text(L10n.t('claude_account_browser_needed')), findsNothing);
  });

  testWidgets('a browser-connected account does not claim it was adopted', (tester) async {
    await _pump(
      tester,
      const ClaudeAccountState(connected: true, source: 'browser', model: 'claude-sonnet-5'),
    );
    expect(find.text(L10n.t('claude_account_connected')), findsOneWidget);
    // adoptedLocally is false for "browser", so the adoption line is absent.
    expect(find.text(L10n.t('claude_account_adopted')), findsNothing);
  });

  testWidgets('disconnected shows the connect button and nothing else', (tester) async {
    await _pump(tester, const ClaudeAccountState());

    expect(find.text(L10n.t('claude_account_connect_cta')), findsOneWidget);
    // No flow has been started yet, so no paste box and no fallback link.
    expect(find.byKey(const Key('claude_account_code_field')), findsNothing);
    expect(find.text(L10n.t('claude_account_manual_link_hint')), findsNothing);
    expect(find.text(L10n.t('claude_account_disconnect_cta')), findsNothing);
  });

  testWidgets('the connected state offers a model picker from the live list', (tester) async {
    await _pump(
      tester,
      const ClaudeAccountState(connected: true, source: 'claude-code-file', model: 'claude-sonnet-5'),
    );
    expect(find.text('claude-sonnet-5'), findsWidgets);
    expect(find.text('claude-haiku-4-5-20251001'), findsNothing);
    // Opening the dropdown must offer BOTH of the account's live models —
    // proving the list comes from the provider, not a local constant.
    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    expect(find.text('claude-haiku-4-5-20251001'), findsWidgets);
  });

  // Turkish runs longer than English; a Row of localized actions overflowed in
  // one language and not the other in the auth gate footer. Guard the shape.
  testWidgets('Turkish labels do not overflow at a narrow width', (tester) async {
    L10n.setLocale(MemoLocale.tr);
    tester.view.physicalSize = const Size(360 * 3, 640 * 3);
    tester.view.devicePixelRatio = 3.0;
    addTearDown(tester.view.reset);

    await _pump(tester, const ClaudeAccountState());
    expect(tester.takeException(), isNull);
  });
}