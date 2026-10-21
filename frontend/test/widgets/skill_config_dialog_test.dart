import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/providers/chat_provider.dart' show apiClientProvider;
import 'package:memo_flutter/providers/skill_provider.dart';
import 'package:memo_flutter/widgets/skill_config_dialog.dart';

class _FakeSkillListNotifier extends SkillListNotifier {
  @override
  Future<List<SkillDefinition>> build() async => const [
        SkillDefinition(name: 'greeter', description: 'Greets the user by name.'),
      ];
}

// A skill is used by calling it from a message ("/greeter ..."), which switches
// it on for that chat. The management dialog therefore has no on/off switch:
// it installs and removes skills and says how to use them.
void main() {
  testWidgets('lists the installed skills with how to use them, and no on/off switch',
      (tester) async {
    L10n.setLocale(MemoLocale.tr);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          apiClientProvider.overrideWithValue(MemoApiClient(baseUrl: 'http://127.0.0.1:1')),
          skillListProvider.overrideWith(_FakeSkillListNotifier.new),
        ],
        child: const MaterialApp(home: Scaffold(body: SkillConfigDialog())),
      ),
    );
    await tester.pump();

    expect(find.text(L10n.t('skill_activation_hint_settings')), findsOneWidget);
    expect(find.byType(Switch), findsNothing, reason: 'there is no per-chat toggle any more');
    expect(find.text('greeter'), findsOneWidget);
    expect(find.byIcon(Icons.delete_outline), findsOneWidget);
    expect(find.text(L10n.t('skill_load')), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('the hint explains the /name call in both languages', (tester) async {
    for (final locale in MemoLocale.values) {
      L10n.setLocale(locale);
      final hint = L10n.t('skill_activation_hint_settings');
      expect(hint, contains('/'), reason: '${locale.name}: names the "/" menu or /name call');
      expect(hint, isNot(contains('giriş çubuğundaki skill menüsü')));
      expect(hint, isNot(contains("input bar to turn one on")));
    }
  });
}
