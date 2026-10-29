// Native Linux regression harness. Run against an isolated XDG_CONFIG_HOME:
// flutter build linux --debug -t tool/mascot_lifecycle_smoke.dart
// xvfb-run -a ./build/linux/x64/debug/bundle/memo_flutter
import 'dart:async';
import 'dart:io';

import 'package:desktop_multi_window/desktop_multi_window.dart';
import 'package:flutter/material.dart';
import 'package:window_manager/window_manager.dart';
import 'package:memo_flutter/core/mascot_channel.dart';
import 'package:memo_flutter/providers/mascot_provider.dart';
import 'package:memo_flutter/widgets/mascot_window.dart';

void main(List<String> args) async {
  if (isMascotSubWindow(args)) return runMascotWindow();
  WidgetsFlutterBinding.ensureInitialized();
  runApp(const MaterialApp(home: SizedBox.shrink()));
  await Future<void>.delayed(const Duration(seconds: 1));
  var skin = 'classic';
  final owner = MascotWindowNotifier(readSkin: () => skin);
  WindowController? visibleWindow;
  final removeListener = owner.addListener((window) => visibleWindow = window);
  try {
    String? originalId;
    for (var i = 0; i < 20; i++) {
      await owner.toggle();
      final controller = visibleWindow;
      if (controller == null) throw StateError('pet did not open');
      originalId ??= controller.windowId;
      if (controller.windowId != originalId) {
        throw StateError('opening replaced the pet engine');
      }
      skin = i.isEven ? 'pixel' : 'classic';
      await owner.updateSkin(skin);
      await Future<void>.delayed(const Duration(milliseconds: 80));
      if (i.isEven) {
        // The mascot's own close control, including its owner notification.
        await controller.invokeMethod<void>('window_close');
      } else {
        await owner.toggle();
      }
      if (visibleWindow != null) throw StateError('pet did not hide');
      await owner.updateSkin(i.isEven ? 'classic' : 'pixel');
      debugPrint('MASCOT_CYCLE_PASS ${i + 1}');
    }
    await Future.wait([
      owner.toggle(),
      owner.toggle(),
      owner.toggle(),
      owner.toggle(),
    ]);
    if (visibleWindow != null) {
      throw StateError('rapid toggles did not serialize');
    }
    final pets = (await WindowController.getAll()).where(
      (w) => w.arguments == mascotWindowArgument,
    );
    if (pets.length != 1) {
      throw StateError('retained ${pets.length} pet engines');
    }
    debugPrint(
      'MASCOT_NATIVE_PASS: 20 cycles, live/hidden skin changes, rapid toggles, one retained engine',
    );
    removeListener();
    owner.dispose();
    await windowManager.ensureInitialized();
    debugPrint(
      'MASCOT_OWNER_CLOSE: destroying the main window with the pet hidden',
    );
    await windowManager.destroy();
    await Future<void>.delayed(const Duration(seconds: 4));
    throw StateError('a hidden pet kept the closed application alive');
  } catch (error, stack) {
    debugPrint('MASCOT_NATIVE_FAIL: $error\n$stack');
    exit(1);
  }
}
