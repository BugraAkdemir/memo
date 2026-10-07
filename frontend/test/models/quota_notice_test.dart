import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/models/quota_notice.dart';

final _now = DateTime.utc(2026, 10, 7, 12, 0, 0);

String _json(Map<String, Object?> m) => json.encode(m);

void main() {
  group('parsing a marker chunk', () {
    test('reads every field of an exhausted notice', () {
      final n = QuotaNotice.tryParse(
        _json({
          'kind': 'exhausted',
          'model': 'gpt-5.5',
          'vendor': 'openai',
          'remaining_percent': 0,
          'reset_at': '2026-10-07T15:00:00Z',
          'window': '5h',
        }),
        now: _now,
      )!;
      expect(n.exhausted, isTrue);
      expect(n.low, isFalse);
      expect(n.model, 'gpt-5.5');
      expect(n.vendor, 'openai');
      expect(n.remainingPercent, 0);
      expect(n.window, '5h');
      expect(n.resetAt!.toUtc(), DateTime.utc(2026, 10, 7, 15));
    });

    test('a low notice parses too, with no reset time at all', () {
      final n = QuotaNotice.tryParse(_json({'kind': 'low', 'remaining_percent': 8}), now: _now)!;
      expect(n.low, isTrue);
      expect(n.remainingPercent, 8);
      expect(n.resetAt, isNull);
    });

    test('anything that is not a notice is rejected, never half-parsed', () {
      expect(QuotaNotice.tryParse('', now: _now), isNull);
      expect(QuotaNotice.tryParse('not json', now: _now), isNull);
      expect(QuotaNotice.tryParse('[1,2]', now: _now), isNull);
      expect(QuotaNotice.tryParse(_json({'kind': 'mystery'}), now: _now), isNull);
      expect(QuotaNotice.tryParse(_json({'model': 'x'}), now: _now), isNull);
    });

    test('a missing percentage is -1 (unknown), not 0 (empty)', () {
      expect(QuotaNotice.tryParse(_json({'kind': 'exhausted'}), now: _now)!.remainingPercent, -1);
    });

    test('an unparseable reset time is treated as unknown', () {
      final n = QuotaNotice.tryParse(_json({'kind': 'exhausted', 'reset_at': 'tomorrow-ish'}), now: _now)!;
      expect(n.resetAt, isNull);
      expect(n.knownReset, isFalse);
    });
  });

  group('when to continue', () {
    test('a refill time still ahead: that time plus the safety margin', () {
      final n = QuotaNotice(kind: 'exhausted', resetAt: _now.add(const Duration(hours: 3)), receivedAt: _now);
      expect(n.knownReset, isTrue);
      expect(n.resumeAt, _now.add(const Duration(hours: 3)).add(kQuotaResumeMargin));
    });

    test('no refill time: five minutes the first time, ten after that', () {
      expect(QuotaNotice(kind: 'exhausted', receivedAt: _now).resumeAt, _now.add(const Duration(minutes: 5)));
      expect(QuotaNotice(kind: 'exhausted', receivedAt: _now, attempt: 1).resumeAt, _now.add(const Duration(minutes: 10)));
      expect(QuotaNotice(kind: 'exhausted', receivedAt: _now, attempt: 7).resumeAt, _now.add(const Duration(minutes: 10)));
      expect(QuotaNotice(kind: 'exhausted', receivedAt: _now).knownReset, isFalse);
    });

    test('a refill time that already passed means the vendor still refuses: back off, do not retry at once', () {
      final n = QuotaNotice(kind: 'exhausted', resetAt: _now.subtract(const Duration(minutes: 1)), receivedAt: _now);
      expect(n.knownReset, isFalse);
      expect(n.resumeAt.isAfter(_now.add(const Duration(minutes: 4))), isTrue,
          reason: 'retrying at once would hammer a refusal that persists');
    });

    test('the time is fixed when the notice is made, not recomputed against a moving clock', () {
      final n = QuotaNotice(kind: 'exhausted', receivedAt: _now);
      final first = n.resumeAt;
      expect(n.resumeAt, first);
    });
  });
}
