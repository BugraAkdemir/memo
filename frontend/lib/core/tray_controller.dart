// SPDX-License-Identifier: AGPL-3.0-or-later

import 'dart:io' show Platform;

import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:path/path.dart' as p;
import 'package:tray_manager/tray_manager.dart';
import 'package:window_manager/window_manager.dart';

import '../providers/models_provider.dart';
import '../providers/settings_provider.dart';
import 'l10n.dart';

/// Whether the system tray / window-close-interception feature can run at
/// all — window_manager and tray_manager only ship native implementations
/// for these three platforms (see their pubspec.yaml `platforms:` blocks);
/// everywhere else (web, mobile) this stays a total no-op.
bool get trayFeatureSupported =>
    !kIsWeb && (Platform.isLinux || Platform.isMacOS || Platform.isWindows);

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

  @override
  void initState() {
    super.initState();
    if (trayFeatureSupported) {
      _init();
    }
  }

  Future<void> _init() async {
    await windowManager.ensureInitialized();
    windowManager.addListener(this);
    // Always prevented at the OS level; onWindowClose (below) decides
    // per-call whether that means "hide" or "actually quit", based on the
    // user's current minimizeToTrayProvider setting — a fixed setPreventClose
    // value can't express that, since the setting can change at runtime.
    await windowManager.setPreventClose(true);

    trayManager.addListener(this);
    // lib/icon/memo.png is already a declared Flutter asset (pubspec.yaml);
    // tray_manager resolves this path against the compiled bundle's
    // data/flutter_assets directory itself, so no manual file materialization
    // is needed here.
    await trayManager.setIcon('lib/icon/memo.png');

    if (!mounted) return;
    setState(() => _trayReady = true);
    await _rebuildMenu();
  }

  Future<void> _rebuildMenu() async {
    if (!_trayReady) return;
    final status = ref.read(modelStatusProvider).valueOrNull;
    final modelLabel = (status != null && status.running)
        ? L10n.t('tray_model_running', {'name': _fileName(status.modelPath)})
        : L10n.t('tray_model_none');

    await trayManager.setContextMenu(Menu(items: [
      MenuItem(label: L10n.t('tray_open'), onClick: (_) => _showWindow()),
      MenuItem.separator(),
      MenuItem(label: modelLabel, disabled: true),
      MenuItem.separator(),
      MenuItem(label: L10n.t('tray_quit'), onClick: (_) => _quit()),
    ]));
  }

  String _fileName(String path) {
    final name = p.basename(path);
    return name.isEmpty ? path : name;
  }

  Future<void> _showWindow() async {
    await windowManager.show();
    await windowManager.focus();
  }

  Future<void> _quit() async {
    await windowManager.setPreventClose(false);
    await windowManager.destroy();
  }

  @override
  void onWindowClose() async {
    if (ref.read(minimizeToTrayProvider)) {
      await windowManager.hide();
    } else {
      await windowManager.setPreventClose(false);
      await windowManager.destroy();
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
    }
    return widget.child;
  }
}
