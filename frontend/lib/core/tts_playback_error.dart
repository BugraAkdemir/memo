import 'package:flutter/services.dart';

import 'l10n.dart';

/// Turns an error thrown by audio playback into a short, localized,
/// actionable message — instead of dumping the raw exception `toString()`
/// into the UI.
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
