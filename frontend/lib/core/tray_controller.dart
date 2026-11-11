// SPDX-License-Identifier: AGPL-3.0-or-later

import 'dart:async';
import 'dart:developer' as developer;
import 'dart:io' show Platform;

import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:path/path.dart' as p;
import 'package:tray_manager/tray_manager.dart';
import 'package:window_manager/window_manager.dart';

import '../providers/chat_provider.dart'
    show apiClientProvider, currentClientIdProvider;
import '../providers/mascot_provider.dart';
import '../providers/models_provider.dart';
import '../providers/settings_provider.dart';
import 'l10n.dart';

/// Whether the system tray / window-close-interception feature can run at
/// all — window_manager and tray_manager only ship native implementations
/// for these three platforms (see their pubspec.yaml `platforms:` blocks);
/// everywhere else (web, mobile) this stays a total no-op.
bool get trayFeatureSupported =>
    !kIsWeb && (Platform.isLinux || Platform.isMacOS || Platform.isWindows);

/// MaterialApp's navigator, so the tray controller (which wraps MaterialApp
/// and so sits above any Navigator) can show the one-time "still running in
/// the background" notice.
final trayNavigatorKey = GlobalKey<NavigatorState>();

/// Set once the person has seen that notice (a device preference).
const _trayHintShownKey = 'memo_tray_hint_shown';

/// Wraps the whole app to add an LM-Studio-style system tray icon: a
/// right-click menu showing whether a local model is running, "Open Memo"
/// to refocus the window, and "Quit Memo" to actually exit. When the
/// user's `minimizeToTrayProvider` setting is on, closing the main window
/// hides it instead of quitting — the app keeps running in the background,
/// same as LM Studio's own tray behavior.
///
/// A pure passthrough (just renders [child]) wherever trayFeatureSupported
/// is false, so this can wrap MemoApp's tree unconditionally without any
/// caller needing to special-case web/mobile.
class TrayController extends ConsumerStatefulWidget {
  final Widget child;
  const TrayController({super.key, required this.child});

  @override
  ConsumerState<TrayController> createState() => _TrayControllerState();
}

