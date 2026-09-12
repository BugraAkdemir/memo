# third_party/

## desktop_multi_window (0.3.1, Apache-2.0)

Vendored from pub.dev instead of a normal `pubspec.yaml` dependency because
it needs one small native patch on Linux that has no supported extension
point: [linux/multi_window_manager.cc](desktop_multi_window/linux/multi_window_manager.cc)
requests an alpha-capable GTK visual for each new sub-window (search for
`MEMO PATCH`), which is required for the desktop mascot's window
(`frontend/lib/widgets/mascot_window.dart`) to render with a real
transparent background instead of an opaque box.

This has to happen in `MultiWindowManager::Create()` itself because GTK
only honours a visual change made *before* `gtk_widget_realize()`, and that
call happens inside `Create()` before the plugin's own documented
extension point (`desktop_multi_window_plugin_set_window_created_callback`,
wired up in `linux/runner/my_application.cc`) ever fires — by the time our
own code gets a callback, the window is already realized and it's too
late. No other part of the plugin was changed; frameless, sizing,
always-on-top, skip-taskbar and dragging are all set from
`mascot_window.dart`'s own Dart code via the plain (unforked)
`window_manager` package, once that plugin is registered for the
sub-window by the `fl_register_plugins` call already added to that same
callback.

**Upgrading:** copy the new version's `lib/`, `linux/`, `macos/`,
`windows/`, `pubspec.yaml`, `LICENSE` and `CHANGELOG.md` over this
directory, then reapply the `MEMO PATCH` block by diffing against this
copy's `multi_window_manager.cc` (or just re-search for `gtk_widget_realize`
in the new file and paste the same block immediately after the window is
created, before that call).
