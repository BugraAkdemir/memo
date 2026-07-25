import 'dart:async';

import 'package:vad/vad.dart';

import 'api_client.dart';
import 'wav_encoder.dart';

/// Drives Voice Live Mode's continuous-listening loop (Faz 1.6, see
/// docs/plans/PLAN_voice_live_mode_faz1.md): wraps `vad`'s VAD handler,
/// turns each detected speech segment into a WAV file via [encodePcm16Wav],
/// and transcribes it through the existing [MemoApiClient.transcribeAudio]
/// -- the same endpoint the push-to-talk STT button already uses, so no
/// backend changes were needed for this step.
///
/// **KNOWN, UNRESOLVED GAP — do not ship without fixing:** the `vad`
/// package's default `baseAssetPath` points at a public CDN
/// (`cdn.jsdelivr.net`) and downloads the Silero VAD `.onnx` model file
/// from there at runtime, on every platform (not just web). This directly
/// contradicts this app's local-first architecture, where every other
/// engine (llama.cpp, whisper.cpp, vec0, Piper) is bundled under
/// `binaries/` and never fetched at runtime (see AGENTS.md's "sqlite-vec
/// extension binary is bundled ... never add a runtime download for it").
/// [_vadBaseAssetPath] is left at the CDN default for now, deliberately
/// NOT silently accepted as fine -- this is flagged in the plan and the
/// handoff log as a real, blocking gap for Faz 1.6 to close before this
/// feature leaves prototype status: the `.onnx` model needs to be bundled
/// the same way Piper's binary/voice files are, with its own
/// download_binaries.sh step, and this constant repointed at the bundled
/// path.
const String _vadBaseAssetPath =
    'https://cdn.jsdelivr.net/npm/@keyurmaru/vad@0.0.1/';

class LiveModeController {
  final MemoApiClient _apiClient;
  VadHandler? _handler;
  bool get isListening => _handler != null;

  final _transcriptController = StreamController<String>.broadcast();
  final _errorController = StreamController<String>.broadcast();

  LiveModeController(this._apiClient);

  /// Fires with the transcribed text of each VAD-detected speech segment.
  /// Errors (transcription failure, VAD misfire) are reported via [onError]
  /// instead of throwing through this stream, so one bad segment doesn't
  /// tear down the whole listening session.
  Stream<String> get onTranscript => _transcriptController.stream;

  /// Fires with a human-readable message on transcription or VAD failure.
  Stream<String> get onError => _errorController.stream;

  Future<void> start() async {
    if (_handler != null) return;
    final handler = VadHandler.create(isDebug: false);
    _handler = handler;

    // Guard every add() with isClosed: dispose() can run (closing these
    // controllers) while a segment's transcribeAudio() call from this very
    // listener is still in flight -- stopListening() only stops new
    // segments from being captured, it doesn't wait for an
    // already-fired onSpeechEnd callback's async body to finish. Without
    // this guard, that in-flight callback's eventual add() throws
    // "Bad state: Cannot add event after closing" as an unhandled
    // exception in the stream-subscription callback (found in review).
    handler.onSpeechEnd.listen((samples) async {
      try {
        final wav = encodePcm16Wav(samples);
        final text = await _apiClient.transcribeAudio(wav);
        if (text.trim().isNotEmpty && !_transcriptController.isClosed) {
          _transcriptController.add(text);
        }
      } catch (e) {
        if (!_errorController.isClosed) _errorController.add('$e');
      }
    });
    handler.onError.listen((message) {
      if (!_errorController.isClosed) _errorController.add(message);
    });

    await handler.startListening(baseAssetPath: _vadBaseAssetPath);
  }

  Future<void> stop() async {
    final handler = _handler;
    if (handler == null) return;
    _handler = null;
    await handler.stopListening();
    handler.dispose();
  }

  /// Releases the stream controllers. Call once this controller itself is
  /// no longer needed (not on every stop() — a Live session may be
  /// stopped/restarted several times before the screen owning this
  /// controller is disposed).
  void dispose() {
    _transcriptController.close();
    _errorController.close();
  }
}
