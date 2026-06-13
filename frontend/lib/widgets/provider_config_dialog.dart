import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/theme.dart';
import '../models/provider_config.dart';
import '../providers/chat_provider.dart';
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

  Future<void> _openModelBrowser() async {
    final apiKey = _apiKeyCtrl.text.trim();
    if (apiKey.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Önce API Key girin')),
      );
      return;
    }

    final api = ref.read(apiClientProvider);

    Map<String, dynamic> result;
    try {
      result = await api.fetchOpenRouterModels(apiKey);
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Modeller alınamadı: $e')),
        );
      }
      return;
    }

    if (result['status'] != 'ok') {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('❌ ${result['error'] ?? 'Modeller alınamadı'}')),
        );
      }
      return;
    }

    final models = (result['models'] as List).cast<Map<String, dynamic>>();
    if (!mounted) return;

    final selected = await showDialog<String>(
      context: context,
      builder: (_) => _ModelBrowserDialog(models: models),
    );

    if (selected != null) {
      _modelCtrl.text = selected;
    }
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
    if (!mounted) return;
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
                Row(
                  children: [
                    Expanded(
                      child: TextField(
                        controller: _modelCtrl,
                        decoration: const InputDecoration(
                          labelText: 'Model',
                          border: OutlineInputBorder(),
                        ),
                      ),
                    ),
                    if (_type == 'openrouter') ...[
                      const SizedBox(width: 8),
                      TextButton.icon(
                        onPressed: _openModelBrowser,
                        icon: const Icon(Icons.search, size: 18),
                        label: const Text('Models'),
                        style: TextButton.styleFrom(
                          padding: const EdgeInsets.symmetric(horizontal: 12),
                        ),
                      ),
                    ],
                  ],
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
                        color: _testResult! ? MemoTheme.green : MemoTheme.red,
                      ),
                      const SizedBox(width: 4),
                      Text(
                        _testResult! ? 'Connected' : 'Failed',
                        style: TextStyle(
                          color: _testResult! ? MemoTheme.green : MemoTheme.red,
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

// ─── OpenRouter Model Browser Dialog ───────────────────────────

class _ModelBrowserDialog extends StatefulWidget {
  final List<Map<String, dynamic>> models;
  const _ModelBrowserDialog({required this.models});

  @override
  State<_ModelBrowserDialog> createState() => _ModelBrowserDialogState();
}

class _ModelBrowserDialogState extends State<_ModelBrowserDialog> {
  late List<Map<String, dynamic>> _filtered;
  final _searchCtrl = TextEditingController();

  @override
  void initState() {
    super.initState();
    _filtered = List.from(widget.models);
    _filtered.sort((a, b) {
      final af = (a['is_free'] as bool?) ?? false;
      final bf = (b['is_free'] as bool?) ?? false;
      if (af != bf) return af ? -1 : 1;
      return ((a['name'] as String?) ?? '').compareTo((b['name'] as String?) ?? '');
    });
  }

  @override
  void dispose() {
    _searchCtrl.dispose();
    super.dispose();
  }

  void _filter(String q) {
    final query = q.toLowerCase();
    setState(() {
      if (query.isEmpty) {
        _filtered = List.from(widget.models);
      } else {
        _filtered = widget.models.where((m) {
          final id = ((m['id'] as String?) ?? '').toLowerCase();
          final name = ((m['name'] as String?) ?? '').toLowerCase();
          return id.contains(query) || name.contains(query);
        }).toList();
      }
      _filtered.sort((a, b) {
        final af = (a['is_free'] as bool?) ?? false;
        final bf = (b['is_free'] as bool?) ?? false;
        if (af != bf) return af ? -1 : 1;
        return ((a['name'] as String?) ?? '').compareTo((b['name'] as String?) ?? '');
      });
    });
  }

  String _priceStr(double p) {
    if (p == 0) return 'Ücretsiz';
    if (p < 0.000001) return '\$${p.toStringAsExponential(1)}/tkn';
    return '\$${p.toStringAsFixed(7)}/tkn';
  }

  String _contextStr(int? ctx) {
    if (ctx == null || ctx == 0) return '';
    if (ctx >= 1000000) return '${(ctx / 1000).toStringAsFixed(0)}K';
    if (ctx >= 1000) return '${(ctx ~/ 1000)}K';
    return '$ctx';
  }

  @override
  Widget build(BuildContext context) {
    return Dialog(
      insetPadding: const EdgeInsets.symmetric(horizontal: 32, vertical: 24),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(
            padding: const EdgeInsets.fromLTRB(20, 16, 20, 8),
            child: Row(
              children: [
                const Icon(Icons.model_training, size: 20),
                const SizedBox(width: 8),
                const Expanded(
                  child: Text(
                    'OpenRouter Modelleri',
                    style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
                  ),
                ),
                Text(
                  '${widget.models.length} model',
                  style: TextStyle(fontSize: 12, color: Colors.grey[500]),
                ),
              ],
            ),
          ),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 20),
            child: TextField(
              controller: _searchCtrl,
              decoration: InputDecoration(
                hintText: 'Model ara...',
                prefixIcon: const Icon(Icons.search, size: 20),
                isDense: true,
                contentPadding: const EdgeInsets.symmetric(vertical: 8),
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(8),
                ),
              ),
              onChanged: _filter,
            ),
          ),
          const SizedBox(height: 8),
          Flexible(
            child: ListView.builder(
              shrinkWrap: true,
              itemCount: _filtered.length,
              padding: const EdgeInsets.symmetric(horizontal: 12),
              itemBuilder: (ctx, i) {
                final m = _filtered[i];
                final id = (m['id'] as String?) ?? '';
                final name = (m['name'] as String?) ?? id;
                final isFree = (m['is_free'] as bool?) ?? false;
                final promptPrice = (m['prompt_price'] as num?)?.toDouble() ?? 0;
                final ctxLen = m['context_length'] as int?;

                return Card(
                  margin: const EdgeInsets.only(bottom: 4),
                  child: ListTile(
                    dense: true,
                    leading: Icon(
                      isFree ? Icons.check_circle : Icons.monetization_on,
                      size: 18,
                      color: isFree ? MemoTheme.green : MemoTheme.warningOrange,
                    ),
                    title: Text(
                      id,
                      style: const TextStyle(
                        fontSize: 13,
                        fontWeight: FontWeight.w500,
                        fontFamily: 'monospace',
                      ),
                    ),
                    subtitle: Text(
                      [
                        if (name != id) name,
                        if (ctxLen != null && ctxLen > 0) _contextStr(ctxLen),
                        _priceStr(promptPrice),
                      ].join(' · '),
                      style: TextStyle(fontSize: 11, color: Colors.grey[500]),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                    trailing: const Icon(Icons.arrow_forward_ios, size: 14),
                    onTap: () => Navigator.of(context).pop(id),
                  ),
                );
              },
            ),
          ),
          Container(
            padding: const EdgeInsets.fromLTRB(20, 8, 20, 12),
            child: Row(
              children: [
                Text(
                  '🟢 Ücretsiz · 🟡 Ücretli',
                  style: TextStyle(fontSize: 11, color: Colors.grey[500]),
                ),
                const Spacer(),
                TextButton(
                  onPressed: () => Navigator.of(context).pop(),
                  child: const Text('İptal'),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
