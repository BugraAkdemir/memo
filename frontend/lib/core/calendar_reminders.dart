import 'dart:convert';
import 'l10n.dart';
import 'notification_service.dart';

class CalendarReminder {
  final String id;
  final String title;
  final DateTime startUtc;
  const CalendarReminder(this.id, this.title, this.startUtc);
  static CalendarReminder? fromJson(Map<String, dynamic> json) {
    final id = json['id'], title = json['title'], start = json['start_time'];
    if (id is! String || id.isEmpty || title is! String || start is! String) {
      return null;
    }
    final time = DateTime.tryParse(start);
    return time == null ? null : CalendarReminder(id, title, time.toUtc());
  }
}

/// Full snapshot reconciliation; commit fingerprints after all OS calls succeed.
class CalendarReminderReconciler {
  final Future<Set<String>> Function() pending;
  final Future<void> Function(String) cancel;
  final Future<void> Function(CalendarReminder, int) schedule;
  CalendarReminderReconciler({
    required this.pending,
    required this.cancel,
    required this.schedule,
  });
  Set<String> _armed = {};
  String? _fingerprint;
  Future<void> _tail = Future.value();
  void invalidate() => _fingerprint = null;
  Future<void> sync(List<CalendarReminder> events, int lead, DateTime now) {
    final copy = List<CalendarReminder>.of(events);
    final next = _tail
        .catchError((Object _) {})
        .then((_) => _sync(copy, lead, now));
    _tail = next;
    return next;
  }

  Future<void> _sync(
    List<CalendarReminder> events,
    int lead,
    DateTime now,
  ) async {
    final desired = <String, CalendarReminder>{};
    for (final e in events) {
      if (NotificationService.shouldSchedule(
        startUtc: e.startUtc,
        leadMinutes: lead,
        nowUtc: now,
      )) {
        desired[e.id] = e;
      }
    }
    final sorted = desired.values.toList()
      ..sort((a, b) => a.id.compareTo(b.id));
    final fingerprint = jsonEncode([
      lead,
      L10n.locale.name,
      for (final e in sorted) [e.id, e.title, e.startUtc.toIso8601String()],
    ]);
    if (fingerprint == _fingerprint) return;
    final previous = {..._armed, ...await pending()};
    for (final id in previous.difference(desired.keys.toSet())) {
      await cancel(id);
    }
    for (final e in sorted) {
      await schedule(e, lead);
    }
    _armed = desired.keys.toSet();
    _fingerprint = fingerprint;
  }
}
