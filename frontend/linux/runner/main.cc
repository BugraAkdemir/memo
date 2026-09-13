#include <stdlib.h>

#include "my_application.h"

int main(int argc, char** argv) {
  // Forces XWayland instead of native Wayland, even on a
  // GDK_BACKEND=wayland session (this machine's KDE/Plasma default). Two
  // real GTK/Wayland gaps this sidesteps, both confirmed live on this
  // exact session: gtk_window_set_keep_above (the desktop mascot's
  // always-on-top) is an X11 EWMH mechanism with no native-Wayland
  // equivalent, and a shaped input region (matching the mascot's window
  // to the character's own silhouette instead of its full rectangle) is
  // an X11-only GDK call too. Must happen before GTK/GDK read the
  // environment, so this is the earliest point in the whole process —
  // setting it from Dart would be too late. setenv's overwrite=1: this
  // must win over whatever GDK_BACKEND the launching shell already set,
  // which is the whole point.
  setenv("GDK_BACKEND", "x11", 1);

  g_autoptr(MyApplication) app = my_application_new();
  return g_application_run(G_APPLICATION(app), argc, argv);
}
