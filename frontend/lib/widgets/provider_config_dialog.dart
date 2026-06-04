import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/provider_config.dart';
import '../providers/provider_provider.dart';

class ProviderConfigDialog extends ConsumerStatefulWidget {
  final ProviderConfig? existing;

  const ProviderConfigDialog({super.key, this.existing});

  @override
  ConsumerState<ProviderConfigDialog> createState() =>
      _ProviderConfigDialogState();
}

class _ProviderConfigDialogState
    extends ConsumerState<ProviderConfigDialog> {
  late String _type;
  late TextEditingController _nameCtrl;
  late TextEditingController _apiKeyCtrl;
  late TextEditingController _baseUrlCtrl;
  late TextEditingController _modelCtrl;
  late bool _enabled;
  bool _testing = false;
  bool? _testResult;

  final _types = [
    'openai',
    'gemini',
    'grok',
    'groq',
    'claude',
    'openrouter',
    'ollama',
  ];

  @override
  void initState() {
    super.initState();
    final existing = widget.existing;
    _type = existing?.type ?? 'openai';
    _nameCtrl = TextEditingController(
      text: existing?.name ?? ProviderDefaults.displayNames[_type] ?? _type,
    );
    _apiKeyCtrl = TextEditingController(text: existing?.apiKey ?? '');
    _baseUrlCtrl = TextEditingController(
      text: existing?.baseUrl ?? ProviderDefaults.defaultBaseUrls[_type] ?? '',
    );
    _modelCtrl = TextEditingController(
      text: existing?.model ?? ProviderDefaults.defaultModels[_type] ?? '',
    );
    _enabled = existing?.enabled ?? false;
  }

  @override
  void dispose() {
    _nameCtrl.dispose();
    _apiKeyCtrl.dispose();
    _baseUrlCtrl.dispose();
    _modelCtrl.dispose();
    super.dispose();
  }

  void _onTypeChanged(String? type) {
    if (type == null) return;
    setState(() {
      _type = type;
      _nameCtrl.text = ProviderDefaults.displayNames[type] ?? type;
      if (_baseUrlCtrl.text.isEmpty ||
          ProviderDefaults.defaultBaseUrls.values
              .any((v) => v == _baseUrlCtrl.text)) {
        _baseUrlCtrl.text =
            ProviderDefaults.defaultBaseUrls[type] ?? '';
      }
      if (_modelCtrl.text.isEmpty ||
          ProviderDefaults.defaultModels.values
              .any((v) => v == _modelCtrl.text)) {
        _modelCtrl.text =
            ProviderDefaults.defaultModels[type] ?? '';
      }
    });
  }

  Future<void> _testConnection() async {
    setState(() {
      _testing = true;
      _testResult = null;
    });
    final config = ProviderConfig(
      type: _type,
      name: _nameCtrl.text,
      apiKey: _apiKeyCtrl.text,
      baseUrl: _baseUrlCtrl.text,
      model: _modelCtrl.text,
    );
    final result = await ref.read(providerListProvider.notifier).testProvider(config);
    setState(() {
      _testing = false;
      _testResult = result;
    });
  }

  Future<void> _save() async {
    final config = ProviderConfig(
      type: _type,
      name: _nameCtrl.text,
      apiKey: _apiKeyCtrl.text,
      baseUrl: _baseUrlCtrl.text,
      model: _modelCtrl.text,
      enabled: _enabled,
    );

    await ref.read(providerListProvider.notifier).updateProvider(config);
    if (mounted) Navigator.of(context).pop(true);
  }

  @override
  Widget build(BuildContext context) {
    return Dialog(
      insetPadding: const EdgeInsets.symmetric(horizontal: 24, vertical: 40),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 520),
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  widget.existing != null
                      ? 'Configure ${widget.existing!.name}'
                      : 'Add API Provider',
                  style: Theme.of(context).textTheme.titleLarge,
                ),
                const SizedBox(height: 20),

                // Provider type dropdown
                DropdownButtonFormField<String>(
                  value: _type,
                  decoration: const InputDecoration(
                    labelText: 'Provider',
                    border: OutlineInputBorder(),
                  ),
                  items: _types.map((t) {
                    return DropdownMenuItem(
                      value: t,
                      child: Text(
                        '${providerIcon(t)} ${ProviderDefaults.displayNames[t] ?? t}',
                      ),
                    );
                  }).toList(),
                  onChanged: widget.existing != null ? null : _onTypeChanged,
                ),
                const SizedBox(height: 12),

                // Display name
                TextField(
                  controller: _nameCtrl,
                  decoration: const InputDecoration(
                    labelText: 'Display Name',
                    border: OutlineInputBorder(),
                  ),
                ),
                const SizedBox(height: 12),

                // API Key (masked)
                TextField(
                  controller: _apiKeyCtrl,
                  obscureText: true,
                  decoration: const InputDecoration(
                    labelText: 'API Key',
                    border: OutlineInputBorder(),
                    helperText: 'Stored encrypted',
                  ),
                ),
                const SizedBox(height: 12),

                // Base URL
                TextField(
                  controller: _baseUrlCtrl,
                  decoration: const InputDecoration(
                    labelText: 'Base URL (optional)',
                    border: OutlineInputBorder(),
                    helperText: 'Leave empty for default',
                  ),
                ),
                const SizedBox(height: 12),

                // Model
                TextField(
                  controller: _modelCtrl,
                  decoration: const InputDecoration(
                    labelText: 'Model',
                    border: OutlineInputBorder(),
                  ),
                ),
                const SizedBox(height: 12),

                // Enabled toggle
                SwitchListTile(
                  title: const Text('Enable this provider'),
                  value: _enabled,
                  contentPadding: EdgeInsets.zero,
                  onChanged: (v) => setState(() => _enabled = v),
                ),
                const SizedBox(height: 16),

                // Test connection button
                Row(
                  children: [
                    OutlinedButton.icon(
                      onPressed: _testing ? null : _testConnection,
                      icon: _testing
                          ? const SizedBox(
                              width: 16,
                              height: 16,
                              child: CircularProgressIndicator(strokeWidth: 2),
                            )
                          : const Icon(Icons.wifi_find),
                      label: const Text('Test Connection'),
                    ),
                    if (_testResult != null) ...[
                      const SizedBox(width: 12),
                      Icon(
                        _testResult! ? Icons.check_circle : Icons.error,
                        color: _testResult! ? Colors.green : Colors.red,
                      ),
                      const SizedBox(width: 4),
                      Text(
                        _testResult! ? 'Connected' : 'Failed',
                        style: TextStyle(
                          color: _testResult! ? Colors.green : Colors.red,
                        ),
                      ),
                    ],
                  ],
                ),
                const SizedBox(height: 20),

                // Actions
                Row(
                  mainAxisAlignment: MainAxisAlignment.end,
                  children: [
                    TextButton(
                      onPressed: () => Navigator.of(context).pop(),
                      child: const Text('Cancel'),
                    ),
                    const SizedBox(width: 8),
                    FilledButton(
                      onPressed: _save,
                      child: const Text('Save'),
                    ),
                  ],
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
