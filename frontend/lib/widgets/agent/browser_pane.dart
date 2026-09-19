import 'dart:convert';
import 'dart:math' as math;
import 'dart:typed_data';
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/l10n.dart';
import '../../core/theme.dart';
import '../../models/browser_frame.dart';
import '../../providers/chat_provider.dart';

/// Default pane width, and the fixed size internal/browserengine/session.go
/// launches its chromedp viewport at (WindowSize(420, 900) — its own
/// comment explains why the two are picked to roughly aspect-ratio-match;
/// Go and Dart can't share a literal). Matching them is what keeps a
/// responsive page rendering the same narrow layout this pane actually
/// displays, fills the available space instead of shrinking into a corner
/// of it, AND is what makes _mapTapToViewport below correct — a tap on the
/// displayed screenshot maps back to real page coordinates only because the
/// screenshot's own pixel dimensions are this fixed, known size.
const _paneDefaultWidth = 420.0;
const _paneMinWidth = 280.0;
const _paneMaxWidth = 820.0;
const _viewportSize = Size(420, 900);

/// User-adjustable pane width — dragged via the handle on its left edge.
/// In-memory only (resets to the default next launch); not worth persisting
/// for a first version of a drag-to-resize control nobody asked to survive
/// a restart.
final browserPaneWidthProvider = StateProvider<double>((ref) => _paneDefaultWidth);

/// Opens the pane in its empty state — no backend call, nothing launched.
/// The Chromium process only actually starts once the user types a URL and
/// submits it (StartSession's own lazy-launch behavior, unchanged); showing
/// the pane by itself costs nothing. Called from chat_screen.dart's manual
/// toggle icon.
void openBrowserPane(WidgetRef ref) {
  ref.read(browserSessionActiveProvider.notifier).state = true;
}

/// Closes whatever interactive session is open (best-effort — hides the
/// pane regardless of whether the backend call succeeds, same tolerance
/// browser_close's own agent tool has) and resets pane state. Shared by
/// BrowserPane's own close button and chat_screen.dart's manual toggle icon
/// so there is exactly one "how do I close this" code path.
Future<void> closeBrowserPane(WidgetRef ref) async {
  try {
    await ref.read(apiClientProvider).closeBrowserSession();
  } catch (_) {
    // best-effort
  }
  ref.read(browserSessionActiveProvider.notifier).state = false;
  ref.read(browserCurrentUrlProvider.notifier).state = null;
  ref.read(browserFrameProvider.notifier).state = null;
}

/// Side panel showing a live view of the agent's interactive browser
/// session — a screenshot pushed over SSE each time the agent calls
/// browser_screenshot (see browserFrameProvider), next to the current URL
/// (from browser_navigate's own tool call, see browserCurrentUrlProvider).
/// The user can also drive the same session directly: type a URL in the
/// bar below the title, click/scroll on the screenshot itself — all of
/// that goes through the direct (non-agent) /api/browser/session/* HTTP
/// endpoints in internal/webserver/handlers_browser_session.go, bypassing
/// the agent permission flow entirely (nothing to ask permission for when
/// the user is the one doing it).
///
/// Mounted two ways from chat_screen.dart, both gated on
/// browserSessionActiveProvider (which the user can flip on directly via a
/// toolbar icon there, or the agent flips on itself via browser_navigate —
/// opening the pane costs nothing either way; the Chromium process only
/// actually launches once a URL is submitted, StartSession's own
/// lazy-launch behavior, unchanged): as a fixed-width, resizable third
/// column in the non-narrow Row next to the chat, or — on a narrow/mobile
/// width, where a 420px fixed column plus a mouse-drag resize handle
/// wouldn't fit or make sense on touch — as a full-width panel that
/// replaces the chat content entirely until closed. `narrow` decides which;
/// chat_screen.dart passes it down rather than this widget re-deriving its
/// own breakpoint via MediaQuery, so both stay driven by the exact same
/// LayoutBuilder constraints ChatScreen already computes `narrow` from.
class BrowserPane extends ConsumerWidget {
  final bool narrow;
  const BrowserPane({super.key, this.narrow = false});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = MemoTheme.of(context);

    if (narrow) {
      return Container(
        color: colors.bgPanel,
        child: const _BrowserPaneBody(),
      );
    }

    final width = ref.watch(browserPaneWidthProvider);
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
          child: const _BrowserPaneBody(),
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

/// Everything below the resize handle: title/close row, URL bar, busy
/// indicator, and the interactive screenshot area. A ConsumerStatefulWidget
/// (not just a build method) because it owns real local state — the URL
/// text field's controller, in-flight-request tracking so a click/scroll
/// can't fire a second HTTP call on top of one still running, an inline
/// error message, and a scroll-wheel debounce timer.
class _BrowserPaneBody extends ConsumerStatefulWidget {
  const _BrowserPaneBody();

