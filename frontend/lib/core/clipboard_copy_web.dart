import 'dart:js_interop';

import 'package:web/web.dart' as web;

/// Web: `navigator.clipboard` first, then the legacy `execCommand('copy')`.
///
/// `navigator.clipboard` is only exposed in a **secure context**, so on a Memo
/// reached over plain HTTP (a Raspberry Pi on the LAN, a VDS with no TLS) it is
/// missing and every async Clipboard call fails. `execCommand('copy')` is the
/// path HTTP pages are still allowed to use — it needs a real, user-initiated
/// selection, so the textarea has to be attached to the document and selected
/// synchronously inside the tap handler, not after an await.
Future<bool> copyToClipboard(String text) async {
  if (web.window.isSecureContext) {
    try {
      await web.window.navigator.clipboard.writeText(text).toDart;
      return true;
    } catch (_) {
      // Permission denied or a browser that rejects the promise — fall through
      // to execCommand, which does not need the permission.
    }
  }
  return _execCommandCopy(text);
}

/// The legacy fallback: a textarea carrying the text, selected, copied, removed.
bool _execCommandCopy(String text) {
  final area = web.document.createElement('textarea') as web.HTMLTextAreaElement;
  // Off-screen but still rendered: `display:none` or `visibility:hidden` make
  // the selection empty, which silently copies nothing.
  area.value = text;
  area.setAttribute('readonly', '');
  area.style.position = 'fixed';
  area.style.top = '-1000px';
  area.style.opacity = '0';
  web.document.body!.append(area);
  area.select();
  area.setSelectionRange(0, text.length);
  bool ok;
  try {
    ok = web.document.execCommand('copy');
  } catch (_) {
    ok = false;
  }
  area.remove();
  return ok;
}