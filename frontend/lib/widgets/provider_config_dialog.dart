import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:url_launcher/url_launcher.dart';

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
  late TextEditingController _contextCtrl;
  late TextEditingController _priorityCtrl;
  late bool _enabled;
  bool _testing = false;
  bool? _testResult;
  bool _isSaving = false;
  bool _showAdvanced = false;
  bool _obscureKey = true;

  final _types = [
    'openai',
    'gemini',
    'grok',
    'groq',
    'claude',
    'openrouter',
    'ollama',
    'custom',
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
    _contextCtrl = TextEditingController(
      text: (existing?.contextTokens ?? 0) > 0
          ? '${existing?.contextTokens ?? 0}'
          : '',
    );
    _priorityCtrl = TextEditingController(
      text: (existing?.priority ?? 0) > 0
          ? '${existing?.priority ?? 0}'
          : '',
    );
    // New providers default to ENABLED. Previously they were added disabled,
    // so users would add a provider, see nothing work, and not know why.
    _enabled = existing?.enabled ?? true;

    // A test result only describes the field values at the moment it ran.
    // Without this, editing the API key/URL/model after a successful test
    // left the stale "✅ Bağlandı" indicator showing — looking like *these*
    // (now different, untested) values were confirmed working.
    _apiKeyCtrl.addListener(_invalidateTestResult);
    _baseUrlCtrl.addListener(_invalidateTestResult);
    _modelCtrl.addListener(_invalidateTestResult);
  }

  void _invalidateTestResult() {
    if (_testResult != null) setState(() => _testResult = null);
  }

  @override
  void dispose() {
    _nameCtrl.dispose();
    _apiKeyCtrl.dispose();
    _baseUrlCtrl.dispose();
    _modelCtrl.dispose();
    _contextCtrl.dispose();
    _priorityCtrl.dispose();
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

    final rawModels = result['models'];
    final models = (rawModels is List) ? rawModels.cast<Map<String, dynamic>>() : <Map<String, dynamic>>[];
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
      name: _nameCtrl.text.trim(),
      apiKey: _apiKeyCtrl.text.trim(),
      baseUrl: _baseUrlCtrl.text.trim(),
      model: _modelCtrl.text.trim(),
    );
    final result = await ref.read(providerListProvider.notifier).testProvider(config);
    if (!mounted) return;
    setState(() {
      _testing = false;
      _testResult = result['connected'] == true;
    });
  }

  Future<void> _openKeyUrl() async {
    final url = ProviderDefaults.apiKeyUrls[_type];
    if (url == null) return;
    final uri = Uri.parse(url);
    if (!await launchUrl(uri, mode: LaunchMode.externalApplication)) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Bağlantı açılamadı: $url')),
        );
      }
    }
  }

  /// Returns [base], or "[base] 2", "[base] 3"… so it doesn't collide with an
  /// existing provider name (providers are keyed by name on the backend).
  String _uniqueName(String base, {String? exclude}) {
    final existingNames = (ref.read(providerListProvider).valueOrNull ?? [])
        .map((p) => p.name)
        .where((n) => n != exclude)
        .toSet();
    if (!existingNames.contains(base)) return base;
    for (var i = 2; i < 100; i++) {
      final candidate = '$base $i';
      if (!existingNames.contains(candidate)) return candidate;
    }
    return '$base ${DateTime.now().millisecondsSinceEpoch}';
  }

  Future<void> _save() async {
    if (_isSaving) return;

    // Guard the one mistake that silently breaks everything: saving a
    // key-requiring provider with an empty API key.
    if (ProviderDefaults.needsApiKey(_type) && _apiKeyCtrl.text.trim().isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('API anahtarını gir (yoksa "Anahtar al" ile edinebilirsin)')),
      );
      return;
    }

    // Custom endpoints have no default URL — without one, requests go nowhere.
    if (ProviderDefaults.needsBaseUrl(_type) && _baseUrlCtrl.text.trim().isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Özel sağlayıcı için Base URL gerekli (örn. https://host/v1)')),
      );
      return;
    }
    if (_modelCtrl.text.trim().isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Model adı gir')),
      );
      return;
    }

    _isSaving = true;
    try {
      final existing = widget.existing;
      // Providers are stored keyed by Name. For a NEW provider, ensure the name
      // is unique so a second provider of the same type doesn't silently
      // overwrite the first.
      final desiredName = _nameCtrl.text.trim().isEmpty
          ? (ProviderDefaults.displayNames[_type] ?? _type)
          : _nameCtrl.text.trim();
      // Ensure a unique name (providers are keyed by name on the backend).
      // When editing, exclude this provider's own current name so renaming it
      // doesn't get suffixed — but renaming onto ANOTHER provider's name still
      // gets disambiguated instead of silently overwriting it.
      final finalName = _uniqueName(desiredName, exclude: existing?.name);
      if (finalName != desiredName && mounted) {
        // _uniqueName silently disambiguated the name the user actually
        // typed — tell them, so "Claude" quietly becoming "Claude 2" in the
        // provider list isn't a surprise they have to notice on their own.
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('"$desiredName" zaten var, "$finalName" olarak kaydedildi')),
        );
      }

      // Trim every free-text field: a stray space/tab pasted into a model id or
      // key silently breaks requests (e.g. an invalid model that stalls the API).
      final config = ProviderConfig(
        type: _type,
        name: finalName,
        apiKey: _apiKeyCtrl.text.trim(),
        baseUrl: _baseUrlCtrl.text.trim(),
        model: _modelCtrl.text.trim(),
        enabled: _enabled,
        contextTokens: int.tryParse(_contextCtrl.text.trim()) ?? 0,
        priority: int.tryParse(_priorityCtrl.text.trim()) ?? existing?.priority ?? 0,
        // Preserve advanced fields the dialog doesn't edit.
        temperature: existing?.temperature ?? 0.7,
        topP: existing?.topP ?? 0.9,
        maxTokens: existing?.maxTokens ?? 0,
      );

      await ref.read(providerListProvider.notifier).updateProvider(config);
      if (mounted) Navigator.of(context).pop(true);
    } finally {
      _isSaving = false;
    }
  }

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final isEditing = widget.existing != null;
    final needsKey = ProviderDefaults.needsApiKey(_type);
    final hint = ProviderDefaults.hints[_type];

    return Dialog(
      insetPadding: const EdgeInsets.symmetric(horizontal: 24, vertical: 40),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 480),
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                // ── Header ──
                Row(
                  children: [
                    Container(
                      width: 40,
                      height: 40,
                      decoration: BoxDecoration(
                        color: MemoTheme.accentMuted,
                        borderRadius: BorderRadius.circular(10),
                      ),
                      child: Center(child: providerLogoWidget(_type, size: 24)),
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            isEditing
                                ? '${widget.existing!.name} ayarları'
                                : 'API sağlayıcı ekle',
                            style: Theme.of(context).textTheme.titleLarge?.copyWith(
                                  fontWeight: FontWeight.bold,
                                ),
                          ),
                          Text(
                            isEditing
                                ? 'Anahtarını ve modelini güncelle'
                                : 'Sağlayıcını seç, anahtarını yapıştır, bitti.',
                            style: TextStyle(fontSize: 12, color: c.textDim),
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 24),

                // ── Step 1: provider picker ──
                DropdownButtonFormField<String>(
                  initialValue: _type,
                  decoration: InputDecoration(
                    labelText: '1. Sağlayıcı',
                    border: const OutlineInputBorder(),
                    helperText: hint,
                    helperMaxLines: 2,
                  ),
                  items: _types.map((t) {
                    return DropdownMenuItem(
                      value: t,
                      child: Text(
                        '${providerIcon(t)} ${ProviderDefaults.displayNames[t] ?? t}',
                      ),
                    );
                  }).toList(),
                  onChanged: isEditing ? null : _onTypeChanged,
                ),
                const SizedBox(height: 16),

                // ── Step 2: API key (the part that matters) ──
                if (needsKey || _type == 'custom') ...[
                  TextField(
                    controller: _apiKeyCtrl,
                    obscureText: _obscureKey,
                    autofocus: !isEditing && needsKey,
                    decoration: InputDecoration(
                      labelText: _type == 'custom'
                          ? 'API anahtarı (opsiyonel)'
                          : '2. API anahtarı',
                      border: const OutlineInputBorder(),
                      helperText: _type == 'custom'
                          ? 'Endpoint gerektiriyorsa gir — şifrelenerek saklanır'
                          : 'Şifrelenerek cihazında saklanır',
                      prefixIcon: const Icon(Icons.key, size: 20),
                      suffixIcon: IconButton(
                        tooltip: _obscureKey ? 'Göster' : 'Gizle',
                        icon: Icon(
                          _obscureKey ? Icons.visibility : Icons.visibility_off,
                          size: 20,
                        ),
                        onPressed: () => setState(() => _obscureKey = !_obscureKey),
                      ),
                    ),
                  ),
                  if (ProviderDefaults.apiKeyUrls[_type] != null)
                    Align(
                      alignment: Alignment.centerLeft,
                      child: TextButton.icon(
                        onPressed: _openKeyUrl,
                        icon: const Icon(Icons.open_in_new, size: 16),
                        label: Text(
                          'Anahtarım yok — ${ProviderDefaults.displayNames[_type]}\'dan al',
                        ),
                        style: TextButton.styleFrom(
                          padding: const EdgeInsets.symmetric(horizontal: 4),
                          visualDensity: VisualDensity.compact,
                        ),
                      ),
                    ),
                  const SizedBox(height: 12),
                ] else ...[
                  Container(
                    padding: const EdgeInsets.all(12),
                    decoration: BoxDecoration(
                      color: MemoTheme.green.withValues(alpha: 0.08),
                      borderRadius: BorderRadius.circular(8),
                    ),
                    child: Row(
                      children: [
                        const Icon(Icons.computer, size: 18, color: MemoTheme.green),
                        const SizedBox(width: 8),
                        Expanded(
                          child: Text(
                            'Yerel sağlayıcı — API anahtarı gerekmez.',
                            style: TextStyle(fontSize: 12, color: c.textSecondary),
                          ),
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(height: 12),
                ],

                // ── Custom endpoint: Base URL + label up front (no defaults) ──
                if (_type == 'custom') ...[
                  TextField(
                    controller: _baseUrlCtrl,
                    decoration: const InputDecoration(
                      labelText: 'Base URL',
                      border: OutlineInputBorder(),
                      helperText: 'OpenAI uyumlu endpoint, örn. https://host/v1',
                      prefixIcon: Icon(Icons.link, size: 20),
                    ),
                  ),
                  const SizedBox(height: 12),
                  TextField(
                    controller: _nameCtrl,
                    decoration: const InputDecoration(
                      labelText: 'Görünen ad',
                      border: OutlineInputBorder(),
                      helperText: 'Listede bunu görürsün — birden fazla için ayırt edici yap',
                    ),
                  ),
                  const SizedBox(height: 12),
                ],

                // ── Model (auto-filled, editable) ──
                Row(
                  children: [
                    Expanded(
                      child: TextField(
                        controller: _modelCtrl,
                        decoration: InputDecoration(
                          labelText: _type == 'custom'
                              ? 'Model'
                              : (needsKey ? '3. Model' : 'Model'),
                          border: const OutlineInputBorder(),
                          helperText: _type == 'custom'
                              ? 'Endpoint\'in beklediği model adı'
                              : 'Varsayılan dolduruldu — değiştirebilirsin',
                        ),
                      ),
                    ),
                    if (_type == 'openrouter') ...[
                      const SizedBox(width: 8),
                      Padding(
                        padding: const EdgeInsets.only(bottom: 20),
                        child: OutlinedButton.icon(
                          onPressed: _openModelBrowser,
                          icon: const Icon(Icons.search, size: 18),
                          label: const Text('Seç'),
                          style: OutlinedButton.styleFrom(
                            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 18),
                          ),
                        ),
                      ),
                    ],
                  ],
                ),
                const SizedBox(height: 16),

                // ── Test connection ──
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
                          : const Icon(Icons.wifi_find, size: 18),
                      label: const Text('Bağlantıyı test et'),
                    ),
                    if (_testResult != null) ...[
                      const SizedBox(width: 12),
                      Icon(
                        _testResult! ? Icons.check_circle : Icons.error,
                        size: 18,
                        color: _testResult! ? MemoTheme.green : MemoTheme.red,
                      ),
                      const SizedBox(width: 4),
                      Text(
                        _testResult! ? 'Bağlandı' : 'Başarısız',
                        style: TextStyle(
                          color: _testResult! ? MemoTheme.green : MemoTheme.red,
                        ),
                      ),
                    ],
                  ],
                ),
                const SizedBox(height: 8),

                // ── Advanced (collapsed) ──
                Theme(
                  data: Theme.of(context).copyWith(dividerColor: Colors.transparent),
                  child: ExpansionTile(
                    tilePadding: EdgeInsets.zero,
                    childrenPadding: const EdgeInsets.only(top: 4, bottom: 8),
                    initiallyExpanded: _showAdvanced,
                    onExpansionChanged: (v) => setState(() => _showAdvanced = v),
                    title: Text(
                      'Gelişmiş ayarlar',
                      style: TextStyle(fontSize: 13, color: c.textDim, fontWeight: FontWeight.w500),
                    ),
                    children: [
                      // For custom providers these two live in the main flow
                      // (above) since they're required, so don't duplicate them.
                      if (_type != 'custom') ...[
                        TextField(
                          controller: _nameCtrl,
                          decoration: const InputDecoration(
                            labelText: 'Görünen ad',
                            border: OutlineInputBorder(),
                            helperText: 'Aynı tipten birden fazla için ayırt edici yap',
                          ),
                        ),
                        const SizedBox(height: 12),
                        TextField(
                          controller: _baseUrlCtrl,
                          decoration: const InputDecoration(
                            labelText: 'Base URL',
                            border: OutlineInputBorder(),
                            helperText: 'Boş = sağlayıcı varsayılanı',
                          ),
                        ),
                        const SizedBox(height: 12),
                      ],
                      TextField(
                        controller: _contextCtrl,
                        keyboardType: TextInputType.number,
                        inputFormatters: [FilteringTextInputFormatter.digitsOnly],
                        decoration: const InputDecoration(
                          labelText: 'Bağlam penceresi (token)',
                          border: OutlineInputBorder(),
                          helperText: 'Boş = varsayılan. Örn. 1000000 = 1M.',
                        ),
                      ),
                      const SizedBox(height: 12),
                      TextField(
                        controller: _priorityCtrl,
                        keyboardType: TextInputType.number,
                        inputFormatters: [FilteringTextInputFormatter.digitsOnly],
                        decoration: const InputDecoration(
                          labelText: 'Öncelik (Priority)',
                          border: OutlineInputBorder(),
                          helperText: 'Yüksek = tercih edilir. Boş = 0.',
                        ),
                      ),
                    ],
                  ),
                ),

                // ── Enable toggle ──
                SwitchListTile(
                  title: const Text('Bu sağlayıcıyı etkinleştir'),
                  subtitle: Text(
                    _enabled ? 'Sohbette kullanılabilir' : 'Kayıtlı ama kapalı',
                    style: TextStyle(fontSize: 12, color: c.textDim),
                  ),
                  value: _enabled,
                  contentPadding: EdgeInsets.zero,
                  onChanged: (v) => setState(() => _enabled = v),
                ),
                const SizedBox(height: 12),

                // ── Actions ──
                Row(
                  mainAxisAlignment: MainAxisAlignment.end,
                  children: [
                    TextButton(
                      onPressed: () => Navigator.of(context).pop(),
                      child: const Text('İptal'),
                    ),
                    const SizedBox(width: 8),
                    FilledButton(
                      onPressed: _isSaving ? null : _save,
                      child: _isSaving
                          ? const SizedBox(
                              width: 16,
                              height: 16,
                              child: CircularProgressIndicator(strokeWidth: 2),
                            )
                          : const Text('Kaydet'),
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
