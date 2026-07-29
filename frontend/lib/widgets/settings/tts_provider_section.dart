import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/l10n.dart';
import '../../core/theme.dart';
import '../../models/tts_provider_config.dart';
import '../../providers/chat_provider.dart';

/// Settings → Beta Features → Voice Response Providers (Faz 2.5).
///
/// Lets the user add/edit/delete/test external TTS providers (currently
/// only OpenAI has a real backend implementation — see
/// TTSProviderDefaults.implementedTypes). If any provider here is enabled,
/// SynthesizeSpeech (internal/app/tts.go) tries it before falling back to
/// the local Piper engine tested by the sibling _LiveModeVoiceTest widget —
/// this section doesn't replace that test, it configures an earlier tier.
class TTSProviderSection extends ConsumerStatefulWidget {
  const TTSProviderSection({super.key});

  @override
  ConsumerState<TTSProviderSection> createState() => _TTSProviderSectionState();
}

class _TTSProviderSectionState extends ConsumerState<TTSProviderSection> {
  List<TTSProviderConfig> _providers = [];
  bool _loading = true;
  String? _loadError;
  bool _showAddForm = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _loadError = null;
    });
    try {
      final providers = await ref.read(apiClientProvider).getTTSProviders();
      if (!mounted) return;
      setState(() {
        _providers = providers;
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loadError = '$e';
        _loading = false;
      });
    }
  }

  Future<void> _delete(TTSProviderConfig cfg) async {
    try {
      await ref.read(apiClientProvider).deleteTTSProvider(cfg.type, name: cfg.name);
      await _load();
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(L10n.t('tts_provider_delete_failed', {'err': '$e'}))),
      );
    }
  }

  Future<void> _toggleEnabled(TTSProviderConfig cfg, bool enabled) async {
    try {
      await ref.read(apiClientProvider).updateTTSProvider(cfg.copyWith(enabled: enabled));
      await _load();
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(L10n.t('tts_provider_save_failed', {'err': '$e'}))),
      );
    }
  }

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
            L10n.t('tts_providers_title'),
            style: TextStyle(fontSize: 13, fontWeight: FontWeight.w600, color: theme.textMain),
          ),
          const SizedBox(height: 4),
          Text(
            L10n.t('tts_providers_desc'),
            style: TextStyle(fontSize: 12, height: 1.4, color: theme.textDim),
          ),
          const SizedBox(height: 10),
          if (_loading)
            const Padding(
              padding: EdgeInsets.symmetric(vertical: 8),
              child: SizedBox(
                width: 16,
                height: 16,
                child: CircularProgressIndicator(strokeWidth: 2),
              ),
            )
          else if (_loadError != null)
            Text(_loadError!, style: TextStyle(fontSize: 12, color: MemoTheme.red))
          else ...[
            if (_providers.isEmpty)
              Text(
                L10n.t('tts_providers_empty'),
                style: TextStyle(fontSize: 12, color: theme.textDim, fontStyle: FontStyle.italic),
              )
            else
              ..._providers.map((cfg) => _ProviderRow(
                    cfg: cfg,
                    onDelete: () => _delete(cfg),
                    onToggle: (v) => _toggleEnabled(cfg, v),
                  )),
            const SizedBox(height: 10),
            if (!_showAddForm)
              OutlinedButton.icon(
                onPressed: () => setState(() => _showAddForm = true),
                icon: const Icon(Icons.add, size: 16),
                label: Text(L10n.t('tts_providers_add')),
              )
            else
              _AddProviderForm(
                onDone: () {
                  setState(() => _showAddForm = false);
                  _load();
                },
                onCancel: () => setState(() => _showAddForm = false),
              ),
          ],
        ],
      ),
    );
  }
}

class _ProviderRow extends StatelessWidget {
  final TTSProviderConfig cfg;
  final VoidCallback onDelete;
  final ValueChanged<bool> onToggle;

  const _ProviderRow({required this.cfg, required this.onDelete, required this.onToggle});

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  '${cfg.name} (${TTSProviderDefaults.displayNames[cfg.type] ?? cfg.type})',
                  style: TextStyle(fontSize: 13, color: theme.textMain, fontWeight: FontWeight.w600),
                ),
                Text(
                  '${L10n.t('tts_provider_voice')}: ${cfg.voice} · ${L10n.t('tts_provider_priority')}: ${cfg.priority}',
                  style: TextStyle(fontSize: 11, color: theme.textDim),
                ),
              ],
            ),
          ),
          Switch(value: cfg.enabled, onChanged: onToggle, activeThumbColor: MemoTheme.accent),
          IconButton(
            icon: Icon(Icons.delete_outline, size: 18, color: theme.textDim),
            onPressed: onDelete,
            tooltip: L10n.t('tts_provider_delete'),
          ),
        ],
      ),
    );
  }
}

class _AddProviderForm extends ConsumerStatefulWidget {
  final VoidCallback onDone;
  final VoidCallback onCancel;

  const _AddProviderForm({required this.onDone, required this.onCancel});

  @override
  ConsumerState<_AddProviderForm> createState() => _AddProviderFormState();
}

