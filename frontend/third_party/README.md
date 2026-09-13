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
   other two. On its own this doesn't work on native Wayland at all (no
   equivalent to the X11 EWMH mechanism it uses, confirmed live: another
   window still covered the mascot once focused, regardless of *when*
   this was called) — Wayland compositors deliberately refuse an
   unprompted "always above everything" request from a client, the same
   security model that blocks unprompted focus-stealing. Rather than
   chase KDE's own compositor protocol
   (`org_kde_plasma_window_management`), `linux/runner/main.cc` forces
   the whole app onto XWayland instead (`setenv("GDK_BACKEND", "x11", 1)`
   as the very first line of `main()`), which gives this call — and the
   input-shape "collider" fix below — the X11 mechanism they need.
   Confirmed live after that change: opening another window directly
   over the mascot's screen position no longer covers it.

Also **not a `MEMO PATCH` in this vendored file**, but the same
before-realize timing idea applied one level up:
`my_application.cc`'s own window-created callback (already registering
every plugin for the mascot's sub-window) sets an elliptical *input*
shape via `gdk_window_input_shape_combine_region` — another X11-only
GDK call — so the transparent margin around the character doesn't
swallow clicks meant for whatever's behind it. Same XWayland
requirement as always-on-top.

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
