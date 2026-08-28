import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/friendly_error.dart';
import '../../../core/l10n.dart';
import '../../../core/theme.dart';
import '../../../core/tts_playback_error.dart';
import '../../../core/wav_player.dart';
import '../../../models/live_mode_config.dart';
import '../../../models/live_mode_engine_config.dart';
import '../../../providers/chat_provider.dart';
import '../../../providers/settings_provider.dart';
import '../tts_provider_section.dart';
import '../tts_voice_section.dart';

/// The four non-local engines, in picker display order. "local" is handled
/// separately (no EngineConfig entry, see LiveModeEngineConfig's doc
/// comment) so it isn't in this list.
const _nonLocalEngines = [
  'google_live',
  'openai_realtime',
  'elevenlabs',
  'custom',
];

/// Engines with a genuine native audio-to-audio reasoning model, where
/// WorkMode/AgentPermissionPolicy are meaningful — mirrors
/// LiveModeEngineDefaults.nativeReasoningEngines.
const _nativeReasoningEngines = ['google_live', 'openai_realtime'];

/// Google Live's documented prebuilt voice names (confirmed against
/// current API docs, 2026-08-27) — a fixed, provider-documented catalog
/// (listenable in full in Google AI Studio), not a "model list" this
/// codebase's own never-hardcode-models rule is about; this is the same
/// class of fixed enum ElevenLabs' own voice_id free-text field already
/// assumes exists on the provider's side, just presented as a dropdown
/// here since the whole set is small and named.
const _googleLiveVoices = ['Puck', 'Charon', 'Kore', 'Fenrir', 'Aoede', 'Leda', 'Orus', 'Zephyr'];

/// OpenAI Realtime's documented voice names (confirmed against current API
/// docs, 2026-08-27) — marin/cedar are OpenAI's own recommended picks for
/// best quality.
const _openaiRealtimeVoices = ['alloy', 'ash', 'ballad', 'coral', 'echo', 'sage', 'shimmer', 'verse', 'marin', 'cedar'];

String _engineLabel(String engine) => switch (engine) {
      'local' => L10n.t('live_mode_engine_local'),
      'google_live' => L10n.t('live_mode_engine_google_live'),
      'openai_realtime' => L10n.t('live_mode_engine_openai_realtime'),
      'elevenlabs' => L10n.t('live_mode_engine_elevenlabs'),
      'custom' => L10n.t('live_mode_engine_custom'),
      _ => engine,
    };

/// Settings → Live Mode.
///
/// Graduated out of Beta Features (see docs/plans/PLAN_live_mode_v2.md,
/// Phase 1) — same precedent RemoteAccess/Tailscale already set: its own
/// config (config.LiveModeConfig), its own on/off toggle, no longer piggy-
/// backed on the experimental-features flag. Phase 3 adds the full engine
/// picker and per-engine config forms (API key/model/voice/base_url) — the
/// Model field is still free text here; live-fetched model dropdowns land
/// in Phase 4.
class LiveModeTab extends ConsumerStatefulWidget {
  const LiveModeTab({super.key});

  @override
  ConsumerState<LiveModeTab> createState() => _LiveModeTabState();
}

class _LiveModeTabState extends ConsumerState<LiveModeTab> {
  bool _busy = false;

