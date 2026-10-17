import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/models/app_update.dart';

void main() {
  group('AppUpdateInfo.fromJson', () {
    test('reads a full answer, components included', () {
      final i = AppUpdateInfo.fromJson({
        'current': 'V4.6.0',
        'latest': 'V4.7.0',
        'available': true,
        'required': true,
        'note_tr': 'tr not',
        'note_en': 'en note',
        'mode': 'auto',
        'size_bytes': 800000000,
        'download_url': 'https://memocpp.com',
        'components': [
          {'name': 'llama', 'folder': 'cpu', 'installed': 'b9441', 'latest': 'b11456', 'available': true, 'size_bytes': 5000},
          {'name': 'whisper', 'reason': 'up_to_date'},
        ],
      });
      expect(i.available, isTrue);
      expect(i.required, isTrue);
      expect(i.canUpdateInPlace, isTrue);
      expect(i.sizeBytes, 800000000);
      expect(i.components.length, 2);
      expect(i.pendingComponents.map((c) => c.name), ['llama']);
      expect(i.components.first.installed, 'b9441');
    });

    test('survives junk instead of throwing', () {
      expect(AppUpdateInfo.fromJson(null).available, isFalse);
      expect(AppUpdateInfo.fromJson('x').mode, 'manual');
      final i = AppUpdateInfo.fromJson({
        'current': 5,
        'available': 'yes', // not a bool: must not count as true
        'required': 1,
        'size_bytes': 'big',
        'components': ['not a map', 7, null, {'name': 'llama', 'available': true}],
      });
      expect(i.available, isFalse);
      expect(i.required, isFalse);
      expect(i.current, '');
      expect(i.sizeBytes, 0);
      expect(i.components.length, 1);
    });

    test('only an in-place mode is one-click', () {
      expect(AppUpdateInfo.fromJson({'mode': 'installer'}).canUpdateInPlace, isTrue);
      expect(AppUpdateInfo.fromJson({'mode': 'manual'}).canUpdateInPlace, isFalse);
      expect(AppUpdateInfo.fromJson({'mode': 'whatever'}).canUpdateInPlace, isFalse);
    });

    test('noteFor picks the UI language and falls back to the other', () {
      const both = AppUpdateInfo(noteTr: 'tr', noteEn: 'en');
      expect(both.noteFor(true), 'tr');
      expect(both.noteFor(false), 'en');
      const onlyEn = AppUpdateInfo(noteEn: 'en');
      expect(onlyEn.noteFor(true), 'en');
      const onlyTr = AppUpdateInfo(noteTr: '  tr  ');
      expect(onlyTr.noteFor(false), '  tr  ');
      expect(const AppUpdateInfo().noteFor(true), '');
    });
  });

  group('UpdateProgress', () {
    test('fraction, busy and terminal states', () {
      final p = UpdateProgress.fromJson({'state': 'downloading', 'done': 25, 'total': 100});
      expect(p.fraction, 0.25);
      expect(p.isBusy, isTrue);
      expect(UpdateProgress.fromJson({'state': 'downloading', 'done': 5, 'total': 0}).fraction, isNull);
      expect(UpdateProgress.fromJson({'state': 'downloading', 'done': 500, 'total': 100}).fraction, 1.0);
      expect(UpdateProgress.fromJson({'state': 'restarting'}).isRestarting, isTrue);
      expect(UpdateProgress.fromJson({'state': 'done'}).isBusy, isFalse);
      expect(UpdateProgress.fromJson({'state': 'error', 'error': 'x'}).isError, isTrue);
      expect(UpdateProgress.fromJson(null).state, 'idle');
      expect(UpdateProgress.fromJson({'state': 7}).state, 'idle');
    });
  });

  test('formatUpdateBytes', () {
    expect(formatUpdateBytes(0), '');
    expect(formatUpdateBytes(-5), '');
    expect(formatUpdateBytes(900), '900 B');
    expect(formatUpdateBytes(2048), '2 KB');
    expect(formatUpdateBytes(5 * 1024 * 1024), '5.0 MB');
    expect(formatUpdateBytes(778 * 1024 * 1024), '778 MB');
    expect(formatUpdateBytes((1.5 * 1024 * 1024 * 1024).round()), '1.5 GB');
  });
}
