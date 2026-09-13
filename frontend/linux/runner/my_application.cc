#include "my_application.h"

#include <flutter_linux/flutter_linux.h>
#ifdef GDK_WINDOWING_X11
#include <gdk/gdkx.h>
#endif

#include "flutter/generated_plugin_registrant.h"

#include <math.h>

#include "desktop_multi_window/desktop_multi_window_plugin.h"

struct _MyApplication {
  GtkApplication parent_instance;
  char** dart_entrypoint_arguments;
};

G_DEFINE_TYPE(MyApplication, my_application, GTK_TYPE_APPLICATION)

// Restricts the desktop mascot's *input* region (what actually receives
// clicks/drags) to a rough ellipse instead of its whole 132x148 window —
// the transparent margin around the character stops swallowing clicks
// meant for whatever is behind it. Only takes effect on X11/XWayland
// (main() forces GDK_BACKEND=x11 for exactly this and the always-on-top
// fix — see that comment); gdk_window_input_shape_combine_region has no
// native-Wayland equivalent either, so this silently no-ops there instead
// of erroring. Built from horizontal bands rather than one shape call
// because cairo_region_t is fundamentally rectangle-based — this is the
// standard way to approximate a curve with one.
static void set_mascot_input_shape(GtkWindow* window) {
  GdkWindow* gdk_win = gtk_widget_get_window(GTK_WIDGET(window));
  if (gdk_win == nullptr) return;

  const double cx = 66.0, cy = 78.0, rx = 54.0, ry = 62.0;
  const int bands = 48;
  cairo_region_t* region = cairo_region_create();
  for (int i = 0; i < bands; i++) {
    double y0 = -ry + (2.0 * ry * i) / bands;
    double y1 = -ry + (2.0 * ry * (i + 1)) / bands;
    double ymid = (y0 + y1) / 2.0;
    double frac = 1.0 - (ymid * ymid) / (ry * ry);
    if (frac < 0) continue;
    double halfw = rx * sqrt(frac);
    cairo_rectangle_int_t rect;
    rect.x = static_cast<int>(cx - halfw);
    rect.y = static_cast<int>(cy + y0);
    rect.width = static_cast<int>(2 * halfw);
    rect.height = static_cast<int>(y1 - y0) + 1;
    cairo_region_union_rectangle(region, &rect);
  }
  // The hover-close button (mascot_window.dart's _CloseButton) sits in the
  // top-right corner, outside the ellipse above — without this, hovering
  // or clicking it would never reach Flutter at all, since a shaped
  // window has no input surface outside the shape to begin with.
  cairo_rectangle_int_t close_button_corner = {100, 0, 32, 30};
  cairo_region_union_rectangle(region, &close_button_corner);

  gdk_window_input_shape_combine_region(gdk_win, region, 0, 0);
  cairo_region_destroy(region);
}

// Called when first Flutter frame received.
static void first_frame_cb(MyApplication* self, FlView* view) {
  gtk_widget_show(gtk_widget_get_toplevel(GTK_WIDGET(view)));
}

