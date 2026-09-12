// Entry point for the standalone desktop mascot — a separate small Flutter
// binary (built with `flutter run/build ... -t lib/mascot_main.dart`), not a
// screen inside the main Memo app. Deliberately its own process rather than
// a second window bolted onto the chat app: it can be started and closed on
// its own, and the extra Flutter engine it costs is the same either way —
// this way it doesn't touch the main app's dependency tree at all.
import 'dart:io' show Platform;

import 'package:flutter/material.dart';
import 'package:window_manager/window_manager.dart';

import 'widgets/memo_mascot.dart';

const _windowSize = Size(170, 190);

void main() async {
  WidgetsFlutterBinding.ensureInitialized();
  await windowManager.ensureInitialized();

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

class _MascotSurfaceState extends State<_MascotSurface> {
  bool _hovering = false;

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
            const Center(child: MemoMascot(size: 128)),
            Positioned(
              top: 4,
              right: 4,
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
        width: 22,
        height: 22,
        decoration: BoxDecoration(
          color: const Color(0xFF2A211A).withValues(alpha: 0.85),
          shape: BoxShape.circle,
        ),
        child: const Icon(Icons.close_rounded, size: 14, color: Color(0xFFE8DCC8)),
      ),
    );
  }
}
