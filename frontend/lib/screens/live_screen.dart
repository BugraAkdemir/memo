import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../core/live_mode_controller.dart';
import '../core/theme.dart';
import '../core/tts_playback_error.dart';
import '../core/wav_player.dart';
import '../providers/chat_provider.dart';

enum _LiveState { idle, listening, thinking, speaking }

/// Voice Live Mode's Faz 1 screen (docs/plans/PLAN_voice_live_mode_faz1.md's
/// 1.6): start/stop continuous VAD listening, send each transcribed
/// utterance through the existing chat pipeline exactly as chat_input.dart
/// does, and speak the reply back via the existing Piper TTS chain (Faz
/// 1.1-1.4). Deliberately does **not** implement barge-in yet (interrupting
/// playback with a new utterance) -- this first cut processes one
/// listen -> think -> speak cycle at a time, dropping any speech detected
/// while a cycle is already in flight, rather than risk racing this
/// screen's calls against chat_provider.dart's carefully-hardened
/// generation-counter/cancel-token state (see AGENTS.md's Riverpod gotcha)
/// without first reading that machinery closely enough to extend it
/// safely. Barge-in is the next remaining sub-step for 1.6.
///
/// **Known, undesigned consequence of no barge-in** (found in review, not
/// yet fixed): the mic is never paused during the `speaking` state, so
/// without headphones VAD can pick up Memo's own TTS playback as a new
/// utterance -- which then gets silently dropped by the busy guard below,
/// wasting a full STT round-trip on audio that was never real user speech.
/// Muting/pausing capture during playback (or at minimum skipping
/// transcription for segments detected while `_state == speaking`) would
/// fix this at the source; not implemented in this cut.
class LiveScreen extends ConsumerStatefulWidget {
  const LiveScreen({super.key});

  @override
  ConsumerState<LiveScreen> createState() => _LiveScreenState();
}

class _LiveScreenState extends ConsumerState<LiveScreen> {
  LiveModeController? _controller;
  final _player = WavPlayer();
  _LiveState _state = _LiveState.idle;
  String? _lastTranscript;
  String? _lastReply;
  String? _error;
  bool _busy = false;

  // Re-entrancy guard for _toggleListening: without it, a rapid double-tap
  // on the mic button races a stop()/dispose() against a still-pending
  // start() on the same controller, and a VAD event that fires after
  // dispose() closes the stream controllers throws (found in review).
  bool _togglingListening = false;

  @override
  void dispose() {
    // Chain dispose() after stop() resolves rather than firing both
    // synchronously: LiveModeController.stop() awaits the VAD handler's
    // real teardown before returning, so this ordering (plus the isClosed
    // guards inside LiveModeController itself) avoids closing the stream
    // controllers while an in-flight onSpeechEnd callback could still try
    // to use them.
    _controller?.stop().then((_) => _controller?.dispose());
    _player.dispose();
    super.dispose();
  }

  Future<void> _toggleListening() async {
    if (_togglingListening) return;
    _togglingListening = true;
    try {
      if (_controller != null) {
        final controller = _controller!;
        _controller = null;
        await controller.stop();
        controller.dispose();
        if (mounted) setState(() => _state = _LiveState.idle);
        return;
      }

      final controller = LiveModeController(ref.read(apiClientProvider));
      controller.onTranscript.listen(_handleTranscript);
      controller.onError.listen((message) {
        if (mounted) setState(() => _error = message);
      });

      try {
        await controller.start();
        if (!mounted) {
          // Screen was disposed while start() was in flight -- don't leak
          // the controller/VadHandler, and don't publish it to _controller
          // (nothing left to read it).
          await controller.stop();
          controller.dispose();
          return;
        }
        _controller = controller;
        setState(() {
          _state = _LiveState.listening;
          _error = null;
        });
      } catch (e) {
        // start() failed (e.g. the VAD model's CDN fetch -- see
        // LiveModeController's own doc comment -- failed while offline).
        // Release the half-initialized controller instead of just
        // dropping the reference, which used to leak the VadHandler and
        // its listeners; also reset _state, which used to stay wherever
        // it was previously (e.g. stuck showing "listening" while
        // _controller was already null).
        await controller.stop();
        controller.dispose();
        if (mounted) {
          setState(() {
            _error = friendlyPlaybackError(e);
            _state = _LiveState.idle;
          });
        }
      }
    } finally {
      _togglingListening = false;
    }
  }

