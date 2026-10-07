import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/providers/chat_provider.dart' show apiClientProvider;
import 'package:memo_flutter/widgets/chat_image.dart';
import 'package:memo_flutter/widgets/image_viewer.dart';

const _png1x1 =
    'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==';
final _pngBytes = base64Decode(_png1x1);

class _ImageAdapter implements HttpClientAdapter {
  @override
  Future<ResponseBody> fetch(RequestOptions options, Stream<Uint8List>? requestStream,
          Future<void>? cancelFuture) async =>
      ResponseBody.fromString(jsonEncode({'data': 'data:image/png;base64,$_png1x1'}), 200,
          headers: {Headers.contentTypeHeader: [Headers.jsonContentType]});
  @override
  void close({bool force = false}) {}
}

Future<void> _pumpViewer(WidgetTester tester, {ImageSaver? saver, MemoLocale locale = MemoLocale.en}) async {
  L10n.setLocale(locale);
  tester.view.physicalSize = const Size(1000, 800);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  await tester.pumpWidget(MaterialApp(
    home: Scaffold(body: ImageViewer(bytes: _pngBytes, saver: saver)),
  ));
  await tester.pump();
}

String _percent(WidgetTester tester) => tester.widget<Text>(find.byKey(const Key('image_zoom_percent'))).data!;

