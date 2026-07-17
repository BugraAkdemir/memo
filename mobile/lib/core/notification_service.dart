import 'dart:convert';

import 'package:flutter_local_notifications/flutter_local_notifications.dart';
import 'package:timezone/data/latest.dart' as tzdata;
import 'package:timezone/timezone.dart' as tz;

import 'l10n.dart';

/// Wraps flutter_local_notifications for calendar reminders.
///
/// Reminders are *scheduled* at the OS level (via [scheduleReminder]) so they
/// fire even when the app is closed or in the background — no live connection
/// to the desktop is required once the event is known.
class NotificationService {
  NotificationService._();

  static final _plugin = FlutterLocalNotificationsPlugin();
  static bool _initialized = false;

  // Reminder notification IDs live in a reserved band so we can cancel/reschedule
  // them without touching transient "added" notifications.
  static const int _reminderBase = 100000;

  // Monotonically incrementing ID for transient (fire-and-forget) notifications.
  // Avoids hash collisions between different titles shown in quick succession.
  static int _transientCounter = 0;
  static int _nextTransientId() => (_transientCounter++ & 0x7FFF);

  static Future<void> init() async {
    if (_initialized) return;

    tzdata.initializeTimeZones();

    const android = AndroidInitializationSettings('@mipmap/ic_launcher');
    const darwin = DarwinInitializationSettings(
      requestAlertPermission: true,
      requestBadgePermission: true,
      requestSoundPermission: true,
    );
    const settings = InitializationSettings(android: android, iOS: darwin, macOS: darwin);

    await _plugin.initialize(settings);

    // Android 13+ runtime notification permission.
    final android13 = _plugin.resolvePlatformSpecificImplementation<
        AndroidFlutterLocalNotificationsPlugin>();
    await android13?.requestNotificationsPermission();

    _initialized = true;
  }

  // Channel titles/descriptions are localized at use time (cannot be const).
  static NotificationDetails get _reminderDetails => NotificationDetails(
        android: AndroidNotificationDetails(
          'calendar_reminders',
          L10n.t('notif_channel_reminders'),
          channelDescription: L10n.t('notif_channel_reminders_desc'),
          importance: Importance.high,
          priority: Priority.high,
        ),
      );

  static int _reminderId(String eventId) =>
      _reminderBase + (eventId.hashCode & 0x7FFF);

  /// Schedules a reminder for [eventId] to fire at [whenUtc] (absolute instant).
  /// Scheduling in UTC means the OS fires at the correct moment regardless of
  /// the device's local timezone configuration. No-op if [whenUtc] is in the past.
  static Future<void> scheduleReminder({
    required String eventId,
    required String title,
    required DateTime whenUtc,
    required int minutesBefore,
  }) async {
    if (!_initialized) return;
    final fireAt = tz.TZDateTime.from(whenUtc.toUtc(), tz.UTC);
    if (fireAt.isBefore(tz.TZDateTime.now(tz.UTC))) return;

    final body = minutesBefore > 0
        ? L10n.t('notif_starts_in_min', {'n': '$minutesBefore'})
        : L10n.t('notif_starts_soon');

    await _plugin.zonedSchedule(
      _reminderId(eventId),
      title,
      body,
      fireAt,
      _reminderDetails,
      androidScheduleMode: AndroidScheduleMode.exactAllowWhileIdle,
      uiLocalNotificationDateInterpretation:
          UILocalNotificationDateInterpretation.absoluteTime,
    );
  }

  /// Cancels a previously scheduled reminder.
  static Future<void> cancelReminder(String eventId) async {
    if (!_initialized) return;
    await _plugin.cancel(_reminderId(eventId));
  }

  /// Shows an immediate "added to calendar" notification.
  static Future<void> showCalendarAdded(String title) async {
    if (!_initialized) return;
    final details = NotificationDetails(
      android: AndroidNotificationDetails(
        'calendar_added',
        L10n.t('notif_channel_added'),
        channelDescription: L10n.t('notif_channel_added_desc'),
        importance: Importance.defaultImportance,
        priority: Priority.defaultPriority,
      ),
    );
    await _plugin.show(
      _nextTransientId(),
      L10n.t('notif_added_title'),
      title,
      details,
    );
  }

  static NotificationDetails get _routineDetails => NotificationDetails(
        android: AndroidNotificationDetails(
          'routines',
          L10n.t('notif_channel_routines'),
          channelDescription: L10n.t('notif_channel_routines_desc'),
          importance: Importance.high,
          priority: Priority.high,
        ),
      );

  // Own reserved ID band, distinct from _reminderBase, so a routine and a
  // calendar reminder can never collide on the same notification ID.
  static const int _routineBase = 200000;

  static int _routineId(String routineId) =>
      _routineBase + (routineId.hashCode & 0x7FFF);

  /// Schedules a routine's pre-generated content to fire at [whenUtc]
  /// (absolute instant) — mirrors scheduleReminder's OS-level exact alarm so
  /// it fires even if the app is closed/asleep by then. The body text is
  /// already fully generated by the backend ahead of time (routines have no
  /// live push channel to compute it at the exact fire moment), unlike
  /// scheduleReminder's minutesBefore-derived text.
  static Future<void> scheduleRoutine({
    required String routineId,
    required String title,
    required String body,
    required DateTime whenUtc,
  }) async {
    if (!_initialized) return;
    final fireAt = tz.TZDateTime.from(whenUtc.toUtc(), tz.UTC);
    if (fireAt.isBefore(tz.TZDateTime.now(tz.UTC))) return;

    await _plugin.zonedSchedule(
      _routineId(routineId),
      title,
      body,
      fireAt,
      _routineDetails,
      androidScheduleMode: AndroidScheduleMode.exactAllowWhileIdle,
      uiLocalNotificationDateInterpretation:
          UILocalNotificationDateInterpretation.absoluteTime,
    );
  }

  /// Cancels a previously scheduled routine notification.
  static Future<void> cancelRoutine(String routineId) async {
    if (!_initialized) return;
    await _plugin.cancel(_routineId(routineId));
  }

  /// Parses a "calendar:reminder" SSE payload and shows it immediately.
  /// Used only as a foreground fallback; primary delivery is via scheduling.
  static Future<void> showCalendarReminder(String payload) async {
    if (!_initialized) return;
    Map<String, dynamic> data;
    try {
      data = jsonDecode(payload) as Map<String, dynamic>;
    } catch (_) {
      return;
    }
    final title =
        data['title'] as String? ?? L10n.t('notif_event_reminder');
    final minutesLeft = data['minutes_left'] as int? ?? 0;
    final body = minutesLeft > 0
        ? L10n.t('notif_starts_in_min', {'n': '$minutesLeft'})
        : L10n.t('notif_starts_now');
    await _plugin.show(
      _nextTransientId(),
      title,
      body,
      _reminderDetails,
    );
  }
}
