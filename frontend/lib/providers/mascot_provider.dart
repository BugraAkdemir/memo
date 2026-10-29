import 'dart:async';

import 'package:desktop_multi_window/desktop_multi_window.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/mascot_channel.dart';
import 'settings_provider.dart';

/// Visibility is separate from the retained window: toggling never destroys
/// the Linux FlView/engine while its renderer can still reference it.
class MascotWindowNotifier extends StateNotifier<WindowController?> {
  MascotWindowNotifier({String Function()? readSkin})
    : _readSkin = readSkin ?? (() => 'classic'),
      super(null) {
    _windowsChanged = onWindowsChanged.listen((_) => _syncState());
  }

  final String Function() _readSkin;
  late final StreamSubscription<void> _windowsChanged;
  WindowController? _window;
  Completer<void>? _ready;
  Future<void> _pending = Future<void>.value();

  Future<void> _syncState() async {
    final tracked = _window;
    if (tracked == null) return;
    try {
      final windows = await WindowController.getAll();
      if (!mounted || _window != tracked) return;
      if (!windows.any((w) => w.windowId == tracked.windowId)) {
        _window = null;
        state = null;
      }
    } catch (e) {
      debugPrint('mascot: window sync: $e');
    }
  }

  // Tray clicks, Settings clicks and skin updates share one ordered queue.
  Future<void> _enqueue(Future<void> Function() action) {
    final operation = _pending.then((_) async {
      if (mounted) await action();
    });
    _pending = operation.catchError((Object e) {
      debugPrint('mascot: window action: $e');
    });
    return operation;
  }

  Future<void> _ensureWindow() async {
    if (_window != null) {
      await _ready?.future.timeout(const Duration(seconds: 15));
      return;
    }
    _ready = Completer<void>();
    await mascotLifecycleChannel.setMethodCallHandler((call) async {
      if (!mounted) return;
      if (call.method == 'ready' && !_ready!.isCompleted) {
        _ready!.complete();
      } else if (call.method == 'hidden') {
        state = null;
      }
    });
    // A provider-scope restart must reuse the existing pet, too.
    final windows = await WindowController.getAll();
    for (final window in windows) {
      if (window.arguments == mascotWindowArgument) {
        _window = window;
        if (!_ready!.isCompleted) _ready!.complete();
        return;
      }
    }
    _window = await WindowController.create(
      const WindowConfiguration(
        arguments: mascotWindowArgument,
        hiddenAtLaunch: true,
      ),
    );
    // Do not send commands into an engine whose Dart handler is not ready.
    await _ready!.future.timeout(const Duration(seconds: 15));
  }

  Future<void> toggle() => _enqueue(() async {
    if (state != null) {
      await mascotLifecycleChannel.invokeMethod<void>('hide');
      if (mounted) state = null;
      return;
    }
    await _ensureWindow();
    if (!mounted) return;
    await mascotLifecycleChannel.invokeMethod<void>('show', _readSkin());
    if (mounted) state = _window;
  });

  Future<void> updateSkin(String skin) => _enqueue(() async {
    if (_window == null) return;
    await _ready?.future.timeout(const Duration(seconds: 15));
    if (mounted) {
      await mascotLifecycleChannel.invokeMethod<void>('skin', skin);
    }
  });

  @override
  void dispose() {
    unawaited(_windowsChanged.cancel());
    unawaited(mascotLifecycleChannel.setMethodCallHandler(null));
    super.dispose();
  }
}

final mascotWindowProvider =
    StateNotifierProvider<MascotWindowNotifier, WindowController?>((ref) {
      final notifier = MascotWindowNotifier(
        readSkin: () => ref.read(mascotSkinProvider),
      );
      ref.listen<String>(mascotSkinProvider, (_, skin) {
        unawaited(notifier.updateSkin(skin));
      });
      return notifier;
    });

final mascotWindowOpenProvider = Provider<bool>(
  (ref) => ref.watch(mascotWindowProvider) != null,
);
