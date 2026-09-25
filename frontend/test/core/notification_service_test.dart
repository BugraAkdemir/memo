import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/notification_service.dart';

/// Covers the parts of the reminder logic that do not need a device.
///
/// The plugin itself cannot be exercised off one — every method early-returns
/// while uninitialized, and init() only runs where notifications are
/// supported — which is exactly why the scheduling decision and the id
/// banding are pure, static and public rather than buried inside
/// scheduleReminder.
void main() {
  group('reminderId', () {
    test('lands inside the reserved band', () {
      // The band exists so a reminder can be cancelled later without having
      // stored its id anywhere; drifting out of it would collide with any
      // other notification this app ever shows.
      for (final id in ['abc', 'event-1', '', 'çok-uzun-bir-etkinlik-kimliği']) {
        final value = NotificationService.reminderId(id);
        expect(value, greaterThanOrEqualTo(100000));
        expect(value, lessThan(100000 + 0x8000));
      }
    });

    test('is stable for the same event id', () {
      expect(
        NotificationService.reminderId('event-42'),
        NotificationService.reminderId('event-42'),
      );
    });

    test('differs between two different event ids', () {
      expect(
        NotificationService.reminderId('event-1'),
        isNot(NotificationService.reminderId('event-2')),
      );
    });
  });

  group('fireTimeUtc', () {
    test('is the lead time before the start, in UTC', () {
      final start = DateTime.utc(2026, 9, 26, 12, 0);
      expect(
        NotificationService.fireTimeUtc(startUtc: start, leadMinutes: 30),
        DateTime.utc(2026, 9, 26, 11, 30),
      );
    });

    test('converts a local start time rather than trusting its fields', () {
      final localStart = DateTime(2026, 9, 26, 12, 0);
      expect(
        NotificationService.fireTimeUtc(startUtc: localStart, leadMinutes: 0),
        localStart.toUtc(),
      );
    });

    test('a zero lead means "at the start"', () {
      final start = DateTime.utc(2026, 9, 26, 12, 0);
      expect(
        NotificationService.fireTimeUtc(startUtc: start, leadMinutes: 0),
        start,
      );
    });
  });

  group('shouldSchedule', () {
    final now = DateTime.utc(2026, 9, 26, 12, 0);

    test('true when the reminder instant is still ahead', () {
      expect(
        NotificationService.shouldSchedule(
          startUtc: DateTime.utc(2026, 9, 26, 13, 0),
          leadMinutes: 30,
          nowUtc: now,
        ),
        isTrue,
      );
    });

    test('false once the reminder window has passed, even if the event has not', () {
      // 12:20 start with a 30-minute lead means the reminder was due at
      // 11:50 — arming it now would fire immediately for something the user
      // was supposed to have been warned about ten minutes ago.
      expect(
        NotificationService.shouldSchedule(
          startUtc: DateTime.utc(2026, 9, 26, 12, 20),
          leadMinutes: 30,
          nowUtc: now,
        ),
        isFalse,
      );
    });

    test('false for an event that already happened', () {
      expect(
        NotificationService.shouldSchedule(
          startUtc: DateTime.utc(2026, 9, 26, 9, 0),
          leadMinutes: 30,
          nowUtc: now,
        ),
        isFalse,
      );
    });

    test('compares in UTC, not by local wall-clock fields', () {
      final localNow = now.toLocal();
      expect(
        NotificationService.shouldSchedule(
          startUtc: DateTime.utc(2026, 9, 26, 14, 0),
          leadMinutes: 0,
          nowUtc: localNow,
        ),
        isTrue,
      );
    });
  });

  group('inert while uninitialized', () {
    // init() returns early off mobile, so these must not reach a platform
    // channel (which would throw MissingPluginException under flutter test).
    test('scheduleReminder and cancelReminder are no-ops', () async {
      await NotificationService.scheduleReminder(
        eventId: 'e1',
        title: 'Standup',
        whenUtc: DateTime.now().toUtc().add(const Duration(hours: 1)),
        minutesBefore: 30,
      );
      await NotificationService.cancelReminder('e1');
    });

    test('requestPermission reports false rather than throwing', () async {
      expect(await NotificationService.requestPermission(), isFalse);
    });
  });
}