  Future<void> _updateConfig(LiveModeConfig next) async {
    if (_busy) return;
    setState(() => _busy = true);
    try {
      await ref.read(apiClientProvider).updateLiveModeConfig(next);
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

  Future<void> _setEnabled(LiveModeConfig current, bool enabled) =>
      _updateConfig(current.copyWith(enabled: enabled));

  Future<void> _setActiveEngine(LiveModeConfig current, String engine) =>
      _updateConfig(current.copyWith(activeEngine: engine));

  Future<void> _setWorkMode(LiveModeConfig current, String mode) =>
      _updateConfig(current.copyWith(workMode: mode));

  Future<void> _setPermissionPolicy(LiveModeConfig current, String policy) =>
      _updateConfig(current.copyWith(agentPermissionPolicy: policy));

  Future<void> _setBargeInSensitivity(LiveModeConfig current, String level) =>
      _updateConfig(current.copyWith(bargeInSensitivity: level));

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
        final isLocal = cfg.activeEngine == 'local';
        final isNativeReasoning = _nativeReasoningEngines.contains(cfg.activeEngine);
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
            if (cfg.enabled) ...[
              const SizedBox(height: 24),
              Text(
                L10n.t('live_mode_engine_label'),
                style: TextStyle(
                  fontSize: 13,
                  fontWeight: FontWeight.w600,
                  color: theme.textMain,
                ),
              ),
              const SizedBox(height: 8),
              DropdownButtonFormField<String>(
                initialValue: cfg.activeEngine,
                decoration: const InputDecoration(
                  isDense: true,
                  border: OutlineInputBorder(),
                ),
                items: [
                  'local',
                  ..._nonLocalEngines,
                ]
                    .map((e) => DropdownMenuItem(
                          value: e,
                          child: Text(_engineLabel(e)),
                        ))
                    .toList(),
                onChanged: _busy
                    ? null
                    : (v) {
                        if (v != null) _setActiveEngine(cfg, v);
                      },
              ),
              if (isLocal) ...[
                const SizedBox(height: 16),
                const _LiveModeVoiceTest(),
                const SizedBox(height: 16),
                const TTSVoiceSection(),
                const SizedBox(height: 16),
                const TTSProviderSection(),
              ] else ...[
                const SizedBox(height: 16),
                _EngineConfigForm(
                  key: ValueKey(cfg.activeEngine),
                  engineType: cfg.activeEngine,
                ),
                if (isNativeReasoning) ...[
                  const SizedBox(height: 16),
                  _WorkModeAndPermissionSection(
                    cfg: cfg,
                    busy: _busy,
                    onWorkModeChanged: (v) => _setWorkMode(cfg, v),
                    onPermissionPolicyChanged: (v) =>
                        _setPermissionPolicy(cfg, v),
                    onBargeInSensitivityChanged: (v) =>
                        _setBargeInSensitivity(cfg, v),
                  ),
                ],
              ],
            ],
          ],
        );
      },
    );
  }
}

/// Config form for one non-local engine (API key/model/voice/base_url,
/// varying by engine type). Keyed by engineType in the parent's build
/// (`ValueKey(cfg.activeEngine)`) so switching engines gets a fresh State
/// with fresh TextEditingControllers instead of stale text from the
/// previously-selected engine.
///
/// The Model field starts as free text and becomes a dropdown once
/// "Fetch Models" successfully returns a live list from that provider's own
/// API (Phase 4, docs/plans/PLAN_live_mode_v2.md §5.1 — never a hardcoded
/// list). This is a plain user-triggered one-shot fetch rather than an
/// ambiently-watched Riverpod provider, so it doesn't need the
/// authGateBlocked guard pattern other FutureProviders in this file follow
/// — that pattern exists for state read automatically at screen-open/app-
/// start time, not a button the user presses well after the auth gate has
/// resolved.
class _EngineConfigForm extends ConsumerStatefulWidget {
  const _EngineConfigForm({super.key, required this.engineType});

  final String engineType;

  @override
  ConsumerState<_EngineConfigForm> createState() => _EngineConfigFormState();
}

class _EngineConfigFormState extends ConsumerState<_EngineConfigForm> {
  final _apiKeyController = TextEditingController();
  final _modelController = TextEditingController();
  final _voiceController = TextEditingController();
  final _baseUrlController = TextEditingController();
  bool _saving = false;
  bool _initializedFromExisting = false;

  bool _fetchingModels = false;
  List<LiveModeModelInfo>? _fetchedModels;
  String? _fetchError;

