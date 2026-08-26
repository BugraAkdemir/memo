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

    // Regression test for a real bug found via a real live test: the
    // playback subprocess launched fine (isPlaying=true immediately after
    // start()) but exited almost instantly because no PulseAudio/
    // PipeWire-pulse socket was reachable in that environment -- with
    // stdout/stderr both drained and discarded and no exitCode listener,
    // that failure was completely invisible; real audio frames kept being
    // written to a dead process's stdin with no error anywhere in the app.
    // Uses `false` (always exits 1, no real audio backend needed) so this
    // is portable to any Linux test runner, matching the existing
    // linuxPlayerCommands-override pattern WavPlayer's own tests use.
    test('onError fires when the subprocess exits unexpectedly', () async {
      final player = LiveModePcmPlayer(linuxPlayerCommands: ['false']);
      final errorFuture = player.onError.first;
      await player.start(24000);
      final message = await errorFuture.timeout(const Duration(seconds: 5));
      expect(message, contains('false'));
      expect(message, contains('exited unexpectedly'));
      expect(player.isPlaying, isFalse);
    });

    test('a deliberate stop() does not fire onError', () async {
      // `yes` treats every argument (including the "--raw"-shaped flags
      // rawArgsFor generates) as a literal string to repeat forever rather
      // than an option it tries to parse -- unlike most coreutils, so it
      // survives start() and just sits there (never touching stdin) until
      // stop() kills it. Picked specifically to avoid the process exiting
      // on its own from unrecognized-option argv parsing, which would
      // otherwise make this test indistinguishable from the real bug.
      final player = LiveModePcmPlayer(linuxPlayerCommands: ['yes']);
      var errorFired = false;
      player.onError.listen((_) => errorFired = true);
      await player.start(24000);
      await player.stop();
      // Give any (incorrect) exitCode-triggered error a moment to surface.
      await Future<void>.delayed(const Duration(milliseconds: 200));
      expect(errorFired, isFalse);
    });
  });
}
