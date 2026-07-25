import 'dart:io';
import 'dart:typed_data';

import 'package:audioplayers/audioplayers.dart';

/// Plays WAV audio bytes (Piper's TTS output). On Linux this deliberately
/// bypasses `audioplayers`, which uses GStreamer there -- and GStreamer's
/// plugin system is the confirmed root cause of a live-reported bug: this
/// dev machine (CachyOS) has `gstreamer` + `gst-plugins-base/bad/ugly`
/// installed but not `gst-plugins-good`, which provides the
/// `wavparse`/`autoaudiosink` elements needed to play a WAV at all --
/// `audioplayers`' own README documents this exact package as a required
/// "app dependency" on every major distro, and the popular alternative
/// (`media_kit`, libmpv-based) has the identical structural requirement
/// ("System shared libraries ... This is how GNU/Linux works" -- its own
/// words). Neither package can be made to carry its own copy of that
/// engine without a real packaging effort (bundling the plugin .so files
/// + their transitive dependencies, e.g. via linuxdeploy's GStreamer
/// plugin during this project's existing AppImage build -- a separate,
/// larger piece of work, not done here).
///
/// Until that's done, Linux instead shells out to `paplay` (PulseAudio,
/// and still present as a compatibility shim on PipeWire-based distros)
/// or `aplay` (plain ALSA) as a fallback -- both are part of the base
/// audio stack nearly every Linux desktop already has, unlike
/// `gst-plugins-good` specifically, and match this codebase's existing
/// subprocess-based pattern for Piper/whisper.cpp/llama.cpp rather than
/// adding a new dependency class. This is NOT zero-dependency (an
/// extremely minimal Linux install with no PulseAudio/PipeWire/ALSA
/// userspace tools at all would still fail) but is a materially smaller,
/// far more commonly-satisfied requirement than gst-plugins-good.
///
/// Windows and macOS are untouched -- `audioplayers` uses each OS's native
/// media framework there (Windows Media Foundation, AVFoundation) with no
/// extra runtime dependency, confirmed against both packages' own docs.
class WavPlayer {
  /// Candidate commands tried in order on Linux. Overridable for tests --
  /// CI runners commonly have neither PulseAudio/PipeWire nor a real ALSA
  /// device configured, so tests substitute fake commands here instead of
  /// depending on this machine's actual audio stack.
  final List<String> linuxPlayerCommands;

  AudioPlayer? _audioPlayer;

  WavPlayer({this.linuxPlayerCommands = const ['paplay', 'aplay']});

  Future<void> play(Uint8List wavBytes) {
    if (Platform.isLinux) return _playViaLinuxSubprocess(wavBytes);
    final player = _audioPlayer ??= AudioPlayer();
    return player.play(BytesSource(wavBytes)).then((_) async {
      await player.onPlayerComplete.first;
    });
  }

  Future<void> _playViaLinuxSubprocess(Uint8List wavBytes) async {
    final tempFile = File(
      '${Directory.systemTemp.path}/memo-tts-${DateTime.now().microsecondsSinceEpoch}.wav',
    );
    await tempFile.writeAsBytes(wavBytes);
    try {
      Object? lastError;
      for (final player in linuxPlayerCommands) {
        try {
          final result = await Process.run(player, [tempFile.path]);
          if (result.exitCode == 0) return;
          lastError = '$player exited ${result.exitCode}: ${result.stderr}';
        } on ProcessException catch (e) {
          lastError = e; // command not found -- try the next one
        }
      }
      throw Exception(
        'No working Linux audio player found (tried ${linuxPlayerCommands.join(", ")}) '
        '— install PulseAudio/PipeWire-pulse or ALSA utilities. Last error: $lastError',
      );
    } finally {
      if (await tempFile.exists()) await tempFile.delete();
    }
  }

  void dispose() {
    _audioPlayer?.dispose();
  }
}
