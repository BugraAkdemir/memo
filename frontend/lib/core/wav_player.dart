import 'dart:io';
import 'dart:typed_data';

import 'package:path/path.dart' as p;

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
///
/// Uses `Process.start` (not `Process.run`) specifically so [stop] can kill
/// a still-playing subprocess mid-clip — needed for voice chat's barge-in
/// (voice_mode_provider.dart): the user starts talking again while Memo (or
/// a filler sound) is still speaking, and playback must stop immediately
/// rather than waiting for the clip to finish naturally.
class WavPlayer {
  /// Candidate commands tried in order on Linux. Overridable for tests --
  /// CI runners commonly have neither PulseAudio/PipeWire nor a real ALSA
  /// device configured, so tests substitute fake commands here instead of
  /// depending on this machine's actual audio stack.
  final List<String> linuxPlayerCommands;

  WavPlayer({this.linuxPlayerCommands = const ['paplay', 'aplay']});

  Process? _activeProcess;
  // Set by stop() so an intentionally-killed process's non-zero/negative
  // exit code isn't reported as a playback failure — killing a process is
  // success from stop()'s point of view, not an error to surface.
  bool _stopRequested = false;

  /// Plays [wavBytes] at [volume] (0.0 silent .. 1.0 full, clamped).
  ///
  /// Volume is only actually honored on Linux via `paplay` and on macOS via
  /// `afplay` -- both take a real volume argument (verified against
  /// `paplay --help`/`man paplay` on this machine: `--volume=N` linear
  /// 0..65536; `afplay`'s `-v` is documented as 0.0..1.0 but not verified
  /// live in this environment, no macOS machine here). `aplay` (the Linux
  /// fallback when `paplay` is missing) has no volume flag at all, and
  /// PowerShell's `SoundPlayer` (Windows) exposes no per-instance gain
  /// either -- both silently play at [volume]'s default full level instead
  /// of failing, since a slightly-wrong volume is a much smaller problem
  /// than a playback error on a working fallback path.
  Future<void> play(Uint8List wavBytes, {double volume = 1.0}) async {
    final clampedVolume = volume.clamp(0.0, 1.0);
    _stopRequested = false;
    final tempFile = File(
      '${Directory.systemTemp.path}/memo-tts-${DateTime.now().microsecondsSinceEpoch}.wav',
    );
    await tempFile.writeAsBytes(wavBytes);
    try {
      if (Platform.isLinux) {
        await _runWithFallback(linuxPlayerCommands, tempFile.path, clampedVolume);
      } else if (Platform.isMacOS) {
        await _runOrThrow('afplay', [
          '-v',
          clampedVolume.toString(),
          tempFile.path,
        ]);
      } else if (Platform.isWindows) {
        // .NET's SoundPlayer — built into every Windows install. No volume
        // parameter exists on this API; see the doc comment above.
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

  /// The `paplay --volume` argument for a linear [volume] in 0.0..1.0.
  /// Exposed as a static, pure function so the volume-scaling math is unit
  /// testable without spawning a real `paplay` process.
  static String paplayVolumeArg(double volume) {
    final linear = (volume.clamp(0.0, 1.0) * 65536).round();
    return '--volume=$linear';
  }

  /// Kills the currently-playing subprocess, if any, so [play] returns
  /// early instead of waiting for natural playback completion. A no-op if
  /// nothing is currently playing.
  void stop() {
    _stopRequested = true;
    _activeProcess?.kill();
  }

  /// Try each command in [candidates] in order; throw if all fail.
  Future<void> _runWithFallback(
    List<String> candidates,
    String path,
    double volume,
  ) async {
    Object? lastError;
    for (final cmd in candidates) {
      try {
        // Only `paplay` understands a volume argument -- `aplay` (the
        // built-in fallback) doesn't take one at all, and would fail to
        // start if handed an argument it doesn't recognize. Compared by
        // basename (not the raw string) so a caller/test can point
        // [linuxPlayerCommands] at an absolute path to a `paplay` binary.
        final args = p.basename(cmd) == 'paplay'
            ? [paplayVolumeArg(volume), path]
            : [path];
        final result = await _startAndWait(cmd, args);
        if (result.exitCode == 0 || _stopRequested) return;
        lastError = '$cmd exited ${result.exitCode}: ${result.stderr}';
      } on ProcessException catch (e) {
        lastError = e; // command not found -- try the next one
      }
    }
    if (_stopRequested) return;
    throw Exception(
      'No working audio player found (tried ${candidates.join(", ")}) '
      '— install PulseAudio/PipeWire-pulse or ALSA utilities. Last error: $lastError',
    );
  }

  /// Run a single command; throw if it fails.
  Future<void> _runOrThrow(String cmd, List<String> args) async {
    try {
      final result = await _startAndWait(cmd, args);
      if (result.exitCode != 0 && !_stopRequested) {
        throw Exception('$cmd exited ${result.exitCode}: ${result.stderr}');
      }
    } on ProcessException catch (e) {
      throw Exception('Audio player not found: $cmd — $e');
    }
  }

  /// Starts [cmd], tracks it as the currently-killable process (for
  /// [stop]), and awaits its exit — the `Process.start` equivalent of
  /// `Process.run`'s single awaitable result. stdout is drained and
  /// discarded; stderr is buffered (bounded by whatever a CLI audio player
  /// actually writes, always small) so a real failure's message is still
  /// available to the caller.
  Future<_ProcResult> _startAndWait(String cmd, List<String> args) async {
    final process = await Process.start(cmd, args);
    _activeProcess = process;
    final stderrBuffer = StringBuffer();
    final stdoutDone = process.stdout.drain<void>();
    final stderrDone = process.stderr
        .transform(const SystemEncoding().decoder)
        .forEach(stderrBuffer.write);
    final exitCode = await process.exitCode;
    await Future.wait([stdoutDone, stderrDone]);
    if (identical(_activeProcess, process)) _activeProcess = null;
    return _ProcResult(exitCode, stderrBuffer.toString());
  }

  void dispose() {
    _activeProcess?.kill();
  }
}

class _ProcResult {
  final int exitCode;
  final String stderr;
  const _ProcResult(this.exitCode, this.stderr);
}
