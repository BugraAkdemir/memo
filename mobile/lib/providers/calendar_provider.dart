import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/api_client.dart';
import '../core/notification_service.dart';
import '../models/calendar_event.dart';
import 'connection_provider.dart';

// ── State ─────────────────────────────────────────────────────────────────────

class CalendarState {
  final List<CalendarEvent> events;
  final bool loading;
  final String? error;
  final DateTime focusedDay;
  final CalendarSettings settings;

  const CalendarState({
    this.events = const [],
    this.loading = false,
    this.error,
    required this.focusedDay,
    this.settings = const CalendarSettings(),
  });

  CalendarState copyWith({
    List<CalendarEvent>? events,
    bool? loading,
    String? error,
    DateTime? focusedDay,
    CalendarSettings? settings,
  }) =>
      CalendarState(
        events: events ?? this.events,
        loading: loading ?? this.loading,
        error: error,
        focusedDay: focusedDay ?? this.focusedDay,
        settings: settings ?? this.settings,
      );
}

// ── Notifier ──────────────────────────────────────────────────────────────────

class CalendarNotifier extends StateNotifier<CalendarState> {
  final MemoApiClient _api;

  CalendarNotifier(this._api)
      : super(CalendarState(focusedDay: DateTime.now())) {
    _load();
  }

  Future<void> _load() async {
    state = state.copyWith(loading: true);
    try {
      final now = DateTime.now();
      final from = DateTime(now.year, now.month, 1);
      final to = DateTime(now.year, now.month + 2, 0);

      final raw = await _api.getCalendarEvents(from: from, to: to);
      final events = raw.map(CalendarEvent.fromJson).toList();

      final settingsRaw = await _api.getCalendarSettings();
      final settings = CalendarSettings.fromJson(settingsRaw);

      state = state.copyWith(events: events, settings: settings, loading: false);

      // Schedule OS-level reminders so they fire even when the app is closed.
      await _scheduleReminders(events, settings.reminderLeadMinutes);
    } catch (e) {
      state = state.copyWith(loading: false, error: e.toString());
    }
  }

  // Reschedules local notifications for all upcoming events. The OS delivers
  // these even if the app is later closed/backgrounded.
  Future<void> _scheduleReminders(List<CalendarEvent> events, int leadMinutes) async {
    final nowUtc = DateTime.now().toUtc();
    for (final e in events) {
      final reminderUtc =
          e.startTime.toUtc().subtract(Duration(minutes: leadMinutes));
      if (reminderUtc.isAfter(nowUtc)) {
        await NotificationService.scheduleReminder(
          eventId: e.id,
          title: e.title,
          whenUtc: reminderUtc,
          minutesBefore: leadMinutes,
        );
      } else {
        // Past or already-fired reminders: make sure no stale one lingers.
        await NotificationService.cancelReminder(e.id);
      }
    }
  }

  Future<void> refresh() => _load();

  Future<void> addEvent({
    required String title,
    required DateTime startTime,
    String description = '',
  }) async {
    try {
      final raw = await _api.addCalendarEvent(
        title: title,
        startTime: startTime,
        description: description,
      );
      final event = CalendarEvent.fromJson(raw);
      state = state.copyWith(events: [...state.events, event]);

      // Schedule a reminder for the freshly added event.
      final reminderUtc = event.startTime
          .toUtc()
          .subtract(Duration(minutes: state.settings.reminderLeadMinutes));
      if (reminderUtc.isAfter(DateTime.now().toUtc())) {
        await NotificationService.scheduleReminder(
          eventId: event.id,
          title: event.title,
          whenUtc: reminderUtc,
          minutesBefore: state.settings.reminderLeadMinutes,
        );
      }
    } catch (e) {
      state = state.copyWith(error: e.toString());
    }
  }

  Future<void> deleteEvent(String id) async {
    try {
      await _api.deleteCalendarEvent(id);
      await NotificationService.cancelReminder(id);
      state = state.copyWith(
        events: state.events.where((e) => e.id != id).toList(),
      );
    } catch (e) {
      state = state.copyWith(error: e.toString());
    }
  }

  Future<void> updateSettings({required int reminderLeadMinutes}) async {
    try {
      await _api.updateCalendarSettings(
          reminderLeadMinutes: reminderLeadMinutes);
      state = state.copyWith(
        settings: CalendarSettings(reminderLeadMinutes: reminderLeadMinutes),
      );
    } catch (e) {
      state = state.copyWith(error: e.toString());
    }
  }

  void setFocusedDay(DateTime day) {
    state = state.copyWith(focusedDay: day);
  }

  /// Events on a specific calendar day.
  List<CalendarEvent> eventsForDay(DateTime day) {
    return state.events.where((e) {
      final d = e.startTime;
      return d.year == day.year && d.month == day.month && d.day == day.day;
    }).toList()
      ..sort((a, b) => a.startTime.compareTo(b.startTime));
  }
}

// ── Provider ──────────────────────────────────────────────────────────────────

final calendarProvider =
    StateNotifierProvider<CalendarNotifier, CalendarState>((ref) {
  final client = ref.watch(apiClientProvider);
  return CalendarNotifier(client);
});
