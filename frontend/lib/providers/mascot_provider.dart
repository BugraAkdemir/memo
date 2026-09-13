import 'package:desktop_multi_window/desktop_multi_window.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

/// Owns the desktop mascot's open/closed state so both the tray menu item
/// and the Settings toggle (general_tab.dart) drive the exact same window
/// instead of each keeping their own — a real desktop_multi_window
/// WindowController when the mascot is open, null when it's closed.
class MascotWindowNotifier extends StateNotifier<WindowController?> {
  MascotWindowNotifier() : super(null) {
    // Catches the window closing from EITHER side — the tray/Settings
    // toggle asking it to close, or the mascot's own hover close button —
    // so whichever UI shows "is it open" stays correct either way.
    onWindowsChanged.listen((_) => _syncState());
  }

  Future<void> _syncState() async {
    final tracked = state;
    if (tracked == null) return;
    final stillOpen = (await WindowController.getAll())
        .any((w) => w.windowId == tracked.windowId);
    if (!stillOpen) state = null;
  }

  /// Opens the mascot if it's closed, closes it if it's open. Not a
  /// window inside this app: closing/quitting Memo does not close it,
  /// same as the CLI staying up independently of the Flutter app.
  Future<void> toggle() async {
    final existing = state;
    if (existing != null) {
      // WindowController has no close() of its own — this is the
      // hand-off point desktop_multi_window's README documents for
      // asking another window to close itself (see
      // mascot_window.dart's setWindowMethodHandler).
      await existing.invokeMethod('window_close');
      return; // onWindowsChanged -> _syncState clears state.
    }
    final controller = await WindowController.create(
      const WindowConfiguration(arguments: '', hiddenAtLaunch: true),
    );
    await controller.show();
    state = controller;
  }
}

final mascotWindowProvider =
    StateNotifierProvider<MascotWindowNotifier, WindowController?>(
  (ref) => MascotWindowNotifier(),
);

/// Convenience bool view for UI that only cares whether it's open —
/// Settings' toggle switch and the tray menu's label both just need this.
final mascotWindowOpenProvider =
    Provider<bool>((ref) => ref.watch(mascotWindowProvider) != null);
