import 'dart:io';
import 'dart:typed_data';

import 'package:flutter/foundation.dart' show TargetPlatform;
import 'package:flutter_test/flutter_test.dart';
import 'package:path/path.dart' as p;

import 'package:memo_flutter/core/native_clip_player.dart';
import 'package:memo_flutter/core/wav_player.dart';

/// Records what it was asked to play instead of touching audio hardware.
class _FakeClipPlayer implements NativeClipPlayer {
  _FakeClipPlayer({this.onPlay});

  /// Runs while the temp file is still supposed to exist, so a test can
  /// assert on it (or delete it, to simulate a cleanup failure).
  final void Function(String path)? onPlay;

  final List<String> playedPaths = [];
  final List<double> playedVolumes = [];
  int stopCount = 0;
  int disposeCount = 0;

  @override
  Future<void> playFile(String path, double volume) async {
    playedPaths.add(path);
    playedVolumes.add(volume);
    onPlay?.call(path);
  }

  @override
  Future<void> stop() async => stopCount++;

  @override
  void dispose() => disposeCount++;
}

// These tests only exercise WavPlayer's Linux subprocess path (the code
// under test checks Platform.isLinux internally, and `flutter test` runs on
// the host OS -- this dev machine is Linux, matching CI). Each one pins
// `backend: WavPlayerBackend.subprocess`, which is required, not decorative:
// flutter_test reports defaultTargetPlatform as android, so without it every
// test here would take the mobile plugin path instead. `true`/`false`
// stand in for `paplay`/`aplay`: both are coreutils that ignore their
// arguments and just exit 0/1 respectively, so tests don't depend on this
// machine's actual audio stack (CI runners commonly have neither PulseAudio/
// PipeWire nor a real ALSA device configured).
void main() {
  // Minimal valid WAV header (44 bytes, zero audio data) -- content doesn't
  // matter here since the fake player commands never actually read it.
  final fakeWav = Uint8List.fromList(List.filled(44, 0));

  test(
    'play() succeeds when the first candidate command exits 0',
    () async {
      final player = WavPlayer(backend: WavPlayerBackend.subprocess, linuxPlayerCommands: const ['true']);
      await player.play(fakeWav); // must not throw
    },
    skip: !Platform.isLinux,
  );

  test(
    'play() falls back to the next candidate when the first is not found',
    () async {
      final player = WavPlayer(
        backend: WavPlayerBackend.subprocess,
        linuxPlayerCommands: const ['memo-test-nonexistent-player', 'true'],
      );
      await player.play(fakeWav); // must not throw -- falls back to "true"
    },
    skip: !Platform.isLinux,
  );

  test(
    'play() falls back to the next candidate when the first exits non-zero',
    () async {
      final player = WavPlayer(backend: WavPlayerBackend.subprocess, linuxPlayerCommands: const ['false', 'true']);
      await player.play(fakeWav); // must not throw -- falls back to "true"
    },
    skip: !Platform.isLinux,
  );

  test(
    'play() throws a clear error when every candidate fails',
    () async {
      final player = WavPlayer(
        backend: WavPlayerBackend.subprocess,
        linuxPlayerCommands: const ['memo-test-nonexistent-player', 'false'],
      );
      await expectLater(
        () => player.play(fakeWav),
        throwsA(
          isA<Exception>().having(
            (e) => e.toString(),
            'message',
            contains('No working audio player found'),
          ),
        ),
      );
    },
    skip: !Platform.isLinux,
  );

  test(
    'stop() kills a still-playing subprocess without throwing',
    () async {
      // `yes <path>` stands in for a long-running audio player: it
      // ignores its argument as data to print forever, never exiting on
      // its own -- if stop() didn't actually kill the process, play()
      // would hang past this test's own timeout below.
      final player = WavPlayer(backend: WavPlayerBackend.subprocess, linuxPlayerCommands: const ['yes']);
      final playFuture = player.play(Uint8List.fromList([]));
      // Let the subprocess actually start before killing it.
      await Future.delayed(const Duration(milliseconds: 100));
      player.stop();
      await playFuture.timeout(const Duration(seconds: 2));
    },
    skip: !Platform.isLinux,
  );

  test(
    'stop() with nothing playing is a no-op',
    () async {
      final player = WavPlayer(backend: WavPlayerBackend.subprocess, linuxPlayerCommands: const ['true']);
      player.stop(); // must not throw
    },
    skip: !Platform.isLinux,
  );

  test('paplayVolumeArg() maps the 0.0..1.0 range onto paplay\'s 0..65536', () {
    expect(WavPlayer.paplayVolumeArg(0.0), '--volume=0');
    expect(WavPlayer.paplayVolumeArg(1.0), '--volume=65536');
    expect(WavPlayer.paplayVolumeArg(0.5), '--volume=32768');
  });

  test('paplayVolumeArg() clamps out-of-range input', () {
    expect(WavPlayer.paplayVolumeArg(-1.0), '--volume=0');
    expect(WavPlayer.paplayVolumeArg(2.0), '--volume=65536');
  });

  test(
    'play() passes a --volume arg only when the command basename is paplay',
    () async {
      // A fake script named exactly "paplay" (matched by basename, not PATH
      // lookup) so _runWithFallback's volume-arg branch actually engages,
      // then dumps its argv to a file this test reads back.
      final fakeBinDir = await Directory.systemTemp.createTemp('memo-paplay-');
      final argsFile = File('${fakeBinDir.path}/args.txt');
      final fakePaplay = File('${fakeBinDir.path}/paplay');
      await fakePaplay.writeAsString(
        '#!/bin/sh\necho "\$@" > "${argsFile.path}"\nexit 0\n',
      );
      await Process.run('chmod', ['+x', fakePaplay.path]);

      final player = WavPlayer(backend: WavPlayerBackend.subprocess, linuxPlayerCommands: [fakePaplay.path]);
      await player.play(fakeWav, volume: 0.3);

      final recordedArgs = (await argsFile.readAsString()).trim();
      // 0.3 * 65536 = 19660.8, rounds to 19661.
      expect(recordedArgs, startsWith('--volume=19661 '));

      await fakeBinDir.delete(recursive: true);
    },
    skip: !Platform.isLinux,
  );

  test(
    'play() cleans up its temp file after a successful play',
    () async {
      final before = Directory.systemTemp
          .listSync()
          .whereType<File>()
          .where((f) => f.path.contains('memo-tts-'))
          .length;

      final player = WavPlayer(backend: WavPlayerBackend.subprocess, linuxPlayerCommands: const ['true']);
      await player.play(fakeWav);

      final after = Directory.systemTemp
          .listSync()
          .whereType<File>()
          .where((f) => f.path.contains('memo-tts-'))
          .length;
      expect(after, before, reason: 'temp WAV file was not cleaned up');
    },
    skip: !Platform.isLinux,
  );

  // ─── Backend selection ────────────────────────────────────────────
  //
  // Pure, so unlike everything above these need no skip and no real audio
  // stack: they are the whole reason pickWavPlayerBackend takes its inputs
  // as parameters rather than reading kIsWeb/defaultTargetPlatform itself.
  group('pickWavPlayerBackend', () {
    test('mobile platforms use the plugin — there is no shell to spawn in', () {
      for (final platform in [TargetPlatform.android, TargetPlatform.iOS]) {
        expect(
          pickWavPlayerBackend(isWeb: false, platform: platform),
          WavPlayerBackend.plugin,
          reason: '$platform should use the plugin backend',
        );
      }
    });

    test('desktop platforms keep the subprocess path', () {
      for (final platform in [
        TargetPlatform.linux,
        TargetPlatform.macOS,
        TargetPlatform.windows,
      ]) {
        expect(
          pickWavPlayerBackend(isWeb: false, platform: platform),
          WavPlayerBackend.subprocess,
          reason: '$platform should keep the subprocess backend',
        );
      }
    });

    test('web has no playback path, whatever the browser reports', () {
      for (final platform in [TargetPlatform.android, TargetPlatform.linux]) {
        expect(
          pickWavPlayerBackend(isWeb: true, platform: platform),
          WavPlayerBackend.web,
        );
      }
    });
  });

  // ─── Plugin (mobile) branch ───────────────────────────────────────
  //
  // Exercised through a fake NativeClipPlayer: the real one needs a device,
  // but the logic around it (temp-file lifetime, volume clamping, stop
  // ordering, error propagation) does not.
  group('plugin backend', () {
    test('plays the temp file it wrote, at the clamped volume', () async {
      final fake = _FakeClipPlayer();
      final player = WavPlayer(
        backend: WavPlayerBackend.plugin,
        clipPlayerFactory: () => fake,
      );

      await player.play(fakeWav, volume: 0.25);

      expect(fake.playedPaths, hasLength(1));
      expect(fake.playedVolumes, [0.25]);
      expect(p.basename(fake.playedPaths.single), startsWith('memo-tts-'));
    });

    test('volume is clamped, not passed through raw', () async {
      final fake = _FakeClipPlayer();
      final player = WavPlayer(
        backend: WavPlayerBackend.plugin,
        clipPlayerFactory: () => fake,
      );

      await player.play(fakeWav, volume: 4.2);
      await player.play(fakeWav, volume: -1);

      expect(fake.playedVolumes, [1.0, 0.0]);
    });

    test('the temp file exists during playback and is gone afterwards', () async {
      String? seenPath;
      var existedDuringPlayback = false;
      final fake = _FakeClipPlayer(onPlay: (path) {
        seenPath = path;
        existedDuringPlayback = File(path).existsSync();
      });
      final player = WavPlayer(
        backend: WavPlayerBackend.plugin,
        clipPlayerFactory: () => fake,
      );

      await player.play(fakeWav);

      expect(existedDuringPlayback, isTrue,
          reason: 'the player must be handed a file that actually exists');
      expect(File(seenPath!).existsSync(), isFalse,
          reason: 'the temp file must not be left behind');
    });

    test('a cleanup failure does not surface as a playback error', () async {
      // Deleting the file from under the cleanup step is the observable
      // stand-in for Android still holding the handle: play() must not throw.
      final fake = _FakeClipPlayer(onPlay: (path) => File(path).deleteSync());
      final player = WavPlayer(
        backend: WavPlayerBackend.plugin,
        clipPlayerFactory: () => fake,
      );

      await player.play(fakeWav); // must not throw
    });

    test('a player error propagates and still cleans up', () async {
      String? seenPath;
      final fake = _FakeClipPlayer(onPlay: (path) {
        seenPath = path;
        throw Exception('device busy');
      });
      final player = WavPlayer(
        backend: WavPlayerBackend.plugin,
        clipPlayerFactory: () => fake,
      );

      await expectLater(() => player.play(fakeWav), throwsA(isA<Exception>()));
      expect(File(seenPath!).existsSync(), isFalse);
    });

    test('stop() reaches the plugin player — this is voice-mode barge-in', () async {
      final fake = _FakeClipPlayer();
      final player = WavPlayer(
        backend: WavPlayerBackend.plugin,
        clipPlayerFactory: () => fake,
      );

      await player.play(fakeWav);
      player.stop();

      expect(fake.stopCount, 1);
    });

    test('dispose() disposes the plugin player exactly once', () async {
      final fake = _FakeClipPlayer();
      final player = WavPlayer(
        backend: WavPlayerBackend.plugin,
        clipPlayerFactory: () => fake,
      );

      await player.play(fakeWav);
      player.dispose();
      player.dispose();

      expect(fake.disposeCount, 1);
    });

    test('the factory is called once, not per clip', () async {
      var built = 0;
      final player = WavPlayer(
        backend: WavPlayerBackend.plugin,
        clipPlayerFactory: () {
          built++;
          return _FakeClipPlayer();
        },
      );

      await player.play(fakeWav);
      await player.play(fakeWav);

      expect(built, 1);
    });
  });
}
