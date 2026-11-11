import 'package:flutter/foundation.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/providers/settings_provider.dart';
import 'package:shared_preferences/shared_preferences.dart';

// X keeps desktop Memo running through its tray by default. A saved choice
// always wins. Tray setup failure must keep the taskbar fallback reachable;
// that behavior is exercised separately by tray_controller_test.dart.
void main() {
  tearDown(() => debugDefaultTargetPlatformOverride = null);

  Future<SharedPreferences> prefs([
    Map<String, Object> values = const {},
  ]) async {
    SharedPreferences.setMockInitialValues(values);
    return SharedPreferences.getInstance();
  }

  test('Windows hides to the tray by default', () async {
    debugDefaultTargetPlatformOverride = TargetPlatform.windows;
    expect(MinimizeToTrayNotifier(await prefs()).state, isTrue);
  });

  test('Linux and macOS hide to the tray by default', () async {
    for (final platform in [TargetPlatform.linux, TargetPlatform.macOS]) {
      debugDefaultTargetPlatformOverride = platform;
      expect(
        MinimizeToTrayNotifier(await prefs()).state,
        isTrue,
        reason: '$platform',
      );
    }
  });

  test('mobile platforms do not enable the desktop tray default', () async {
    for (final platform in [TargetPlatform.android, TargetPlatform.iOS]) {
      debugDefaultTargetPlatformOverride = platform;
      expect(MinimizeToTrayNotifier(await prefs()).state, isFalse);
    }
  });

  test('a saved choice beats the platform default', () async {
    debugDefaultTargetPlatformOverride = TargetPlatform.windows;
    expect(
      MinimizeToTrayNotifier(
        await prefs({'memo_minimize_to_tray': false}),
      ).state,
      isFalse,
    );
    debugDefaultTargetPlatformOverride = TargetPlatform.linux;
    expect(
      MinimizeToTrayNotifier(
        await prefs({'memo_minimize_to_tray': false}),
      ).state,
      isFalse,
    );
  });
}
