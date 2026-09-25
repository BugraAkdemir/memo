import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:file_picker/file_picker.dart';
import 'package:cross_file/cross_file.dart';
import 'package:file_picker_platform_interface/file_picker_platform_interface.dart';
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

/// Stands in for the platform implementation behind FilePicker's static
/// methods — swappable via MockPlatformInterfaceMixin (the same escape hatch
/// the package itself uses to let each platform package install its own
/// implementation without holding the real PlatformInterface token).
///
/// file_picker 13's FilePicker is a final class with static methods, so the
/// old `FilePicker.platform = fake` seam is gone; `FilePickerPlatform.instance`
/// is the replacement. Always returns [file], regardless of the arguments.
class _FakeFilePickerPlatform extends FilePickerPlatform
    with MockPlatformInterfaceMixin {
  _FakeFilePickerPlatform(this.file);
  final PlatformFile? file;

  @override
  Future<PlatformFile?> pickFile({
    String? dialogTitle,
    String? initialDirectory,
    FileType type = FileType.any,
    List<String>? allowedExtensions,
    Function(FilePickerStatus)? onFileLoading,
    int compressionQuality = 0,
    AndroidOptions androidOptions = const AndroidOptions(),
    DarwinOptions darwinOptions = const DarwinOptions(),
    WindowsOptions windowsOptions = const WindowsOptions(),
    LinuxOptions linuxOptions = const LinuxOptions(),
    WebOptions webOptions = const WebOptions(),
  }) async {
    return file;
  }
}

/// A picked file that lives only in memory, with no path — the shape
/// file_picker's web backend produces (and, since 13, an Android
/// content:// pick too). PlatformFile is abstract now, so the test has to
/// bring its own instead of constructing one with a `bytes:` argument.
final class _InMemoryPlatformFile extends PlatformFile {
  _InMemoryPlatformFile({required this.name, required this.bytes});

  @override
  final String name;
  final Uint8List bytes;

  /// A data: URI, so `path` (which is `uri.scheme == 'file' ? ... : null`)
  /// correctly reports null — exactly the no-path case under test.
  @override
  Uri get uri => Uri.dataFromBytes(bytes);

  @override
  XFile get xFile => XFile.fromData(bytes, name: name, length: bytes.length);

  @override
  int? lengthSync() => bytes.length;

  @override
  Future<int?> length() async => bytes.length;

  @override
  Future<Uint8List> readAsBytes() async => bytes;

  @override
  Stream<Uint8List> readAsByteStream() => Stream.value(bytes);
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
/// MockPlatformInterfaceMixin swapping FilePickerPlatform.instance for a
/// fake that hands back a fixed result.
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
    // No path, bytes only — exactly what file_picker's web backend hands
    // back, and what the old path-only gate in chat_input.dart dropped.
    FilePickerPlatform.instance = _FakeFilePickerPlatform(
      _InMemoryPlatformFile(name: 'screenshot.png', bytes: _tinyPngBytes),
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
    FilePickerPlatform.instance = _FakeFilePickerPlatform(
      _InMemoryPlatformFile(name: 'empty.png', bytes: Uint8List(0)),
    );

    await pumpChatInput(tester);
    await tester.tap(find.byIcon(Icons.image_outlined));
    await tester.pumpAndSettle();

    expect(tester.takeException(), isNull);
    expect(find.text('empty.png'), findsNothing);
  });
}
