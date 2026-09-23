import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/providers/skill_provider.dart';
import 'package:memo_flutter/widgets/settings/tabs/skills_tab.dart';

// Companion to skill_config_dialog_test.dart, covering the other half of
// the per-chat activation UI split: Settings > Skills itself
// (skills_tab.dart) is an installed-skills list ONLY — no chat context
// exists there to activate/deactivate against, so it must render no
// activation control at all (see the "Activation is per-chat now" comment
// on SkillsTab.build's itemBuilder). Previously only manually eyeballed.

class _FakeSkillListNotifier extends SkillListNotifier {
  _FakeSkillListNotifier(this._skills);
  final List<SkillDefinition> _skills;

  @override
  Future<List<SkillDefinition>> build() async => _skills;
}

final _twoSkills = [
  const SkillDefinition(name: 'greeter', description: 'Greets the user by name.'),
  const SkillDefinition(name: 'reviewer', description: 'Reviews code changes.'),
];

Future<void> _pumpTab(WidgetTester tester) async {
  L10n.setLocale(MemoLocale.tr);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        skillListProvider.overrideWith(() => _FakeSkillListNotifier(_twoSkills)),
      ],
      child: const MaterialApp(home: Scaffold(body: SkillsTab())),
    ),
  );
  await tester.pump();
}

void main() {
  testWidgets(
      'lists installed skills with no Switch/Checkbox activation control',
      (tester) async {
    await _pumpTab(tester);

    expect(find.text('greeter'), findsOneWidget);
    expect(find.text('reviewer'), findsOneWidget);
    expect(find.byType(Switch), findsNothing);
    expect(find.byType(Checkbox), findsNothing);
    expect(tester.takeException(), isNull);
  });
}
