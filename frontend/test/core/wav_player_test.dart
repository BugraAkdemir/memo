import 'dart:io';
import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/core/wav_player.dart';

// These tests only exercise WavPlayer's Linux subprocess path (the code
// under test checks Platform.isLinux internally, and `flutter test` runs on
// the host OS -- this dev machine is Linux, matching CI). `true`/`false`
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
      final player = WavPlayer(linuxPlayerCommands: const ['true']);
      await player.play(fakeWav); // must not throw
    },
    skip: !Platform.isLinux,
  );

  test(
    'play() falls back to the next candidate when the first is not found',
    () async {
      final player = WavPlayer(
        linuxPlayerCommands: const ['memo-test-nonexistent-player', 'true'],
      );
      await player.play(fakeWav); // must not throw -- falls back to "true"
    },
    skip: !Platform.isLinux,
  );

  test(
    'play() falls back to the next candidate when the first exits non-zero',
    () async {
      final player = WavPlayer(linuxPlayerCommands: const ['false', 'true']);
      await player.play(fakeWav); // must not throw -- falls back to "true"
    },
    skip: !Platform.isLinux,
  );

  test(
    'play() throws a clear error when every candidate fails',
    () async {
      final player = WavPlayer(
        linuxPlayerCommands: const ['memo-test-nonexistent-player', 'false'],
      );
      await expectLater(
        () => player.play(fakeWav),
        throwsA(
          isA<Exception>().having(
            (e) => e.toString(),
            'message',
            contains('No working Linux audio player found'),
          ),
        ),
      );
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

      final player = WavPlayer(linuxPlayerCommands: const ['true']);
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
}
