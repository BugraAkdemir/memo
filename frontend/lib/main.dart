import 'dart:ui';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'core/l10n.dart';
import 'core/notification_service.dart';
import 'core/theme.dart';
import 'core/tray_controller.dart';
import 'providers/settings_provider.dart';
import 'screens/app_shell.dart';
import 'widgets/mascot_window.dart';

void main(List<String> args) async {
  // The tray's "Desktop mascot" item spawns this as a second, same-process
  // window via desktop_multi_window (see tray_controller.dart), which
  // passes 'multi_window' as this entrypoint's first argument for any
  // window it creates — see mascot_window.dart's doc comment.
  if (isMascotSubWindow(args)) {
    return runMascotWindow();
  }

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
  // Mobile only, and a no-op elsewhere — NotificationService.init() checks
  // notificationsSupported itself, so this costs desktop and web a single
  // boolean. Before runApp so a reminder tapped from the notification
  // shade has a fully initialized plugin to report itself to.
  await NotificationService.init();
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

    // Status bar / navigation bar styling on Android. Ported from the retired
    // mobile client, but theme-aware rather than a fixed dark style set once
    // in main(): this app has light and dark themes, and mobile/ only ever had
    // one. AnnotatedRegion re-applies it whenever the resolved brightness
    // changes, including a live system-theme switch under ThemeMode.system.
    // A harmless no-op off Android.
    final brightness = switch (themeMode) {
      ThemeMode.light => Brightness.light,
      ThemeMode.dark => Brightness.dark,
      ThemeMode.system => MediaQuery.platformBrightnessOf(context),
    };
    final darkUi = brightness == Brightness.dark;

    return AnnotatedRegion<SystemUiOverlayStyle>(
      value: SystemUiOverlayStyle(
        statusBarColor: Colors.transparent,
        statusBarIconBrightness: darkUi ? Brightness.light : Brightness.dark,
        statusBarBrightness: brightness,
        systemNavigationBarColor:
            darkUi ? MemoTheme.dark.bgApp : MemoTheme.light.bgApp,
        systemNavigationBarIconBrightness:
            darkUi ? Brightness.light : Brightness.dark,
      ),
      child: TrayController(
        child: MaterialApp(
          title: 'Memo',
          debugShowCheckedModeBanner: false,
          theme: MemoTheme.themeData,
          darkTheme: MemoTheme.darkThemeData,
          themeMode: themeMode,
          home: AppShell(),
        ),
      ),
    );
  }
}
