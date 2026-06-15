import 'dart:convert';

import 'package:flutter_local_notifications/flutter_local_notifications.dart';
import 'package:timezone/data/latest.dart' as tzdata;
import 'package:timezone/timezone.dart' as tz;

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

  static const _reminderDetails = NotificationDetails(
    android: AndroidNotificationDetails(
      'calendar_reminders',
      'Takvim Hatırlatmaları',
      channelDescription: 'Yaklaşan etkinlikler için hatırlatmalar',
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
        ? '$minutesBefore dakika sonra başlıyor'
        : 'Birazdan başlıyor';

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
    const details = NotificationDetails(
      android: AndroidNotificationDetails(
        'calendar_added',
        'Takvim Eklentileri',
        channelDescription: 'Otomatik eklenen etkinlik bildirimleri',
        importance: Importance.defaultImportance,
        priority: Priority.defaultPriority,
      ),
    );
    await _plugin.show(
      title.hashCode & 0x7FFF,
      'Takvime eklendi',
      title,
      details,
    );
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
    final title = data['title'] as String? ?? 'Etkinlik hatırlatması';
    final minutesLeft = data['minutes_left'] as int? ?? 0;
    final body =
        minutesLeft > 0 ? '$minutesLeft dakika sonra başlıyor' : 'Şimdi başlıyor';
    await _plugin.show(
      (title + 'r').hashCode & 0x7FFF,
      title,
      body,
      _reminderDetails,
    );
  }
}