  @override
  void dispose() {
    _apiKeyController.dispose();
    _modelController.dispose();
    _voiceController.dispose();
    _baseUrlController.dispose();
    super.dispose();
  }

  Future<void> _fetchModels() async {
    if (_fetchingModels) return;
    setState(() {
      _fetchingModels = true;
      _fetchError = null;
    });
    try {
      final models = await ref
          .read(apiClientProvider)
          .listLiveModeEngineModels(widget.engineType, _apiKeyController.text.trim());
      if (!mounted) return;
      setState(() {
        _fetchedModels = models;
        // Keep the current Model value if it's one of the fetched IDs;
        // otherwise default to the first result so the dropdown always
        // shows a valid selection rather than an empty one.
        if (models.isNotEmpty &&
            !models.any((m) => m.id == _modelController.text)) {
          _modelController.text = models.first.id;
        }
      });
    } catch (e) {
      if (mounted) {
        setState(() => _fetchError = FriendlyError.describeGeneric(e));
      }
    } finally {
      if (mounted) setState(() => _fetchingModels = false);
    }
  }

  void _initFrom(LiveModeEngineConfig cfg) {
    if (_initializedFromExisting) return;
    _initializedFromExisting = true;
    _apiKeyController.text = cfg.apiKey ?? '';
    _modelController.text = cfg.model;
    _voiceController.text = cfg.voice;
    _baseUrlController.text = cfg.baseURL;
  }