class _AddProviderFormState extends ConsumerState<_AddProviderForm> {
  final _nameCtrl = TextEditingController();
  final _apiKeyCtrl = TextEditingController();
  final _voiceCtrl = TextEditingController(text: 'alloy');
  final _priorityCtrl = TextEditingController(text: '0');
  String _type = TTSProviderDefaults.implementedTypes.first;
  bool _saving = false;
  bool _testing = false;
  String? _status;
  bool _statusIsError = false;

  @override
  void dispose() {
    _nameCtrl.dispose();
    _apiKeyCtrl.dispose();
    _voiceCtrl.dispose();
    _priorityCtrl.dispose();
    super.dispose();
  }

  TTSProviderConfig? _buildConfig() {
    final name = _nameCtrl.text.trim();
    final apiKey = _apiKeyCtrl.text.trim();
    final voice = _voiceCtrl.text.trim();
    if (name.isEmpty || apiKey.isEmpty || voice.isEmpty) {
      setState(() {
        _status = L10n.t('tts_provider_validation_error');
        _statusIsError = true;
      });
      return null;
    }
    return TTSProviderConfig(
      type: _type,
      name: name,
      apiKey: apiKey,
      voice: voice,
      enabled: true,
      priority: int.tryParse(_priorityCtrl.text.trim()) ?? 0,
    );
  }

  Future<void> _test() async {
    final cfg = _buildConfig();
    if (cfg == null) return;
    setState(() {
      _testing = true;
      _status = null;
    });
    try {
      final result = await ref.read(apiClientProvider).testTTSProvider(cfg);
      final connected = result['connected'] as bool? ?? false;
      if (!mounted) return;
      setState(() {
        _testing = false;
        _statusIsError = !connected;
        _status = connected
            ? L10n.t('tts_provider_test_success')
            : L10n.t('tts_provider_test_failed', {'err': '${result['error'] ?? ''}'});
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _testing = false;
        _statusIsError = true;
        _status = L10n.t('tts_provider_test_failed', {'err': '$e'});
      });
    }
  }

  Future<void> _save() async {
    final cfg = _buildConfig();
    if (cfg == null) return;
    setState(() {
      _saving = true;
      _status = null;
    });
    try {
      await ref.read(apiClientProvider).updateTTSProvider(cfg);
      if (!mounted) return;
      widget.onDone();
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _saving = false;
        _statusIsError = true;
        _status = L10n.t('tts_provider_save_failed', {'err': '$e'});
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    final busy = _saving || _testing;
    return Container(
      margin: const EdgeInsets.only(top: 4),
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: theme.bgApp,
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: theme.borderSoft),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          DropdownButton<String>(
            value: _type,
            isDense: true,
            items: TTSProviderDefaults.implementedTypes
                .map((t) => DropdownMenuItem(
                      value: t,
                      child: Text(TTSProviderDefaults.displayNames[t] ?? t),
                    ))
                .toList(),
            onChanged: busy ? null : (v) => setState(() => _type = v ?? _type),
          ),
          const SizedBox(height: 8),
          TextField(
            controller: _nameCtrl,
            enabled: !busy,
            decoration: InputDecoration(
              labelText: L10n.t('tts_provider_name'),
              hintText: L10n.t('tts_provider_name_hint'),
              isDense: true,
              border: const OutlineInputBorder(),
            ),
          ),
          const SizedBox(height: 8),
          TextField(
            controller: _apiKeyCtrl,
            enabled: !busy,
            obscureText: true,
            decoration: InputDecoration(
              labelText: L10n.t('tts_provider_api_key'),
              isDense: true,
              border: const OutlineInputBorder(),
            ),
          ),
          const SizedBox(height: 8),
          TextField(
            controller: _voiceCtrl,
            enabled: !busy,
            decoration: InputDecoration(
              labelText: L10n.t('tts_provider_voice'),
              hintText: L10n.t('tts_provider_voice_hint'),
              isDense: true,
              border: const OutlineInputBorder(),
            ),
          ),
          const SizedBox(height: 8),
          TextField(
            controller: _priorityCtrl,
            enabled: !busy,
            keyboardType: TextInputType.number,
            decoration: InputDecoration(
              labelText: L10n.t('tts_provider_priority'),
              isDense: true,
              border: const OutlineInputBorder(),
            ),
          ),
          const SizedBox(height: 10),
          Row(
            children: [
              FilledButton(
                onPressed: busy ? null : _save,
                child: Text(_saving ? L10n.t('tts_provider_testing') : L10n.t('tts_provider_save')),
              ),
              const SizedBox(width: 8),
              OutlinedButton(
                onPressed: busy ? null : _test,
                child: Text(_testing ? L10n.t('tts_provider_testing') : L10n.t('tts_provider_test')),
              ),
              const SizedBox(width: 8),
              TextButton(
                onPressed: busy ? null : widget.onCancel,
                child: Text(L10n.t('cancel')),
              ),
            ],
          ),
          if (_status != null) ...[
            const SizedBox(height: 8),
            Text(
              _status!,
              style: TextStyle(fontSize: 12, color: _statusIsError ? MemoTheme.red : MemoTheme.accent),
            ),
          ],
        ],
      ),
    );
  }
}
