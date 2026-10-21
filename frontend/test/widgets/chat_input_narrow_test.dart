import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/providers/auth_gate_provider.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/settings_provider.dart';
import 'package:memo_flutter/widgets/chat_input.dart';
import 'package:memo_flutter/widgets/prompt_templates.dart';

/// Answers 401 for everything — this suite only cares about layout, and a
/// gated backend keeps every provider on its safe default instead of
/// hanging on a real network call.
class _UnauthorizedAdapter implements HttpClientAdapter {
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    return ResponseBody.fromString(
      '{"error":"unauthorized"}',
      401,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

/// Reported live as "there is no text box" on a phone-width viewport: the
/// composer laid its 4-6 fixed-width icon buttons, their gaps, and the 42px
/// send button out in the same Row as the Expanded text field. Those fixed
/// children take ~225px no matter how little width is available, so the
/// field was squeezed down to ~60-80px and its hint wrapped one character
/// per line — a tall, unusable sliver that read as a missing input.
void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  Future<ProviderContainer> harness() async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    final client = MemoApiClient(baseUrl: 'http://memo.test');
    client.dio.httpClientAdapter = _UnauthorizedAdapter();
    final container = ProviderContainer(overrides: [
      apiClientProvider.overrideWithValue(client),
      prefsProvider.overrideWithValue(prefs),
      authGateProvider.overrideWith(
        (ref) => Stream.value(
          AuthGateInfo(AuthGateState.ok, authMode: 'password'),
        ),
      ),
    ]);
    addTearDown(container.dispose);
    return container;
  }

  Future<void> pumpAt(WidgetTester tester, Size size) async {
    final container = await harness();
    tester.view.physicalSize = size;
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(UncontrolledProviderScope(
      container: container,
      child: const MaterialApp(
        home: Scaffold(
          // Bottom-aligned so the composer sits where it really does, and
          // the rest of the column doesn't fight it for vertical space.
          body: Column(
            mainAxisAlignment: MainAxisAlignment.end,
            children: [ChatInput()],
          ),
        ),
      ),
    ));
    await tester.pumpAndSettle();
  }

  /// The rendered width of the message TextField, found via its hint.
  double fieldWidth(WidgetTester tester) {
    final hint = find.text('${L10n.t('type_message')} (/)');
    expect(hint, findsOneWidget);
    return tester.getSize(find.ancestor(
      of: hint,
      matching: find.byType(TextField),
    )).width;
  }

  testWidgets(
      'slash arrows keep the selection visible and Enter inserts it without losing focus',
      (tester) async {
    await pumpAt(tester, const Size(1000, 800));
    final field = find.byType(TextField);
    await tester.tap(field);
    final before = tester.state<EditableTextState>(find.byType(EditableText));
    await tester.enterText(field, '/');
    await tester.pumpAndSettle();
    for (var i = 0; i < 7; i++) {
      await tester.sendKeyEvent(LogicalKeyboardKey.arrowDown);
      await tester.pumpAndSettle();
      expect(tester.widget<PromptTemplatesPopup>(find.byType(PromptTemplatesPopup)).selectedIndex, i + 1);
    }
    final popup = tester.getRect(find.byType(PromptTemplatesPopup));
    final selected = tester.getRect(find.text('/translate'));
    expect(selected.top, greaterThanOrEqualTo(popup.top));
    expect(selected.bottom, lessThanOrEqualTo(popup.bottom));
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await tester.pumpAndSettle();
    expect(tester.widget<TextField>(field).controller!.text, templates[7].text);
    expect(identical(tester.state<EditableTextState>(find.byType(EditableText)), before), isTrue);
    expect(tester.widget<EditableText>(find.byType(EditableText)).focusNode.hasFocus, isTrue);
  });

  testWidgets(
      'at a phone width the message field still gets a usable width '
      '(regression: fixed-width icon + send buttons in the same Row left it '
      'a ~60px sliver with the hint wrapping one character per line)',
      (tester) async {
    await pumpAt(tester, const Size(375, 812));

    expect(tester.takeException(), isNull);
    // Comfortably wider than the ~60-80px sliver the bug produced, while
    // still well under the 375px viewport so this can't pass trivially.
    expect(fieldWidth(tester), greaterThan(200));
  });

  testWidgets(
      'typing "@" opens the file list and the text field KEEPS the keyboard '
      '(regression: the popup was inserted as the first child of the composer\'s '
      'Column, which re-created the input bar and dropped its focus — the person '
      'had to click the box again to keep typing)',
      (tester) async {
    await pumpAt(tester, const Size(1000, 800));
    final field = find.byType(TextField);
    await tester.tap(field);
    await tester.pump();
    bool focused() => tester.widget<EditableText>(find.byType(EditableText)).focusNode.hasFocus;
    expect(focused(), isTrue, reason: 'setup: tapping the field focuses it');
    // The actual symptom: a re-created text field has a NEW EditableTextState
    // that never opens the platform keyboard connection (its focus node already
    // "has focus", so no focus change fires), so typing does nothing until the
    // person clicks the box again. The same state object must survive.
    final before = tester.state<EditableTextState>(find.byType(EditableText));

    await tester.enterText(field, '@');
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 50));

    // The popup is open ...
    expect(find.text(L10n.t('file_mention_none')), findsOneWidget);
    // ... and the very same text field is still there, with focus, so the next
    // key goes into it.
    expect(identical(tester.state<EditableTextState>(find.byType(EditableText)), before), isTrue,
        reason: 'the text field was re-created when the popup opened');
    expect(focused(), isTrue);
    await tester.enterText(field, '@src');
    await tester.pump();
    expect(tester.widget<TextField>(field).controller!.text, '@src');
    expect(focused(), isTrue);
  });

  testWidgets('at a desktop width the composer still lays out in one row',
      (tester) async {
    await pumpAt(tester, const Size(1400, 900));

    expect(tester.takeException(), isNull);

    // Wide layout keeps the icons beside the field, not stacked above it:
    // the field's vertical center should line up with the send button's.
    final fieldCenter = tester
        .getCenter(find.ancestor(
          of: find.text('${L10n.t('type_message')} (/)'),
          matching: find.byType(TextField),
        ))
        .dy;
    final sendCenter = tester.getCenter(find.byIcon(Icons.send_rounded)).dy;
    expect((fieldCenter - sendCenter).abs(), lessThan(30));
  });
}
