import 'dart:io';
import 'dart:typed_data';

/// Plays WAV audio bytes (Piper's TTS output) via platform-native subprocesses.
///
/// Uses the same subprocess pattern as Piper/whisper.cpp/llama.cpp — no
/// third-party audio library needed. Every platform ships with a built-in
/// WAV-capable command:
///
/// - **Linux:** `paplay` (PulseAudio/PipeWire shim) → `aplay` (ALSA).
/// - **macOS:** `afplay` (ships with every macOS install).
/// - **Windows:** PowerShell's `System.Media.SoundPlayer` (.NET built-in).
///
/// This replaces the earlier `audioplayers` dependency, which required
/// GStreamer on Linux (`gst-plugins-good` specifically) — a widespread
/// missing dependency on minimal/headless/ci Linux installs. The
/// subprocess approach has a far smaller dependency footprint: only
/// base audio stack tools that are nearly universally present on desktop
/// installs of each OS.
class WavPlayer {
  /// Candidate commands tried in order on Linux. Overridable for tests --
  /// CI runners commonly have neither PulseAudio/PipeWire nor a real ALSA
  /// device configured, so tests substitute fake commands here instead of
  /// depending on this machine's actual audio stack.
  final List<String> linuxPlayerCommands;

  WavPlayer({this.linuxPlayerCommands = const ['paplay', 'aplay']});

  Future<void> play(Uint8List wavBytes) async {
    final tempFile = File(
      '${Directory.systemTemp.path}/memo-tts-${DateTime.now().microsecondsSinceEpoch}.wav',
    );
    await tempFile.writeAsBytes(wavBytes);
    try {
      if (Platform.isLinux) {
        await _runWithFallback(linuxPlayerCommands, tempFile.path);
      } else if (Platform.isMacOS) {
        await _runOrThrow('afplay', [tempFile.path]);
      } else if (Platform.isWindows) {
        // .NET's SoundPlayer — built into every Windows install.
        await _runOrThrow('powershell', [
          '-NoProfile',
          '-Command',
          '(New-Object System.Media.SoundPlayer \'${tempFile.path}\').PlaySync()',
        ]);
      } else {
        throw UnsupportedError('Unsupported platform: ${Platform.operatingSystem}');
      }
    } finally {
      if (await tempFile.exists()) await tempFile.delete();
    }
  }

  /// Try each command in [candidates] in order; throw if all fail.
  Future<void> _runWithFallback(List<String> candidates, String path) async {
    Object? lastError;
    for (final cmd in candidates) {
      try {
        final result = await Process.run(cmd, [path]);
        if (result.exitCode == 0) return;
        lastError = '$cmd exited ${result.exitCode}: ${result.stderr}';
      } on ProcessException catch (e) {
        lastError = e; // command not found -- try the next one
      }
    }
    throw Exception(
      'No working audio player found (tried ${candidates.join(", ")}) '
      '— install PulseAudio/PipeWire-pulse or ALSA utilities. Last error: $lastError',
    );
  }

  /// Run a single command; throw if it fails.
  Future<void> _runOrThrow(String cmd, List<String> args) async {
    try {
      final result = await Process.run(cmd, args);
      if (result.exitCode != 0) {
        throw Exception('$cmd exited ${result.exitCode}: ${result.stderr}');
      }
    } on ProcessException catch (e) {
      throw Exception('Audio player not found: $cmd — $e');
    }
  }

  void dispose() {}
}
