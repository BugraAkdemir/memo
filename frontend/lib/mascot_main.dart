// Fast dev-iteration entrypoint for the standalone desktop mascot:
// `flutter run -d linux -t lib/mascot_main.dart` boots straight into it
// without the main app's own startup work. The production path is
// lib/main.dart handling desktop_multi_window's 'multi_window' arguments;
// both entrypoints call the same function.
import 'widgets/mascot_window.dart';

void main() => runMascotWindow();
