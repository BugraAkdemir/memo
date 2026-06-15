import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/date_symbol_data_local.dart';

import 'core/notification_service.dart';
import 'core/theme.dart';
import 'providers/calendar_provider.dart';
import 'providers/connection_provider.dart';
import 'screens/calendar_screen.dart';
import 'screens/chat_screen.dart';
import 'screens/connect_screen.dart';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();
  // Required for DateFormat(..., 'tr') used in the calendar screen.
  await initializeDateFormatting('tr', null);
  await NotificationService.init();
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
  int _selectedIndex = 0;
  Timer? _eventPollTimer;
  final Set<String> _seenEvents = {};

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(connectionStateProvider.notifier).loadSavedUrl();
      _startEventPolling();
    });
  }

  @override
  void dispose() {
    _eventPollTimer?.cancel();
    super.dispose();
  }

  void _startEventPolling() {
    _eventPollTimer = Timer.periodic(const Duration(seconds: 30), (_) async {
      final conn = ref.read(connectionStateProvider);
      if (!conn.connected) return;

      final client = ref.read(apiClientProvider);
      final events = await client.getAppEvents();
      for (final e in events) {
        final name = e['name'] as String? ?? '';
        final data = e['data'] as String? ?? '';
        final key = '$name:$data';
        if (_seenEvents.contains(key)) continue;
        _seenEvents.add(key);

        // Reminders themselves are delivered by OS-level scheduling (so they
        // work when the app is closed). The poll only reacts to *new* events:
        // refresh the list, which reschedules notifications accordingly.
        if (name == 'calendar:added') {
          try {
            final payload = jsonDecode(data) as Map<String, dynamic>;
            final title = payload['title'] as String? ?? 'Etkinlik';
            await NotificationService.showCalendarAdded(title);
            ref.read(calendarProvider.notifier).refresh();
          } catch (_) {}
        } else if (name == 'calendar:removed') {
          try {
            final payload = jsonDecode(data) as Map<String, dynamic>;
            final id = payload['id'] as String? ?? '';
            if (id.isNotEmpty) {
              await NotificationService.cancelReminder(id);
            }
            ref.read(calendarProvider.notifier).refresh();
          } catch (_) {}
        }
      }
      // Keep seen set bounded.
      if (_seenEvents.length > 500) _seenEvents.clear();
    });
  }

  @override
  Widget build(BuildContext context) {
    final connection = ref.watch(connectionStateProvider);

    // When the connection comes up, force a calendar load so OS reminders get
    // scheduled even if the user never opens the Takvim tab this session.
    ref.listen(connectionStateProvider, (prev, next) {
      if ((prev == null || !prev.connected) && next.connected) {
        ref.read(calendarProvider.notifier).refresh();
      }
    });

    if (!connection.connected) {
      return const ConnectScreen();
    }

    final screens = [
      const ChatScreen(),
      const CalendarScreen(),
    ];

    return Scaffold(
      body: IndexedStack(index: _selectedIndex, children: screens),
      bottomNavigationBar: NavigationBar(
        backgroundColor: MemoTheme.surface,
        selectedIndex: _selectedIndex,
        onDestinationSelected: (i) => setState(() => _selectedIndex = i),
        destinations: [
          NavigationDestination(
            icon: Icon(Icons.chat_outlined, color: MemoTheme.textDim),
            selectedIcon: Icon(Icons.chat, color: MemoTheme.accent),
            label: 'Sohbet',
          ),
          NavigationDestination(
            icon: Icon(Icons.calendar_month_outlined,
                color: MemoTheme.textDim),
            selectedIcon:
                Icon(Icons.calendar_month, color: MemoTheme.accent),
            label: 'Takvim',
          ),
        ],
      ),
    );
  }
}
