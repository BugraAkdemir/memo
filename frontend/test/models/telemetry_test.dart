import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/models/telemetry.dart';

void main() {
  group('TelemetryStatus.fromJson', () {
    test('reads the backend answer', () {
      final s = TelemetryStatus.fromJson({
        'enabled': true,
        'consent': 'yes',
        'env_off': false,
        'build': 'official',
        'endpoint': 'https://tel.memocpp.com',
        'device_id': '0f3c9a1e',
        'consent_day': '2026-10-26',
        'last_sent_day': '2026-10-27',
      });
      expect(s.enabled, isTrue);
      expect(s.sharing, isTrue);
      expect(s.answered, isTrue);
      expect(s.build, 'official');
      expect(s.deviceId, '0f3c9a1e');
      expect(s.lastSentDay, '2026-10-27');
    });

    test('a source build without an answer shares nothing', () {
      final s = TelemetryStatus.fromJson({'consent': '', 'build': 'source'});
      expect(s.answered, isFalse);
      expect(s.sharing, isFalse);
    });

    test('an environment switch beats an answer of yes', () {
      final s = TelemetryStatus.fromJson({
        'consent': 'yes',
        'enabled': false,
        'env_off': true,
      });
      expect(s.envOff, isTrue);
      expect(
        s.sharing,
        isFalse,
        reason:
            'MEMO_TELEMETRY=off means nothing is sent, whatever was answered',
      );
    });

    test('junk never turns anything on', () {
      for (final junk in [
        null,
        'x',
        5,
        <Object>[],
        {'enabled': 'yes', 'consent': 7},
      ]) {
        final s = TelemetryStatus.fromJson(junk);
        expect(s.enabled, isFalse);
        expect(s.sharing, isFalse);
      }
    });
  });

  group('TelemetryPreview.fromJson', () {
    test('keeps the exact JSON text as sent', () {
      const raw = '{"schema":1,"day":"2026-10-26","usage":[]}';
      final p = TelemetryPreview.fromJson({
        'day': '2026-10-26',
        'empty': false,
        'json': raw,
      });
      expect(p.json, raw);
      expect(p.empty, isFalse);
      expect(p.day, '2026-10-26');
    });

    test('a quiet day is empty', () {
      final p = TelemetryPreview.fromJson({
        'day': '',
        'empty': true,
        'json': '',
      });
      expect(p.empty, isTrue);
    });

    test('junk is an empty preview, not a crash', () {
      expect(TelemetryPreview.fromJson(null).empty, isTrue);
      expect(TelemetryPreview.fromJson({'json': 5}).json, '');
    });
  });
}
