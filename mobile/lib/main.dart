import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'core/theme.dart';
import 'providers/connection_provider.dart';
import 'screens/chat_screen.dart';
import 'screens/connect_screen.dart';

void main() {
  WidgetsFlutterBinding.ensureInitialized();
  SystemChrome.setSystemUIOverlayStyle(
    const SystemUiOverlayStyle(
      statusBarColor: Colors.transparent,
      statusBarIconBrightness: Brightness.light,
      systemNavigationBarColor: MemoTheme.bg,
      systemNavigationBarIconBrightness: Brightness.light,
    ),
  );
  runApp(const ProviderScope(child: MemoMobileApp()));
}

class MemoMobileApp extends ConsumerWidget {
  const MemoMobileApp({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return MaterialApp(
      title: 'Memo Mobile',
      debugShowCheckedModeBanner: false,
      theme: MemoTheme.dark,
      home: const AppShell(),
    );
  }
}

class AppShell extends ConsumerStatefulWidget {
  const AppShell({super.key});

  @override
  ConsumerState<AppShell> createState() => _AppShellState();
}

class _AppShellState extends ConsumerState<AppShell> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(connectionStateProvider.notifier).loadSavedUrl();
    });
  }

  @override
  Widget build(BuildContext context) {
    final connection = ref.watch(connectionStateProvider);

    return AnimatedSwitcher(
      duration: const Duration(milliseconds: 300),
      child: connection.connected
          ? const ChatScreen()
          : const ConnectScreen(),
    );
  }
}
