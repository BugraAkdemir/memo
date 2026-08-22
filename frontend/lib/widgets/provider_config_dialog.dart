import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:url_launcher/url_launcher.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/provider_config.dart';
import '../providers/chat_provider.dart';
import '../providers/provider_provider.dart';
import '../core/friendly_error.dart';

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
  String? _saveError;
  String _effortLevel = '';
  List<String> _availableEffortLevels = [];
  bool _loadingEffortLevels = false;

  final _types = [
    'openai',
    'gemini',
    'grok',
    'groq',
    'claude',
    'openrouter',
    'opencode-zen',
    'opencode-go',
    'kilo',
    'claude-code-cli',
    'codex-cli',
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

    _effortLevel = existing?.effortLevel ?? '';
    _loadEffortLevels();
  }

  void _invalidateTestResult() {
    if (_testResult != null) setState(() => _testResult = null);
  }

  /// Types whose effort-level discovery depends on the chosen model, not
  /// just the type — mirrors the backend's effortDiscoveredTypes
  /// (handlers_oauth.go). Gates the manual refresh button, since these are
  /// the only types where re-checking after editing the model field
  /// matters.
  static const _modelDependentEffortTypes = {'openrouter', 'claude', 'gemini', 'ollama'};
  bool get _effortLevelsAreModelDependent => _modelDependentEffortTypes.contains(_type);

  /// Refetches which effort labels _type + the current model field actually
  /// accepts — see MemoApiClient.getEffortLevels's doc comment. Called on
  /// init, on type change, and via the manual refresh button next to the
  /// dropdown for the live-discovered types (model text isn't watched live
  /// to avoid a network call per keystroke). Passing model unconditionally
  /// is harmless for types that don't use it — the backend just ignores it.
  Future<void> _loadEffortLevels() async {
    setState(() => _loadingEffortLevels = true);
    List<String> levels;
    try {
      levels = await ref
          .read(apiClientProvider)
          .getEffortLevels(_type, model: _modelCtrl.text.trim());
    } catch (_) {
      levels = [];
    }
    if (!mounted) return;
    setState(() {
      _availableEffortLevels = levels;
      _loadingEffortLevels = false;
      // A level that's no longer in the fresh list (type changed, or this
      // OpenRouter model doesn't support what a previous model did) must
      // not linger as a silently-invalid selection.
      if (_effortLevel.isNotEmpty && !levels.contains(_effortLevel)) {
        _effortLevel = '';
      }
    });
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
    _loadEffortLevels();
  }

  Future<void> _pickProviderType(BuildContext context) async {
    final selected = await showDialog<String>(
      context: context,
      builder: (_) => _ProviderTypePickerDialog(types: _types, current: _type),
    );
    if (selected != null && selected != _type) {
      _onTypeChanged(selected);
    }
  }

  Future<void> _openModelBrowser() async {
    // Kilo's /models endpoint needs no API key at all (kilo.ai/docs/
    // gateway/models-and-providers — public catalog), so it's the one type
    // that can browse before a key is even entered; every other type here
    // still requires one first since their /models calls actually forward
    // it upstream.
    final apiKey = _apiKeyCtrl.text.trim();
    if (apiKey.isEmpty && _type != 'kilo') {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(L10n.t('enter_api_key_first'))),
      );
      return;
    }

    final selected = switch (_type) {
      'openrouter' => await _browseOpenRouterModels(apiKey),
      'kilo' => await _browseKiloModels(),
      _ => await _browseGenericModels(apiKey),
    };

    if (selected != null) {
      _modelCtrl.text = selected;
    }
  }

  Future<String?> _browseOpenRouterModels(String apiKey) async {
    final api = ref.read(apiClientProvider);

    Map<String, dynamic> result;
    try {
      result = await api.fetchOpenRouterModels(apiKey);
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(L10n.t('models_fetch_error', {'e': FriendlyError.describeGeneric(e)}))),
        );
      }
      return null;
    }

    if (result['status'] != 'ok') {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('❌ ${result['error'] ?? L10n.t('models_fetch_error_short')}')),
        );
      }
      return null;
    }

    final rawModels = result['models'];
    final models = (rawModels is List) ? rawModels.cast<Map<String, dynamic>>() : <Map<String, dynamic>>[];
    if (!mounted) return null;

    return showDialog<String>(
      context: context,
      builder: (_) => _ModelBrowserDialog(
        models: models,
        title: L10n.t('openrouter_models'),
      ),
    );
  }

  /// Kilo Code AI Gateway's model browser — same rich pricing/free-aware
  /// dialog as OpenRouter's (their /models response shapes match closely
  /// enough to reuse _ModelBrowserDialog verbatim), fed by the dedicated
  /// /api/kilo/models endpoint (internal/webserver's handleKiloModels)
  /// rather than the generic plain-string one _browseGenericModels uses,
  /// since Kilo's real endpoint carries pricing/context/isFree metadata the
  /// generic ListModels interface (Provider.ListModels: []string) can't.
  Future<String?> _browseKiloModels() async {
    final api = ref.read(apiClientProvider);

    Map<String, dynamic> result;
    try {
      result = await api.fetchKiloModels();
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(L10n.t('models_fetch_error', {'e': FriendlyError.describeGeneric(e)}))),
        );
      }
      return null;
    }

    if (result['status'] != 'ok') {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('❌ ${result['error'] ?? L10n.t('models_fetch_error_short')}')),
        );
      }
      return null;
    }

    final rawModels = result['models'];
    final models = (rawModels is List) ? rawModels.cast<Map<String, dynamic>>() : <Map<String, dynamic>>[];
    if (!mounted) return null;

    return showDialog<String>(
      context: context,
      builder: (_) => _ModelBrowserDialog(
        models: models,
        title: L10n.t('kilo_models'),
      ),
    );
  }

  /// Model browser for providers with a plain OpenAI-compatible /models
  /// endpoint (no pricing/context metadata) — OpenCode Zen/Go today, and any
  /// future provider using the same generic backend endpoint.
  Future<String?> _browseGenericModels(String apiKey) async {
    final api = ref.read(apiClientProvider);

    Map<String, dynamic> result;
    try {
      result = await api.fetchProviderModels(
        type: _type,
        apiKey: apiKey,
        baseUrl: _baseUrlCtrl.text.trim(),
      );
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(L10n.t('models_fetch_error', {'e': FriendlyError.describeGeneric(e)}))),
        );
      }
      return null;
    }

    if (result['status'] != 'ok') {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('❌ ${result['error'] ?? L10n.t('models_fetch_error_short')}')),
        );
      }
      return null;
    }

    final rawModels = result['models'];
    final models = (rawModels is List) ? rawModels.whereType<String>().toList() : <String>[];
    if (!mounted) return null;

    return showDialog<String>(
      context: context,
      builder: (_) => _SimpleModelBrowserDialog(
        title: '${ProviderDefaults.displayNames[_type] ?? _type} — ${L10n.t('fetching_models')}',
        models: models,
      ),
    );
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
          SnackBar(content: Text(L10n.t('link_open_failed', {'url': url}))),
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

    // These used to show as a SnackBar — invisible in practice, since this
    // dialog is itself a modal Dialog: ScaffoldMessenger.of(context) bubbles
    // up to the app's root Scaffold, which sits BEHIND the modal barrier
    // (same class of bug already fixed once for MemoryImportTab — see
    // AGENTS.md). A click that silently fails a validation guard looks
    // exactly like "nothing happened". Shown as an inline banner instead,
    // which is guaranteed visible regardless of the dialog stack.
    setState(() => _saveError = null);

    // Guard the one mistake that silently breaks everything: saving a
    // key-requiring provider with an empty API key.
    if (ProviderDefaults.needsApiKey(_type) && _apiKeyCtrl.text.trim().isEmpty) {
      setState(() => _saveError = L10n.t('api_key_hint_get'));
      return;
    }

    // Custom endpoints have no default URL — without one, requests go nowhere.
    if (ProviderDefaults.needsBaseUrl(_type) && _baseUrlCtrl.text.trim().isEmpty) {
      setState(() => _saveError = L10n.t('custom_base_url_required'));
      return;
    }
    if (_modelCtrl.text.trim().isEmpty) {
      setState(() => _saveError = L10n.t('enter_model_name'));
      return;
    }

    setState(() => _isSaving = true);
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
        effortLevel: _effortLevel,
      );

      // Called directly (not via providerListProvider.notifier.updateProvider)
      // deliberately: that notifier method catches its own errors and only
      // reports them into errorMessageProvider — a provider nobody may be
      // listening to right now (e.g. opened from the setup wizard, before
      // ChatScreen's listener even exists). Catching it here means a failed
      // save always surfaces in THIS dialog, which is guaranteed visible.
      await ref.read(apiClientProvider).updateProvider(config);
      ref.invalidate(providerListProvider);
      ref.invalidate(activeProviderTypeProvider);
      if (!mounted) return;
      if (finalName != desiredName) {
        // _uniqueName silently disambiguated the name the user actually
        // typed — tell them, so "Claude" quietly becoming "Claude 2" in the
        // provider list isn't a surprise they have to notice on their own.
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(L10n.t('provider_renamed_on_conflict', {'desired': desiredName, 'final': finalName}))),
        );
      }
      Navigator.of(context).pop(true);
    } catch (e) {
      if (mounted) setState(() => _saveError = '${L10n.t('error')}: ${FriendlyError.describeGeneric(e)}');
    } finally {
      if (mounted) setState(() => _isSaving = false);
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
                                ? L10n.t('configure_provider_title', {'name': widget.existing!.name})
                                : L10n.t('provider_add_title'),
                            style: Theme.of(context).textTheme.titleLarge?.copyWith(
                                  fontWeight: FontWeight.bold,
                                ),
                          ),
                          Text(
                            isEditing
                                ? L10n.t('provider_edit_subtitle')
                                : L10n.t('provider_add_subtitle'),
                            style: TextStyle(fontSize: 12, color: c.textDim),
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 24),

                // ── Step 1: provider picker ──
                // A custom rounded, height-constrained, scrollable dialog
                // (_ProviderTypePickerDialog) instead of a native
                // DropdownButtonFormField: with 13 real-logo entries, the
                // stock dropdown menu (a) rendered in the app-level Overlay
                // rather than clipped to this modal, visibly spilling past
                // this dialog's own rounded card when opened near the
                // bottom of a shorter window; (b) used Material's default
                // small/sharp popup corner radius instead of Memo's
                // consistently-rounded shape; (c) prefixed each row with a
                // plain Unicode glyph (○ ◆ ✕ ...) instead of the provider's
                // real logo (providerLogoWidget, already used in this same
                // dialog's own header avatar just above). InputDecorator
                // keeps the identical label/helper-text field look while
                // being tappable instead of a native dropdown.
                InputDecorator(
                  decoration: InputDecoration(
                    labelText: L10n.t('provider_step1'),
                    border: const OutlineInputBorder(),
                    helperText: hint,
                    helperMaxLines: 2,
                    contentPadding:
                        const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
                  ),
                  child: InkWell(
                    onTap: isEditing ? null : () => _pickProviderType(context),
                    child: Row(
                      children: [
                        providerLogoWidget(_type, size: 20),
                        const SizedBox(width: 10),
                        Expanded(
                          child: Text(
                            ProviderDefaults.displayNames[_type] ?? _type,
                            style: const TextStyle(fontSize: 14),
                          ),
                        ),
                        if (!isEditing)
                          Icon(Icons.unfold_more, size: 18, color: c.textDim),
                      ],
                    ),
                  ),
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
                          ? L10n.t('api_key_optional')
                          : L10n.t('api_key_step2'),
                      border: const OutlineInputBorder(),
                      helperText: _type == 'custom'
                          ? L10n.t('api_key_custom_hint')
                          : L10n.t('api_key_stored'),
                      prefixIcon: const Icon(Icons.key, size: 20),
                      suffixIcon: IconButton(
                        tooltip: _obscureKey ? L10n.t('show_key') : L10n.t('hide_key'),
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
                          L10n.t('get_api_key_from', {'name': ProviderDefaults.displayNames[_type] ?? _type}),
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
                            L10n.t('local_provider_no_key'),
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
                    decoration: InputDecoration(
                      labelText: L10n.t('base_url'),
                      border: const OutlineInputBorder(),
                      helperText: L10n.t('base_url_openai_hint'),
                      prefixIcon: const Icon(Icons.link, size: 20),
                    ),
                  ),
                  const SizedBox(height: 12),
                  TextField(
                    controller: _nameCtrl,
                    decoration: InputDecoration(
                      labelText: L10n.t('display_name'),
                      border: const OutlineInputBorder(),
                      helperText: L10n.t('display_name_helper'),
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
                              ? L10n.t('model_label')
                              : (needsKey ? L10n.t('model_step3') : L10n.t('model_label')),
                          border: const OutlineInputBorder(),
                          helperText: _type == 'custom'
                              ? L10n.t('model_custom_hint')
                              : L10n.t('model_default_hint'),
                        ),
                      ),
                    ),
                    if (ProviderDefaults.hasModelBrowser(_type)) ...[
                      const SizedBox(width: 8),
                      Padding(
                        padding: const EdgeInsets.only(bottom: 20),
                        child: OutlinedButton.icon(
                          onPressed: _openModelBrowser,
                          icon: const Icon(Icons.search, size: 18),
                          label: Text(L10n.t('select')),
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
                      label: Text(L10n.t('test_connection')),
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
                        _testResult! ? L10n.t('test_passed') : L10n.t('test_failed'),
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
                      L10n.t('advanced_settings'),
                      style: TextStyle(fontSize: 13, color: c.textDim, fontWeight: FontWeight.w500),
                    ),
                    children: [
                      // For custom providers these two live in the main flow
                      // (above) since they're required, so don't duplicate them.
                      if (_type != 'custom') ...[
                        TextField(
                          controller: _nameCtrl,
                          decoration: InputDecoration(
                            labelText: L10n.t('display_name'),
                            border: const OutlineInputBorder(),
                            helperText: L10n.t('display_name_helper_dup'),
                          ),
                        ),
                        const SizedBox(height: 12),
                        TextField(
                          controller: _baseUrlCtrl,
                          decoration: InputDecoration(
                            labelText: L10n.t('base_url'),
                            border: const OutlineInputBorder(),
                            helperText: L10n.t('base_url_default_hint'),
                          ),
                        ),
                        const SizedBox(height: 12),
                      ],
                      TextField(
                        controller: _contextCtrl,
                        keyboardType: TextInputType.number,
                        inputFormatters: [FilteringTextInputFormatter.digitsOnly],
                        decoration: InputDecoration(
                          labelText: L10n.t('context_window_label'),
                          border: const OutlineInputBorder(),
                          helperText: L10n.t('context_window_hint'),
                        ),
                      ),
                      const SizedBox(height: 12),
                      TextField(
                        controller: _priorityCtrl,
                        keyboardType: TextInputType.number,
                        inputFormatters: [FilteringTextInputFormatter.digitsOnly],
                        decoration: InputDecoration(
                          labelText: L10n.t('priority_label'),
                          border: const OutlineInputBorder(),
                          helperText: L10n.t('priority_hint'),
                        ),
                      ),
                      // Reasoning effort — only shown when this provider
                      // type actually has one, discovered live against the
                      // exact model configured (openrouter/claude/gemini/
                      // ollama — see MemoApiClient.getEffortLevels; every
                      // other type has no known capability signal at all
                      // and never shows this section). All four depend on
                      // the chosen model, so they share a manual-refresh
                      // affordance instead of a per-keystroke network call.
                      if (_loadingEffortLevels || _availableEffortLevels.isNotEmpty) ...[
                        const SizedBox(height: 12),
                        Row(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Expanded(
                              child: DropdownButtonFormField<String>(
                                initialValue: _availableEffortLevels.contains(_effortLevel)
                                    ? _effortLevel
                                    : null,
                                decoration: InputDecoration(
                                  labelText: L10n.t('effort_level_label'),
                                  border: const OutlineInputBorder(),
                                  helperText: L10n.t('effort_level_hint'),
                                  helperMaxLines: 3,
                                ),
                                items: [
                                  DropdownMenuItem(
                                    value: null,
                                    child: Text(L10n.t('effort_level_default')),
                                  ),
                                  ..._availableEffortLevels.map(
                                    (level) => DropdownMenuItem(value: level, child: Text(level)),
                                  ),
                                ],
                                onChanged: (v) => setState(() => _effortLevel = v ?? ''),
                              ),
                            ),
                            if (_effortLevelsAreModelDependent) ...[
                              const SizedBox(width: 8),
                              IconButton(
                                tooltip: L10n.t('effort_level_refresh'),
                                icon: _loadingEffortLevels
                                    ? const SizedBox(
                                        width: 16,
                                        height: 16,
                                        child: CircularProgressIndicator(strokeWidth: 2),
                                      )
                                    : const Icon(Icons.refresh, size: 20),
                                onPressed: _loadingEffortLevels ? null : _loadEffortLevels,
                              ),
                            ],
                          ],
                        ),
                      ],
                    ],
                  ),
                ),

                // ── Enable toggle ──
                SwitchListTile(
                  title: Text(L10n.t('enable_provider')),
                  subtitle: Text(
                    _enabled ? L10n.t('provider_enabled_sub') : L10n.t('provider_disabled_sub'),
                    style: TextStyle(fontSize: 12, color: c.textDim),
                  ),
                  value: _enabled,
                  contentPadding: EdgeInsets.zero,
                  onChanged: (v) => setState(() => _enabled = v),
                ),
                const SizedBox(height: 12),

                if (_saveError != null) ...[
                  Container(
                    padding: const EdgeInsets.all(10),
                    decoration: BoxDecoration(
                      color: MemoTheme.red.withValues(alpha: 0.08),
                      borderRadius: BorderRadius.circular(8),
                    ),
                    child: Row(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        const Icon(Icons.error_outline, size: 16, color: MemoTheme.red),
                        const SizedBox(width: 8),
                        Expanded(
                          child: Text(
                            _saveError!,
                            style: const TextStyle(fontSize: 12, color: MemoTheme.red),
                          ),
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(height: 12),
                ],

                // ── Actions ──
                Row(
                  mainAxisAlignment: MainAxisAlignment.end,
                  children: [
                    TextButton(
                      onPressed: () => Navigator.of(context).pop(),
                      child: Text(L10n.t('cancel')),
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
                          : Text(L10n.t('save')),
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

// ─── Rich (pricing/free-aware) Model Browser Dialog ────────────
// Shared by OpenRouter and Kilo Code — both APIs shape their /models
// response closely enough (id/name/context_length/pricing/is_free) to
// reuse this one dialog rather than duplicating it per provider.

class _ModelBrowserDialog extends StatefulWidget {
  final List<Map<String, dynamic>> models;
  final String title;
  const _ModelBrowserDialog({required this.models, required this.title});

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
    if (p == 0) return L10n.t('free');
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
                Expanded(
                  child: Text(
                    widget.title,
                    style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
                  ),
                ),
                Text(
                  L10n.t('model_count', {'count': '${widget.models.length}'}),
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
                hintText: L10n.t('model_search'),
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
                  L10n.t('free_paid_legend'),
                  style: TextStyle(fontSize: 11, color: Colors.grey[500]),
                ),
                const Spacer(),
                TextButton(
                  onPressed: () => Navigator.of(context).pop(),
                  child: Text(L10n.t('cancel')),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

// ─── Generic Model Browser Dialog (plain OpenAI-compatible /models) ────

class _SimpleModelBrowserDialog extends StatefulWidget {
  final String title;
  final List<String> models;
  const _SimpleModelBrowserDialog({required this.title, required this.models});

  @override
  State<_SimpleModelBrowserDialog> createState() => _SimpleModelBrowserDialogState();
}

class _SimpleModelBrowserDialogState extends State<_SimpleModelBrowserDialog> {
  late List<String> _filtered;
  final _searchCtrl = TextEditingController();

  @override
  void initState() {
    super.initState();
    _filtered = List.from(widget.models)..sort();
  }

  @override
  void dispose() {
    _searchCtrl.dispose();
    super.dispose();
  }

  void _filter(String q) {
    final query = q.toLowerCase();
    setState(() {
      _filtered = (query.isEmpty
          ? List<String>.from(widget.models)
          : widget.models.where((m) => m.toLowerCase().contains(query)).toList())
        ..sort();
    });
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
                Expanded(
                  child: Text(
                    widget.title,
                    style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
                  ),
                ),
                Text(
                  L10n.t('model_count', {'count': '${widget.models.length}'}),
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
                hintText: L10n.t('model_search'),
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
            child: _filtered.isEmpty
                ? Padding(
                    padding: const EdgeInsets.symmetric(vertical: 24),
                    child: Text(L10n.t('model_not_found')),
                  )
                : ListView.builder(
                    shrinkWrap: true,
                    itemCount: _filtered.length,
                    padding: const EdgeInsets.symmetric(horizontal: 12),
                    itemBuilder: (ctx, i) {
                      final id = _filtered[i];
                      return Card(
                        margin: const EdgeInsets.only(bottom: 4),
                        child: ListTile(
                          dense: true,
                          title: Text(
                            id,
                            style: const TextStyle(
                              fontSize: 13,
                              fontWeight: FontWeight.w500,
                              fontFamily: 'monospace',
                            ),
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
              mainAxisAlignment: MainAxisAlignment.end,
              children: [
                TextButton(
                  onPressed: () => Navigator.of(context).pop(),
                  child: Text(L10n.t('cancel')),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

// ─── Provider Type Picker Dialog ────────────────────────────────
// Replaces the old DropdownButtonFormField provider picker — see Step 1's
// own comment above (in _ProviderConfigDialogState.build) for exactly why:
// overflow past this app's modal bounds, mismatched corner radius, and
// plain-glyph rows instead of real logos. Height-capped and internally
// scrollable so it can never grow past the screen regardless of how many
// provider types exist, and shares this app's own rounded Dialog shape
// (MemoTheme.radiusLg) instead of Material's small default popup radius.

class _ProviderTypePickerDialog extends StatelessWidget {
  final List<String> types;
  final String current;
  const _ProviderTypePickerDialog({required this.types, required this.current});

  @override
  Widget build(BuildContext context) {
    final screenHeight = MediaQuery.of(context).size.height;

    return Dialog(
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(MemoTheme.radiusLg),
      ),
      insetPadding: const EdgeInsets.symmetric(horizontal: 32, vertical: 24),
      child: ConstrainedBox(
        // Capped relative to the actual viewport, not a fixed pixel count —
        // this is exactly what a native dropdown menu should have done and
        // didn't: guaranteed to always fit on screen, scrolling internally
        // once the list is taller than the cap, never spilling past the
        // window like the widget this replaces did.
        constraints: BoxConstraints(
          maxWidth: 360,
          maxHeight: (screenHeight * 0.6).clamp(280.0, 480.0),
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(20, 16, 20, 8),
              child: Row(
                children: [
                  Expanded(
                    child: Text(
                      L10n.t('provider_step1'),
                      style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
                    ),
                  ),
                  IconButton(
                    icon: const Icon(Icons.close, size: 18),
                    onPressed: () => Navigator.of(context).pop(),
                    visualDensity: VisualDensity.compact,
                  ),
                ],
              ),
            ),
            const Divider(height: 1),
            Flexible(
              child: ListView.builder(
                shrinkWrap: true,
                padding: const EdgeInsets.symmetric(vertical: 8),
                itemCount: types.length,
                itemBuilder: (ctx, i) {
                  final t = types[i];
                  final isSelected = t == current;
                  return ListTile(
                    dense: true,
                    leading: providerLogoWidget(t, size: 22),
                    title: Text(
                      ProviderDefaults.displayNames[t] ?? t,
                      style: TextStyle(
                        fontSize: 14,
                        fontWeight: isSelected ? FontWeight.w600 : FontWeight.normal,
                      ),
                    ),
                    trailing: isSelected
                        ? Icon(Icons.check, size: 18, color: MemoTheme.accent)
                        : null,
                    selected: isSelected,
                    selectedTileColor: MemoTheme.accentMuted,
                    onTap: () => Navigator.of(context).pop(t),
                  );
                },
              ),
            ),
          ],
        ),
      ),
    );
  }
}
