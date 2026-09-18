import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:plugin_platform_interface/plugin_platform_interface.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/providers/auth_gate_provider.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/settings_provider.dart';
import 'package:memo_flutter/widgets/chat_input.dart';

/// Same reasoning as chat_input_narrow_test.dart's adapter — this suite
/// only cares about the picker/preview path, never actually sends.
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

/// Stands in for FilePicker.platform — swappable via MockPlatformInterfaceMixin
/// (the same escape hatch the file_picker package itself uses to let each
/// platform implementation replace FilePicker.platform without needing the
/// real PlatformInterface token). Always returns [result], regardless of
/// what pickFiles() was called with.
class _FakeFilePicker extends FilePicker with MockPlatformInterfaceMixin {
  _FakeFilePicker(this.result);
  final FilePickerResult? result;

  @override
  Future<FilePickerResult?> pickFiles({
    String? dialogTitle,
    String? initialDirectory,
    FileType type = FileType.any,
    List<String>? allowedExtensions,
    bool allowMultiple = false,
    void Function(FilePickerStatus)? onFileLoading,
    bool allowCompression = true,
    int compressionQuality = 20,
    bool withData = false,
    bool withReadStream = false,
    bool lockParentWindow = false,
    bool readSequential = false,
  }) async {
    return result;
  }
}

// A 1x1 red PNG — enough for Image.memory to decode without error.
final _tinyPngBytes = Uint8List.fromList([
  0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D,
  0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
  0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xDE, 0x00, 0x00, 0x00,
  0x0C, 0x49, 0x44, 0x41, 0x54, 0x08, 0xD7, 0x63, 0xF8, 0xCF, 0xC0, 0x00,
  0x00, 0x03, 0x01, 0x01, 0x00, 0x18, 0xDD, 0x8D, 0xB0, 0x00, 0x00, 0x00,
  0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
]);

/// Regression coverage for the reported bug: on Memo's web build,
/// attaching an image did nothing at all — no preview, no error, no
/// network call. Root cause: file_picker's web implementation never
/// populates PlatformFile.path (reading it there even throws — see
/// platform_file.dart's getter), but chat_input.dart's attach handlers
/// gated the whole action on `path != null`, so the picked file was
/// silently discarded before ever reaching the composer's state. This
/// test simulates exactly that shape — a PlatformFile with real bytes and
/// no path — without needing a real browser or native file dialog, via
/// MockPlatformInterfaceMixin swapping FilePicker.platform for a fake
/// that hands back a fixed result.
void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  Future<void> pumpChatInput(WidgetTester tester) async {
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

    await tester.pumpWidget(UncontrolledProviderScope(
      container: container,
      child: const MaterialApp(
        home: Scaffold(
          body: Column(
            mainAxisAlignment: MainAxisAlignment.end,
            children: [ChatInput()],
          ),
        ),
      ),
    ));
    await tester.pumpAndSettle();
  }

  testWidgets(
      'picking a bytes-only file (web shape — no path) shows the attach '
      'preview instead of silently doing nothing', (tester) async {
    FilePicker.platform = _FakeFilePicker(
      FilePickerResult([
        PlatformFile(
          name: 'screenshot.png',
          size: _tinyPngBytes.length,
          bytes: _tinyPngBytes,
          // path deliberately omitted — this is exactly what file_picker's
          // web backend hands back, and what the old path-only gate in
          // chat_input.dart silently dropped.
        ),
      ]),
    );

    await pumpChatInput(tester);
    expect(tester.takeException(), isNull);

    await tester.tap(find.byIcon(Icons.image_outlined));
    await tester.pumpAndSettle();

    expect(tester.takeException(), isNull);
    expect(find.text('screenshot.png'), findsOneWidget);
    expect(find.byType(Image), findsWidgets);
  });

  testWidgets(
      'picking a file with neither path nor bytes shows no preview '
      '(nothing usable came back — should no-op, not crash)',
      (tester) async {
    FilePicker.platform = _FakeFilePicker(
      FilePickerResult([PlatformFile(name: 'empty.png', size: 0)]),
    );

    await pumpChatInput(tester);
    await tester.tap(find.byIcon(Icons.image_outlined));
    await tester.pumpAndSettle();

    expect(tester.takeException(), isNull);
    expect(find.text('empty.png'), findsNothing);
  });
}
