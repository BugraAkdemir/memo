import 'package:flutter_local_notifications/flutter_local_notifications.dart';
import 'package:timezone/data/latest.dart' as tzdata;
import 'package:timezone/timezone.dart' as tz;

import 'l10n.dart';
import 'platform_capabilities.dart';

/// OS-level calendar reminders on Android and iOS.
///
/// Reminders are *scheduled with the OS* rather than delivered over a live
/// connection, so they fire even when the app is closed and the phone has
/// never heard from the backend since — which is the whole point of a phone
/// client for a self-hosted assistant.
///
/// Ported from the retired mobile/ client, minus its routine notifications:
/// those polled `GET /api/routines/mobile-ready`, an endpoint the backend
/// removed in v3.9.0, so that half had already been dead code for months.
///
/// Inert on desktop and web. [init] itself checks
/// [notificationsSupported] and returns without doing anything, and every
/// other method early-returns while uninitialized — so no call site needs a
/// platform check of its own. Desktop notifications are possible (this
/// package supports Linux and macOS) but deliberately out of scope: macOS
/// needs a signed app with the right entitlement, and none of it can be
/// verified from this development setup.
class NotificationService {
  NotificationService._();

  static final _plugin = FlutterLocalNotificationsPlugin();
  static bool _initialized = false;

  /// Visible for tests only — lets a suite reset the one-shot [init] guard.
  static void resetForTest() => _initialized = false;

  /// Reminder IDs live in a reserved band so they can be cancelled and
  /// rescheduled without touching any other notification this app shows.
  static const int _reminderBase = 100000;

  static Future<void> init() async {
    if (_initialized || !notificationsSupported) return;

    tzdata.initializeTimeZones();

    const android = AndroidInitializationSettings('@mipmap/ic_launcher');
    const darwin = DarwinInitializationSettings(
      // Deliberately false, unlike the mobile client this is ported from:
      // it requested permission inside init(), which fired a system prompt
      // on first launch before the user had seen any of Memo's own UI.
      // [requestPermission] below is called from the setup wizard instead,
      // where there is room to say why it is being asked for.
      requestAlertPermission: false,
      requestBadgePermission: false,
      requestSoundPermission: false,
    );
    const settings =
        InitializationSettings(android: android, iOS: darwin, macOS: darwin);

    await _plugin.initialize(settings);
    _initialized = true;
  }

  /// Asks for notification permission, at a moment chosen by the caller.
  ///
  /// Returns false when permission was refused or is unavailable. Safe to
  /// call more than once: both platforms return the already-granted answer
  /// rather than prompting again.
  static Future<bool> requestPermission() async {
    if (!_initialized) return false;
    final android = _plugin.resolvePlatformSpecificImplementation<
        AndroidFlutterLocalNotificationsPlugin>();
    if (android != null) {
      return await android.requestNotificationsPermission() ?? false;
    }
    final darwin = _plugin.resolvePlatformSpecificImplementation<
        IOSFlutterLocalNotificationsPlugin>();
    if (darwin != null) {
      return await darwin.requestPermissions(
            alert: true,
            badge: true,
            sound: true,
          ) ??
          false;
    }
    return false;
  }

  // Channel title/description are localized at use time, so this cannot be
  // const.
  static NotificationDetails get _reminderDetails => NotificationDetails(
        android: AndroidNotificationDetails(
          'calendar_reminders',
          L10n.t('notif_channel_reminders'),
          channelDescription: L10n.t('notif_channel_reminders_desc'),
          importance: Importance.high,
          priority: Priority.high,
        ),
      );

  /// Stable per-event notification id inside the reserved band.
  ///
  /// Exposed for tests: the banding is the reason a reminder can be
  /// cancelled later without having stored anything, so it is worth
  /// asserting rather than trusting.
  static int reminderId(String eventId) =>
      _reminderBase + (eventId.hashCode & 0x7FFF);

  /// Whether a reminder for an event starting at [startUtc] with a
  /// [leadMinutes] lead time should be scheduled at all, given [nowUtc].
  ///
  /// Pure, and separate from [scheduleReminder], so the decision is testable
  /// without a platform channel — the plugin itself cannot be exercised off
  /// a device.
  static bool shouldSchedule({
    required DateTime startUtc,
    required int leadMinutes,
    required DateTime nowUtc,
  }) =>
      fireTimeUtc(startUtc: startUtc, leadMinutes: leadMinutes)
          .isAfter(nowUtc.toUtc());

  /// The instant a reminder should fire: [leadMinutes] before [startUtc].
  static DateTime fireTimeUtc({
    required DateTime startUtc,
    required int leadMinutes,
  }) =>
      startUtc.toUtc().subtract(Duration(minutes: leadMinutes));

  /// Schedules a reminder for [eventId] at [whenUtc] (an absolute instant).
  ///
  /// Scheduled in UTC so the OS fires at the right moment regardless of how
  /// the device's timezone is configured. A past [whenUtc] is a no-op rather
  /// than an error — a calendar load will routinely contain events whose
  /// reminder window has already gone by.
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
      reminderId(eventId),
      title,
      body,
      fireAt,
      _reminderDetails,
      androidScheduleMode: AndroidScheduleMode.exactAllowWhileIdle,
      uiLocalNotificationDateInterpretation:
          UILocalNotificationDateInterpretation.absoluteTime,
    );
  }

  /// Cancels a previously scheduled reminder. A no-op when nothing was
  /// scheduled for [eventId].
  static Future<void> cancelReminder(String eventId) async {
    if (!_initialized) return;
    await _plugin.cancel(reminderId(eventId));
  }
}
