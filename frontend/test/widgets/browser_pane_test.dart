import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/models/browser_frame.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/widgets/agent/browser_pane.dart';

// A 1x1 transparent PNG, base64-encoded — small, always-valid image bytes
// for widget tests, not meant to look like anything.
const _tinyPngBase64 =
    'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=';

Future<void> _pumpBrowserPane(
  WidgetTester tester, {
  required List<Override> overrides,
}) async {
  await tester.pumpWidget(
    ProviderScope(
      overrides: overrides,
      child: const MaterialApp(
        home: Scaffold(body: BrowserPane()),
      ),
    ),
  );
}

/// Mirrors chat_screen.dart's actual layout — BrowserPane sits beside an
/// Expanded sibling that absorbs whatever width it doesn't take, not alone
/// in an unconstrained body. The resize tests need this: a standalone
/// BrowserPane in a plain Scaffold body has nothing to shrink when dragged
/// wide, so it overflows in the test even though the same drag is
/// perfectly safe in the app (a real, if test-only, RenderFlex overflow
/// this setup caught on the first attempt).
Future<WidgetRef> _pumpBrowserPaneBesideExpanded(WidgetTester tester) async {
  tester.view.physicalSize = const Size(1400, 900);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  late WidgetRef capturedRef;
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        browserFrameProvider.overrideWith((ref) => null),
        browserCurrentUrlProvider.overrideWith((ref) => null),
      ],
      child: MaterialApp(
        home: Scaffold(
          body: Row(
            children: [
              const Expanded(child: SizedBox()),
              Consumer(
                builder: (context, ref, _) {
                  capturedRef = ref;
                  return const BrowserPane();
                },
              ),
            ],
          ),
        ),
      ),
    ),
  );
  return capturedRef;
}

void main() {
  testWidgets('shows the empty state (with a URL bar) before any frame arrives', (tester) async {
    await _pumpBrowserPane(tester, overrides: [
      browserFrameProvider.overrideWith((ref) => null),
      browserCurrentUrlProvider.overrideWith((ref) => null),
    ]);

    expect(find.byIcon(Icons.public), findsOneWidget); // header icon, always present
    expect(find.byIcon(Icons.language), findsOneWidget); // empty-state body icon
    expect(find.byType(TextField), findsOneWidget); // the manual URL bar
    expect(find.byType(Image), findsNothing);
  });

  testWidgets('pre-fills the URL bar from browserCurrentUrlProvider', (tester) async {
    await _pumpBrowserPane(tester, overrides: [
      browserFrameProvider.overrideWith((ref) => null),
      browserCurrentUrlProvider.overrideWith((ref) => 'https://example.com'),
    ]);

    expect(find.text('https://example.com'), findsOneWidget);
    expect(find.byType(Image), findsNothing);
  });

  testWidgets('renders the screenshot once a frame arrives', (tester) async {
    await _pumpBrowserPane(tester, overrides: [
      browserFrameProvider.overrideWith(
        (ref) => const BrowserFrame(screenshotBase64: _tinyPngBase64, ts: 1234),
      ),
      browserCurrentUrlProvider.overrideWith((ref) => 'https://example.com/signup'),
    ]);

    expect(find.byType(Image), findsOneWidget);
    expect(find.text('https://example.com/signup'), findsOneWidget);
  });

  testWidgets('a malformed base64 frame shows an error instead of crashing', (tester) async {
    await _pumpBrowserPane(tester, overrides: [
      browserFrameProvider.overrideWith(
        (ref) => const BrowserFrame(screenshotBase64: 'not-valid-base64!!', ts: 1),
      ),
      browserCurrentUrlProvider.overrideWith((ref) => null),
    ]);

    expect(find.byType(Image), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets('narrow mode fills the available width with no resize handle', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          browserFrameProvider.overrideWith((ref) => null),
          browserCurrentUrlProvider.overrideWith((ref) => null),
        ],
        child: const MaterialApp(
          home: Scaffold(body: BrowserPane(narrow: true)),
        ),
      ),
    );

    expect(
      find.byWidgetPredicate((w) => w.runtimeType.toString() == '_ResizeHandle'),
      findsNothing,
    );
    expect(find.byType(TextField), findsOneWidget);
  });

  testWidgets('dragging the resize handle widens and narrows the pane', (tester) async {
    final capturedRef = await _pumpBrowserPaneBesideExpanded(tester);
    final initialWidth = capturedRef.read(browserPaneWidthProvider);

    // Drag left (negative dx) widens the pane — the handle sits on its
    // left edge, so moving it further left grows the pane to its right.
    final handle = find.byWidgetPredicate((w) => w.runtimeType.toString() == '_ResizeHandle');
    expect(handle, findsOneWidget);
    await tester.drag(handle, const Offset(-60, 0));
    await tester.pump();
    expect(capturedRef.read(browserPaneWidthProvider), greaterThan(initialWidth));

    // Drag back right past the starting point narrows it below the default.
    await tester.drag(handle, const Offset(200, 0));
    await tester.pump();
    expect(capturedRef.read(browserPaneWidthProvider), lessThan(initialWidth));
  });

  testWidgets('resizing clamps to a sane min/max instead of growing unbounded', (tester) async {
    final capturedRef = await _pumpBrowserPaneBesideExpanded(tester);
    final handle = find.byWidgetPredicate((w) => w.runtimeType.toString() == '_ResizeHandle');

    // Try to drag it absurdly wide.
    await tester.drag(handle, const Offset(-5000, 0));
    await tester.pump();
    final maxed = capturedRef.read(browserPaneWidthProvider);
    expect(maxed, lessThan(1000)); // clamped well below an unbounded 5000+px

    // And absurdly narrow.
    await tester.drag(handle, const Offset(5000, 0));
    await tester.pump();
    final minned = capturedRef.read(browserPaneWidthProvider);
    expect(minned, greaterThan(100)); // clamped well above 0/negative
  });
}
