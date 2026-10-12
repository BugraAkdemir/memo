import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/core/calendar_reminders.dart';

void main() {
  final now = DateTime.utc(2026, 10, 12, 10);
  final event = CalendarReminder(
    'one',
    'old title',
    now.add(const Duration(hours: 2)),
  );
  test(
    'deletes out-of-band alarms including alarms from a previous launch',
    () async {
      final pending = {'one', 'removed-on-server'};
      final cancelled = <String>[];
      final armed = <String>[];
      final r = CalendarReminderReconciler(
        pending: () async => pending,
        cancel: (id) async {
          cancelled.add(id);
        },
        schedule: (e, _) async {
          armed.add(e.id);
        },
      );
      await r.sync([event], 30, now);
      expect(cancelled, ['removed-on-server']);
      expect(armed, ['one']);
      await r.sync([], 30, now);
      expect(cancelled, contains('one'));
    },
  );
  test(
    'title, time and lead changes rearm; identical snapshot does not',
    () async {
      var count = 0;
      final r = CalendarReminderReconciler(
        pending: () async => {},
        cancel: (_) async {},
        schedule: (_, _) async {
          count++;
        },
      );
      await r.sync([event], 30, now);
      await r.sync([event], 30, now);
      expect(count, 1);
      await r.sync(
        [CalendarReminder('one', 'new title', event.startUtc)],
        30,
        now,
      );
      expect(count, 2);
      await r.sync(
        [
          CalendarReminder(
            'one',
            'new title',
            event.startUtc.add(const Duration(hours: 1)),
          ),
        ],
        30,
        now,
      );
      expect(count, 3);
      await r.sync([event], 60, now);
      expect(count, 4);
    },
  );
  test('failed platform operation does not commit fingerprint', () async {
    var fail = true;
    var count = 0;
    final r = CalendarReminderReconciler(
      pending: () async => {},
      cancel: (_) async {},
      schedule: (_, _) async {
        count++;
        if (fail) throw StateError('platform unavailable');
      },
    );
    await expectLater(r.sync([event], 30, now), throwsStateError);
    fail = false;
    await r.sync([event], 30, now);
    expect(count, 2);
  });
  test('invalid dates ignored and elapsed reminders cancelled', () async {
    expect(
      CalendarReminder.fromJson({
        'id': 'one',
        'title': 'bad',
        'start_time': 'no date',
      }),
      isNull,
    );
    final cancelled = <String>[];
    var count = 0;
    final r = CalendarReminderReconciler(
      pending: () async => {'one'},
      cancel: (id) async {
        cancelled.add(id);
      },
      schedule: (_, _) async {
        count++;
      },
    );
    await r.sync([event], 30, now.add(const Duration(hours: 3)));
    expect(count, 0);
    expect(cancelled, ['one']);
  });
}
