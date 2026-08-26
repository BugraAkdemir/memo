import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/friendly_error.dart';
import '../../../core/l10n.dart';
import '../../../core/theme.dart';
import '../../../core/tts_playback_error.dart';
import '../../../core/wav_player.dart';
import '../../../models/live_mode_config.dart';
import '../../../providers/chat_provider.dart';
import '../../../providers/settings_provider.dart';
import '../tts_provider_section.dart';
import '../tts_voice_section.dart';

/// Settings → Live Mode.
///
/// Graduated out of Beta Features (see docs/plans/PLAN_live_mode_v2.md,
/// Phase 1) — same precedent RemoteAccess/Tailscale already set: its own
/// config (config.LiveModeConfig), its own on/off toggle, no longer piggy-
/// backed on the experimental-features flag. This phase only wires the
/// top-level Enabled switch and keeps today's local-engine (Whisper+Piper)
/// configuration widgets working under it — the full engine picker (Google
/// Live/OpenAI/ElevenLabs/Custom) and live-fetched model dropdowns land in
/// later phases of the same plan.
class LiveModeTab extends ConsumerStatefulWidget {
  const LiveModeTab({super.key});

  @override
  ConsumerState<LiveModeTab> createState() => _LiveModeTabState();
}

class _LiveModeTabState extends ConsumerState<LiveModeTab> {
  bool _busy = false;

  Future<void> _setEnabled(LiveModeConfig current, bool enabled) async {
    if (_busy) return;
    setState(() => _busy = true);
    try {
      await ref
          .read(apiClientProvider)
          .updateLiveModeConfig(current.copyWith(enabled: enabled));
      ref.invalidate(liveModeConfigProvider);
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(
              '${L10n.t('error')}: ${FriendlyError.describeGeneric(e)}',
            ),
          ),
        );
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    final cfgAsync = ref.watch(liveModeConfigProvider);

    return cfgAsync.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (err, _) => Center(
        child: Padding(
          padding: const EdgeInsets.all(32),
          child: Text(
            L10n.t('remote_load_failed', {'err': '$err'}),
            style: TextStyle(color: MemoTheme.red),
          ),
        ),
      ),
      data: (cfg) {
        // Phase 1: only the "local" engine (today's Whisper+Piper pipeline)
        // is implemented, so its config widgets are always relevant while
        // ActiveEngine stays at its "local" default. Later phases replace
        // this unconditional block with a real engine picker.
        final showLocalEngineConfig = cfg.activeEngine == 'local';
        return ListView(
          padding: const EdgeInsets.all(32),
          children: [
            Text(
              L10n.t('live_mode_tab_title'),
              style: TextStyle(
                fontSize: 18,
                fontWeight: FontWeight.w700,
                color: theme.textMain,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              L10n.t('live_mode_tab_desc'),
              style: TextStyle(fontSize: 13, height: 1.45, color: theme.textDim),
            ),
            const SizedBox(height: 24),
            SwitchListTile(
              title: Text(
                L10n.t('live_mode_enabled_title'),
                style: TextStyle(fontSize: 14, color: theme.textMain),
              ),
              subtitle: Text(
                L10n.t('live_mode_enabled_desc'),
                style: TextStyle(fontSize: 12, color: theme.textDim),
              ),
              value: cfg.enabled,
              onChanged: _busy ? null : (v) => _setEnabled(cfg, v),
              dense: true,
              contentPadding: EdgeInsets.zero,
              activeThumbColor: MemoTheme.accent,
            ),
            if (_busy) ...[
              const SizedBox(height: 8),
              const LinearProgressIndicator(minHeight: 2),
            ],
            if (cfg.enabled && showLocalEngineConfig) ...[
              const SizedBox(height: 16),
              const _LiveModeVoiceTest(),
              const SizedBox(height: 16),
              const TTSVoiceSection(),
              const SizedBox(height: 16),
              const TTSProviderSection(),
            ],
          ],
        );
      },
    );
  }
}

/// Moved from beta_features_tab.dart's _LiveModeVoiceTest (Faz 1, see
/// docs/plans/PLAN_voice_live_mode_faz1.md) as part of Live Mode's
/// graduation out of Beta — lets a user type text, synthesize it via the
/// backend's currently-active TTS engine, and hear the result, without
/// needing to actually go through the voice loop.
class _LiveModeVoiceTest extends ConsumerStatefulWidget {
  const _LiveModeVoiceTest();

  @override
  ConsumerState<_LiveModeVoiceTest> createState() =>
      _LiveModeVoiceTestState();
}

class _LiveModeVoiceTestState extends ConsumerState<_LiveModeVoiceTest> {
  final _controller = TextEditingController();
  final _player = WavPlayer();
  bool _synthesizing = false;
  bool _playing = false;
  String? _error;

  @override
  void dispose() {
    _controller.dispose();
    _player.dispose();
    super.dispose();
  }

  Future<void> _speak() async {
    final text = _controller.text.trim();
    if (text.isEmpty || _synthesizing) return;
    setState(() {
      _synthesizing = true;
      _error = null;
    });
    try {
      final audio = await ref.read(apiClientProvider).synthesizeSpeech(text);
      if (!mounted) return;
      setState(() {
        _synthesizing = false;
        _playing = true;
      });
      await _player.play(audio);
    } catch (e) {
      if (mounted) setState(() => _error = friendlyPlaybackError(e));
    } finally {
      if (mounted) {
        setState(() {
          _synthesizing = false;
          _playing = false;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    final busy = _synthesizing || _playing;
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: theme.bgPanel,
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: theme.borderSoft),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            L10n.t('live_mode_test_tts_title'),
            style: TextStyle(
              fontSize: 13,
              fontWeight: FontWeight.w600,
              color: theme.textMain,
            ),
          ),
          const SizedBox(height: 4),
          Text(
            L10n.t('live_mode_test_tts_desc'),
            style: TextStyle(fontSize: 12, height: 1.4, color: theme.textDim),
          ),
          const SizedBox(height: 10),
          TextField(
            controller: _controller,
            enabled: !busy,
            style: TextStyle(fontSize: 13, color: theme.textMain),
            decoration: InputDecoration(
              hintText: L10n.t('live_mode_test_tts_hint'),
              isDense: true,
              border: const OutlineInputBorder(),
            ),
          ),
          const SizedBox(height: 10),
          Row(
            children: [
              FilledButton.icon(
                onPressed: busy ? null : _speak,
                icon: const Icon(Icons.play_arrow_rounded, size: 18),
                label: Text(
                  _synthesizing
                      ? L10n.t('live_mode_test_tts_synthesizing')
                      : _playing
                          ? L10n.t('live_mode_test_tts_playing')
                          : L10n.t('live_mode_test_tts_button'),
                ),
              ),
              if (busy) ...[
                const SizedBox(width: 12),
                const SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(strokeWidth: 2),
                ),
              ],
            ],
          ),
          if (_error != null) ...[
            const SizedBox(height: 8),
            Text(
              _error!,
              style: TextStyle(fontSize: 12, color: MemoTheme.red),
            ),
          ],
        ],
      ),
    );
  }
}
