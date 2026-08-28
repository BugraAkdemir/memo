import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/core/live_pcm_player.dart';

void main() {
  group('LiveModePcmPlayer.rawArgsFor', () {
    // No trailing filename/"-" argument for either command: both read
    // stdin automatically when none is given (confirmed against the
    // pulseaudio-utils/alsa-utils man pages) -- passing "-" to pacat's
    // sibling `paplay` was the real bug a real live test surfaced (it
    // tried to open("-") as a literal filename and failed with ENOENT).
    test('pacat: playback mode, s16le mono at the given rate, reading stdin', () {
      expect(
        LiveModePcmPlayer.rawArgsFor('pacat', 24000),
        ['--playback', '--format=s16le', '--channels=1', '--rate=24000'],
      );
    });

    test('aplay: raw s16le mono at the given rate, reading stdin', () {
      expect(
        LiveModePcmPlayer.rawArgsFor('aplay', 16000),
        ['-t', 'raw', '-f', 'S16_LE', '-c', '1', '-r', '16000'],
      );
    });

    test('matches by basename, not full path', () {
      expect(
        LiveModePcmPlayer.rawArgsFor('/usr/bin/pacat', 24000),
        ['--playback', '--format=s16le', '--channels=1', '--rate=24000'],
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

    // Regression test for a second bug found in the same real live test:
    // the first fix reported "exited unexpectedly (code 1)" with the
    // stderr detail always empty, because process.exitCode resolving
    // doesn't guarantee the separate stderr stream has already delivered
    // its data to a plain listen() callback. `cat` reliably refuses the
    // pacat-shaped `--xxx`-style flags rawArgsFor generates (GNU
    // coreutils treats unrecognized `--xxx` arguments as invalid options,
    // not filenames) and writes a real message to stderr before exiting
    // nonzero -- exactly the shape of failure this needs to survive.
    test('onError includes the actual stderr detail, not just the exit code', () async {
      final player = LiveModePcmPlayer(linuxPlayerCommands: ['cat']);
      final errorFuture = player.onError.first;
      await player.start(24000);
      final message = await errorFuture.timeout(const Duration(seconds: 5));
      expect(message, contains('cat'));
      expect(message, contains('exited unexpectedly'));
      expect(message, contains(':'), reason: 'expected a ": <stderr detail>" suffix, got: $message');
    });

    test('a deliberate stop() does not fire onError', () async {
      // `yes` treats every argument (including the pacat-shaped flags
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
