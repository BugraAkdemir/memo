import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/friendly_error.dart';
import '../../core/l10n.dart';
import '../../core/theme.dart';
import '../../models/stt_provider_config.dart';
import '../../providers/chat_provider.dart';

/// Settings → Live Mode → Speech-to-Text Providers.
///
/// The input-side counterpart to [TTSProviderSection]: add/enable/delete
/// external STT providers (ElevenLabs, or a custom Whisper-compatible
/// endpoint). When one is enabled, transcription tries it before falling
/// back to the local whisper.cpp engine. Backend: /api/stt/providers.
class STTProviderSection extends ConsumerStatefulWidget {
  const STTProviderSection({super.key});

  @override
  ConsumerState<STTProviderSection> createState() => _STTProviderSectionState();
}

class _STTProviderSectionState extends ConsumerState<STTProviderSection> {
  List<STTProviderConfig> _providers = [];
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
      final providers = await ref.read(apiClientProvider).getSTTProviders();
      if (!mounted) return;
      setState(() {
        _providers = providers;
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loadError = FriendlyError.describeGeneric(e);
        _loading = false;
      });
    }
  }

  Future<void> _delete(STTProviderConfig cfg) async {
    try {
      await ref.read(apiClientProvider).deleteSTTProvider(cfg.type, name: cfg.name);
      await _load();
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(
        content: Text(L10n.t('stt_provider_delete_failed',
            {'err': FriendlyError.describeGeneric(e)})),
      ));
    }
  }

  Future<void> _toggleEnabled(STTProviderConfig cfg, bool enabled) async {
    try {
      await ref.read(apiClientProvider).updateSTTProvider(STTProviderConfig(
            type: cfg.type,
            name: cfg.name,
            apiKey: cfg.apiKey,
            baseUrl: cfg.baseUrl,
            enabled: enabled,
            priority: cfg.priority,
          ));
      await _load();
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(
        content: Text(L10n.t('stt_provider_save_failed',
            {'err': FriendlyError.describeGeneric(e)})),
      ));
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
            L10n.t('stt_providers_title'),
            style: TextStyle(
                fontSize: 13, fontWeight: FontWeight.w600, color: theme.textMain),
          ),
          const SizedBox(height: 4),
          Text(
            L10n.t('stt_providers_desc'),
            style: TextStyle(fontSize: 12, height: 1.4, color: theme.textDim),
          ),
          const SizedBox(height: 10),
          if (_loading)
            const Padding(
              padding: EdgeInsets.symmetric(vertical: 8),
              child: SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(strokeWidth: 2)),
            )
          else if (_loadError != null)
            Text(_loadError!,
                style: TextStyle(fontSize: 12, color: MemoTheme.red))
          else ...[
            if (_providers.isEmpty)
              Text(
                L10n.t('stt_providers_empty'),
                style: TextStyle(
                    fontSize: 12,
                    color: theme.textDim,
                    fontStyle: FontStyle.italic),
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
                label: Text(L10n.t('stt_providers_add')),
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
  final STTProviderConfig cfg;
  final VoidCallback onDelete;
  final ValueChanged<bool> onToggle;

  const _ProviderRow(
      {required this.cfg, required this.onDelete, required this.onToggle});

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
                  '${cfg.name} (${STTProviderDefaults.displayNames[cfg.type] ?? cfg.type})',
                  style: TextStyle(
                      fontSize: 13,
                      color: theme.textMain,
                      fontWeight: FontWeight.w600),
                ),
                Text(
                  '${L10n.t('tts_provider_priority')}: ${cfg.priority}',
                  style: TextStyle(fontSize: 11, color: theme.textDim),
                ),
              ],
            ),
          ),
          Switch(
              value: cfg.enabled,
              onChanged: onToggle,
              activeThumbColor: MemoTheme.accent),
          IconButton(
            icon: Icon(Icons.delete_outline, size: 18, color: theme.textDim),
            onPressed: onDelete,
            tooltip: L10n.t('stt_provider_delete'),
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
  final _baseUrlCtrl = TextEditingController();
  final _priorityCtrl = TextEditingController(text: '0');
  String _type = STTProviderDefaults.implementedTypes.first;
  bool _saving = false;
  String? _status;

  bool get _needsBaseUrl => STTProviderDefaults.needsBaseUrl.contains(_type);

  @override
  void dispose() {
    _nameCtrl.dispose();
    _apiKeyCtrl.dispose();
    _baseUrlCtrl.dispose();
    _priorityCtrl.dispose();
    super.dispose();
  }

  Future<void> _save() async {
    final name = _nameCtrl.text.trim();
    final apiKey = _apiKeyCtrl.text.trim();
    final baseUrl = _baseUrlCtrl.text.trim();
    if (name.isEmpty ||
        (!_needsBaseUrl && apiKey.isEmpty) ||
        (_needsBaseUrl && baseUrl.isEmpty)) {
      setState(() => _status = L10n.t('tts_provider_validation_error'));
      return;
    }
    setState(() {
      _saving = true;
      _status = null;
    });
    try {
      await ref.read(apiClientProvider).updateSTTProvider(STTProviderConfig(
            type: _type,
            name: name,
            apiKey: apiKey,
            baseUrl: _needsBaseUrl ? baseUrl : null,
            enabled: true,
            priority: int.tryParse(_priorityCtrl.text.trim()) ?? 0,
          ));
      if (!mounted) return;
      widget.onDone();
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _saving = false;
        _status = L10n.t('stt_provider_save_failed',
            {'err': FriendlyError.describeGeneric(e)});
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
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
            items: STTProviderDefaults.implementedTypes
                .map((t) => DropdownMenuItem(
                      value: t,
                      child: Text(STTProviderDefaults.displayNames[t] ?? t),
                    ))
                .toList(),
            onChanged:
                _saving ? null : (v) => setState(() => _type = v ?? _type),
          ),
          const SizedBox(height: 8),
          TextField(
            controller: _nameCtrl,
            enabled: !_saving,
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
            enabled: !_saving,
            obscureText: true,
            decoration: InputDecoration(
              labelText: L10n.t('tts_provider_api_key'),
              isDense: true,
              border: const OutlineInputBorder(),
            ),
          ),
          if (_needsBaseUrl) ...[
            const SizedBox(height: 8),
            TextField(
              controller: _baseUrlCtrl,
              enabled: !_saving,
              keyboardType: TextInputType.url,
              decoration: InputDecoration(
                labelText: L10n.t('tts_provider_base_url'),
                hintText: 'https://api.example.com/v1',
                isDense: true,
                border: const OutlineInputBorder(),
              ),
            ),
          ],
          const SizedBox(height: 8),
          TextField(
            controller: _priorityCtrl,
            enabled: !_saving,
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
                onPressed: _saving ? null : _save,
                child: Text(_saving
                    ? L10n.t('tts_provider_saving')
                    : L10n.t('tts_provider_save')),
              ),
              const SizedBox(width: 8),
              TextButton(
                onPressed: _saving ? null : widget.onCancel,
                child: Text(L10n.t('cancel')),
              ),
            ],
          ),
          if (_status != null) ...[
            const SizedBox(height: 8),
            Text(_status!,
                style: const TextStyle(fontSize: 12, color: MemoTheme.red)),
          ],
        ],
      ),
    );
  }
}
