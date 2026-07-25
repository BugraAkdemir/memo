import 'package:flutter/services.dart';

import 'l10n.dart';

/// Turns an error thrown by [AudioPlayer.play]/[MemoApiClient.synthesizeSpeech]
/// into a short, localized, actionable message — instead of dumping the raw
/// exception `toString()` into the UI (e.g. a raw `PlatformException(...)`
/// with internal GStreamer domain/code numbers, which a non-technical user
/// can't act on).
///
/// The specific case this exists for: `audioplayers`' Linux backend uses
/// GStreamer, and a distro that ships only `gst-plugins-base` (no
/// `gst-plugins-good`, which provides the `wavparse`/`autoaudiosink`
/// elements needed to play the WAV bytes Piper produces) throws a
/// `PlatformException(LinuxAudioError, ..., "... missing a plug-in
/// (Domain: gst-core-error-quark, Code: 12)", null)`. That's a real, common
/// Linux packaging gap (confirmed on this project's own dev machine —
/// CachyOS ships `gstreamer`+`gst-plugins-base/bad/ugly` but not
/// `gst-plugins-good` by default), not a Memo bug — but the error text
/// should tell the user what to actually do about it.
String friendlyPlaybackError(Object error) {
  if (error is PlatformException && _isMissingGstreamerPlugin(error)) {
    return L10n.t('live_mode_error_missing_gstreamer_plugins');
  }
  return L10n.t('live_mode_error_playback_generic', {'err': '$error'});
}

bool _isMissingGstreamerPlugin(PlatformException e) {
  final haystack = '${e.code} ${e.message} ${e.details}';
  return haystack.contains('GStreamer') || haystack.contains('gst-core-error');
}
