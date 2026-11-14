import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/screens/oobe/korteks_oobe_screen.dart';

void main() {
  testWidgets('KorteksOobeScreen walks through 5 steps and submits config',
      (tester) async {
    tester.view.physicalSize = const Size(1280, 900);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    KorteksOobeConfig? submitted;
    var completedCalls = 0;

    await tester.pumpWidget(
      MaterialApp(
        home: KorteksOobeScreen(
          onSubmitConfig: (cfg) async {
            submitted = cfg;
          },
          onCompleted: () {
            completedCalls++;
          },
        ),
      ),
    );
    await tester.pumpAndSettle();

    // Step 1: Hardware & APU Boost
    expect(find.text('Adım 1/5'), findsOneWidget);
    expect(find.byKey(const Key('oobe_apu_boost_switch')), findsOneWidget);
    await tester.tap(find.byKey(const Key('oobe_next_btn')));
    await tester.pumpAndSettle();

    // Step 2: Model selection
    expect(find.text('Adım 2/5'), findsOneWidget);
    await tester.tap(find.byKey(const Key('oobe_model_Llama-3.2-3B-Instruct-Q4_K_M')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('oobe_next_btn')));
    await tester.pumpAndSettle();

    // Step 3: Ring Mode
    expect(find.text('Adım 3/5'), findsOneWidget);
    await tester.tap(find.byKey(const Key('oobe_ring_autonomous')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('oobe_next_btn')));
    await tester.pumpAndSettle();

    // Step 4: Distrobox stacks (select rust too)
    expect(find.text('Adım 4/5'), findsOneWidget);
    await tester.tap(find.byKey(const Key('oobe_stack_rust')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('oobe_next_btn')));
    await tester.pumpAndSettle();

    // Step 5: Voice & Submit
    expect(find.text('Adım 5/5'), findsOneWidget);
    await tester.tap(find.byKey(const Key('oobe_finish_btn')));
    await tester.pumpAndSettle();

    expect(submitted, isNotNull);
    expect(submitted!.apuBoost, isTrue);
    expect(submitted!.selectedModel, 'Llama-3.2-3B-Instruct-Q4_K_M');
    expect(submitted!.ringMode, 'autonomous');
    expect(submitted!.distroboxStacks, containsAll(['go', 'python-ai', 'rust']));
    expect(submitted!.voiceEnabled, isTrue);
    expect(completedCalls, 1);
  });
}
