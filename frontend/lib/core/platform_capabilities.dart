import 'package:flutter/foundation.dart'
    show TargetPlatform, defaultTargetPlatform, kIsWeb;

/// Which host capabilities this build actually has, in one place.
///
/// Every getter here is `kIsWeb`-first and then a pure function of the
/// target platform. Two rules make that ordering non-negotiable rather than
/// stylistic:
///
/// - On web `defaultTargetPlatform` reports the *browser's* platform, so a
///   page open in Chrome on an Android phone answers
///   `TargetPlatform.android` while still being the web build. Anything that
///   asks "am I on a phone?" to decide about native plugins or a local
///   backend would get the wrong answer. app_shell.dart documents the same
///   rule for `dart:io`'s `Platform`.
/// - `defaultTargetPlatform` is used rather than `dart:io`'s `Platform` so
///   this file stays importable from the web build.
///
/// The decisions are also exposed as pure `(isWeb, platform)` functions, not
/// only as getters: `flutter test` runs on the VM and, worse, overrides
/// `defaultTargetPlatform` to android for every test, so a getter alone can
/// only ever be exercised in whatever state the harness happens to be in.

/// Whether this build runs on a phone/tablet.
bool get isMobilePlatform =>
    isMobilePlatformFor(isWeb: kIsWeb, platform: defaultTargetPlatform);

bool isMobilePlatformFor({
  required bool isWeb,
  required TargetPlatform platform,
}) =>
    !isWeb &&
    (platform == TargetPlatform.android || platform == TargetPlatform.iOS);

/// Whether a Memo backend could plausibly be running on this same device.
///
/// True on desktop (the user may well have started one) and on web (the page
/// was served by one). False on mobile: a phone never runs Memo's Go
/// backend, so anything offering to "use the local backend" there is
/// offering a guaranteed broken state, not a safe default.
bool get localBackendPossible =>
    localBackendPossibleFor(isWeb: kIsWeb, platform: defaultTargetPlatform);

bool localBackendPossibleFor({
  required bool isWeb,
  required TargetPlatform platform,
}) =>
    !isMobilePlatformFor(isWeb: isWeb, platform: platform);

/// Whether Memo's own CLI (`~/.memo/bin/memo`) can be installed on this
/// host, which is what gates the CLI actions in Settings > General.
///
/// Windows is excluded because the installer flow there is different, and
/// mobile because there is no shell, no home directory to install into, and
/// no terminal to run it from. Web has no filesystem access at all.
bool get cliInstallSupported =>
    cliInstallSupportedFor(isWeb: kIsWeb, platform: defaultTargetPlatform);

bool cliInstallSupportedFor({
  required bool isWeb,
  required TargetPlatform platform,
}) =>
    !isWeb &&
    (platform == TargetPlatform.linux || platform == TargetPlatform.macOS);
