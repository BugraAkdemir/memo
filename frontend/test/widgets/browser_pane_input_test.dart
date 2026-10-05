import 'dart:convert';
import 'dart:io' show ZLibEncoder;
import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/models/browser_frame.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/widgets/agent/browser_pane.dart';

/// A real 1x1 PNG.
const _png1x1 =
    'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==';

/// The 24-byte head of a PNG claiming [w] x [h] — all pngPixelSize reads.
Uint8List _pngHead(int w, int h) {
  final b = Uint8List(24);
  b.setAll(0, [0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0, 0, 0, 13, 0x49, 0x48, 0x44, 0x52]);
  final bd = ByteData.sublistView(b);
  bd.setUint32(16, w);
  bd.setUint32(20, h);
  return b;
}

/// A real, decodable, all-white grayscale PNG of [w] x [h].
Uint8List _whitePng(int w, int h) {
  int crc(List<int> bytes) {
    var c = 0xFFFFFFFF;
    for (final b in bytes) {
      c ^= b;
      for (var k = 0; k < 8; k++) {
        c = (c & 1) != 0 ? 0xEDB88320 ^ (c >> 1) : c >> 1;
      }
    }
    return c ^ 0xFFFFFFFF;
  }

  List<int> chunk(String type, List<int> data) {
    final t = ascii.encode(type);
    final len = ByteData(4)..setUint32(0, data.length);
    final c = ByteData(4)..setUint32(0, crc([...t, ...data]));
    return [...len.buffer.asUint8List(), ...t, ...data, ...c.buffer.asUint8List()];
  }

  final ihdr = ByteData(13)
    ..setUint32(0, w)
    ..setUint32(4, h)
    ..setUint8(8, 8) // bit depth
    ..setUint8(9, 0); // grayscale
  final raw = <int>[];
  for (var y = 0; y < h; y++) {
    raw.add(0); // filter: none
    raw.addAll(List.filled(w, 255));
  }
  return Uint8List.fromList([
    0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
    ...chunk('IHDR', ihdr.buffer.asUint8List()),
    ...chunk('IDAT', ZLibEncoder().convert(raw)),
    ...chunk('IEND', const []),
  ]);
}

/// Records where the pane asked the backend to click.
class _RecordingClient extends MemoApiClient {
  _RecordingClient() : super(baseUrl: 'http://127.0.0.1:1');
  Offset? clickedAt;

  @override
  Future<BrowserSessionAction> clickBrowserSession(double x, double y) async {
    clickedAt = Offset(x, y);
    return const BrowserSessionAction();
  }
}

void main() {
  test('pngPixelSize reads the real size and rejects non-PNGs', () {
    expect(pngPixelSize(_pngHead(500, 757)), const Size(500, 757));
    expect(pngPixelSize(base64Decode(_png1x1)), const Size(1, 1));
    expect(pngPixelSize(Uint8List.fromList([1, 2, 3])), isNull);
  });

  // Reproduces the live failure: the backend produced 500x757 screenshots
  // while the pane mapped taps as if they were 420x900, so a tap on the
  // centre of a button at page (53, 150) reached the page at about
  // (11, 219) — below the button, which then never got clicked.
  test('a tap maps back to the page through the image\'s real size', () {
    const box = Size(392, 700);
    const image = Size(500, 757);
    const page = Offset(53, 150);
    final scale = [box.width / image.width, box.height / image.height].reduce((a, b) => a < b ? a : b);
    final offY = (box.height - image.height * scale) / 2;
    final offX = (box.width - image.width * scale) / 2;
    final tap = Offset(offX + page.dx * scale, offY + page.dy * scale);

    final mapped = mapTapToViewportForTest(tap, box, image)!;
    expect((mapped - page).distance, lessThan(0.5), reason: 'mapped to $mapped, want $page');

    // What the old constant-size mapping did with the same tap.
    final old = mapTapToViewportForTest(tap, box, null)!;
    expect((old - page).distance, greaterThan(40));
  });

  testWidgets('the keyboard row appears once there is a page to type into', (tester) async {
    L10n.setLocale(MemoLocale.en);
    final container = ProviderContainer(overrides: [
      apiClientProvider.overrideWithValue(MemoApiClient(baseUrl: 'http://127.0.0.1:1')),
    ]);
    addTearDown(container.dispose);
    container.read(browserSessionActiveProvider.notifier).state = true;

    await tester.pumpWidget(UncontrolledProviderScope(
      container: container,
      child: const MaterialApp(home: Scaffold(body: BrowserPane(narrow: true))),
    ));
    await tester.pump();
    expect(find.byKey(const Key('browser_pane_type_field')), findsNothing);

    container.read(browserFrameProvider.notifier).state = BrowserFrame(screenshotBase64: _png1x1, ts: 1);
    await tester.pump();
    expect(find.byKey(const Key('browser_pane_type_field')), findsOneWidget);
    expect(find.text(L10n.t('browser_pane_type_hint')), findsOneWidget);
  });

  // The wiring, not just the math: a real tap on the pane, over a screenshot
  // whose size is NOT the 420x900 constant, must reach the backend as the
  // page coordinates of what was under the finger.
  testWidgets('tapping the screenshot clicks the page where the finger was', (tester) async {
    L10n.setLocale(MemoLocale.en);
    tester.view.physicalSize = const Size(420, 900);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    final client = _RecordingClient();
    final container = ProviderContainer(overrides: [apiClientProvider.overrideWithValue(client)]);
    addTearDown(container.dispose);
    container.read(browserSessionActiveProvider.notifier).state = true;
    container.read(browserFrameProvider.notifier).state =
        BrowserFrame(screenshotBase64: base64Encode(_whitePng(500, 757)), ts: 1);

    await tester.pumpWidget(UncontrolledProviderScope(
      container: container,
      child: const MaterialApp(home: Scaffold(body: BrowserPane(narrow: true))),
    ));
    await tester.pumpAndSettle();

    final imageBox = tester.getRect(find.byType(Image));
    const image = Size(500, 757);
    const page = Offset(53, 150);
    final scale = [imageBox.width / image.width, imageBox.height / image.height].reduce((a, b) => a < b ? a : b);
    final shown = Size(image.width * scale, image.height * scale);
    final topLeft = imageBox.topLeft + Offset((imageBox.width - shown.width) / 2, (imageBox.height - shown.height) / 2);
    await tester.tapAt(topLeft + page * scale);
    await tester.pump();

    expect(client.clickedAt, isNotNull);
    expect((client.clickedAt! - page).distance, lessThan(1.0),
        reason: 'clicked at ${client.clickedAt}, want $page');
  });
}
