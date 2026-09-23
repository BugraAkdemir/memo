import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/providers/chat_provider.dart' show apiClientProvider;
import 'package:memo_flutter/providers/skill_provider.dart';
import 'package:memo_flutter/widgets/skill_config_dialog.dart';

// Regression coverage for the per-chat skill activation UI split
// (2026-09-19): skill activation moved from one global always-on list to a
// list scoped per chat, and SkillConfigDialog grew a chatId-based branch to
// match it — no per-skill Switch and an explanatory hint when opened with
// no chat context (Settings > Skills), a Switch per skill and no hint when
// opened from inside a chat. The backend half of that split was proven with
// real e2e tests (internal/e2e's skill-activation-scope tests, see
// AGENTS.md's handoff history); this was the Flutter half, previously only
// an "eyeball it manually in the running app" TODO — this file replaces
// that manual check with an automated one.

class _FakeSkillListNotifier extends SkillListNotifier {
  _FakeSkillListNotifier(this._skills);
  final List<SkillDefinition> _skills;

  @override
  Future<List<SkillDefinition>> build() async => _skills;
}

final _oneSkill = [
  const SkillDefinition(name: 'greeter', description: 'Greets the user by name.'),
];

Future<void> _pumpDialog(WidgetTester tester, {String? chatId}) async {
  L10n.setLocale(MemoLocale.tr);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        // Port 1 is a privileged port nothing listens on in a test sandbox
        // — same unreachable-client trick settings_dialog_test.dart uses.
        // Only chatActiveSkillsProvider would ever reach it here (via
        // getActiveSkills), and this dialog already treats that provider's
        // error/loading state as "nothing active" (.valueOrNull ?? {}), so
        // letting that one call fail fast is fine for what these tests
        // check: Switch/hint presence, not a skill's actual active state.
        apiClientProvider.overrideWithValue(MemoApiClient(baseUrl: 'http://127.0.0.1:1')),
        skillListProvider.overrideWith(() => _FakeSkillListNotifier(_oneSkill)),
      ],
      child: MaterialApp(home: Scaffold(body: SkillConfigDialog(chatId: chatId))),
    ),
  );
  await tester.pump();
}

void main() {
  testWidgets(
      'opened from Settings (no chatId) shows the per-chat hint and no Switch',
      (tester) async {
    await _pumpDialog(tester);

    expect(find.text(L10n.t('skill_activation_hint_settings')), findsOneWidget);
    expect(find.byType(Switch), findsNothing);
    expect(find.text('greeter'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('opened from a chat shows a Switch per skill and no hint',
      (tester) async {
    await _pumpDialog(tester, chatId: 'chat-1');

    expect(find.text(L10n.t('skill_activation_hint_settings')), findsNothing);
    expect(find.byType(Switch), findsOneWidget);
    expect(find.text('greeter'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });
}
