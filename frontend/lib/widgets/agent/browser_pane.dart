import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/l10n.dart';
import '../../core/theme.dart';
import '../../providers/chat_provider.dart';

/// Default pane width — also what internal/browserengine/session.go's
/// WindowSize(420, 900) call is picked to roughly aspect-ratio-match (its
/// own comment explains why; Go and Dart can't share a literal). Matching
/// them is what keeps a responsive page rendering the same narrow layout
/// this pane actually displays, and keeps the screenshot filling the
/// available space instead of shrinking into a corner of it (a real,
/// visually broken first version had a 1280x800 desktop viewport crammed
/// into a much narrower, taller pane). The user can still drag the pane
/// wider or narrower at runtime (see browserPaneWidthProvider below) — this
/// is only the aspect ratio chromedp itself renders at, independent of
/// however wide the user ends up displaying it on screen.
const _paneDefaultWidth = 420.0;
const _paneMinWidth = 280.0;
const _paneMaxWidth = 820.0;

/// User-adjustable pane width — dragged via the handle on its left edge.
/// In-memory only (resets to the default next launch); not worth persisting
/// for a first version of a drag-to-resize control nobody asked to survive
/// a restart.
final browserPaneWidthProvider = StateProvider<double>((ref) => _paneDefaultWidth);

