import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/core/live_pcm_player.dart';

void main() {
  group('LiveModePcmPlayer.rawArgsFor', () {
    test('paplay: raw s16le mono at the given rate, reading stdin', () {
      expect(
        LiveModePcmPlayer.rawArgsFor('paplay', 24000),
        ['--raw', '--format=s16le', '--channels=1', '--rate=24000', '-'],
      );
    });

    test('aplay: raw s16le mono at the given rate', () {
      expect(
        LiveModePcmPlayer.rawArgsFor('aplay', 16000),
        ['-t', 'raw', '-f', 'S16_LE', '-c', '1', '-r', '16000', '-'],
      );
    });

    test('matches by basename, not full path', () {
      expect(
        LiveModePcmPlayer.rawArgsFor('/usr/bin/paplay', 24000),
        ['--raw', '--format=s16le', '--channels=1', '--rate=24000', '-'],
      );
    });
  });

  group('LiveModePcmPlayer lifecycle', () {
    test('isPlaying is false before start()', () {
      final player = LiveModePcmPlayer();
      expect(player.isPlaying, isFalse);
    });

    test('write() before start() does not throw', () {
      final player = LiveModePcmPlayer();
      player.write(Uint8List.fromList([1, 2, 3, 4]));
    });

    test('stop() before start() does not throw', () async {
      final player = LiveModePcmPlayer();
      await player.stop(); // must not throw
    });

    test('dispose() before start() does not throw', () {
      final player = LiveModePcmPlayer();
      player.dispose(); // must not throw
    });
  });
}
