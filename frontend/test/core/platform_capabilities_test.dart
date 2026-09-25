import 'package:flutter/foundation.dart' show TargetPlatform;
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/platform_capabilities.dart';

/// Exercises the capability decisions as pure functions.
///
/// The getters in platform_capabilities.dart cannot be covered here: this
/// suite runs on the VM, and flutter_test overrides defaultTargetPlatform to
/// android for every test — so a getter would only ever report one state, and
/// the web-vs-mobile ordering that the whole file hinges on would go
/// completely untested. That is exactly why each decision is also exposed as
/// a function taking (isWeb, platform).
void main() {
  const mobile = [TargetPlatform.android, TargetPlatform.iOS];
  const desktop = [
    TargetPlatform.linux,
    TargetPlatform.macOS,
    TargetPlatform.windows,
  ];

  group('isMobilePlatformFor', () {
    test('true for android and iOS when not web', () {
      for (final p in mobile) {
        expect(isMobilePlatformFor(isWeb: false, platform: p), isTrue,
            reason: '$p should count as mobile');
      }
    });

    test('false for every desktop platform', () {
      for (final p in desktop) {
        expect(isMobilePlatformFor(isWeb: false, platform: p), isFalse,
            reason: '$p should not count as mobile');
      }
    });

    // The ordering rule this file exists to enforce: a browser on a phone
    // reports TargetPlatform.android, but it is the web build — served BY
    // the backend it talks to, with no native plugins of its own.
    test('false on web even when the browser reports a phone platform', () {
      for (final p in mobile) {
        expect(isMobilePlatformFor(isWeb: true, platform: p), isFalse,
            reason: 'web on $p is still the web build');
      }
    });
  });

  group('localBackendPossibleFor', () {
    test('false on mobile — a phone never runs the Go backend', () {
      for (final p in mobile) {
        expect(localBackendPossibleFor(isWeb: false, platform: p), isFalse);
      }
    });

    test('true on desktop, where the user may have started one', () {
      for (final p in desktop) {
        expect(localBackendPossibleFor(isWeb: false, platform: p), isTrue);
      }
    });

    test('true on web — the page was served by a backend', () {
      expect(
        localBackendPossibleFor(isWeb: true, platform: TargetPlatform.android),
        isTrue,
      );
    });
  });

  group('cliInstallSupportedFor', () {
    test('true on linux and macOS only', () {
      expect(
        cliInstallSupportedFor(isWeb: false, platform: TargetPlatform.linux),
        isTrue,
      );
      expect(
        cliInstallSupportedFor(isWeb: false, platform: TargetPlatform.macOS),
        isTrue,
      );
    });

    test('false on windows, which has its own installer flow', () {
      expect(
        cliInstallSupportedFor(isWeb: false, platform: TargetPlatform.windows),
        isFalse,
      );
    });

    // The actual bug: the old `!kIsWeb && !Platform.isWindows` showed the
    // "reinstall CLI" action on a phone.
    test('false on mobile — no shell, no home directory, no terminal', () {
      for (final p in mobile) {
        expect(cliInstallSupportedFor(isWeb: false, platform: p), isFalse);
      }
    });

    test('false on web — no filesystem access at all', () {
      expect(
        cliInstallSupportedFor(isWeb: true, platform: TargetPlatform.linux),
        isFalse,
      );
    });
  });

  group('liveRealtimePlaybackSupportedFor', () {
    test('linux only — live_pcm_player.dart has no other implementation', () {
      expect(
        liveRealtimePlaybackSupportedFor(
            isWeb: false, platform: TargetPlatform.linux),
        isTrue,
      );
      for (final p in [
        TargetPlatform.macOS,
        TargetPlatform.windows,
        ...mobile,
      ]) {
        expect(
          liveRealtimePlaybackSupportedFor(isWeb: false, platform: p),
          isFalse,
          reason: '$p has no streaming PCM playback path',
        );
      }
    });

    test('false on web', () {
      expect(
        liveRealtimePlaybackSupportedFor(
            isWeb: true, platform: TargetPlatform.linux),
        isFalse,
      );
    });
  });
}
