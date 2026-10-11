import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/models/provider_config.dart';
import 'package:memo_flutter/models/provider_models.dart';
import 'package:memo_flutter/screens/chat_screen.dart';
import 'package:memo_flutter/widgets/model_picker.dart';

void main() {
  testWidgets(
    'legacy OpenCode provider reveals its current and additional catalog model',
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
                      ),
                      catalog: const [
                        ProviderModel(id: 'space-bunny-free'),
                        ProviderModel(id: 'second-model'),
                      ],
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