  @override
  ConsumerState<_BrowserPaneBody> createState() => _BrowserPaneBodyState();
}

class _BrowserPaneBodyState extends ConsumerState<_BrowserPaneBody> {
  late final TextEditingController _urlController;
  bool _busy = false;
  String? _error;
  Timer? _scrollDebounce;
  double _pendingScrollDy = 0;

  @override
  void initState() {
    super.initState();
    _urlController = TextEditingController(text: ref.read(browserCurrentUrlProvider) ?? '');
  }

  @override
  void dispose() {
    _urlController.dispose();
    _scrollDebounce?.cancel();
    super.dispose();
  }

  void _applyAction(BrowserSessionAction action) {
    if (!mounted) return;
    setState(() => _error = (action.error?.isNotEmpty ?? false) ? action.error : null);
    if (action.url != null && action.url!.isNotEmpty) {
      ref.read(browserCurrentUrlProvider.notifier).state = action.url;
      if (!_urlFieldFocused) _urlController.text = action.url!;
    }
    if (action.screenshotBase64 != null && action.screenshotBase64!.isNotEmpty) {
      ref.read(browserFrameProvider.notifier).state = BrowserFrame(
        screenshotBase64: action.screenshotBase64!,
        ts: DateTime.now().millisecondsSinceEpoch,
      );
    }
  }

  bool _urlFieldFocused = false;

  Future<void> _navigate(String rawUrl) async {
    final url = rawUrl.trim();
    if (url.isEmpty || _busy) return;
    setState(() {
      _busy = true;
      _error = null;
    });
    ref.read(browserSessionActiveProvider.notifier).state = true;
    try {
      final action = await ref.read(apiClientProvider).navigateBrowserSession(url);
      _applyAction(action);
    } catch (e) {
      if (mounted) setState(() => _error = e.toString());
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _click(double x, double y) async {
    if (_busy || ref.read(browserFrameProvider) == null) return;
    setState(() => _busy = true);
    try {
      final action = await ref.read(apiClientProvider).clickBrowserSession(x, y);
      _applyAction(action);
    } catch (e) {
      if (mounted) setState(() => _error = e.toString());
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  void _onScroll(double dy) {
    if (ref.read(browserFrameProvider) == null) return;
    _pendingScrollDy += dy;
    _scrollDebounce?.cancel();
    _scrollDebounce = Timer(const Duration(milliseconds: 200), () async {
      final dy = _pendingScrollDy;
      _pendingScrollDy = 0;
      if (_busy) return;
      setState(() => _busy = true);
      try {
        final action = await ref.read(apiClientProvider).scrollBrowserSession(0, dy.round());
        _applyAction(action);
      } catch (_) {
        // Scroll failures are silent — not worth an error banner for a
        // gesture this transient; the next successful action clears any
        // stale state anyway.
      } finally {
        if (mounted) setState(() => _busy = false);
      }
    });
  }

  Future<void> _close() async {
    setState(() => _busy = true);
    await closeBrowserPane(ref);
  }

  @override
  Widget build(BuildContext context) {
    final frame = ref.watch(browserFrameProvider);
    final colors = MemoTheme.of(context);

    // Keep the URL bar in sync with agent-driven navigation, but only when
    // the user isn't actively typing in it — otherwise every agent_event
    // update would yank the cursor out from under a manual edit in
    // progress.
    ref.listen<String?>(browserCurrentUrlProvider, (prev, next) {
      if (!_urlFieldFocused && next != null && next != _urlController.text) {
        _urlController.text = next;
      }
    });

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _BrowserPaneHeader(onClose: _close),
        Padding(
          padding: const EdgeInsets.fromLTRB(14, 0, 14, 10),
          child: Focus(
            onFocusChange: (has) => _urlFieldFocused = has,
            child: TextField(
              controller: _urlController,
              onSubmitted: _navigate,
              style: TextStyle(fontSize: 12.5, color: colors.textMain),
              decoration: InputDecoration(
                isDense: true,
                hintText: L10n.t('browser_pane_url_hint'),
                hintStyle: TextStyle(fontSize: 12.5, color: colors.textDim),
                contentPadding: const EdgeInsets.symmetric(horizontal: 12, vertical: 9),
                filled: true,
                fillColor: colors.bgApp,
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(8),
                  borderSide: BorderSide(color: colors.borderSoft),
                ),
                suffixIcon: IconButton(
                  icon: Icon(Icons.arrow_forward, size: 16, color: colors.textDim),
                  onPressed: () => _navigate(_urlController.text),
                ),
              ),
            ),
          ),
        ),
        if (_busy) const LinearProgressIndicator(minHeight: 2),
        if (_error != null)
          Padding(
            padding: const EdgeInsets.fromLTRB(14, 0, 14, 8),
            child: Text(
              _error!,
              style: TextStyle(color: MemoTheme.red, fontSize: 11.5),
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
            ),
          ),
        Divider(height: 1, color: colors.borderSoft),
        Expanded(
          child: Padding(
            padding: const EdgeInsets.all(14),
            child: frame == null
                ? const _BrowserPaneEmpty()
                : _BrowserPaneFrame(
                    base64: frame.screenshotBase64,
                    onTapAt: _click,
                    onScroll: _onScroll,
                  ),
          ),
        ),
      ],
    );
  }
}

class _BrowserPaneHeader extends StatelessWidget {
  final VoidCallback onClose;
  const _BrowserPaneHeader({required this.onClose});

