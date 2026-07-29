import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../core/live_mode_controller.dart';
import '../core/tts_playback_error.dart';
import '../core/wav_player.dart';
import 'chat_provider.dart';

/// Cross-modal voice mode for the normal chat screen: unlike the old,
/// removed standalone Live Mode tab, this lives at the provider level so
/// it works from the regular chat input (chat_input.dart's mic-adjacent
/// toggle button) and keeps running across whatever screen is on top of
/// the IndexedStack, not a screen of its own. Typing still works exactly
/// as before while this is on — a user can speak OR type, in either
/// order, and every assistant reply gets spoken back for as long as
/// voice mode is active. Ported from the removed live_screen.dart, same
/// underlying capture engine (LiveModeController) and the same
/// deliberately-simple one-cycle-at-a-time behavior (no barge-in yet —
/// see LiveModeController's and live_mode_controller.dart's own doc
/// comments for why, and for the known VAD-model-from-CDN stability gap
/// this inherits unchanged).
enum VoiceModeState { idle, listening, thinking, speaking }

final voiceModeProvider =
    StateNotifierProvider<VoiceModeNotifier, VoiceModeState>(
      VoiceModeNotifier.new,
    );

class VoiceModeNotifier extends StateNotifier<VoiceModeState> {
  final Ref _ref;
  final WavPlayer _player = WavPlayer();
  LiveModeController? _controller;
  bool _busy = false;
  bool _toggling = false;

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
    // Drop overlapping utterances instead of racing a second sendMessage()
    // against one already in flight — real barge-in isn't implemented yet
    // (see class doc).
    if (_busy) return;

    if (_ref.read(isSendingProvider)) {
      // chat_provider.dart's sendMessage() silently no-ops when this is
      // already true (a typed message is mid-send elsewhere) — surface
      // that instead of the utterance just vanishing with zero feedback.
      _ref.read(errorMessageProvider.notifier).state =
          L10n.t('live_screen_error_busy_elsewhere');
      return;
    }

    _busy = true;
    state = VoiceModeState.thinking;

    // _busy stays true for this whole cycle — sendMessage AND the playback
    // that follows it — so a VAD segment detected while Memo is still
    // speaking is dropped by the guard above instead of starting a second,
    // overlapping cycle.
    try {
      String reply = '';
      try {
        final messagesBefore = _ref.read(messagesProvider).valueOrNull ?? [];
        await _ref.read(messagesProvider.notifier).sendMessage(text);

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
        _ref.read(errorMessageProvider.notifier).state =
            L10n.t('live_screen_error_send_failed', {'err': '$e'});
        return;
      }

      if (reply.trim().isEmpty) return;
      try {
        state = VoiceModeState.speaking;
        final audio = await _ref.read(apiClientProvider).synthesizeSpeech(reply);
        await _player.play(audio);
      } catch (e) {
        _ref.read(errorMessageProvider.notifier).state = friendlyPlaybackError(e);
      }
    } finally {
      _busy = false;
      state = _controller != null ? VoiceModeState.listening : VoiceModeState.idle;
    }
  }

  @override
  void dispose() {
    _controller?.stop().then((_) => _controller?.dispose());
    _player.dispose();
    super.dispose();
  }
}