  Future<void> _handleTranscript(String text) async {
    // Drop overlapping utterances instead of racing a second sendMessage()
    // against one still in flight -- see the class doc for why real
    // barge-in isn't implemented yet.
    if (_busy || !mounted) return;

    if (ref.read(isSendingProvider)) {
      // chat_provider.dart's sendMessage() silently no-ops when this is
      // already true (some other part of the app is mid-send) -- calling
      // it now would make this utterance vanish with zero feedback and,
      // worse, _lastReply below could pick up a stale prior assistant
      // message and speak it back as if it were the reply to this
      // utterance (found in review). Surface this instead of proceeding.
      setState(() => _error = L10n.t('live_screen_error_busy_elsewhere'));
      return;
    }

    setState(() {
      _busy = true;
      _state = _LiveState.thinking;
      _lastTranscript = text;
      _error = null;
    });

    // _busy stays true for this whole cycle -- sendMessage AND the
    // playback that follows it -- so a VAD segment detected while Memo is
    // still speaking is dropped by the guard above instead of starting a
    // second, overlapping cycle. Nested try/catch blocks below give each
    // phase its own error message without an inner `finally` resetting
    // _busy early (a mistake made and caught in review: splitting this
    // into two top-level try/finally blocks reset _busy right after
    // sendMessage, before speaking even started, which reopened the
    // overlapping-utterance window for the entire playback phase).
    try {
      String reply = '';
      try {
        final messagesBefore = ref.read(messagesProvider).valueOrNull ?? [];
        await ref.read(messagesProvider.notifier).sendMessage(text);
        if (!mounted) return;

        final messages = ref.read(messagesProvider).valueOrNull ?? [];
        // A genuine new reply must have actually grown the list, not just
        // still end in an assistant message left over from an earlier
        // turn -- sendMessage() appends the user bubble unconditionally
        // before it can fail, so on failure the list grows by exactly one
        // (the user message) and still ends in 'user'. Requiring both
        // growth and a trailing assistant message rules out both the
        // "silently failed" and the "stale reply" cases found in review.
        final gotNewReply = messages.length > messagesBefore.length &&
            messages.last.role == 'assistant';
        if (!gotNewReply) {
          setState(() => _error = L10n.t('live_screen_error_no_reply'));
          return;
        }
        reply = messages.last.content;
        _lastReply = reply;
      } catch (e) {
        if (mounted) {
          setState(
            () =>
                _error = L10n.t('live_screen_error_send_failed', {'err': '$e'}),
          );
        }
        return;
      }

      if (reply.trim().isEmpty) return;
      try {
        if (mounted) setState(() => _state = _LiveState.speaking);
        final audio = await ref.read(apiClientProvider).synthesizeSpeech(reply);
        if (!mounted) return;
        await _player.play(audio);
      } catch (e) {
        if (mounted) setState(() => _error = friendlyPlaybackError(e));
      }
    } finally {
      if (mounted) {
        setState(() {
          _busy = false;
          _state = _controller != null ? _LiveState.listening : _LiveState.idle;
        });
      }
    }
  }

  String _stateLabel() {
    switch (_state) {
      case _LiveState.idle:
        return L10n.t('live_screen_state_idle');
      case _LiveState.listening:
        return L10n.t('live_screen_state_listening');
      case _LiveState.thinking:
        return L10n.t('live_screen_state_thinking');
      case _LiveState.speaking:
        return L10n.t('live_screen_state_speaking');
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    final listening = _controller != null;

    return Scaffold(
      backgroundColor: theme.bgApp,
      body: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              L10n.t('live_screen_title'),
              style: TextStyle(
                fontSize: 18,
                fontWeight: FontWeight.w700,
                color: theme.textMain,
              ),
            ),
            const SizedBox(height: 16),
            Text(
              _stateLabel(),
              style: TextStyle(
                fontSize: 20,
                fontWeight: FontWeight.w700,
                color: theme.textMain,
              ),
            ),
            const SizedBox(height: 24),
            if (_lastTranscript != null) ...[
              Text(
                L10n.t('live_screen_you_said'),
                style: TextStyle(fontSize: 12, color: theme.textDim),
              ),
              const SizedBox(height: 4),
              Text(
                _lastTranscript!,
                style: TextStyle(fontSize: 14, color: theme.textMain),
              ),
              const SizedBox(height: 16),
            ],
            if (_lastReply != null) ...[
              Text(
                L10n.t('live_screen_memo_replied'),
                style: TextStyle(fontSize: 12, color: theme.textDim),
              ),
              const SizedBox(height: 4),
              Text(
                _lastReply!,
                style: TextStyle(fontSize: 14, color: theme.textMain),
              ),
              const SizedBox(height: 16),
            ],
            if (_error != null) ...[
              Text(
                _error!,
                style: TextStyle(fontSize: 12, color: MemoTheme.red),
              ),
              const SizedBox(height: 16),
            ],
            const Spacer(),
            Center(
              child: FilledButton.icon(
                onPressed: _toggleListening,
                icon: Icon(listening ? Icons.stop : Icons.mic),
                label: Text(
                  listening
                      ? L10n.t('live_screen_stop_button')
                      : L10n.t('live_screen_start_button'),
                ),
                style: FilledButton.styleFrom(
                  minimumSize: const Size(220, 52),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