void main() {
  group('helpers', () {
    test('sniffImageFormat reads the signature, not a name', () {
      expect(sniffImageFormat(_pngBytes).label, 'PNG');
      expect(sniffImageFormat(Uint8List.fromList([0xFF, 0xD8, 0xFF, 0xE0, 0, 0])).label, 'JPEG');
      expect(sniffImageFormat(Uint8List.fromList('GIF89a....'.codeUnits)).label, 'GIF');
      expect(sniffImageFormat(Uint8List.fromList('RIFF\x00\x00\x00\x00WEBPVP8 '.codeUnits)).label, 'WebP');
      expect(sniffImageFormat(Uint8List.fromList('RIFF\x00\x00\x00\x00WAVEfmt '.codeUnits)).label, '?');
      expect(sniffImageFormat(Uint8List(0)).label, '?');
      expect(sniffImageFormat(Uint8List.fromList([1, 2, 3])).label, '?');
    });

    test('formatByteSize and the default file name', () {
      expect(formatByteSize(812), '812 B');
      expect(formatByteSize(35020), '34.2 KB');
      expect(formatByteSize(1572864), '1.5 MB');
      expect(defaultImageFileName(ImageFormat.jpeg, DateTime(2026, 10, 8, 3, 4, 5)), 'memo-image-20261008-030405.jpg');
    });
  });

  group('viewer', () {
    testWidgets('zoom buttons, reset, limits and the keyboard', (tester) async {
      await _pumpViewer(tester);
      expect(_percent(tester), '100%');

      await tester.tap(find.byKey(const Key('image_zoom_in')));
      await tester.pump();
      expect(_percent(tester), '125%');
      await tester.tap(find.byKey(const Key('image_zoom_out')));
      await tester.tap(find.byKey(const Key('image_zoom_out')));
      await tester.pump();
      expect(_percent(tester), '80%');

      await tester.tap(find.byKey(const Key('image_zoom_reset')));
      await tester.pump();
      expect(_percent(tester), '100%');

      for (var i = 0; i < 20; i++) {
        await tester.tap(find.byKey(const Key('image_zoom_in')));
      }
      await tester.pump();
      expect(_percent(tester), '800%', reason: 'zoom stops at the maximum');
      for (var i = 0; i < 30; i++) {
        await tester.tap(find.byKey(const Key('image_zoom_out')));
      }
      await tester.pump();
      expect(_percent(tester), '50%', reason: 'zoom stops at the minimum');

      await tester.tap(find.byKey(const Key('image_zoom_reset')));
      await tester.sendKeyEvent(LogicalKeyboardKey.equal);
      await tester.pump();
      expect(_percent(tester), '125%');
      await tester.sendKeyEvent(LogicalKeyboardKey.minus);
      await tester.pump();
      expect(_percent(tester), '100%');
      await tester.sendKeyEvent(LogicalKeyboardKey.equal);
      await tester.sendKeyEvent(LogicalKeyboardKey.digit0);
      await tester.pump();
      expect(_percent(tester), '100%');
    });

    testWidgets('double tap zooms in and back out', (tester) async {
      await _pumpViewer(tester);
      final surface = find.byKey(const Key('image_viewer_surface'));
      await tester.tap(surface);
      await tester.pump(const Duration(milliseconds: 40));
      await tester.tap(surface);
      await tester.pumpAndSettle();
      expect(_percent(tester), '300%');

      await tester.tap(surface);
      await tester.pump(const Duration(milliseconds: 40));
      await tester.tap(surface);
      await tester.pumpAndSettle();
      expect(_percent(tester), '100%');
    });

    testWidgets('details show format, size and — once decoded — the resolution', (tester) async {
      await _pumpViewer(tester);
      expect(find.byKey(const Key('image_info_card')), findsNothing);
      await tester.tap(find.byKey(const Key('image_info')));
      await tester.pump();
      expect(find.byKey(const Key('image_info_card')), findsOneWidget);
      expect(find.text('PNG'), findsOneWidget);
      expect(find.text('${_pngBytes.length} B'), findsOneWidget);

      await tester.runAsync(() => Future<void>.delayed(const Duration(milliseconds: 200)));
      await tester.pump();
      expect(find.text('1 × 1'), findsOneWidget);
      expect(find.text('1:1'), findsOneWidget);

      await tester.tap(find.byKey(const Key('image_info')));
      await tester.pump();
      expect(find.byKey(const Key('image_info_card')), findsNothing);
    });

    testWidgets('download hands the picture to the saver and says where it went', (tester) async {
      String? name;
      Uint8List? got;
      await _pumpViewer(tester, saver: ({required fileName, required bytes, required format}) async {
        name = fileName;
        got = bytes;
        return Uri.file('/home/me/Pictures/$fileName');
      });
      await tester.tap(find.byKey(const Key('image_download')));
      await tester.pump();
      await tester.pump();

      expect(name, matches(RegExp(r'^memo-image-\d{8}-\d{6}\.png$')));
      expect(got, _pngBytes);
      expect(find.textContaining('/home/me/Pictures/memo-image-'), findsOneWidget);
    });

    testWidgets('a cancelled save says nothing; a failed one says why', (tester) async {
      await _pumpViewer(tester, saver: ({required fileName, required bytes, required format}) async => null);
      await tester.tap(find.byKey(const Key('image_download')));
      await tester.pump();
      await tester.pump();
      expect(find.byType(SnackBar), findsNothing);

      await _pumpViewer(tester, saver: ({required fileName, required bytes, required format}) async {
        throw Exception('disk is full');
      });
      await tester.tap(find.byKey(const Key('image_download')));
      await tester.pump();
      await tester.pump();
      expect(find.textContaining('disk is full'), findsOneWidget);
    });

    testWidgets('the toolbar speaks Turkish too', (tester) async {
      await _pumpViewer(tester, locale: MemoLocale.tr);
      // Read through L10n under the Turkish locale (a test must not hardcode either
      // language) and prove it is not the English wording.
      for (final key in ['image_zoom_in', 'image_download', 'image_info']) {
        final tr = L10n.t(key);
        expect(find.byTooltip(tr), findsOneWidget, reason: key);
        L10n.setLocale(MemoLocale.en);
        expect(L10n.t(key), isNot(tr), reason: '$key must differ between the languages');
        L10n.setLocale(MemoLocale.tr);
      }
      L10n.setLocale(MemoLocale.en);
    });
  });

  group('in the chat', () {
    testWidgets('tapping a message picture opens the viewer; the corner buttons are there', (tester) async {
      L10n.setLocale(MemoLocale.en);
      tester.view.physicalSize = const Size(1000, 900);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.reset);
      final client = MemoApiClient(baseUrl: 'http://memo.test');
      client.dio.httpClientAdapter = _ImageAdapter();
      await tester.pumpWidget(ProviderScope(
        overrides: [apiClientProvider.overrideWithValue(client)],
        child: const MaterialApp(home: Scaffold(body: ChatImage(path: '/data/generated-images/x.png'))),
      ));
      await tester.pumpAndSettle();

      expect(find.byKey(const Key('chat_image_enlarge')), findsOneWidget);
      expect(find.byKey(const Key('chat_image_download')), findsOneWidget);
      expect(find.byType(ImageViewer), findsNothing);

      // The picture only has a size (and so can be hit) once it has decoded, which
      // takes real event-loop time, not fake-clock frames.
      await tester.runAsync(() => Future<void>.delayed(const Duration(milliseconds: 300)));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('chat_image_tap')));
      await tester.pumpAndSettle();
      expect(find.byType(ImageViewer), findsOneWidget);

      await tester.tap(find.byKey(const Key('image_close')));
      await tester.pumpAndSettle();
      expect(find.byType(ImageViewer), findsNothing);

      await tester.tap(find.byKey(const Key('chat_image_enlarge')));
      await tester.pumpAndSettle();
      expect(find.byType(ImageViewer), findsOneWidget);
    });
  });
}
