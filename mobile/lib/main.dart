import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/date_symbol_data_local.dart';

import 'core/l10n.dart';
import 'core/notification_service.dart';
import 'core/theme.dart';
import 'providers/calendar_provider.dart';
import 'providers/connection_provider.dart';
import 'providers/locale_provider.dart';
import 'providers/routine_provider.dart';
import 'screens/calendar_screen.dart';
import 'screens/chat_screen.dart';
import 'screens/connect_screen.dart';
import 'screens/routines_screen.dart';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();
  // Calendar DateFormat needs both locales registered up front.
  await initializeDateFormatting('tr', null);
  await initializeDateFormatting('en', null);
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
    // Rebuild the whole tree when language changes so every L10n.t() call refreshes.
    final locale = ref.watch(localeProvider);
    L10n.setLocale(locale);

    return MaterialApp(
      title: L10n.t('app_title'),
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
      if (!mounted) return;
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
            final title =
                payload['title'] as String? ?? L10n.t('event_fallback');
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
        } else if (name == 'routine:ready') {
          // Content was just generated ahead of a routine's fire time (see
          // internal/routine's mobileLeadDuration) — fetch it and schedule a
          // local notification for the exact fire instant.
          await ref.read(routineProvider.notifier).checkMobileReady();
        }
      }
      // Keep seen set bounded.
      if (_seenEvents.length > 500) _seenEvents.clear();
    });
  }

  @override
  Widget build(BuildContext context) {
    final connection = ref.watch(connectionStateProvider);
    // Ensure nav labels rebuild with localeProvider (watched in MemoMobileApp too).
    ref.watch(localeProvider);

    // When the connection comes up, force a calendar load so OS reminders get
    // scheduled even if the user never opens the calendar tab this session.
    // Routines get the same catch-up (BUG-L3): checkMobileReady only used to
    // fire reactively off a "routine:ready" event seen by the 30s poll — if
    // the app was fully closed (or the event scrolled out of the backend's
    // 64-entry ring buffer) while one fired, that day's notification was
    // never scheduled. A poll-independent catch-up on every (re)connect
    // means it's picked up the next time the app is opened at all, exactly
    // like calendar reminders already are.
    ref.listen(connectionStateProvider, (prev, next) {
      if ((prev == null || !prev.connected) && next.connected) {
        ref.read(calendarProvider.notifier).refresh();
        ref.read(routineProvider.notifier).refresh();
        ref.read(routineProvider.notifier).checkMobileReady();
      }
    });

    if (!connection.connected) {
      return const ConnectScreen();
    }

    final screens = [
      const ChatScreen(),
      const CalendarScreen(),
      const RoutinesScreen(),
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
            label: L10n.t('nav_chat'),
          ),
          NavigationDestination(
            icon: Icon(Icons.calendar_month_outlined,
                color: MemoTheme.textDim),
            selectedIcon:
                Icon(Icons.calendar_month, color: MemoTheme.accent),
            label: L10n.t('nav_calendar'),
          ),
          NavigationDestination(
            icon: Icon(Icons.schedule_outlined, color: MemoTheme.textDim),
            selectedIcon: Icon(Icons.schedule, color: MemoTheme.accent),
            label: L10n.t('nav_routines'),
          ),
        ],
      ),
    );
  }
}
