// Copy text to the clipboard, on every platform, without lying about it.
//
// Flutter web's `Clipboard.setData` goes through `navigator.clipboard`
// (flutter/engine/src/flutter/lib/web_ui/lib/src/engine/clipboard.dart), and
// that object is **null in an insecure context** — the engine throws
// `StateError('Clipboard is not available in the context.')` for any page
// served over plain HTTP. A Memo reached at `http://192.168.1.50:8090` on a
// Raspberry Pi, or a VDS with no TLS, is exactly that page: every copy button
// in the app silently did nothing, and the UI still said "Copied" because the
// Future was never awaited (and its error never surfaced).
//
// So: try the async Clipboard API first (the only path browsers accept without
// a user gesture), then fall back to the legacy `document.execCommand('copy')`
// on a detached textarea, which HTTP pages are still allowed to use. Only when
// both fail does this return false, so the caller can tell the user instead of
// claiming a copy that never happened.
export 'clipboard_copy_io.dart'
    if (dart.library.js_interop) 'clipboard_copy_web.dart';