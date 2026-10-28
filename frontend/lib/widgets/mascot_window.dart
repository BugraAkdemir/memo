import 'dart:async';
import 'dart:io' show Platform, exit;

import 'package:desktop_multi_window/desktop_multi_window.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:window_manager/window_manager.dart';

import '../core/backend_url.dart';
import '../core/l10n.dart';
import '../core/mascot_channel.dart';
import 'memo_mascot.dart';

const _petAreaSize = Size(132, 148);
const _bubbleGap = 8.0;
const _bubbleAreaHeight = 54.0;
const _windowSize = Size(132, 148 + _bubbleGap + _bubbleAreaHeight);

Future<void> runMascotWindow() async {
  WidgetsFlutterBinding.ensureInitialized();
  await windowManager.ensureInitialized();
  // Closing a secondary FlView tears down its engine on Linux while pending
  // compositor work can still reference it. Keep one hidden window instead.
  await windowManager.setPreventClose(true);

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

class _MascotSurfaceState extends State<_MascotSurface> with WindowListener {
  bool _visible = false;
  bool _managed = false;
  String? _ownerWindowId;
  bool _ownerGone = false;
  StreamSubscription<void>? _ownerEvents;
  String _baseUrl = '';
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
    windowManager.addListener(this);
    _initialize();
  }

  Future<void> _initialize() async {
    final prefs = await SharedPreferences.getInstance();
    _baseUrl = normalizeBackendUrl(prefs.getString('memo_api_base_url') ?? '');
    final skin = MascotSkinPrefValue.fromPrefValue(prefs.getString('memo_mascot_skin'));
    if (!mounted) return;
    _skin = skin;
    final controller = await WindowController.fromCurrentEngine();
    _managed = controller.arguments == mascotWindowArgument;
    if (_managed) {
      final windows = await WindowController.getAll();
      for (final window in windows) {
        if (window.windowId != controller.windowId && window.arguments != mascotWindowArgument) {
          _ownerWindowId = window.windowId;
          break;
        }
      }
      _ownerEvents = onWindowsChanged.listen((_) => unawaited(_checkOwner()));
    }
    await controller.setWindowMethodHandler((call) async {
      if (call.method == 'window_close') await _hideWindow();
    });
    await mascotLifecycleChannel.setMethodCallHandler((call) async {
      if (!mounted) return;
      switch (call.method) {
        case 'show':
          await _showWindow(MascotSkinPrefValue.fromPrefValue(call.arguments as String?));
        case 'hide':
          await _hideWindow(notifyOwner: false);
        case 'skin':
          setState(() => _skin = MascotSkinPrefValue.fromPrefValue(call.arguments as String?));
      }
    });
    if (!mounted) return;
    if (_managed) {
      await mascotLifecycleChannel.invokeMethod<void>('ready');
    } else {
      await _showWindow(skin);
    }
  }

  Future<void> _showWindow(MascotSkin skin) async {
    if (!mounted) return;
    setState(() {
      _skin = skin;
      _visible = true;
      _hovering = false;
    });
    _stopPolling();
    _dio = Dio(BaseOptions(baseUrl: _baseUrl, connectTimeout: const Duration(seconds: 2)));
    unawaited(_poll());
    _pollTimer = Timer.periodic(_pollInterval, (_) => _poll());
    await windowManager.show();
    await windowManager.focus();
  }

  void _stopPolling() {
    _pollGeneration++;
    _pollTimer?.cancel();
    _pollTimer = null;
    _dio?.close(force: true);
    _dio = null;
  }

  Future<void> _hideWindow({bool notifyOwner = true}) async {
    _stopPolling();
    if (mounted) {
      setState(() {
        _visible = false;
        _mood = MascotMood.idle;
        _toolName = null;
      });
    }
    await windowManager.hide();
    await _checkOwner();
    if (_managed && notifyOwner) {
      await mascotLifecycleChannel.invokeMethod<void>('hidden');
    }
  }

  // A cached, hidden pet must not keep an otherwise-closed application alive.
  // A visible pet retains the existing behavior of surviving the main window.
  Future<void> _checkOwner() async {
    final owner = _ownerWindowId;
    if (!_managed || owner == null) return;
    final windows = await WindowController.getAll();
    if (!mounted) return;
    _ownerGone = !windows.any((window) => window.windowId == owner);
    if (_ownerGone && !_visible) exit(0);
  }

  @override
  void onWindowClose() => unawaited(_hideWindow());

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
    windowManager.removeListener(this);
    unawaited(_ownerEvents?.cancel());
    _stopPolling();
    unawaited(mascotLifecycleChannel.setMethodCallHandler(null));
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (!_visible) return const SizedBox.shrink();
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
                        child: _CloseButton(onTap: () => unawaited(_hideWindow())),
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
