import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../core/live_mode_controller.dart';
import '../core/tts_playback_error.dart';
import '../core/wav_player.dart';
import 'chat_provider.dart';
import '../core/friendly_error.dart';

/// Cross-modal voice mode for the normal chat screen: unlike the old,
/// removed standalone Live Mode tab, this lives at the provider level so
/// it works from the regular chat input (chat_input.dart's mic-adjacent
/// toggle button) and keeps running across whatever screen is on top of
/// the IndexedStack, not a screen of its own. Typing still works exactly
/// as before while this is on — a user can speak OR type, in either
/// order, and every assistant reply gets spoken back for as long as
/// voice mode is active. Ported from the removed live_screen.dart, same
/// underlying capture engine (LiveModeController).
///
/// **One-directional barge-in**: the user can interrupt Memo mid-reply
/// (mid-"thinking" or mid-"speaking") by just talking again — the new
/// utterance cancels whatever's in flight (WavPlayer.stop() on both
/// players, chat_provider's stopStreaming() on the LLM call) instead of
/// being silently dropped. The reverse — Memo interrupting the user, or
/// backchannel sounds while the user is still talking — is not
/// implemented (that's the parent plan's Faz 4, full duplex + AEC).
///
/// **Known, accepted risk this creates, worse than the old no-barge-in
/// behavior**: without headphones or real echo cancellation (AEC,
/// explicitly out of scope for this pass — see PLAN_voice_live_mode.md's
/// Faz 4), VAD can pick up Memo's own TTS/filler audio playing through
/// the speakers as if it were the user talking again. Previously that
/// misfire was silently dropped (see the old _busy guard) — wasted one STT
/// round-trip, otherwise harmless. Now it's treated as an intentional
/// barge-in and actually cuts Memo off mid-sentence. This is inherent to
/// implementing real barge-in without AEC, not a bug to fix here — a
/// speaker-only setup should expect self-interruptions; headphones avoid
/// the problem entirely (no acoustic path from output back to the mic).
enum VoiceModeState { idle, listening, thinking, speaking }

final voiceModeProvider =
    StateNotifierProvider<VoiceModeNotifier, VoiceModeState>(
      VoiceModeNotifier.new,
    );

class VoiceModeNotifier extends StateNotifier<VoiceModeState> {
  final Ref _ref;
  final WavPlayer _player = WavPlayer();
  // Separate player instance for filler sounds (Faz 3) — deliberately not
  // the same _player used for the real reply: a filler is fire-and-forget
  // and may still be finishing its own subprocess-backed playback right as
  // the real reply's play() call starts, and the two shouldn't block or
  // race on shared player state to do that.
  final WavPlayer _fillerPlayer = WavPlayer();
  LiveModeController? _controller;
  bool _busy = false;
  bool _toggling = false;

  // Bumped every time a cycle starts (fresh utterance OR a barge-in that
  // preempts one already running). A cycle's own async continuations
  // (after each await) compare their captured generation against the
  // live one before touching shared state (_busy, state) — the same
  // "stale call self-invalidates on a newer one" technique
  // chat_provider.dart's MessagesNotifier already uses for the identical
  // problem (see AGENTS.md's Riverpod gotcha) — so a barged-in cycle's
  // `finally` can't stomp on the state the interrupting cycle is setting.
  int _generation = 0;

  VoiceModeNotifier(this._ref) : super(VoiceModeState.idle);

  bool get isActive => _controller != null;

  Future<void> toggle() async {
    // Re-entrancy guard: a rapid double-tap on the toggle button would
    // otherwise race a stop()/dispose() against a still-pending start() on
    // the same controller (the exact bug live_screen.dart's own
    // _toggleListening comment documented and guarded against — carried
    // over here unchanged).
    if (_toggling) return;
    _toggling = true;
    try {
      if (_controller != null) {
        final controller = _controller!;
        _controller = null;
        await controller.stop();
        controller.dispose();
        state = VoiceModeState.idle;
        return;
      }

      final controller = LiveModeController(_ref.read(apiClientProvider));
      controller.onTranscript.listen(_handleTranscript);
      controller.onError.listen((message) {
        _ref.read(errorMessageProvider.notifier).state = message;
      });

      try {
        await controller.start();
        _controller = controller;
        state = VoiceModeState.listening;
      } catch (e) {
        // start() failed (e.g. the VAD model's CDN fetch failed while
        // offline — see LiveModeController's doc comment on the known,
        // unbundled-model gap). Release the half-initialized controller
        // rather than leaking it.
        await controller.stop();
        controller.dispose();
        _ref.read(errorMessageProvider.notifier).state = friendlyPlaybackError(e);
        state = VoiceModeState.idle;
      }
    } finally {
      _toggling = false;
    }
  }

