import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/providers/auth_gate_provider.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/settings_provider.dart';
import 'package:memo_flutter/widgets/chat_input.dart';

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
