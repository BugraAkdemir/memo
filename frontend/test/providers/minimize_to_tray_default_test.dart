import 'package:flutter/foundation.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/providers/settings_provider.dart';
import 'package:shared_preferences/shared_preferences.dart';

// Closing Memo on Windows used to quit it outright while the tray had no Memo
// icon, so people saw a vanished window and engines still running. Windows
// now hides to the tray by default; Linux and macOS keep closing as before;
// a saved choice always wins.
void main() {
  tearDown(() => debugDefaultTargetPlatformOverride = null);

  Future<SharedPreferences> prefs([Map<String, Object> values = const {}]) async {
    SharedPreferences.setMockInitialValues(values);
    return SharedPreferences.getInstance();
  }

  test('Windows hides to the tray by default', () async {
    debugDefaultTargetPlatformOverride = TargetPlatform.windows;
    expect(MinimizeToTrayNotifier(await prefs()).state, isTrue);
  });

  test('Linux and macOS keep closing by default', () async {
    for (final platform in [TargetPlatform.linux, TargetPlatform.macOS]) {
      debugDefaultTargetPlatformOverride = platform;
      expect(MinimizeToTrayNotifier(await prefs()).state, isFalse, reason: '$platform');
    }
  });

  test('a saved choice beats the platform default', () async {
    debugDefaultTargetPlatformOverride = TargetPlatform.windows;
    expect(MinimizeToTrayNotifier(await prefs({'memo_minimize_to_tray': false})).state, isFalse);
    debugDefaultTargetPlatformOverride = TargetPlatform.linux;
    expect(MinimizeToTrayNotifier(await prefs({'memo_minimize_to_tray': true})).state, isTrue);
  });
}
