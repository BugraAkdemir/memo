import 'package:just_audio/just_audio.dart';

/// Plays one short audio clip from a local file, on platforms where a plugin
/// does the work instead of a subprocess.
///
/// Exists as an interface purely so [WavPlayer]'s plugin branch is testable
/// without a device: temp-file lifetime, volume clamping, stop/barge-in
/// ordering and error propagation are all logic worth covering, and none of
/// it should need real audio hardware to exercise — the same reasoning
/// `WavPlayer.linuxPlayerCommands` already documents for the subprocess
/// branch ("CI runners commonly have neither PulseAudio/PipeWire nor a real
/// ALSA device configured"). Faking just_audio's own `AudioPlayer` instead
/// would mean implementing a large, fast-moving surface; this leaves only
/// the ~15-line adapter below unproven without hardware.
abstract class NativeClipPlayer {
  /// Plays the file at [path] at [volume] (0.0..1.0) and completes when
  /// playback finishes — or when [stop] cuts it short.
  Future<void> playFile(String path, double volume);

  /// Stops playback immediately, so a pending [playFile] completes early.
  Future<void> stop();

  void dispose();
}

/// [NativeClipPlayer] over just_audio, used on Android and iOS.
class JustAudioClipPlayer implements NativeClipPlayer {
  final AudioPlayer _player = AudioPlayer();

  @override
  Future<void> playFile(String path, double volume) async {
    await _player.setFilePath(path);
    // Unlike the desktop fallbacks (`aplay` has no volume flag, PowerShell's
    // SoundPlayer exposes no gain), volume is genuinely honored here.
    await _player.setVolume(volume);
    // just_audio's play() completes when playback completes, is paused, or
    // is stopped — which is what makes stop() work as barge-in.
    await _player.play();
  }

  @override
  Future<void> stop() => _player.stop();

  @override
  void dispose() {
    _player.dispose();
  }
}