  @override
  Widget build(BuildContext context) {
    final colors = MemoTheme.of(context);
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 14, 8, 10),
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
            child: Text(
              L10n.t('browser_pane_title'),
              style: TextStyle(
                color: colors.textMain,
                fontSize: 13,
                fontWeight: FontWeight.w600,
              ),
            ),
          ),
          IconButton(
            icon: Icon(Icons.close, size: 18, color: colors.textDim),
            tooltip: L10n.t('browser_pane_close'),
            onPressed: onClose,
          ),
        ],
      ),
    );
  }
}

/// The screenshot's own "device frame" — a rounded, bordered, subtly
/// shadowed card that reads as an intentional preview surface rather than
/// a bare image floating on the panel background. Shared by the empty
/// state and the actual frame so the pane doesn't visually jump between
/// "nothing" and "a screenshot" — same frame, different contents.
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
  const _BrowserPaneEmpty();

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
              Icon(Icons.language, size: 28, color: colors.textDim),
              const SizedBox(height: 12),
              Text(
                L10n.t('browser_pane_empty'),
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

/// Maps a tap position in the displayed (possibly letterboxed) image widget
/// back to real page coordinates in the chromedp viewport — see
/// _viewportSize's doc comment for why this is correct only because the
/// screenshot's actual pixel size is that fixed, known constant. Returns
/// null for a tap that landed in the letterbox padding around the image
/// (box aspect ratio not exactly matching the viewport's), not on it.
Offset? _mapTapToViewport(Offset local, Size box) {
  final scale = math.min(box.width / _viewportSize.width, box.height / _viewportSize.height);
  final displayed = Size(_viewportSize.width * scale, _viewportSize.height * scale);
  final offsetX = (box.width - displayed.width) / 2;
  final offsetY = (box.height - displayed.height) / 2;
  final x = local.dx - offsetX;
  final y = local.dy - offsetY;
  if (x < 0 || y < 0 || x > displayed.width || y > displayed.height) return null;
  return Offset(x / scale, y / scale);
}

class _BrowserPaneFrame extends StatelessWidget {
  final String base64;
  final void Function(double x, double y) onTapAt;
  final void Function(double dy) onScroll;
  const _BrowserPaneFrame({required this.base64, required this.onTapAt, required this.onScroll});

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
    // against; without them (an early version wrapped this in an
    // InteractiveViewer, which lays its child out with unbounded
    // constraints for panning) the image rendered at close to its native
    // pixel size instead of filling the available card.
    //
    // pngBytes (not just reusing `bytes`) matters: Dart doesn't promote a
    // nullable local's type across a closure boundary (LayoutBuilder's
    // builder below is one), even after an early-return null check right
    // above — a fresh non-nullable local sidesteps that entirely.
    final pngBytes = bytes;
    return _FrameCard(
      child: LayoutBuilder(
        builder: (context, constraints) {
          final box = constraints.biggest;
          return MouseRegion(
            cursor: SystemMouseCursors.click,
            child: Listener(
              onPointerSignal: (event) {
                if (event is PointerScrollEvent) onScroll(event.scrollDelta.dy);
              },
              child: GestureDetector(
                onTapUp: (details) {
                  final point = _mapTapToViewport(details.localPosition, box);
                  if (point != null) onTapAt(point.dx, point.dy);
                },
                child: SizedBox.expand(
                  child: Image.memory(
                    pngBytes,
                    fit: BoxFit.contain,
                    gaplessPlayback: true,
                  ),
                ),
              ),
            ),
          );
        },
      ),
    );
  }
}
