# third_party/

## desktop_multi_window (0.3.1, Apache-2.0)

Vendored from pub.dev instead of a normal `pubspec.yaml` dependency because
the desktop mascot's window (`frontend/lib/widgets/mascot_window.dart`)
needs a few native tweaks on Linux with no supported extension point.
Search `linux/multi_window_manager.cc` for `MEMO PATCH` — three separate
ones, all in `MultiWindowManager::Create()`, all for the same underlying
reason: GTK/the compositor only honours certain properties when they're
set *before* `gtk_widget_realize()`/the window is first mapped, and that
happens inside `Create()` **before** the plugin's own documented extension
point (`desktop_multi_window_plugin_set_window_created_callback`, wired up
in `linux/runner/my_application.cc`) ever fires — by the time our own code
gets a callback, it's too late for any of these three:

1. **Alpha-capable GTK visual** — without it the window renders as an
   opaque box instead of a real transparent background.
2. **Small size + no decoration set at creation time** — asking for these
   later (window_manager's own `setSize`/`setAsFrameless`, from Dart, once
   the sub-window is running) looked like it worked (`getSize()` agreed)
   but the actual Wayland surface stayed at the upstream default 1280x720
   with a KWin server-side titlebar regardless, confirmed live via a
   screenshot with the window's background forced to opaque red to reveal
   its true bounds. The empty `gtk_window_set_titlebar()` call right next
   to `gtk_window_set_decorated(FALSE)` matters too — decorated(FALSE)
   alone left the server-side titlebar in place; GTK only negotiates
   client-side decoration with the compositor once the app has gone
   through `set_titlebar` at all, even with nothing in it.
3. **`gtk_window_set_keep_above`** — moved here for consistency with the
   other two, but this one is a known **partial** fix, not a real one:
   confirmed live that on native Wayland (`GDK_BACKEND=wayland`, KDE/KWin)
   another window can still cover the mascot once focused, regardless of
   *when* this is called. `gtk_window_set_keep_above` is an X11 EWMH
   mechanism with no native-Wayland equivalent in plain GTK — Wayland
   compositors deliberately refuse an unprompted "always above everything"
   request from a client, the same security model that blocks unprompted
   focus-stealing. A real fix needs a compositor-specific protocol (KDE's
   `org_kde_plasma_window_management`) that plain GTK doesn't expose. Left
   in anyway since it's the correct, working call on X11 sessions.

Frameless, skip-taskbar and dragging (once actually realized at the right
size) are still set from `mascot_window.dart`'s own Dart code via the
plain (unforked) `window_manager` package, once that plugin is registered
for the sub-window by the `fl_register_plugins` call already added to
that same callback — only the three items above needed to move here.

**Upgrading:** copy the new version's `lib/`, `linux/`, `macos/`,
`windows/`, `pubspec.yaml`, `LICENSE` and `CHANGELOG.md` over this
directory, then reapply the `MEMO PATCH` block by diffing against this
copy's `multi_window_manager.cc` (or just re-search for `gtk_widget_realize`
in the new file and paste the same block immediately after the window is
created, before that call).
