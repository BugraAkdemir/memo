import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter/foundation.dart' show debugPrint;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:web_socket_channel/web_socket_channel.dart';

import '../core/duplex_audio_engine.dart';
import '../core/friendly_error.dart';
import '../core/live_pcm_player.dart';
import '../models/chat.dart';
import 'chat_provider.dart';

/// Connection lifecycle for a Google Live/OpenAI Realtime session — see
/// docs/plans/PLAN_live_mode_v2.md's Phase 6. Deliberately NOT the same
/// idle/listening/thinking/speaking shape [VoiceModeNotifier]
/// (voice_mode_provider.dart) uses for Local/ElevenLabs/Custom: those
/// engines are half-duplex, discrete-turn (one STT call, one chat send, one
/// TTS call per utterance); the two native engines are full-duplex,
/// continuous-stream, with turn-taking handled server-side by the provider
/// — there is no meaningful "listening vs. thinking vs. speaking" instant
/// to report from the client's point of view, only whether the pipe is up.
enum LiveRealtimeSessionStatus { idle, connecting, connected, error, closed }

class LiveRealtimeSessionState {
  final LiveRealtimeSessionStatus status;
  final String? error;

  const LiveRealtimeSessionState({
    this.status = LiveRealtimeSessionStatus.idle,
    this.error,
  });
}

/// The engines this provider knows how to drive — mirrors the backend's
/// livemode.EngineType strings for "google_live"/"openai_realtime" (the
/// only two that ever reach this notifier; Local/ElevenLabs/Custom stay on
/// [VoiceModeNotifier]'s discrete STT/TTS loop, see chat_input.dart's
/// engine branch).
class LiveRealtimeEngineAudioConfig {
  final int captureSampleRate;
  final int playbackSampleRate;
  const LiveRealtimeEngineAudioConfig({
    required this.captureSampleRate,
    required this.playbackSampleRate,
  });
}

/// Per-engine PCM contract, confirmed against current provider docs
/// (2026-08-26, see google/client.go and openai_realtime/client.go's own
/// inputSampleRateHz/outputSampleRateHz constants — this is the Dart-side
/// mirror of those, since the WS bridge itself never resamples): Google
/// Live captures at 16 kHz and plays back at 24 kHz; OpenAI Realtime uses
/// 24 kHz for both directions.
const _engineAudioConfigs = <String, LiveRealtimeEngineAudioConfig>{
  'google_live': LiveRealtimeEngineAudioConfig(captureSampleRate: 16000, playbackSampleRate: 24000),
  'openai_realtime': LiveRealtimeEngineAudioConfig(captureSampleRate: 24000, playbackSampleRate: 24000),
};

/// The JSON shape a text WS frame carries — the Dart mirror of
/// internal/webserver/handlers_livemode_session.go's
/// liveModeSessionControlFrame. A free function/class (not tangled into the
/// notifier) so parsing is unit-testable without any WebSocket/Riverpod
/// machinery.
class LiveModeSessionControlFrame {
  final String type;
  final String? transcript;
  final String? role; // "transcript" frames only — "user" or "model"
  final String? error;

  const LiveModeSessionControlFrame({required this.type, this.transcript, this.role, this.error});

  factory LiveModeSessionControlFrame.fromJson(Map<String, dynamic> json) {
    return LiveModeSessionControlFrame(
      type: json['type'] as String? ?? '',
      transcript: json['transcript'] as String?,
      role: json['role'] as String?,
      error: json['error'] as String?,
    );
  }
}

/// Builds the WebSocket URL for Live Mode's realtime session bridge from
/// the REST API's own base URL (http(s) -> ws(s), same host/port, fixed
/// path) — a free function so it's unit-testable without any Riverpod/
/// WebSocket machinery involved.
String buildLiveModeSessionWsUrl(String httpBaseUrl) {
  final uri = Uri.parse(httpBaseUrl);
  final wsScheme = uri.scheme == 'https' ? 'wss' : 'ws';
  return uri.replace(scheme: wsScheme, path: '/api/livemode/session').toString();
}

