import 'dart:async';
import 'dart:io' show Platform;

import 'package:desktop_multi_window/desktop_multi_window.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:window_manager/window_manager.dart';

import '../core/backend_url.dart';
import 'memo_mascot.dart';

/// Window size the character is designed against — `set_mascot_input_shape`
/// in linux/runner/my_application.cc hand-codes an ellipse (plus a small
/// rectangle for the hover close button's corner) in these exact
/// coordinates, so this and that native function must be changed together.
const _windowSize = Size(132, 148);

/// Boots the standalone floating desktop mascot in place of the normal chat
/// UI. This runs as a `desktop_multi_window` sub-window inside the SAME
/// process as the main chat window — one Memo, one running app, not a
/// second one — reached when `lib/main.dart`'s `main(args)` sees the
/// `multi_window` marker that plugin passes as the first Dart entrypoint
/// argument for any window it creates (see tray_controller.dart, which is
/// what actually calls `WindowController.create(...)` to spawn this).
///
/// Frameless, skip-taskbar and drag-anywhere all come from the plain
/// (unforked) `window_manager` package, which works here exactly as it
/// does in the main window: `desktop_multi_window` gives every sub-window
/// its own Flutter engine, and `linux/runner/my_application.cc` registers
/// window_manager's plugin for each one via the exact callback the
/// package's README documents. Always-on-top (`alwaysOnTop: true` below,
/// backed by `gtk_window_set_keep_above` in the vendored patch) has no
/// native-Wayland equivalent in plain GTK — main.cc forces the whole app
/// onto XWayland specifically so this (and the input-shape "collider"
/// fix) have the X11 mechanism to call; see main.cc's own comment. Real
/// per-pixel transparency needed one thing that callback fires too late
/// for — see
/// frontend/third_party/README.md for that one vendored native patch.
Future<void> runMascotWindow() async {
  WidgetsFlutterBinding.ensureInitialized();
  await windowManager.ensureInitialized();

  // Lets the main window ask this one to close itself (see
  // tray_controller.dart) — WindowController has no close() of its own;
  // this is the hand-off point the package's README documents for that.
  // Only reachable when this is actually a desktop_multi_window sub-window;
  // the standalone `flutter run -t lib/mascot_main.dart` dev entrypoint has
  // no such window to attach to, so this is best-effort.
  try {
    final controller = await WindowController.fromCurrentEngine();
    await controller.setWindowMethodHandler((call) async {
      if (call.method == 'window_close') {
        await windowManager.close();
      }
    });
  } catch (_) {
    // Standalone dev run — nothing to wire up, the on-window close button
    // already calls windowManager.close() directly.
  }

  const options = WindowOptions(
    size: _windowSize,
    center: true,
    backgroundColor: Colors.transparent,
    skipTaskbar: true,
    alwaysOnTop: true,
    titleBarStyle: TitleBarStyle.hidden,
  );
  await windowManager.waitUntilReadyToShow(options, () async {
    await windowManager.setAsFrameless();
    // setHasShadow isn't implemented on Linux (window_manager's Linux
    // plugin has no such method) — only call it where it exists.
    if (!Platform.isLinux) {
      await windowManager.setHasShadow(false);
    }
    await windowManager.setResizable(false);
    await windowManager.show();
    await windowManager.focus();
  });

  runApp(const MascotWindowApp());
}

/// True for a Dart entrypoint invocation that's a desktop_multi_window
/// sub-window rather than the app's normal launch — see runMascotWindow's
/// doc comment. Checked against `main(args)`'s own argument list.
bool isMascotSubWindow(List<String> args) =>
    args.isNotEmpty && args.first == 'multi_window';

class MascotWindowApp extends StatelessWidget {
  const MascotWindowApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      debugShowCheckedModeBanner: false,
      color: Colors.transparent,
      theme: ThemeData(
        scaffoldBackgroundColor: Colors.transparent,
        canvasColor: Colors.transparent,
      ),
      home: const Scaffold(
        backgroundColor: Colors.transparent,
        body: _MascotSurface(),
      ),
    );
  }
}

/// The whole window's content: drag-anywhere by default, a small close
/// button that only appears on hover (there's no OS title bar to carry one).
class _MascotSurface extends StatefulWidget {
  const _MascotSurface();

  @override
  State<_MascotSurface> createState() => _MascotSurfaceState();
}

/// How often to poll GET /api/mascot/activity. This is a decorative,
/// coarse-grained signal (see models.ActivityStatus on the Go side) — a
/// second of lag between a tool call starting and the mascot noticing
/// doesn't matter the way it would for, say, streamed chat tokens.
const _pollInterval = Duration(milliseconds: 1200);

class _MascotSurfaceState extends State<_MascotSurface> {
  bool _hovering = false;
  MascotMood _mood = MascotMood.idle;
  Timer? _pollTimer;
  Dio? _dio;

  @override
  void initState() {
    super.initState();
    _startPolling();
  }

  Future<void> _startPolling() async {
    // Same SharedPreferences store the main chat window reads/writes
    // (memo_api_base_url) — a separate Flutter engine/isolate, but the
    // same underlying prefs file, so a server the user changed from
    // Settings is picked up here too, not just Memo's own loopback default.
    final prefs = await SharedPreferences.getInstance();
    final baseUrl = normalizeBackendUrl(prefs.getString('memo_api_base_url') ?? '');
    if (!mounted) return;
    _dio = Dio(BaseOptions(baseUrl: baseUrl, connectTimeout: const Duration(seconds: 2)));
    _poll();
    _pollTimer = Timer.periodic(_pollInterval, (_) => _poll());
  }

  Future<void> _poll() async {
    final dio = _dio;
    if (dio == null) return;
    try {
      final res = await dio.get('/api/mascot/activity');
      final state = res.data is Map ? res.data['state'] as String? : null;
      final mood = _moodFor(state);
      if (mounted && mood != _mood) {
        setState(() => _mood = mood);
      }
    } catch (_) {
      // Backend not reachable (not started yet, or briefly restarting) —
      // idle is always a safe, non-alarming default to fall back to.
      if (mounted && _mood != MascotMood.idle) {
        setState(() => _mood = MascotMood.idle);
      }
    }
  }

  MascotMood _moodFor(String? state) => switch (state) {
        'thinking' => MascotMood.thinking,
        'tool' => MascotMood.tool,
        'writing' => MascotMood.writing,
        'generating' => MascotMood.generating,
        _ => MascotMood.idle,
      };

  @override
  void dispose() {
    _pollTimer?.cancel();
    _dio?.close();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return MouseRegion(
      onEnter: (_) => setState(() => _hovering = true),
      onExit: (_) => setState(() => _hovering = false),
      child: GestureDetector(
        onPanStart: (_) => windowManager.startDragging(),
        behavior: HitTestBehavior.translucent,
        child: Stack(
          children: [
            Center(child: MemoMascot(mood: _mood, size: 100)),
            Positioned(
              top: 2,
              right: 2,
              child: AnimatedOpacity(
                opacity: _hovering ? 1 : 0,
                duration: const Duration(milliseconds: 150),
                child: IgnorePointer(
                  ignoring: !_hovering,
                  child: _CloseButton(onTap: () => windowManager.close()),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _CloseButton extends StatelessWidget {
  final VoidCallback onTap;
  const _CloseButton({required this.onTap});

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        width: 20,
        height: 20,
        decoration: BoxDecoration(
          color: const Color(0xFF2A211A).withValues(alpha: 0.85),
          shape: BoxShape.circle,
        ),
        child: const Icon(Icons.close_rounded, size: 13, color: Color(0xFFE8DCC8)),
      ),
    );
  }
}
