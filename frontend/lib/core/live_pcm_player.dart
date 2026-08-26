import 'dart:async';
import 'dart:io';
import 'dart:typed_data';

import 'package:flutter/foundation.dart' show kIsWeb;

/// Continuous, low-latency PCM playback for Live Mode's native realtime
/// engines (Google Live/OpenAI Realtime) — see
/// docs/plans/PLAN_live_mode_v2.md's §6 open question ("existing WavPlayer
/// is one-shot-file-oriented"), resolved here: [WavPlayer] (wav_player.dart)
/// writes a whole clip to a temp file and spawns one subprocess per play()
/// call, which is the right shape for a single TTS reply but wrong for a
/// realtime session's steady stream of small PCM chunks (spawning a
/// subprocess per chunk would add gaps/glitches between chunks, not smooth
/// playback).
///
/// Keeps one subprocess alive for the session's whole lifetime instead,
/// piping raw PCM straight to its stdin as chunks arrive from the server —
/// no temp files, no per-chunk process-spawn latency.
///
/// **Linux-only for now.** `paplay --raw` (PulseAudio/PipeWire) is the only
/// platform tool in WavPlayer's existing "no third-party audio library"
/// set that accepts a continuous raw PCM stream over stdin; `afplay`
/// (macOS) and PowerShell's `SoundPlayer` (Windows) are both file-oriented,
/// with no documented stdin-streaming mode. Rather than guess at an
/// undocumented streaming path for either, [start] throws
/// [UnsupportedError] on non-Linux platforms — an honest, visible failure
/// (surfaced through FriendlyError like any other) instead of silent
/// non-playback. Real cross-platform streaming playback is follow-up work.
class LiveModePcmPlayer {
  /// Candidate commands tried in order — mirrors [WavPlayer]'s own
  /// fallback list, but only `paplay` actually supports `--raw` stdin
  /// streaming; `aplay` is included as a same-family fallback (ALSA's
  /// `aplay` also supports raw/stdin playback via `-t raw`).
  final List<String> linuxPlayerCommands;

  LiveModePcmPlayer({this.linuxPlayerCommands = const ['paplay', 'aplay']});

  Process? _process;
  final _errorController = StreamController<String>.broadcast();

  bool get isPlaying => _process != null;

  /// Fires when the playback subprocess dies on its own (not via [stop]/
  /// [dispose]) — found necessary via real-world testing: `start()`
  /// succeeding only proves the executable launched, not that it actually
  /// produced sound. A process that starts but then fails to reach the
  /// audio backend (no PulseAudio/PipeWire-pulse socket, wrong device,
  /// etc.) exits shortly after with a stderr message that, before this,
  /// was drained and silently discarded — real audio frames were being
  /// written to a stdin nobody was reading, with no visible failure
  /// anywhere in the app.
  Stream<String> get onError => _errorController.stream;

  /// Starts the persistent player subprocess for [sampleRate] Hz mono
  /// 16-bit PCM. Must be called once before [write]; call [stop] before a
  /// second [start] (starting twice without stopping leaks the first
  /// subprocess).
  Future<void> start(int sampleRate) async {
    if (kIsWeb) {
      throw UnsupportedError('Audio playback is not yet supported on web.');
    }
    if (!Platform.isLinux) {
      throw UnsupportedError(
        'Live voice playback is only implemented on Linux so far '
        '(platform: ${Platform.operatingSystem}).',
      );
    }
    Object? lastError;
    for (final cmd in linuxPlayerCommands) {
      try {
        final args = rawArgsFor(cmd, sampleRate);
        final process = await Process.start(cmd, args);
        _process = process;
        process.stdout.drain<void>().ignore();

        final stderrBuffer = StringBuffer();
        process.stderr.transform(const SystemEncoding().decoder).listen(stderrBuffer.write);

        // Only reports if this is still the active process by the time it
        // exits -- stop()/dispose() both null out _process before killing,
        // so a deliberate stop never looks like an unexpected failure here.
        process.exitCode.then((code) {
          if (!identical(_process, process)) return;
          _process = null;
          if (_errorController.isClosed) return;
          final detail = stderrBuffer.toString().trim();
          _errorController.add(
            'Playback process ($cmd) exited unexpectedly (code $code)'
            '${detail.isEmpty ? '' : ': $detail'}',
          );
        });
        return;
      } on ProcessException catch (e) {
        lastError = e; // command not found -- try the next one
      }
    }
    throw Exception(
      'No streaming audio player found (tried ${linuxPlayerCommands.join(", ")}) '
      '— install PulseAudio/PipeWire-pulse or ALSA utilities. Last error: $lastError',
    );
  }

  /// The raw-PCM CLI arguments for [cmd] at [sampleRate] — a pure, public
  /// static function (mirroring WavPlayer.paplayVolumeArg's own reason for
  /// being public: unit-testable without spawning a real process).
  static List<String> rawArgsFor(String cmd, int sampleRate) {
    final base = cmd.split('/').last;
    if (base == 'aplay') {
      return ['-t', 'raw', '-f', 'S16_LE', '-c', '1', '-r', '$sampleRate', '-'];
    }
    // paplay --raw
    return ['--raw', '--format=s16le', '--channels=1', '--rate=$sampleRate', '-'];
  }

  /// Feeds one PCM16 chunk to the playing subprocess's stdin. A no-op if
  /// [start] hasn't been called (or [stop] already ran) — the caller
  /// doesn't need to track player lifecycle separately from session
  /// lifecycle.
  void write(Uint8List pcm) {
    _process?.stdin.add(pcm);
  }

  /// Stops playback and releases the subprocess. Safe to call when not
  /// playing.
  Future<void> stop() async {
    final process = _process;
    _process = null;
    if (process == null) return;
    try {
      await process.stdin.close();
    } catch (_) {
      // stdin may already be broken if the process exited on its own —
      // kill() below is the real cleanup guarantee either way.
    }
    process.kill();
  }

  void dispose() {
    final process = _process;
    _process = null;
    process?.kill();
    _errorController.close();
  }
}