/// Owns the WebSocket connection to `/api/livemode/session`, plus the real
/// microphone capture ([DuplexAudioEngine]) and speaker playback
/// ([LiveModePcmPlayer]) either side of it — one owner for the whole
/// duplex pipe, mirroring [VoiceModeNotifier]'s ownership shape for the
/// discrete engines.
///
/// [_generation] guards against a stale, still-in-flight `connect()` (or a
/// message that arrives after `disconnect()`/a newer `connect()` already
/// started) clobbering state that no longer belongs to it — the same
/// general async-race hygiene `VoiceModeNotifier`
/// (voice_mode_provider.dart) already applies for its own barge-in
/// handling, not specific to this being a StateNotifier vs. the newer
/// Notifier API (see AGENTS.md's Riverpod gotcha — that one is about a
/// different failure mode, instance reuse across build() cycles, which
/// StateNotifierProvider doesn't exhibit; this generation counter exists
/// for the same defensive reason regardless of which API creates it).
class LiveRealtimeSessionNotifier extends StateNotifier<LiveRealtimeSessionState> {
  LiveRealtimeSessionNotifier(this._ref) : super(const LiveRealtimeSessionState());

  final Ref _ref;
  WebSocketChannel? _channel;
  StreamSubscription? _sub;
  DuplexAudioEngine? _captureEngine;
  StreamSubscription? _captureSub;
  LiveModePcmPlayer? _player;
  int _generation = 0;

  bool get isActive =>
      state.status == LiveRealtimeSessionStatus.connecting || state.status == LiveRealtimeSessionStatus.connected;

  /// Sets the error state AND pushes it to the app-wide error toast
  /// ([errorMessageProvider]) — this notifier's own state.error has no UI
  /// reader of its own (unlike VoiceModeState, which chat_input.dart reads
  /// directly for its status label), so without this an error here would
  /// otherwise vanish silently instead of ever reaching the user.
  void _setError(String message) {
    state = LiveRealtimeSessionState(status: LiveRealtimeSessionStatus.error, error: message);
    _ref.read(errorMessageProvider.notifier).state = message;
  }

  /// Opens the session for [engineType] ("google_live" | "openai_realtime")
  /// — connects the WS bridge, then starts real mic capture (streamed to
  /// the server as it's captured) and a playback subprocess for whatever
  /// audio the server sends back, at the sample rates that engine's own
  /// wire protocol requires (see [_engineAudioConfigs]).
  Future<void> connect(String engineType) async {
    final audioConfig = _engineAudioConfigs[engineType];
    if (audioConfig == null) {
      _setError('Unsupported realtime engine: $engineType');
      return;
    }

    final myGeneration = ++_generation;
    state = const LiveRealtimeSessionState(status: LiveRealtimeSessionStatus.connecting);
    try {
      final baseUrl = _ref.read(apiClientProvider).baseUrl;
      final channel = WebSocketChannel.connect(Uri.parse(buildLiveModeSessionWsUrl(baseUrl)));
      await channel.ready;
      if (myGeneration != _generation) {
        // A newer connect()/disconnect() already happened while this one
        // was still opening — this connection is stale, throw it away.
        unawaited(channel.sink.close());
        return;
      }
      _channel = channel;

      final player = LiveModePcmPlayer();
      // Listen before start(): the subprocess can die (and this fire)
      // within milliseconds of launching, so subscribing after start()
      // returns would risk missing it. See LiveModePcmPlayer.onError's doc
      // comment — this is the real bug a real test surfaced: the process
      // launched fine (isPlaying=true) but exited almost immediately with
      // no PulseAudio/PipeWire-pulse socket reachable, and that failure
      // was previously discarded entirely.
      player.onError.listen((message) {
        if (myGeneration != _generation) return;
        _setError(message);
      });
      // Diagnostic: confirms whether the playback subprocess actually
      // started (vs. e.g. neither paplay nor aplay being installed, which
      // would otherwise fail silently from the user's point of view since
      // there's no visible UI for "playback backend ready").
      debugPrint('live realtime: starting playback at ${audioConfig.playbackSampleRate}Hz');
      await player.start(audioConfig.playbackSampleRate);
      debugPrint('live realtime: playback started');
      if (myGeneration != _generation) {
        await player.stop();
        unawaited(channel.sink.close());
        return;
      }
      _player = player;

      final captureEngine = NoAecDuplexAudioEngine(sampleRate: audioConfig.captureSampleRate);
      await captureEngine.start();
      if (myGeneration != _generation) {
        await captureEngine.stop();
        captureEngine.dispose();
        await player.stop();
        unawaited(channel.sink.close());
        return;
      }
      _captureEngine = captureEngine;
      _captureSub = captureEngine.captureStream.listen(
        (chunk) {
          if (myGeneration != _generation) return;
          _channel?.sink.add(chunk);
        },
        onError: (Object e) {
          if (myGeneration != _generation) return;
          _setError(FriendlyError.describeGeneric(e));
        },
      );

      state = const LiveRealtimeSessionState(status: LiveRealtimeSessionStatus.connected);
      _sub = channel.stream.listen(
        (message) {
          if (myGeneration != _generation) return;
          if (message is Uint8List) {
            debugPrint('live realtime: received ${message.length}-byte audio frame, player=${_player?.isPlaying}');
            _player?.write(message);
          } else if (message is List<int>) {
            debugPrint('live realtime: received ${message.length}-byte audio frame (List<int>), player=${_player?.isPlaying}');
            _player?.write(Uint8List.fromList(message));
          } else if (message is String) {
            _handleControlFrame(message);
          }
        },
        onDone: () {
          if (myGeneration != _generation) return;
          state = const LiveRealtimeSessionState(status: LiveRealtimeSessionStatus.closed);
          _teardownAudio();
        },
        onError: (Object e) {
          if (myGeneration != _generation) return;
          _setError(FriendlyError.describeGeneric(e));
          _teardownAudio();
        },
      );
    } catch (e) {
      if (myGeneration != _generation) return;
      _setError(FriendlyError.describeGeneric(e));
      _teardownAudio();
    }
  }

