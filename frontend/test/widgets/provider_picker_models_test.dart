import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/models/provider_config.dart';
import 'package:memo_flutter/models/provider_models.dart';
import 'package:memo_flutter/screens/chat_screen.dart';
import 'package:memo_flutter/widgets/model_picker.dart';

void main() {
  test('ordinary providers expose only saved IDs and their current model', () {
    for (final type in ['cline', 'kilo', 'opencode-zen']) {
      final entries = providerModelPickerEntries(
        ProviderConfig(
          type: type,
          name: type,
          model: 'current-model',
          models: const [ConfiguredModel(id: 'saved-model')],
          enabled: true,
        ),
      );
      expect(
        entries
            .where((entry) => entry.value != null)
            .map((entry) => decodeModelSelection(entry.value!)?.model),
        ['saved-model', 'current-model'],
      );
    }
  });

  test('legacy provider keeps its single current model without duplicates', () {
    final entries = providerModelPickerEntries(
      const ProviderConfig(
        type: 'cline',
        name: 'Cline',
        model: 'current-model',
        enabled: true,
      ),
      isActive: true,
    );
    expect(entries.where((entry) => entry.value != null).length, 1);
    expect(entries.last.active, isTrue);
  });

  testWidgets(
    'OpenCode provider shows and switches between explicitly saved models',
    (tester) async {
      resetModelPickerFolds();
      String? picked;
      await tester.pumpWidget(
        MaterialApp(
          home: Builder(
            builder: (context) => Scaffold(
              body: TextButton(
                onPressed: () async {
                  picked = await showModelPicker(
                    context: context,
                    anchor: const Rect.fromLTWH(350, 10, 100, 30),
                    entries: providerModelPickerEntries(
                      const ProviderConfig(
                        type: 'opencode-zen',
                        name: 'OpenCode Zen',
                        model: 'space-bunny-free',
                        enabled: true,
                        models: [
                          ConfiguredModel(id: 'space-bunny-free'),
                          ConfiguredModel(id: 'second-model'),
                        ],
                      ),
                      isActive: true,
                    ),
                  );
                },
                child: const Icon(Icons.expand_more),
              ),
            ),
          ),
        ),
      );
      await tester.tap(find.byType(TextButton));
      await tester.pumpAndSettle();
      expect(find.text('space-bunny-free'), findsOneWidget);
      expect(find.text('second-model'), findsOneWidget);
      await tester.tap(find.text('second-model'));
      await tester.pumpAndSettle();
      final selected = decodeModelSelection(picked!);
      expect(selected?.provider, 'OpenCode Zen');
      expect(selected?.model, 'second-model');
      expect(tester.takeException(), isNull);
    },
  );
}
