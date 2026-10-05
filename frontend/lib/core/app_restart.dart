// Restart the client: quit the process natively, reload the page on web.
//
// Every "restart Memo" / "close" action used to call dart:io's exit(0)
// directly. On the web build that throws UnsupportedError, so the button did
// nothing — and RestartRequiredDialog's auto-restart countdown kept firing
// the failing call every second and counted on into negative numbers
// ("Restarting automatically in -6s", found live). Reloading the page is the
// web equivalent: it rebuilds every provider against the newly saved backend
// address, which is what the restart was for.
export 'app_restart_io.dart'
    if (dart.library.js_interop) 'app_restart_web.dart';