  void _handleControlFrame(String raw) {
    try {
      final frame = LiveModeSessionControlFrame.fromJson(jsonDecode(raw) as Map<String, dynamic>);
      if (frame.type == 'error' && frame.error != null && frame.error!.isNotEmpty) {
        _setError(FriendlyError.describeGeneric(frame.error!));
        return;
      }
      if (frame.type == 'transcript' && frame.transcript != null && frame.transcript!.isNotEmpty) {
        // Displays the live conversation as a normal chat — purely local,
        // never routed through sendMessage()/the real send-to-LLM path (the
        // actual "ana model" routing for Live Mode happens server-side via
        // delegate/standalone tool calls, entirely independent of this).
        // Confirmed with the user: these bubbles stay in the chat's history
        // permanently once added, not cleared when the session ends — see
        // docs/plans/PLAN_live_mode_v2.md's follow-up plan.
        _ref.read(messagesProvider.notifier).addMessage(
              ChatMessage(
                role: frame.role == 'model' ? 'assistant' : 'user',
                content: frame.transcript!,
                timestamp: DateTime.now().toIso8601String(),
              ),
            );
      }
      // function_call frames: no UI consumer (the backend resolves
      // function calls and voice-based permission prompting itself — see
      // docs/plans/PLAN_live_mode_v2.md Phase 12). Nothing further to do
      // with them client-side.
    } catch (_) {
      // Malformed control frame -- ignore rather than tear down a working
      // audio session over a display-only parse failure.
    }
  }

  /// Sends one raw PCM chunk from the microphone to the backend. Exposed
  /// for tests; normal operation feeds this from [_captureSub] instead.
  void sendAudio(Uint8List pcm) {
    _channel?.sink.add(pcm);
  }

  Future<void> disconnect() async {
    _generation++;
    await _sub?.cancel();
    _sub = null;
    await _channel?.sink.close();
    _channel = null;
    await _teardownAudio();
    state = const LiveRealtimeSessionState();
  }

  Future<void> _teardownAudio() async {
    await _captureSub?.cancel();
    _captureSub = null;
    final engine = _captureEngine;
    _captureEngine = null;
    if (engine != null) {
      await engine.stop();
      engine.dispose();
    }
    final player = _player;
    _player = null;
    if (player != null) await player.stop();
  }

  @override
  void dispose() {
    _generation++;
    _sub?.cancel();
    _channel?.sink.close();
    _captureSub?.cancel();
    _captureEngine?.stop();
    _captureEngine?.dispose();
    _player?.stop();
    super.dispose();
  }
}

final liveRealtimeSessionProvider =
    StateNotifierProvider<LiveRealtimeSessionNotifier, LiveRealtimeSessionState>(
  (ref) => LiveRealtimeSessionNotifier(ref),
);
