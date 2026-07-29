import 'dart:async';
import 'dart:typed_data';

import 'package:record/record.dart';

/// The duplex audio contract Faz 4 (`docs/plans/PLAN_voice_live_mode_faz4.md`)
/// is built around: one owner for both microphone capture and TTS/filler
/// render, so a real AEC engine can see both signals and cancel the speaker
/// bleeding into the mic. [captureStream] always yields 16 kHz mono PCM16 --
/// AEC-processed when [aecAvailable] is true, raw microphone otherwise.
///
/// [start]/[stop]/[dispose] intentionally mirror `VadHandler`'s own
/// lifecycle (`vad` package) so a caller can hold one of each without the
/// two objects fighting over "am I running" state.
abstract class DuplexAudioEngine {
  /// True once [start] has produced a capture stream (does not by itself
  /// imply [aecAvailable] -- the fallback engine is "active" with no AEC).
  bool get isActive;

  /// True only when frames written via [writeRenderFrame] actually feed a
  /// real echo-cancellation path. Faz 4.3's automatic barge-in gate must
  /// check this rather than assume every engine cancels echo.
  bool get aecAvailable;

  /// 16 kHz mono PCM16 capture frames, AEC-processed when [aecAvailable].
  Stream<Uint8List> get captureStream;

  Future<void> start();

  /// Feeds one render (speaker-bound) PCM16 frame as the AEC's echo
  /// reference. A no-op on engines with [aecAvailable] false.
  void writeRenderFrame(Uint8List pcm);

  Future<void> stop();

  void dispose();
}

/// Fallback [DuplexAudioEngine]: plain microphone capture via the `record`
/// package, no AEC. This is today's Faz 1 behavior, ported to the new
/// contract unchanged -- it is not a placeholder to be deleted, it is the
/// permanent answer for any platform/device 4.2's native engine can't cover
/// (see the plan's "başarısız/cihaz-desteksiz durumda davranış").
/// [writeRenderFrame] is deliberately a no-op: nothing here reads render
/// audio, so there's no chance of it hiding a wiring bug the way pretending
/// to consume it silently would.
class NoAecDuplexAudioEngine implements DuplexAudioEngine {
  final AudioRecorder _recorder;

  NoAecDuplexAudioEngine({AudioRecorder? recorder})
    : _recorder = recorder ?? AudioRecorder();

  bool _isActive = false;
  StreamController<Uint8List>? _captureController;
  StreamSubscription<Uint8List>? _recorderSubscription;

  @override
  bool get isActive => _isActive;

  @override
  bool get aecAvailable => false;

  @override
  Stream<Uint8List> get captureStream =>
      _captureController?.stream ?? const Stream.empty();

  @override
  Future<void> start() async {
    if (_isActive) return;
    final controller = StreamController<Uint8List>.broadcast();
    _captureController = controller;

    final stream = await _recorder.startStream(
      const RecordConfig(
        encoder: AudioEncoder.pcm16bits,
        sampleRate: 16000,
        numChannels: 1,
        autoGain: true,
        echoCancel: true,
        noiseSuppress: true,
      ),
    );
    _recorderSubscription = stream.listen(
      (chunk) {
        if (!controller.isClosed) controller.add(chunk);
      },
      onError: (Object e) {
        if (!controller.isClosed) controller.addError(e);
      },
    );
    _isActive = true;
  }

  @override
  void writeRenderFrame(Uint8List pcm) {
    // No AEC path to feed -- see class doc.
  }

  @override
  Future<void> stop() async {
    if (!_isActive) return;
    _isActive = false;
    await _recorderSubscription?.cancel();
    _recorderSubscription = null;
    await _recorder.stop();
    await _captureController?.close();
    _captureController = null;
  }

  @override
  void dispose() {
    _recorderSubscription?.cancel();
    _captureController?.close();
    _recorder.dispose();
  }
}