/// Side panel showing a live view of the agent's interactive browser
/// session — a screenshot pushed over SSE each time the agent calls
/// browser_screenshot (see browserFrameProvider), next to the current URL
/// (from browser_navigate's own tool call, see browserCurrentUrlProvider).
/// Mounted in chat_screen.dart's non-narrow Row as a third child, right of
/// the chat content, only while browserSessionActiveProvider is true.
///
/// Owns no subscription of its own — it only `ref.watch`es plain
/// StateProviders that chat_provider.dart's single shared SSE loop already
/// writes to, so there is nothing here to leak or dispose regardless of
/// this widget's own mount/unmount cycle (unlike a polling widget under
/// AppShell's IndexedStack, which does need to guard against that).
class BrowserPane extends ConsumerWidget {
  const BrowserPane({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final frame = ref.watch(browserFrameProvider);
    final url = ref.watch(browserCurrentUrlProvider);
    final width = ref.watch(browserPaneWidthProvider);
    final colors = MemoTheme.of(context);

    return Row(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _ResizeHandle(
          onDrag: (dx) {
            final next = (width - dx).clamp(_paneMinWidth, _paneMaxWidth);
            ref.read(browserPaneWidthProvider.notifier).state = next;
          },
        ),
        Container(
          width: width,
          decoration: BoxDecoration(color: colors.bgPanel),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              _BrowserPaneHeader(url: url),
              Divider(height: 1, color: colors.borderSoft),
              Expanded(
                child: Padding(
                  padding: const EdgeInsets.all(14),
                  child: frame == null
                      ? _BrowserPaneEmpty(hasUrl: url != null)
                      : _BrowserPaneFrame(base64: frame.screenshotBase64),
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }
}

/// A thin draggable strip on the pane's left edge — grab it to widen or
/// narrow the pane. `onDrag` receives the pointer's horizontal movement
/// since the last update (positive = moved right); BrowserPane.build turns
/// that into a new clamped width. Widens on hover/drag so it's findable
/// without being intrusive at rest.
class _ResizeHandle extends StatefulWidget {
  final ValueChanged<double> onDrag;
  const _ResizeHandle({required this.onDrag});

  @override
  State<_ResizeHandle> createState() => _ResizeHandleState();
}

class _ResizeHandleState extends State<_ResizeHandle> {
  bool _hovering = false;
  bool _dragging = false;

  @override
  Widget build(BuildContext context) {
    final colors = MemoTheme.of(context);
    final active = _hovering || _dragging;
    return MouseRegion(
      cursor: SystemMouseCursors.resizeLeftRight,
      onEnter: (_) => setState(() => _hovering = true),
      onExit: (_) => setState(() => _hovering = false),
      child: GestureDetector(
        behavior: HitTestBehavior.translucent,
        onHorizontalDragStart: (_) => setState(() => _dragging = true),
        onHorizontalDragEnd: (_) => setState(() => _dragging = false),
        onHorizontalDragCancel: () => setState(() => _dragging = false),
        onHorizontalDragUpdate: (details) => widget.onDrag(details.delta.dx),
        child: SizedBox(
          width: 10,
          child: Center(
            child: AnimatedContainer(
              duration: const Duration(milliseconds: 120),
              width: active ? 3 : 1,
              color: active ? MemoTheme.accent : colors.borderSoft,
            ),
          ),
        ),
      ),
    );
  }
}

class _BrowserPaneHeader extends StatelessWidget {
  final String? url;
  const _BrowserPaneHeader({required this.url});

  @override
  Widget build(BuildContext context) {
    final colors = MemoTheme.of(context);
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 14, 16, 14),
      child: Row(
        children: [
          Container(
            width: 30,
            height: 30,
            decoration: BoxDecoration(
              color: MemoTheme.accentMuted,
              borderRadius: BorderRadius.circular(8),
            ),
            child: Icon(Icons.public, size: 16, color: MemoTheme.accent),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  L10n.t('browser_pane_title'),
                  style: TextStyle(
                    color: colors.textMain,
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  url ?? L10n.t('browser_pane_empty'),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(color: colors.textDim, fontSize: 11.5),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// The screenshot's own "device frame" — a rounded, bordered, subtly
/// shadowed card that reads as an intentional preview surface rather than
/// a bare image floating on the panel background. Shared by the empty/
/// loading state and the actual frame so the pane doesn't visually jump
/// between "nothing" and "a screenshot" — same frame, different contents.
class _FrameCard extends StatelessWidget {
  final Widget child;
  const _FrameCard({required this.child});

  @override
  Widget build(BuildContext context) {
    final colors = MemoTheme.of(context);
    return Container(
      decoration: BoxDecoration(
        color: colors.bgApp,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: colors.borderSoft),
        boxShadow: [
          BoxShadow(
            color: Colors.black.withValues(alpha: 0.08),
            blurRadius: 12,
            offset: const Offset(0, 4),
          ),
        ],
      ),
      clipBehavior: Clip.antiAlias,
      child: child,
    );
  }
}

class _BrowserPaneEmpty extends StatelessWidget {
  final bool hasUrl;
  const _BrowserPaneEmpty({required this.hasUrl});

  @override
  Widget build(BuildContext context) {
    final colors = MemoTheme.of(context);
    return _FrameCard(
      child: Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              SizedBox(
                width: 22,
                height: 22,
                child: CircularProgressIndicator(
                  strokeWidth: 2,
                  color: colors.textDim,
                ),
              ),
              const SizedBox(height: 12),
              Text(
                L10n.t(hasUrl ? 'browser_pane_loading' : 'browser_pane_empty'),
                textAlign: TextAlign.center,
                style: TextStyle(color: colors.textDim, fontSize: 12.5),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _BrowserPaneFrame extends StatelessWidget {
  final String base64;
  const _BrowserPaneFrame({required this.base64});

  @override
  Widget build(BuildContext context) {
    Uint8List? bytes;
    try {
      bytes = base64Decode(base64);
    } catch (_) {
      bytes = null;
    }
    if (bytes == null) {
      final colors = MemoTheme.of(context);
      return _FrameCard(
        child: Center(
          child: Text(
            L10n.t('browser_pane_error'),
            style: TextStyle(color: colors.textDim, fontSize: 12.5),
          ),
        ),
      );
    }
    // SizedBox.expand — not left implicit — matters here: Image.memory
    // needs BOUNDED constraints for BoxFit.contain to actually scale
    // against; without them (the first version wrapped this in an
    // InteractiveViewer, which lays its child out with unbounded
    // constraints for panning) the image rendered at close to its native
    // pixel size instead of filling the available card, which combined
    // with the old 1280x800 desktop viewport to make the screenshot look
    // tiny with a large dead area below it.
    return _FrameCard(
      child: SizedBox.expand(
        child: Image.memory(
          bytes,
          fit: BoxFit.contain,
          gaplessPlayback: true,
        ),
      ),
    );
  }
}
