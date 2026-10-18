import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/providers/auth_gate_provider.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/settings_provider.dart';
import 'package:memo_flutter/widgets/chat_input.dart';

/// Serves the file list behind "@" and 401s everything else.
class _FilesAdapter implements HttpClientAdapter {
  @override
  Future<ResponseBody> fetch(RequestOptions options, Stream<Uint8List>? requestStream, Future<void>? cancelFuture) async {
    if (options.path.contains('/api/files/mentions')) {
      return ResponseBody.fromString(
        jsonEncode({'files': ['AGENTS.md', 'LICENSE', 'README.md', 'src/main.go']}),
        200,
        headers: {Headers.contentTypeHeader: [Headers.jsonContentType]},
      );
    }
    return ResponseBody.fromString('{"error":"unauthorized"}', 401,
        headers: {Headers.contentTypeHeader: [Headers.jsonContentType]});
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  Future<void> pump(WidgetTester tester) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    final client = MemoApiClient(baseUrl: 'http://memo.test');
    client.dio.httpClientAdapter = _FilesAdapter();
    final container = ProviderContainer(overrides: [
      apiClientProvider.overrideWithValue(client),
      prefsProvider.overrideWithValue(prefs),
      activeChatCLIWorkdirProvider.overrideWith((ref) async => '/proj'),
      authGateProvider.overrideWith(
        (ref) => Stream.value(AuthGateInfo(AuthGateState.ok, authMode: 'password')),
      ),
    ]);
    addTearDown(container.dispose);
    tester.view.physicalSize = const Size(1000, 800);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(UncontrolledProviderScope(
      container: container,
      child: const MaterialApp(
        home: Scaffold(
          body: Column(mainAxisAlignment: MainAxisAlignment.end, children: [ChatInput()]),
        ),
      ),
    ));
    await tester.pumpAndSettle();
  }

  bool focused(WidgetTester t) => t.widget<EditableText>(find.byType(EditableText)).focusNode.hasFocus;

  // Reported live: pick a file under "@" with the arrow keys and Tab — the name
  // is written, and then the text box lets go of the keyboard.
  for (final key in [LogicalKeyboardKey.tab, LogicalKeyboardKey.enter]) {
    testWidgets('choosing a file with ${key.keyLabel} writes it and keeps the keyboard', (tester) async {
      await pump(tester);
      final field = find.byType(TextField);
      await tester.tap(field);
      await tester.pump();
      final before = tester.state<EditableTextState>(find.byType(EditableText));

      await tester.enterText(field, '@');
      await tester.pumpAndSettle();
      expect(find.text('LICENSE'), findsOneWidget, reason: 'the file list is open');

      await tester.sendKeyEvent(LogicalKeyboardKey.arrowDown);
      await tester.pump();
      await tester.sendKeyEvent(key);
      await tester.pumpAndSettle();

      final text = tester.widget<TextField>(field).controller!.text;
      expect(text, '@LICENSE ');
      expect(find.text('LICENSE'), findsNothing, reason: 'the list closed');
      expect(identical(tester.state<EditableTextState>(find.byType(EditableText)), before), isTrue);
      expect(focused(tester), isTrue, reason: 'the text box lost the keyboard after the choice');
      // And typing carries on right after the inserted name.
      expect(tester.widget<TextField>(field).controller!.selection.baseOffset, text.length);
    });
  }
}
