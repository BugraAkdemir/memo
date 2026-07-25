import 'package:audioplayers/audioplayers.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../core/live_mode_controller.dart';
import '../core/theme.dart';
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
class LiveScreen extends ConsumerStatefulWidget {
  const LiveScreen({super.key});

  @override
  ConsumerState<LiveScreen> createState() => _LiveScreenState();
}

class _LiveScreenState extends ConsumerState<LiveScreen> {
  LiveModeController? _controller;
  final _player = AudioPlayer();
  _LiveState _state = _LiveState.idle;
  String? _lastTranscript;
  String? _lastReply;
  String? _error;
  bool _busy = false;

  @override
  void dispose() {
    _controller?.stop();
    _controller?.dispose();
    _player.dispose();
    super.dispose();
  }

  Future<void> _toggleListening() async {
    if (_controller != null) {
      await _controller!.stop();
      _controller!.dispose();
      _controller = null;
      setState(() => _state = _LiveState.idle);
      return;
    }

    final controller = LiveModeController(ref.read(apiClientProvider));
    _controller = controller;
    controller.onTranscript.listen(_handleTranscript);
    controller.onError.listen((message) {
      if (mounted) setState(() => _error = message);
    });

    try {
      await controller.start();
      if (!mounted) return;
      setState(() {
        _state = _LiveState.listening;
        _error = null;
      });
    } catch (e) {
      if (mounted) setState(() => _error = '$e');
      _controller = null;
    }
  }

  Future<void> _handleTranscript(String text) async {
    // Drop overlapping utterances instead of racing a second sendMessage()
    // against one still in flight -- see the class doc for why real
    // barge-in isn't implemented yet.
    if (_busy || !mounted) return;
    setState(() {
      _busy = true;
      _state = _LiveState.thinking;
      _lastTranscript = text;
      _error = null;
    });

    try {
      final notifier = ref.read(messagesProvider.notifier);
      await notifier.sendMessage(text);
      if (!mounted) return;

      final messages = ref.read(messagesProvider).valueOrNull ?? [];
      final reply = messages.isNotEmpty && messages.last.role == 'assistant'
          ? messages.last.content
          : '';
      _lastReply = reply;

      if (reply.trim().isNotEmpty) {
        setState(() => _state = _LiveState.speaking);
        final audio = await ref.read(apiClientProvider).synthesizeSpeech(reply);
        if (!mounted) return;
        await _player.play(BytesSource(audio));
        await _player.onPlayerComplete.first;
      }
    } catch (e) {
      if (mounted) setState(() => _error = '$e');
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
      appBar: AppBar(
        title: Text(L10n.t('live_screen_title')),
        backgroundColor: theme.bgApp,
        elevation: 0,
      ),
      body: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
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