  Future<void> _save() async {
    if (_saving) return;
    setState(() => _saving = true);
    try {
      await ref.read(apiClientProvider).updateLiveModeEngine(
            LiveModeEngineConfig(
              type: widget.engineType,
              apiKey: _apiKeyController.text.trim(),
              model: _modelController.text.trim(),
              voice: _voiceController.text.trim(),
              baseURL: _baseUrlController.text.trim(),
              enabled: true,
            ),
          );
      ref.invalidate(liveModeEnginesProvider);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(L10n.t('live_mode_save_success'))),
        );
      }
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
      if (mounted) setState(() => _saving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    final isCustom = widget.engineType == 'custom';
    final isElevenLabs = widget.engineType == 'elevenlabs';

    final enginesAsync = ref.watch(liveModeEnginesProvider);
    enginesAsync.whenData((engines) {
      final existing = engines.where((e) => e.type == widget.engineType);
      if (existing.isNotEmpty) {
        _initFrom(existing.first);
      } else {
        _initializedFromExisting = true;
      }
    });

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
            L10n.t('live_mode_engine_config_title'),
            style: TextStyle(
              fontSize: 13,
              fontWeight: FontWeight.w600,
              color: theme.textMain,
            ),
          ),
          const SizedBox(height: 12),
          if (!isCustom) ...[
            TextField(
              controller: _apiKeyController,
              obscureText: true,
              style: TextStyle(fontSize: 13, color: theme.textMain),
              decoration: InputDecoration(
                labelText: L10n.t('live_mode_api_key_label'),
                hintText: L10n.t('live_mode_api_key_hint'),
                isDense: true,
                border: const OutlineInputBorder(),
              ),
            ),
            const SizedBox(height: 10),
          ],
          if (isCustom) ...[
            TextField(
              controller: _baseUrlController,
              style: TextStyle(fontSize: 13, color: theme.textMain),
              decoration: InputDecoration(
                labelText: L10n.t('live_mode_base_url_label'),
                hintText: L10n.t('live_mode_base_url_hint'),
                isDense: true,
                border: const OutlineInputBorder(),
              ),
            ),
            const SizedBox(height: 10),
            TextField(
              controller: _apiKeyController,
              obscureText: true,
              style: TextStyle(fontSize: 13, color: theme.textMain),
              decoration: InputDecoration(
                labelText: L10n.t('live_mode_api_key_label'),
                hintText: L10n.t('live_mode_api_key_hint'),
                isDense: true,
                border: const OutlineInputBorder(),
              ),
            ),
            const SizedBox(height: 10),
          ],
          if (_fetchedModels != null && _fetchedModels!.isNotEmpty) ...[
            DropdownButtonFormField<String>(
              initialValue: _fetchedModels!.any((m) => m.id == _modelController.text)
                  ? _modelController.text
                  : _fetchedModels!.first.id,
              decoration: InputDecoration(
                labelText: L10n.t('live_mode_model_label'),
                isDense: true,
                border: const OutlineInputBorder(),
              ),
              items: _fetchedModels!
                  .map((m) => DropdownMenuItem(
                        value: m.id,
                        child: Text(m.displayName.isNotEmpty ? m.displayName : m.id),
                      ))
                  .toList(),
              onChanged: (v) {
                if (v != null) setState(() => _modelController.text = v);
              },
            ),
          ] else ...[
            TextField(
              controller: _modelController,
              style: TextStyle(fontSize: 13, color: theme.textMain),
              decoration: InputDecoration(
                labelText: L10n.t('live_mode_model_label'),
                helperText: L10n.t('live_mode_model_hint_manual'),
                isDense: true,
                border: const OutlineInputBorder(),
              ),
            ),
          ],
          if (!isCustom) ...[
            const SizedBox(height: 8),
            Row(
              children: [
                OutlinedButton(
                  onPressed: _fetchingModels ? null : _fetchModels,
                  child: Text(
                    _fetchingModels
                        ? L10n.t('live_mode_fetching_models')
                        : L10n.t('live_mode_fetch_models_button'),
                  ),
                ),
                if (_fetchingModels) ...[
                  const SizedBox(width: 12),
                  const SizedBox(
                    width: 16,
                    height: 16,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  ),
                ],
              ],
            ),
            if (!_fetchingModels &&
                _fetchedModels != null &&
                _fetchedModels!.isEmpty) ...[
              const SizedBox(height: 6),
              Text(
                L10n.t('live_mode_no_models_found'),
                style: TextStyle(fontSize: 12, color: MemoTheme.warningOrange),
              ),
            ],
            if (_fetchError != null) ...[
              const SizedBox(height: 6),
              Text(
                '${L10n.t('error')}: $_fetchError',
                style: TextStyle(fontSize: 12, color: MemoTheme.red),
              ),
            ],
          ],
          if (isElevenLabs) ...[
            const SizedBox(height: 10),
            TextField(
              controller: _voiceController,
              style: TextStyle(fontSize: 13, color: theme.textMain),
              decoration: InputDecoration(
                labelText: L10n.t('live_mode_voice_label'),
                isDense: true,
                border: const OutlineInputBorder(),
              ),
            ),
          ] else if (_nativeReasoningEngines.contains(widget.engineType)) ...[
            const SizedBox(height: 10),
            () {
              final voices = widget.engineType == 'google_live' ? _googleLiveVoices : _openaiRealtimeVoices;
              return DropdownButtonFormField<String>(
                initialValue: voices.contains(_voiceController.text) ? _voiceController.text : '',
                decoration: InputDecoration(
                  labelText: L10n.t('live_mode_voice_label'),
                  isDense: true,
                  border: const OutlineInputBorder(),
                ),
                items: [
                  DropdownMenuItem(value: '', child: Text(L10n.t('live_mode_voice_default_option'))),
                  ...voices.map((v) => DropdownMenuItem(value: v, child: Text(v))),
                ],
                onChanged: (v) => setState(() => _voiceController.text = v ?? ''),
              );
            }(),
          ],
          const SizedBox(height: 12),
          Row(
            children: [
              FilledButton(
                onPressed: _saving ? null : _save,
                child: Text(L10n.t('live_mode_save_button')),
              ),
              if (_saving) ...[
                const SizedBox(width: 12),
                const SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(strokeWidth: 2),
                ),
              ],
            ],
          ),
        ],
      ),
    );
  }
}

