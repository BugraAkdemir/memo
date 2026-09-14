import 'dart:async';
import 'dart:io' show Platform;

import 'package:desktop_multi_window/desktop_multi_window.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:window_manager/window_manager.dart';

import '../core/backend_url.dart';
import '../core/l10n.dart';
import 'memo_mascot.dart';

const _petAreaSize = Size(132, 148);
const _bubbleGap = 8.0;
const _bubbleAreaHeight = 54.0;
const _windowSize = Size(132, 148 + _bubbleGap + _bubbleAreaHeight);

Future<void> runMascotWindow() async {
  WidgetsFlutterBinding.ensureInitialized();
  await windowManager.ensureInitialized();
  try {
    final controller = await WindowController.fromCurrentEngine();
    await controller.setWindowMethodHandler((call) async {
      if (call.method == 'window_close') {
        await windowManager.close();
      }
    });
  } catch (_) {}

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
    if (!Platform.isLinux) {
      await windowManager.setHasShadow(false);
    }
    await windowManager.setResizable(false);
    await windowManager.show();
    await windowManager.focus();
  });

  runApp(const MascotWindowApp());
}

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

class _MascotSurface extends StatefulWidget {
  const _MascotSurface();

  @override
  State<_MascotSurface> createState() => _MascotSurfaceState();
}

const _pollInterval = Duration(milliseconds: 1200);

class _MascotSurfaceState extends State<_MascotSurface> {
  bool _hovering = false;
  MascotMood _mood = MascotMood.idle;
  MascotSkin _skin = MascotSkin.classic;
  String? _toolName;
  Timer? _pollTimer;
  Dio? _dio;
  int _pollGeneration = 0;

  @override
  void initState() {
    super.initState();
    _startPolling();
  }

  Future<void> _startPolling() async {
    final prefs = await SharedPreferences.getInstance();
    final baseUrl = normalizeBackendUrl(prefs.getString('memo_api_base_url') ?? '');
    final skin = MascotSkinPrefValue.fromPrefValue(prefs.getString('memo_mascot_skin'));
    if (!mounted) return;
    setState(() => _skin = skin);
    _dio = Dio(BaseOptions(baseUrl: baseUrl, connectTimeout: const Duration(seconds: 2)));
    _poll();
    _pollTimer = Timer.periodic(_pollInterval, (_) => _poll());
  }

  Future<void> _poll() async {
    final dio = _dio;
    if (dio == null) return;
    final generation = ++_pollGeneration;
    try {
      final res = await dio.get('/api/mascot/activity');
      if (!mounted || generation != _pollGeneration) return;
      final data = res.data;
      final state = data is Map ? data['state'] as String? : null;
      final toolName = data is Map ? data['tool_name'] as String? : null;
      final mood = _moodFor(state);
      if (mood != _mood || toolName != _toolName) {
        setState(() {
          _mood = mood;
          _toolName = toolName;
        });
      }
    } catch (_) {
      if (!mounted || generation != _pollGeneration) return;
      if (_mood != MascotMood.idle) {
        setState(() {
          _mood = MascotMood.idle;
          _toolName = null;
        });
      }
    }
  }

  MascotMood _moodFor(String? state) => switch (state) {
        'thinking' => MascotMood.thinking,
        'tool' => MascotMood.tool,
        'writing' => MascotMood.writing,
        'generating' => MascotMood.generating,
        'done' => MascotMood.done,
        'speaking' => MascotMood.speaking,
        _ => MascotMood.idle,
      };

  @override
  void dispose() {
    _pollGeneration++;
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
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            SizedBox(
              width: _petAreaSize.width,
              height: _petAreaSize.height,
              child: Stack(
                children: [
                  Center(child: MemoMascot(mood: _mood, skin: _skin, size: 100)),
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
            const SizedBox(height: _bubbleGap),
            _StatusBubble(mood: _mood, toolName: _toolName),
          ],
        ),
      ),
    );
  }
}

class _StatusBubble extends StatelessWidget {
  final MascotMood mood;
  final String? toolName;

  const _StatusBubble({required this.mood, required this.toolName});

  String? _textFor(MascotMood mood, String? toolName) => switch (mood) {
        MascotMood.idle => null,
        MascotMood.thinking => L10n.t('mascot_status_thinking'),
        MascotMood.writing => L10n.t('mascot_status_writing'),
        MascotMood.generating => L10n.t('mascot_status_generating'),
        MascotMood.tool => (toolName == null || toolName.isEmpty)
            ? L10n.t('mascot_status_tool')
            : L10n.t('mascot_status_tool_named', {'tool': toolName}),
        MascotMood.done => L10n.t('mascot_status_done'),
        MascotMood.speaking => L10n.t('mascot_status_speaking'),
      };

  @override
  Widget build(BuildContext context) {
    final text = _textFor(mood, toolName);
    return SizedBox(
      height: _bubbleAreaHeight,
      width: _petAreaSize.width,
      child: Center(
        child: AnimatedSwitcher(
          duration: const Duration(milliseconds: 200),
          child: text == null
              ? const SizedBox.shrink(key: ValueKey('empty'))
              : Container(
                  key: ValueKey(text),
                  constraints: BoxConstraints(maxWidth: _petAreaSize.width),
                  padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
                  decoration: BoxDecoration(
                    color: const Color(0xE6231B14),
                    borderRadius: BorderRadius.circular(14),
                    border: Border.all(color: const Color(0x33E8DCC8)),
                    boxShadow: [
                      BoxShadow(
                        color: Colors.black.withValues(alpha: 0.35),
                        blurRadius: 10,
                        offset: const Offset(0, 4),
                      ),
                    ],
                  ),
                  child: Text(
                    text,
                    textAlign: TextAlign.center,
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(
                      color: Color(0xFFE8DCC8),
                      fontSize: 11.5,
                      fontWeight: FontWeight.w500,
                      height: 1.25,
                    ),
                  ),
                ),
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
