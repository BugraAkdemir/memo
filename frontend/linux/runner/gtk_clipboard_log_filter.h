#ifndef FLUTTER_GTK_CLIPBOARD_LOG_FILTER_H_
#define FLUTTER_GTK_CLIPBOARD_LOG_FILTER_H_

#include <glib.h>

// Flutter's Linux engine implements both Clipboard.getData and
// Clipboard.hasStrings via gtk_clipboard_request_text(), and every mounted
// EditableText (TextField, TextFormField, SelectableText / selectable
// MarkdownBody blocks) calls Clipboard.hasStrings() on mount and on every
// AppLifecycleState.resumed. Because main.cc forces GDK_BACKEND=x11 (XWayland)
// so the desktop mascot gets real always-on-top and a shaped input region, any
// time the X11/XWayland clipboard carries non-UTF-8 data (images/screenshots,
// binary selections, or broken Wayland->XWayland UTF-8 conversions), GTK3 and
// GDK3 emit g_warning() on every concurrent clipboard probe before handing
// NULL cleanly to Flutter's callback. Nothing fails functionally, but every
// window focus or dialog open floods stderr/logs with bursts of identical
// warnings.
//
// Filter ONLY those specific, known-benign GTK3/GDK3 clipboard charset
// conversion warnings (G_LOG_LEVEL_WARNING only) and forward every other
// warning, critical, or error to g_log_default_handler unchanged.
inline bool is_benign_gtk_clipboard_warning(const gchar* log_domain,
                                            const gchar* message) {
  if (log_domain == nullptr || message == nullptr) {
    return false;
  }
  if (g_strcmp0(log_domain, "Gtk") == 0) {
    // gtk/gtkselection.c: "Error converting from %s to %s: %s"
    return g_str_has_prefix(message, "Error converting from ");
  }
  if (g_strcmp0(log_domain, "Gdk") == 0) {
    // gdk/x11/gdkselection-x11.c:
    //   "Error converting selection"
    //   "Error converting selection from STRING: %s"
    //   "Error converting selection from UTF8_STRING"
    //   "Error converting to UTF-8 from '%s': %s"
    //   "Error converting from UTF-8 to '%s': %s"
    return g_str_has_prefix(message, "Error converting selection") ||
           g_str_has_prefix(message, "Error converting to UTF-8 from ") ||
           g_str_has_prefix(message, "Error converting from UTF-8 to ");
  }
  return false;
}

inline void gtk_clipboard_warning_log_handler(const gchar* log_domain,
                                              GLogLevelFlags log_level,
                                              const gchar* message,
                                              gpointer /*user_data*/) {
  if (is_benign_gtk_clipboard_warning(log_domain, message)) {
    return;
  }
  g_log_default_handler(log_domain, log_level, message, nullptr);
}

inline void install_gtk_clipboard_log_filter() {
  static gboolean installed = FALSE;
  if (installed) {
    return;
  }
  installed = TRUE;
  g_log_set_handler("Gtk", G_LOG_LEVEL_WARNING,
                    gtk_clipboard_warning_log_handler, nullptr);
  g_log_set_handler("Gdk", G_LOG_LEVEL_WARNING,
                    gtk_clipboard_warning_log_handler, nullptr);
}

#endif  // FLUTTER_GTK_CLIPBOARD_LOG_FILTER_H_
