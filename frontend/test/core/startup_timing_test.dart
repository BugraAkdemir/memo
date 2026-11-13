import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/core/startup_timing.dart';

void main() {
  group('StartupTiming', () {
    test('records monotonic stages and external epoch markers in order', () {
      var tickMs = 100;
      final timing = StartupTiming(
        nowEpochMs: () => 1700000000000 + tickMs,
        elapsedMs: () => tickMs,
        environment: const {
          'MEMO_LAUNCH_START_MS': '1699999999500',
          'MEMO_BACKEND_READY_MS': '1699999999750',
          'MEMO_NATIVE_ENTRY_MS': '1699999999900',
          'MEMO_NATIVE_CONTROLLER_MS': '1700000000050',
          'MEMO_NATIVE_PLUGINS_MS': '1700000000080',
        },
      );

      timing.mark('dart_main');
      tickMs = 135;
      timing.mark('prefs_ready');
      tickMs = 210;
      timing.mark('first_frame_rasterized');

      // Marking the same stage twice keeps the first timestamp.
      tickMs = 999;
      timing.mark('dart_main');

      final report = timing.formatReport();
      expect(report, contains('[startup]'));
      expect(report, contains('launcher_start=0ms'));
      expect(report, contains('backend_ready=250ms'));
      expect(report, contains('native_entry=400ms'));
      expect(report, contains('dart_main=600ms'));
      expect(report, contains('prefs_ready=635ms'));
      expect(report, contains('first_frame_rasterized=710ms'));
      expect(report, contains('total=710ms'));
    });

    test('ignores malformed external markers without throwing', () {
      var tickMs = 0;
      final timing = StartupTiming(
        nowEpochMs: () => 1700000000000 + tickMs,
        elapsedMs: () => tickMs,
        environment: const {
          'MEMO_LAUNCH_START_MS': 'not-a-number',
          'MEMO_NATIVE_ENTRY_MS': '-5',
        },
      );

      timing.mark('dart_main');
      tickMs = 42;
      timing.mark('first_frame_rasterized');

      final report = timing.formatReport();
      expect(report, contains('dart_main=0ms'));
      expect(report, contains('first_frame_rasterized=42ms'));
      expect(report, isNot(contains('launcher_start=')));
    });

    test('appends report line to configured log path without throwing on bad paths', () async {
      final dir = await Directory.systemTemp.createTemp('memo_startup_test_');
      addTearDown(() => dir.delete(recursive: true));
      final logPath = '${dir.path}/nested/startup-timing.log';

      var tickMs = 0;
      final timing = StartupTiming(
        nowEpochMs: () => 1700000000000 + tickMs,
        elapsedMs: () => tickMs,
        environment: {'MEMO_STARTUP_LOG': logPath},
      );

      timing.mark('dart_main');
      tickMs = 15;
      timing.mark('first_frame_rasterized');

      await timing.flushToLog();

      final contents = await File(logPath).readAsString();
      expect(contents.trim(), contains('[startup]'));
      expect(contents, contains('first_frame_rasterized=15ms'));
    });

    test('swallows log writer failures without throwing', () async {
      final timing = StartupTiming(
        nowEpochMs: () => 1700000000000,
        elapsedMs: () => 5,
        environment: const {'MEMO_STARTUP_LOG': '/tmp/ignored.log'},
        logWriter: (_, _) => Future<void>.error(const FileSystemException('denied')),
      );

      timing.mark('dart_main');
      await expectLater(timing.flushToLog(), completes);
    });
  });
}