  Future<void> _handleTranscript(String text) async {
    if (_busy) {
      // Memo is still thinking or speaking — treat this new utterance as
      // a barge-in rather than dropping it: bump _generation first (so
      // the in-flight cycle's own continuations below see a mismatch and
      // skip touching shared state once they unwind), then actually stop
      // what's in flight. WavPlayer.stop() is a no-op if that particular
      // player isn't playing (e.g. barging in during "thinking", before
      // any audio exists yet); stopStreaming() is a no-op if the LLM call
      // already finished (e.g. barging in during "speaking").
      _generation++;
      _fillerPlayer.stop();
      _player.stop();
      _ref.read(messagesProvider.notifier).stopStreaming();
    }

    if (_ref.read(isSendingProvider)) {
      // Only reachable for a send that ISN'T this notifier's own (that one
      // was just cancelled above, and stopStreaming() clears
      // isSendingProvider synchronously) — a typed message sent from the
      // normal chat input, genuinely unrelated and still in flight.
      // chat_provider.dart's sendMessage() would silently no-op in this
      // case — surface it instead of the utterance just vanishing.
      _ref.read(errorMessageProvider.notifier).state =
          L10n.t('live_screen_error_busy_elsewhere');
      return;
    }

    final myGeneration = ++_generation;
    _busy = true;
    state = VoiceModeState.thinking;
    _playFillerBestEffort(myGeneration);

    try {
      String reply = '';
      try {
        final messagesBefore = _ref.read(messagesProvider).valueOrNull ?? [];
        await _ref.read(messagesProvider.notifier).sendMessage(text);
        if (myGeneration != _generation) return; // barged in again mid-send

        final messages = _ref.read(messagesProvider).valueOrNull ?? [];
        // A genuine new reply must have actually grown the list, not just
        // still end in an assistant message left over from an earlier
        // turn — sendMessage() appends the user bubble unconditionally
        // before it can fail, so on failure the list grows by exactly one
        // (the user message) and still ends in 'user'.
        final gotNewReply = messages.length > messagesBefore.length &&
            messages.last.role == 'assistant';
        if (!gotNewReply) {
          _ref.read(errorMessageProvider.notifier).state =
              L10n.t('live_screen_error_no_reply');
          return;
        }
        reply = messages.last.content;
      } catch (e) {
        if (myGeneration != _generation) return; // cancelled by a barge-in
        _ref.read(errorMessageProvider.notifier).state =
            L10n.t('live_screen_error_send_failed', {'err': FriendlyError.describeGeneric(e)});
        return;
      }

      if (reply.trim().isEmpty) return;
      try {
        state = VoiceModeState.speaking;
        final audio = await _ref.read(apiClientProvider).synthesizeSpeech(reply);
        if (myGeneration != _generation) return; // barged in during synthesis
        // Cut any filler still playing/queued so it doesn't overlap the
        // real reply — the loop in _playFillerBestEffort also exits on its
        // own now that state != thinking, but a clip already mid-subprocess
        // needs an explicit stop.
        _fillerPlayer.stop();
        await _player.play(audio);
      } catch (e) {
        if (myGeneration != _generation) return;
        _ref.read(errorMessageProvider.notifier).state = friendlyPlaybackError(e);
      }
    } finally {
      // A stale (barged-in) cycle must not reset _busy/state out from
      // under the cycle that interrupted it — only the still-current
      // generation gets to do that.
      if (myGeneration == _generation) {
        _busy = false;
        state = _controller != null ? VoiceModeState.listening : VoiceModeState.idle;
      }
    }
  }

  /// Plays cached local filler sounds (Faz 3, GET /api/tts/filler) on a loop
  /// while sendMessage() is in flight, to mask the reply's latency —
  /// replaying with a short human-ish pause between clips instead of playing
  /// exactly one and then sitting silent for the rest of a multi-second
  /// (delegated / agentic) reply. Stops the instant this cycle goes stale
  /// (barge-in bumped [_generation]) or the real reply has started
  /// ([state] left `thinking`). Deliberately fire-and-forget: a
  /// missing/unconfigured local Piper voice (filler sounds are local-only,
  /// never routed through an external TTS provider — see
  /// internal/tts/filler.go's own doc comment) just means no filler plays,
  /// silently — a nice-to-have, not something worth an error toast every turn.
  void _playFillerBestEffort(int myGeneration) {
    () async {
      try {
        while (myGeneration == _generation && state == VoiceModeState.thinking) {
          final audio = await _ref.read(apiClientProvider).getTTSFiller();
          if (myGeneration != _generation || state != VoiceModeState.thinking) {
            return;
          }
          await _fillerPlayer.play(audio);
          // A brief, human-ish pause before the next thinking sound.
          await Future<void>.delayed(const Duration(milliseconds: 1200));
        }
      } catch (_) {
        // best-effort, see doc comment above
      }
    }();
  }

  @override
  void dispose() {
    _controller?.stop().then((_) => _controller?.dispose());
    _player.dispose();
    _fillerPlayer.dispose();
    super.dispose();
  }
}
