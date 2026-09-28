import 'package:web/web.dart' as web;

/// Web: a page reload is the restart — the tab can't be "quit".
void restartApp() => web.window.location.reload();
