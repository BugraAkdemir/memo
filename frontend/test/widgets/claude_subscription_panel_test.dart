import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/models/dev_gateway.dart';
import 'package:memo_flutter/providers/chat_provider.dart' show apiClientProvider;
import 'package:memo_flutter/providers/settings_provider.dart';
import 'package:memo_flutter/widgets/settings/tabs/claude_subscription_panel.dart';

/// Direct widget coverage for the Claude Subscription panel (mounted inside
/// the Beta tab while Beta is on). The equivalent tab
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

  // The panel re-reads while the capability probe is pending. The real reload
  // would hit the unreachable test client and turn the state into an error.
  int reloads = 0;
  @override
  Future<void> reload() async => reloads++;
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

/// A measured table for [model], so the "measuring" spinner (an endless
/// animation pumpAndSettle would wait on forever) is not on screen.
ClaudeCapabilities _caps(String model) =>
    ClaudeCapabilities(model: model, plain: true, tools: true, thinking: true);

Future<void> _pump(WidgetTester tester, ClaudeAccountState st, {bool settle = true}) async {
  L10n.setLocale(MemoLocale.en);
  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: _containerWith(st),
      // The panel is a Column meant to sit inside the Beta tab's list.
      child: const MaterialApp(
        home: Scaffold(body: SingleChildScrollView(child: ClaudeSubscriptionPanel())),
      ),
    ),
  );
  if (settle) {
    await tester.pumpAndSettle();
  } else {
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));
  }
}

void main() {
  testWidgets('a local login shows connected state and no paste box', (tester) async {
    await _pump(
      tester,
      ClaudeAccountState(
        connected: true,
        source: 'claude-code-file',
        model: 'claude-sonnet-5',
        capabilities: _caps('claude-sonnet-5'),
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
      ClaudeAccountState(
        connected: true,
        source: 'browser',
        model: 'claude-sonnet-5',
        capabilities: _caps('claude-sonnet-5'),
      ),
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
      ClaudeAccountState(
        connected: true,
        source: 'claude-code-file',
        model: 'claude-sonnet-5',
        capabilities: _caps('claude-sonnet-5'),
      ),
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

  testWidgets('capabilities not measured yet say so instead of an empty table', (tester) async {
    await _pump(
      tester,
      const ClaudeAccountState(connected: true, source: 'browser', model: 'claude-sonnet-5'),
      settle: false,
    );
    expect(find.text(L10n.t('claude_caps_measuring')), findsOneWidget);
    expect(find.byKey(const Key('claude_caps_table')), findsNothing);
    // Unmount so the bounded re-read timer is cancelled by dispose().
    await tester.pumpWidget(const SizedBox());
  });

  testWidgets('a measured table is shown, with the gate warning only when the gate refused', (tester) async {
    await _pump(
      tester,
      const ClaudeAccountState(
        connected: true,
        source: 'browser',
        model: 'claude-haiku-4-5-20251001',
        capabilities: ClaudeCapabilities(
          model: 'claude-haiku-4-5-20251001',
          plain: true,
          tools: true,
        ),
      ),
    );
    expect(find.byKey(const Key('claude_caps_table')), findsOneWidget);
    expect(find.text(L10n.t('claude_caps_tools')), findsOneWidget);
    expect(find.text(L10n.t('claude_caps_thinking')), findsOneWidget);
    // Haiku simply lacks thinking: that is the model, not the plan.
    expect(find.byKey(const Key('claude_caps_blocked')), findsNothing);

    await _pump(
      tester,
      const ClaudeAccountState(
        connected: true,
        source: 'browser',
        model: 'claude-opus-5',
        capabilities: ClaudeCapabilities(model: 'claude-opus-5', plain: true, entitlementBlocked: true),
      ),
    );
    expect(find.byKey(const Key('claude_caps_blocked')), findsOneWidget);
  });

  // After a model switch the backend re-probes in the background; the table it
  // still holds describes the PREVIOUS model and must not be presented as the
  // new one's.
  testWidgets("a previous model's table is not shown as the current one's", (tester) async {
    await _pump(
      tester,
      const ClaudeAccountState(
        connected: true,
        source: 'browser',
        model: 'claude-opus-5',
        capabilities: ClaudeCapabilities(model: 'claude-haiku-4-5-20251001', plain: true, tools: true),
      ),
      settle: false,
    );
    expect(find.byKey(const Key('claude_caps_table')), findsNothing);
    expect(find.text(L10n.t('claude_caps_measuring')), findsOneWidget);
    await tester.pumpWidget(const SizedBox());
  });

  test('capabilities parse defensively from odd payloads', () {
    final st = ClaudeAccountState.fromJson({
      'connected': true,
      'model': 'm',
      'capabilities': {'model': 'm', 'plain': 'yes', 'tools': true, 'one_m_context': 1},
    });
    expect(st.capabilities, isNotNull);
    expect(st.capabilities!.plain, false);
    expect(st.capabilities!.tools, true);
    expect(st.capabilities!.oneMContext, false);
    expect(ClaudeAccountState.fromJson({'capabilities': 'nope'}).capabilities, isNull);
  });
}