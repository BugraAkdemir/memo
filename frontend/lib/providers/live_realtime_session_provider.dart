import 'dart:async';
import 'dart:typed_data';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:web_socket_channel/web_socket_channel.dart';

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

/// Builds the WebSocket URL for Live Mode's realtime session bridge from
/// the REST API's own base URL (http(s) -> ws(s), same host/port, fixed
/// path) — a free function so it's unit-testable without any Riverpod/
/// WebSocket machinery involved.
String buildLiveModeSessionWsUrl(String httpBaseUrl) {
  final uri = Uri.parse(httpBaseUrl);
  final wsScheme = uri.scheme == 'https' ? 'wss' : 'ws';
  return uri.replace(scheme: wsScheme, path: '/api/livemode/session').toString();
}

/// Owns the WebSocket connection to `/api/livemode/session` — Phase 6 only
/// proves the duplex transport (connect, send binary audio frames, receive
/// them back from the backend's stub EchoSession) works end-to-end; wiring
/// this to real microphone capture/playback and a real engine (instead of
/// the backend's echo stub) lands in Phase 7/8 once Google Live/OpenAI
/// Realtime clients exist.
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
  int _generation = 0;

  Future<void> connect() async {
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
      state = const LiveRealtimeSessionState(status: LiveRealtimeSessionStatus.connected);
      _sub = channel.stream.listen(
        (message) {
          if (myGeneration != _generation) return;
          // Phase 6 only proves messages arrive — playback (binary frames)
          // and control-frame parsing (text frames, see
          // handlers_livemode_session.go's liveModeSessionControlFrame)
          // land once a real engine is on the other end (Phase 7/8).
        },
        onDone: () {
          if (myGeneration != _generation) return;
          state = const LiveRealtimeSessionState(status: LiveRealtimeSessionStatus.closed);
        },
        onError: (Object e) {
          if (myGeneration != _generation) return;
          state = LiveRealtimeSessionState(status: LiveRealtimeSessionStatus.error, error: '$e');
        },
      );
    } catch (e) {
      if (myGeneration != _generation) return;
      state = LiveRealtimeSessionState(status: LiveRealtimeSessionStatus.error, error: '$e');
    }
  }

  /// Sends one raw PCM chunk from the microphone to the backend.
  void sendAudio(Uint8List pcm) {
    _channel?.sink.add(pcm);
  }

  Future<void> disconnect() async {
    _generation++;
    await _sub?.cancel();
    _sub = null;
    await _channel?.sink.close();
    _channel = null;
    state = const LiveRealtimeSessionState();
  }

  @override
  void dispose() {
    _generation++;
    _sub?.cancel();
    _channel?.sink.close();
    super.dispose();
  }
}

final liveRealtimeSessionProvider =
    StateNotifierProvider<LiveRealtimeSessionNotifier, LiveRealtimeSessionState>(
  (ref) => LiveRealtimeSessionNotifier(ref),
);
