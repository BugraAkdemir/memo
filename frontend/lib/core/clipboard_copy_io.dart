import 'package:flutter/services.dart';

/// Native: Flutter's platform channel is backed by the real system clipboard
/// and needs no fallback. It can still fail (a locked display server, a
/// sandboxed app), so the failure is swallowed and reported as `false` rather
/// than thrown — every caller shows the user a message, and an exception
/// escaping an onPressed would be a red screen instead.
Future<bool> copyToClipboard(String text) async {
  try {
    await Clipboard.setData(ClipboardData(text: text));
    return true;
  } catch (_) {
    return false;
  }
}