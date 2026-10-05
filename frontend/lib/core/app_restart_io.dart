import 'dart:io';

/// Native: quit. The OS-level entry point (desktop icon, AppImage,
/// run_memo.sh) is what starts Memo again — see RestartRequiredDialog.
void restartApp() => exit(0);