class _TrayControllerState extends ConsumerState<TrayController>
    with WindowListener, TrayListener {
  bool _trayReady = false;
  bool _handlingClose = false;
  bool _quitting = false;
  Future<bool>? _traySetup;

  @override
  void initState() {
    super.initState();
    if (trayFeatureSupported) {
      unawaited(_init());
    }
  }

  Future<void> _init() async {
    try {
      await windowManager.ensureInitialized();
      if (!mounted) return;
      windowManager.addListener(this);
      // Intercept X even when the tray fails. The fallback keeps the window
      // reachable from the taskbar instead of quitting or hiding it.
      await windowManager.setPreventClose(true);
      if (!mounted) return;
      trayManager.addListener(this);
      await _ensureTray();
    } on PlatformException catch (error, stack) {
      _logTrayFailure(error, stack);
    } on MissingPluginException catch (error, stack) {
      _logTrayFailure(error, stack);
    }
  }

  void _logTrayFailure(Object error, StackTrace stack) {
    developer.log(
      'System tray unavailable',
      name: 'memo.tray',
      error: error,
      stackTrace: stack,
    );
  }

  Future<bool> _ensureTray() {
    // One native setup at a time, including X during startup. Clear a failed
    // attempt so the next close can recover when the shell is available again.
    return _traySetup ??= _setUpTray().whenComplete(() => _traySetup = null);
  }

  Future<bool> _setUpTray() async {
    _trayReady = false;
    try {
      await trayManager.setIcon(
        Platform.isWindows ? 'lib/icon/memo.ico' : 'lib/icon/memo.png',
      );
      if (!mounted || _quitting) return false;
      // An icon alone is insufficient: people must be able to Open and Quit.
      await _setMenu();
      if (!mounted || _quitting) return false;
      _trayReady = true;
      return true;
    } on PlatformException catch (error, stack) {
      _logTrayFailure(error, stack);
    } on MissingPluginException catch (error, stack) {
      _logTrayFailure(error, stack);
    }
    return false;
  }

  Future<void> _rebuildMenu() async {
    if (!_trayReady) return;
    try {
      await _setMenu();
    } on PlatformException catch (error, stack) {
      _trayReady = false;
      _logTrayFailure(error, stack);
      // Menu loss while hidden must not leave an inaccessible application.
      await _showWindow();
    } on MissingPluginException catch (error, stack) {
      _trayReady = false;
      _logTrayFailure(error, stack);
      await _showWindow();
    }
  }

  Future<void> _setMenu() async {
    final status = ref.read(modelStatusProvider).valueOrNull;
    final modelLabel = (status != null && status.running)
        ? L10n.t('tray_model_running', {'name': _fileName(status.modelPath)})
        : L10n.t('tray_model_none');

    await trayManager.setContextMenu(
      Menu(
        items: [
          MenuItem(label: L10n.t('tray_open'), onClick: (_) => _showWindow()),
          MenuItem.separator(),
          MenuItem(label: modelLabel, disabled: true),
          MenuItem.separator(),
          MenuItem(
            label: ref.read(mascotWindowOpenProvider)
                ? L10n.t('tray_mascot_hide')
                : L10n.t('tray_mascot_show'),
            onClick: (_) => ref.read(mascotWindowProvider.notifier).toggle(),
          ),
          MenuItem.separator(),
          MenuItem(label: L10n.t('tray_quit'), onClick: (_) => _quit()),
        ],
      ),
    );
  }

  String _fileName(String path) {
    final name = p.basename(path);
    return name.isEmpty ? path : name;
  }

  Future<void> _showWindow() async {
    if (!mounted || _quitting) return;
    await windowManager.show();
    await windowManager.focus();
  }

  Future<void> _quit() async {
    if (_quitting) return;
    _quitting = true;
    await _unregisterClient();
    await windowManager.setPreventClose(false);
    await windowManager.destroy();
  }

  @override
  void onWindowClose() async {
    if (_handlingClose || _quitting) return;
    _handlingClose = true;
    try {
      if (ref.read(minimizeToTrayProvider)) {
        // Native Windows setup verifies registration too (not just a cached
        // Dart flag). Retry on each X so a disappeared shell icon is repaired.
        final ready = await _ensureTray().timeout(
          const Duration(seconds: 2),
          onTimeout: () => false,
        );
        if (!mounted) return;
        if (!ready) {
          await _minimizeWithoutTray();
          return;
        }
        if (await _tellOnceThatMemoKeepsRunning()) return;
        if (!mounted || _quitting) return;
        // Explorer may have restarted while the first-close dialog was open.
        // The patched Windows plugin returns null if the shell lost the icon.
        if (Platform.isWindows && !await _windowsTrayPresent()) {
          if (mounted && !_quitting) await _minimizeWithoutTray();
          return;
        }
        if (mounted && !_quitting) await windowManager.hide();
      } else {
        await _quit();
      }
    } finally {
      _handlingClose = false;
    }
  }

  Future<bool> _windowsTrayPresent() async {
    try {
      return await trayManager.getBounds().timeout(
            const Duration(seconds: 2),
          ) !=
          null;
    } on PlatformException catch (error, stack) {
      _logTrayFailure(error, stack);
    } on MissingPluginException catch (error, stack) {
      _logTrayFailure(error, stack);
    } on TimeoutException catch (error, stack) {
      _logTrayFailure(error, stack);
    }
    return false;
  }

  Future<void> _minimizeWithoutTray() async {
    // Preserve both the preference and the backend connection. A failed tray
    // should neither quit Memo nor mark the successful-tray hint as seen.
    final context = trayNavigatorKey.currentContext;
    if (context != null && context.mounted) {
      ScaffoldMessenger.maybeOf(
        context,
      )?.showSnackBar(SnackBar(content: Text(L10n.t('tray_unavailable'))));
    }
    await windowManager.setSkipTaskbar(false);
    await windowManager.minimize();
  }

  /// The first time the window is hidden instead of closed, say so — a window
  /// that vanishes while the app keeps running is otherwise indistinguishable
  /// from one that crashed. Returns true when the person chose to quit there
  /// (the app is already closing); false to go on and hide the window.
  Future<bool> _tellOnceThatMemoKeepsRunning() async {
    final prefs = ref.read(prefsProvider);
    if (prefs.getBool(_trayHintShownKey) ?? false) return false;
    if (trayNavigatorKey.currentContext == null) return false;
    await prefs.setBool(_trayHintShownKey, true);
    final ctx = trayNavigatorKey.currentContext;
    if (ctx == null || !ctx.mounted) return false;
    final quit = await showDialog<bool>(
      context: ctx,
      builder: (dctx) => AlertDialog(
        title: Text(L10n.t('tray_hint_title')),
        content: Text(L10n.t('tray_hint_body')),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dctx).pop(true),
            child: Text(L10n.t('tray_quit')),
          ),
          FilledButton(
            onPressed: () => Navigator.of(dctx).pop(false),
            child: Text(L10n.t('tray_hint_ok')),
          ),
        ],
      ),
    );
    if (quit == true) {
      await _quit();
      return true;
    }
    return false;
  }

  /// Sends the graceful goodbye an on-demand backend (main.go's
  /// --auto-shutdown spawn) is waiting for, so it shuts itself down the
  /// moment this was the last attached client instead of only noticing via
  /// the ~90s missed-heartbeat staleness window (connectionStatusProvider's
  /// own doc comment used to say there was no reliable close hook on
  /// desktop for this — there is, via this same WindowListener the tray
  /// feature already uses). Only reached on an actual quit, never on
  /// minimize-to-tray (the app keeps running there, so it stays registered).
  /// Best-effort: MemoApiClient.unregisterClient already swallows its own
  /// errors, and the app is quitting either way.
  Future<void> _unregisterClient() async {
    final id = ref.read(currentClientIdProvider);
    if (id != null) {
      await ref.read(apiClientProvider).unregisterClient(id);
    }
  }

  @override
  void onTrayIconMouseDown() {
    trayManager.popUpContextMenu();
  }

  @override
  void onTrayIconRightMouseDown() {
    trayManager.popUpContextMenu();
  }

  @override
  void dispose() {
    if (trayFeatureSupported) {
      windowManager.removeListener(this);
      trayManager.removeListener(this);
    }
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (trayFeatureSupported) {
      // Keeps the tray menu's model-status line in sync with the same
      // polling modelStatusProvider already does for the in-app engine
      // strip (see engine_strip.dart) — no separate polling loop needed.
      ref.listen(modelStatusProvider, (_, _) => _rebuildMenu());
      // Keeps the tray menu's "Show/Hide Desktop Mascot" label correct
      // whether it was opened/closed from here, from Settings, or by the
      // mascot's own hover close button (mascot_provider.dart's
      // onWindowsChanged listener is what notices that last one).
      ref.listen(mascotWindowProvider, (_, _) => _rebuildMenu());
    }
    return widget.child;
  }
}
