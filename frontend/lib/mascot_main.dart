// Fast dev-iteration entrypoint for the standalone desktop mascot:
// `flutter run -d linux -t lib/mascot_main.dart` boots straight into it
// without the main app's own startup work. The production path is
// lib/main.dart detecting MEMO_MASCOT_WINDOW on itself instead (see
// widgets/mascot_window.dart's doc comment) — both call the same function.
import 'widgets/mascot_window.dart';

void main() => runMascotWindow();