// Implements GApplication::activate.
static void my_application_activate(GApplication* application) {
  MyApplication* self = MY_APPLICATION(application);
  GtkWindow* window =
      GTK_WINDOW(gtk_application_window_new(GTK_APPLICATION(application)));

  // Use a header bar when running in GNOME as this is the common style used
  // by applications and is the setup most users will be using (e.g. Ubuntu
  // desktop).
  // If running on X and not using GNOME then just use a traditional title bar
  // in case the window manager does more exotic layout, e.g. tiling.
  // If running on Wayland assume the header bar will work (may need changing
  // if future cases occur).
  gboolean use_header_bar = TRUE;
#ifdef GDK_WINDOWING_X11
  GdkScreen* screen = gtk_window_get_screen(window);
  if (GDK_IS_X11_SCREEN(screen)) {
    const gchar* wm_name = gdk_x11_screen_get_window_manager_name(screen);
    if (g_strcmp0(wm_name, "GNOME Shell") != 0) {
      use_header_bar = FALSE;
    }
  }
#endif
  if (use_header_bar) {
    GtkHeaderBar* header_bar = GTK_HEADER_BAR(gtk_header_bar_new());
    gtk_widget_show(GTK_WIDGET(header_bar));
    gtk_header_bar_set_title(header_bar, "Memo");
    gtk_header_bar_set_show_close_button(header_bar, TRUE);
    gtk_window_set_titlebar(window, GTK_WIDGET(header_bar));
  } else {
    gtk_window_set_title(window, "Memo");
  }

  // Set app icon from bundle directory
  g_autofree gchar* exe_path = g_file_read_link("/proc/self/exe", NULL);
  if (exe_path != NULL) {
    g_autofree gchar* exe_dir = g_path_get_dirname(exe_path);
    g_autofree gchar* icon_path = g_build_filename(exe_dir, "icon.png", NULL);
    if (g_file_test(icon_path, G_FILE_TEST_EXISTS)) {
      gtk_window_set_icon_from_file(window, icon_path, NULL);
    }
  }

  gtk_window_set_default_size(window, 1280, 720);

  g_autoptr(FlDartProject) project = fl_dart_project_new();
  fl_dart_project_set_dart_entrypoint_arguments(
      project, self->dart_entrypoint_arguments);

  FlView* view = fl_view_new(project);
  GdkRGBA background_color;
  // Light base so the Glass Light theme's BackdropFilter samples a light
  // backdrop (a black clear colour made frosted surfaces render dark gray).
  // Opaque dark theme paints over this fully, save a brief startup frame.
  gdk_rgba_parse(&background_color, "#F2EDE6");
  fl_view_set_background_color(view, &background_color);
  gtk_widget_show(GTK_WIDGET(view));
  gtk_container_add(GTK_CONTAINER(window), GTK_WIDGET(view));

  // Show the window when Flutter renders.
  // Requires the view to be realized so we can start rendering.
  g_signal_connect_swapped(view, "first-frame", G_CALLBACK(first_frame_cb),
                           self);
  gtk_widget_realize(GTK_WIDGET(view));

  fl_register_plugins(FL_PLUGIN_REGISTRY(view));

  // Registers every plugin (window_manager included) for the desktop
  // mascot's window too — desktop_multi_window gives each sub-window its
  // own Flutter engine, so nothing is registered for it automatically.
  // Also clears its FlView to fully transparent, which — unlike the GTK
  // window's own alpha visual — isn't subject to the realize-before-set
  // constraint documented in third_party/README.md, so it's fine to do
  // here rather than in the vendored patch.
  desktop_multi_window_plugin_set_window_created_callback(
      [](FlPluginRegistry* registry) {
        fl_register_plugins(registry);
        GdkRGBA transparent;
        gdk_rgba_parse(&transparent, "#000000");
        transparent.alpha = 0;
        fl_view_set_background_color(FL_VIEW(registry), &transparent);
        GtkWidget* toplevel = gtk_widget_get_toplevel(GTK_WIDGET(FL_VIEW(registry)));
        set_mascot_input_shape(GTK_WINDOW(toplevel));
      });

  gtk_widget_grab_focus(GTK_WIDGET(view));
}

// Implements GApplication::local_command_line.
static gboolean my_application_local_command_line(GApplication* application,
                                                  gchar*** arguments,
                                                  int* exit_status) {
  MyApplication* self = MY_APPLICATION(application);
  // Strip out the first argument as it is the binary name.
  self->dart_entrypoint_arguments = g_strdupv(*arguments + 1);

  g_autoptr(GError) error = nullptr;
  if (!g_application_register(application, nullptr, &error)) {
    g_warning("Failed to register: %s", error->message);
    *exit_status = 1;
    return TRUE;
  }

  g_application_activate(application);
  *exit_status = 0;

  return TRUE;
}

// Implements GApplication::startup.
static void my_application_startup(GApplication* application) {
  // MyApplication* self = MY_APPLICATION(object);

  // Perform any actions required at application startup.

  G_APPLICATION_CLASS(my_application_parent_class)->startup(application);
}

// Implements GApplication::shutdown.
static void my_application_shutdown(GApplication* application) {
  // MyApplication* self = MY_APPLICATION(object);

  // Perform any actions required at application shutdown.

  G_APPLICATION_CLASS(my_application_parent_class)->shutdown(application);
}

// Implements GObject::dispose.
static void my_application_dispose(GObject* object) {
  MyApplication* self = MY_APPLICATION(object);
  g_clear_pointer(&self->dart_entrypoint_arguments, g_strfreev);
  G_OBJECT_CLASS(my_application_parent_class)->dispose(object);
}

static void my_application_class_init(MyApplicationClass* klass) {
  G_APPLICATION_CLASS(klass)->activate = my_application_activate;
  G_APPLICATION_CLASS(klass)->local_command_line =
      my_application_local_command_line;
  G_APPLICATION_CLASS(klass)->startup = my_application_startup;
  G_APPLICATION_CLASS(klass)->shutdown = my_application_shutdown;
  G_OBJECT_CLASS(klass)->dispose = my_application_dispose;
}

static void my_application_init(MyApplication* self) {}

MyApplication* my_application_new() {
  // Set the program name to the application ID, which helps various systems
  // like GTK and desktop environments map this running application to its
  // corresponding .desktop file. This ensures better integration by allowing
  // the application to be recognized beyond its binary name.
  g_set_prgname(APPLICATION_ID);

  return MY_APPLICATION(g_object_new(my_application_get_type(),
                                     "application-id", APPLICATION_ID, "flags",
                                     G_APPLICATION_NON_UNIQUE, nullptr));
}