/// WorkMode (delegate/standalone) + AgentPermissionPolicy pickers — only
/// meaningful for the two native-reasoning engines (see
/// LiveModeConfig.workMode's doc comment).
class _WorkModeAndPermissionSection extends StatelessWidget {
  const _WorkModeAndPermissionSection({
    required this.cfg,
    required this.busy,
    required this.onWorkModeChanged,
    required this.onPermissionPolicyChanged,
    required this.onBargeInSensitivityChanged,
  });

  final LiveModeConfig cfg;
  final bool busy;
  final ValueChanged<String> onWorkModeChanged;
  final ValueChanged<String> onPermissionPolicyChanged;
  final ValueChanged<String> onBargeInSensitivityChanged;

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
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
            L10n.t('live_mode_work_mode_label'),
            style: TextStyle(
              fontSize: 13,
              fontWeight: FontWeight.w600,
              color: theme.textMain,
            ),
          ),
          const SizedBox(height: 8),
          DropdownButtonFormField<String>(
            initialValue: cfg.workMode,
            decoration: const InputDecoration(
              isDense: true,
              border: OutlineInputBorder(),
            ),
            items: [
              DropdownMenuItem(
                value: 'delegate',
                child: Text(L10n.t('live_mode_work_mode_delegate')),
              ),
              DropdownMenuItem(
                value: 'standalone',
                child: Text(L10n.t('live_mode_work_mode_standalone')),
              ),
            ],
            onChanged: busy
                ? null
                : (v) {
                    if (v != null) onWorkModeChanged(v);
                  },
          ),
          if (cfg.workMode == 'standalone') ...[
            const SizedBox(height: 8),
            Text(
              L10n.t('live_mode_work_mode_standalone_warning'),
              style: TextStyle(
                fontSize: 12,
                height: 1.4,
                color: MemoTheme.warningOrange,
              ),
            ),
          ],
          const SizedBox(height: 16),
          Text(
            L10n.t('live_mode_permission_policy_label'),
            style: TextStyle(
              fontSize: 13,
              fontWeight: FontWeight.w600,
              color: theme.textMain,
            ),
          ),
          const SizedBox(height: 8),
          DropdownButtonFormField<String>(
            initialValue: cfg.agentPermissionPolicy,
            decoration: const InputDecoration(
              isDense: true,
              border: OutlineInputBorder(),
            ),
            items: [
              DropdownMenuItem(
                value: 'voice_prompt',
                child: Text(L10n.t('live_mode_permission_policy_voice_prompt')),
              ),
              DropdownMenuItem(
                value: 'auto_allow_once',
                child: Text(L10n.t('live_mode_permission_policy_auto_allow')),
              ),
            ],
            onChanged: busy
                ? null
                : (v) {
                    if (v != null) onPermissionPolicyChanged(v);
                  },
          ),
          const SizedBox(height: 16),
          Text(
            L10n.t('live_mode_barge_in_label'),
            style: TextStyle(
              fontSize: 13,
              fontWeight: FontWeight.w600,
              color: theme.textMain,
            ),
          ),
          const SizedBox(height: 8),
          DropdownButtonFormField<String>(
            initialValue: cfg.bargeInSensitivity == 'low' ? 'low' : 'high',
            decoration: const InputDecoration(
              isDense: true,
              border: OutlineInputBorder(),
            ),
            items: [
              DropdownMenuItem(
                value: 'high',
                child: Text(L10n.t('live_mode_barge_in_high')),
              ),
              DropdownMenuItem(
                value: 'low',
                child: Text(L10n.t('live_mode_barge_in_low')),
              ),
            ],
            onChanged: busy
                ? null
                : (v) {
                    if (v != null) onBargeInSensitivityChanged(v);
                  },
          ),
          const SizedBox(height: 6),
          Text(
            L10n.t('live_mode_barge_in_desc'),
            style: TextStyle(fontSize: 12, height: 1.4, color: theme.textDim),
          ),
        ],
      ),
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
