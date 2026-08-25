import 'dart:ui';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'core/l10n.dart';
import 'core/theme.dart';
import 'core/tray_controller.dart';
import 'providers/settings_provider.dart';
import 'screens/app_shell.dart';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();

  // A ConsumerStatefulWidget's Element can go defunct (torn down mid-
  // navigation/rebuild) at the exact moment an autoDispose
  // StateNotifierProvider it was watching notifies a state update —
  // Riverpod wraps each listener's callback in its own zone-guarded call
  // (ProviderElementBase._notifyListeners -> Zone.runBinaryGuarded) so one
  // broken listener can't corrupt the whole notification chain, but that
  // also means the resulting "_lifecycleState != _ElementLifecycle.defunct"
  // assertion in Element.markNeedsBuild surfaces here, as an unhandled zone
  // error — no try/catch around the notifier's own state update can ever
  // catch it, since it isn't thrown synchronously into that call. Confirmed
  // live (WhatsAppStatusNotifier's periodic status poll, seen firing this
  // repeatedly): harmless — the app keeps running correctly and the poll
  // keeps working either way — but it floods the console. Filtered here by
  // matching the specific assertion text, rather than "fixed" in the
  // notifier itself, since the actual race is a Flutter/Riverpod
  // framework-timing interaction that app code doesn't control. Anything
  // else still gets Flutter's normal error reporting.
  PlatformDispatcher.instance.onError = (Object error, StackTrace stack) {
    if (error.toString().contains('_lifecycleState != _ElementLifecycle.defunct')) {
      return true; // handled — swallow, don't print
    }
    return false; // not ours — let Flutter's default handling report it
  };

  final prefs = await SharedPreferences.getInstance();
  runApp(ProviderScope(
    overrides: [
      prefsProvider.overrideWithValue(prefs),
    ],
    child: const MemoApp(),
  ));
}

class MemoApp extends ConsumerWidget {
  const MemoApp({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final locale = ref.watch(localeProvider);
    final themeModeStr = ref.watch(themeModeProvider);
    L10n.setLocale(locale);

    final themeMode = switch (themeModeStr) {
      'light' => ThemeMode.light,
      'dark' => ThemeMode.dark,
      _ => ThemeMode.system,
    };

    return TrayController(
      child: MaterialApp(
        title: 'Memo',
        debugShowCheckedModeBanner: false,
        theme: MemoTheme.themeData,
        darkTheme: MemoTheme.darkThemeData,
        themeMode: themeMode,
        home: AppShell(),
      ),
    );
  }
}
