import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/theme.dart';
import '../models/orchestra_config.dart';
import '../models/provider_config.dart';
import '../providers/chat_provider.dart';
import '../providers/orchestra_provider.dart';
import '../providers/provider_provider.dart';

class OrchestraConfigDialog extends ConsumerStatefulWidget {
  const OrchestraConfigDialog({super.key});

  @override
  ConsumerState<OrchestraConfigDialog> createState() => _OrchestraConfigDialogState();
}

class _OrchestraConfigDialogState extends ConsumerState<OrchestraConfigDialog> {
  OrchestraConfig? _config;
  bool _saving = false;
  bool _loaded = false;

  /// Role ids whose card is expanded (progressive disclosure — collapsed by default).
  final Set<String> _expanded = {};

  /// Role ids whose advanced "system prompt" editor is open.
  final Set<String> _promptOpen = {};

  /// One-line, plain-language description of what each built-in role does.
  static const Map<String, String> _roleDesc = {
    'planner': 'İsteği alt görevlere böler',
    'frontend': 'Arayüz ve görsel işler',
    'backend': 'Sunucu ve veri tarafı',
    'bug_fixer': 'Hata bulur ve düzeltir',
    'reviewer': 'Kodu gözden geçirir',
    'security': 'Güvenlik denetimi yapar',
    'devops': 'Derleme, dağıtım, altyapı',
    'general': 'Genel amaçlı uzman',
  };

  @override
  Widget build(BuildContext context) {
    final configAsync = ref.watch(orchestraConfigProvider);
    final providersAsync = ref.watch(providerListProvider);

    return Dialog(
      backgroundColor: MemoTheme.of(context).bgApp,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(MemoTheme.radiusLg)),
      constraints: const BoxConstraints(maxWidth: 720, maxHeight: 650),
      child: configAsync.when(
        data: (config) {
          // Only set _config from API data on first load
          if (!_loaded) {
            _config = config;
            _loaded = true;
          }
          final cfg = _config ?? config;
          return _buildContent(context, cfg, providersAsync);
        },
        loading: () => const SizedBox(height: 200, child: Center(child: CircularProgressIndicator())),
        error: (e, _) => SizedBox(height: 200, child: Center(child: Text('Error: $e'))),
      ),
    );
  }

  Widget _buildContent(BuildContext context, OrchestraConfig config, AsyncValue<List<ProviderConfig>> providersAsync) {
    final modelChoices = _buildModelChoices(providersAsync);

    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        _buildHeader(context, config),
        Flexible(child: _buildBody(context, config, modelChoices)),
        _buildFooter(context, config),
      ],
    );
  }

  Widget _buildHeader(BuildContext context, OrchestraConfig config) {
    final c = MemoTheme.of(context);
    return Container(
      padding: const EdgeInsets.fromLTRB(24, 20, 20, 20),
      decoration: BoxDecoration(border: Border(bottom: BorderSide(color: c.borderSoft))),
      child: Row(
        children: [
          Container(
            width: 40,
            height: 40,
            decoration: BoxDecoration(
              color: MemoTheme.accentPale,
              borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
              border: Border.all(color: MemoTheme.accent.withValues(alpha: 0.3)),
            ),
            alignment: Alignment.center,
            child: const Icon(Icons.hub_outlined, size: 20, color: MemoTheme.accent),
          ),
          const SizedBox(width: 14),
          Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('Orchestra', style: TextStyle(fontSize: 17, fontWeight: FontWeight.w700, color: c.textMain)),
              const SizedBox(height: 2),
              Text('Birden çok modeli bir ekip gibi çalıştır', style: TextStyle(fontSize: 12, color: c.textDim)),
            ],
          ),
          const Spacer(),
          Text(
            config.enabled ? 'Açık' : 'Kapalı',
            style: TextStyle(fontSize: 13, fontWeight: FontWeight.w500, color: config.enabled ? MemoTheme.accent : c.textDim),
          ),
          const SizedBox(width: 8),
          Switch(
            value: config.enabled,
            onChanged: (v) => setState(() => _config = config.copyWith(enabled: v)),
            activeThumbColor: MemoTheme.accent,
          ),
        ],
      ),
    );
  }

  /// Compact visual explainer: how Orchestra processes a request.
  Widget _buildExplainer(BuildContext context) {
    final c = MemoTheme.of(context);
    Widget step(IconData icon, String title, String sub) => Expanded(
          child: Column(
            children: [
              Icon(icon, size: 20, color: MemoTheme.accent),
              const SizedBox(height: 6),
              Text(title, style: TextStyle(fontSize: 12, fontWeight: FontWeight.w600, color: c.textMain), textAlign: TextAlign.center),
              const SizedBox(height: 2),
              Text(sub, style: TextStyle(fontSize: 10.5, color: c.textDim, height: 1.3), textAlign: TextAlign.center),
            ],
          ),
        );
    Widget arrow() => Padding(
          padding: const EdgeInsets.only(bottom: 18),
          child: Icon(Icons.chevron_right, size: 18, color: c.textDim),
        );
    return Container(
      margin: const EdgeInsets.only(bottom: 20),
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 16),
      decoration: BoxDecoration(
        color: c.bgPanel,
        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
        border: Border.all(color: c.borderSoft),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          step(Icons.account_tree_outlined, 'Şef planlar', 'İsteği alt görevlere böler'),
          arrow(),
          step(Icons.groups_outlined, 'Uzmanlar', 'Paralel çalışır'),
          arrow(),
          step(Icons.auto_awesome_outlined, 'Sentez', 'Sonuçları birleştirir'),
        ],
      ),
    );
  }

  Widget _buildBody(BuildContext context, OrchestraConfig config, List<_ModelChoice> modelChoices) {
    final c = MemoTheme.of(context);
    final enabledCount = config.roles.where((r) => r.enabled).length;
    return ListView(
      padding: const EdgeInsets.all(24),
      children: [
        _buildExplainer(context),
        _buildQuickSetup(context, config, modelChoices),
        const SizedBox(height: 24),

        _sectionTitle('Şef model'),
        const SizedBox(height: 4),
        Text('İsteği analiz eder, görev dağıtır ve sonuçları birleştirir.', style: TextStyle(fontSize: 12, color: c.textDim)),
        const SizedBox(height: 10),
        _buildChiefSelector(config, modelChoices),
        const SizedBox(height: 24),

        Row(
          crossAxisAlignment: CrossAxisAlignment.baseline,
          textBaseline: TextBaseline.alphabetic,
          children: [
            _sectionTitle('Uzman rolleri'),
            const SizedBox(width: 8),
            Text('$enabledCount açık', style: TextStyle(fontSize: 12, color: MemoTheme.accent, fontWeight: FontWeight.w500)),
          ],
        ),
        const SizedBox(height: 4),
        Text('Sadece açık roller çalışır. Karta dokunup model ve talimatı düzenle.', style: TextStyle(fontSize: 12, color: c.textDim)),
        const SizedBox(height: 12),
        ...List.generate(config.roles.length, (i) => _buildRoleCard(context, config, i, modelChoices)),
        const SizedBox(height: 4),
        OutlinedButton.icon(
          onPressed: () => _addCustomRole(config),
          icon: const Icon(Icons.add, size: 18),
          label: const Text('Özel rol ekle'),
          style: OutlinedButton.styleFrom(
            side: BorderSide(color: c.borderSoft),
            foregroundColor: c.textMuted,
            minimumSize: const Size(0, 44),
            padding: const EdgeInsets.symmetric(horizontal: 16),
          ),
        ),
      ],
    );
  }

  /// One-tap setup: assign a single model to the chief and every enabled role.
  Widget _buildQuickSetup(BuildContext context, OrchestraConfig config, List<_ModelChoice> choices) {
    final c = MemoTheme.of(context);
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: MemoTheme.accentMuted,
        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
        border: Border.all(color: MemoTheme.accent.withValues(alpha: 0.25)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              const Icon(Icons.bolt, size: 18, color: MemoTheme.accent),
              const SizedBox(width: 6),
              Text('Hızlı kurulum', style: TextStyle(fontSize: 13, fontWeight: FontWeight.w600, color: c.textMain)),
            ],
          ),
          const SizedBox(height: 4),
          Text('Tek bir modeli şefe ve tüm açık rollere bir kerede ata.', style: TextStyle(fontSize: 12, color: c.textDim)),
          const SizedBox(height: 10),
          DropdownButtonFormField<String>(
            key: ValueKey('quick_${choices.length}'),
            value: null,
            isExpanded: true,
            hint: Text('Model seç ve uygula', style: TextStyle(fontSize: 13, color: c.textDim)),
            decoration: InputDecoration(
              isDense: true,
              filled: true,
              fillColor: c.bgApp,
              contentPadding: const EdgeInsets.symmetric(horizontal: 12, vertical: 12),
              border: OutlineInputBorder(borderRadius: BorderRadius.circular(MemoTheme.radiusMd), borderSide: BorderSide(color: c.borderSoft)),
            ),
            items: choices.map((ch) => DropdownMenuItem(value: ch.key, child: Text('${ch.icon} ${ch.label}', style: const TextStyle(fontSize: 13), overflow: TextOverflow.ellipsis))).toList(),
            onChanged: (val) {
              if (val == null) return;
              _applyQuickModel(config, choices.firstWhere((ch) => ch.key == val));
            },
          ),
        ],
      ),
    );
  }

  void _applyQuickModel(OrchestraConfig config, _ModelChoice choice) {
    final newRoles = config.roles
        .map((r) => r.enabled ? r.copyWith(modelType: choice.type, modelName: choice.model) : r)
        .toList();
    setState(() => _config = config.copyWith(
          chiefType: choice.type,
          chiefModel: choice.model,
          roles: newRoles,
        ));
    if (mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('${choice.label} şefe ve açık rollere uygulandı'), behavior: SnackBarBehavior.floating),
      );
    }
  }

  void _addCustomRole(OrchestraConfig config) {
    final newRoles = List<RoleConfig>.from(config.roles);
    newRoles.add(RoleConfig(
      role: 'custom_${DateTime.now().millisecondsSinceEpoch}',
      enabled: true,
      modelType: 'local',
      modelName: 'local',
      systemPrompt: 'Sen bir yardımcı asistansın.',
    ));
    setState(() => _config = config.copyWith(roles: newRoles));
  }

  Widget _buildFooter(BuildContext context, OrchestraConfig config) {
    return Container(
      padding: const EdgeInsets.all(24),
      decoration: BoxDecoration(border: Border(top: BorderSide(color: MemoTheme.of(context).borderSoft))),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.end,
        children: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(),
            style: TextButton.styleFrom(minimumSize: const Size(0, 44)),
            child: const Text('İptal'),
          ),
          const SizedBox(width: 12),
          FilledButton(
            onPressed: _saving ? null : () => _save(config),
            style: FilledButton.styleFrom(backgroundColor: MemoTheme.accent, minimumSize: const Size(96, 44)),
            child: _saving
                ? const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2))
                : const Text('Kaydet'),
          ),
        ],
      ),
    );
  }

  Widget _sectionTitle(String title) {
    return Text(title, style: TextStyle(fontSize: 14, fontWeight: FontWeight.w600, color: MemoTheme.of(context).textMain));
  }

  List<_ModelChoice> _buildModelChoices(AsyncValue<List<ProviderConfig>> providersAsync) {
    final choices = <_ModelChoice>[];
    choices.add(_ModelChoice('local', 'local', '🖥️', 'Local Model (llama.cpp)'));
    if (providersAsync case AsyncData(:final value)) {
      for (final p in value) {
        if (p.enabled) {
          choices.add(_ModelChoice(p.name, p.model, providerIcon(p.type), '${p.name} → ${p.model}'));
        }
      }
    }
    return choices;
  }

  Widget _buildChiefSelector(OrchestraConfig config, List<_ModelChoice> choices) {
    final currentKey = '${config.chiefType}/${config.chiefModel}';
    final validChoice = choices.any((c) => c.key == currentKey);
    return DropdownButtonFormField<String>(
      key: ValueKey('chief_${choices.length}'),
      value: validChoice ? currentKey : null,
      hint: Text('Model seç', style: TextStyle(fontSize: 12, color: MemoTheme.of(context).textDim)),
      decoration: InputDecoration(isDense: true, contentPadding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10), border: OutlineInputBorder(borderRadius: BorderRadius.circular(MemoTheme.radiusMd))),
      items: choices.map((c) => DropdownMenuItem(value: c.key, child: Text('${c.icon} ${c.label}', style: const TextStyle(fontSize: 12)))).toList(),
      onChanged: (val) {
        if (val == null) return;
        final choice = choices.firstWhere((c) => c.key == val);
        setState(() => _config = config.copyWith(chiefType: choice.type, chiefModel: choice.model));
      },
    );
  }

  bool _isBuiltinRole(String role) {
    return OrchestraDefaults.defaultRoles.any((r) => r['role'] == role);
  }

  Widget _buildRoleCard(BuildContext context, OrchestraConfig config, int index, List<_ModelChoice> choices) {
    final c = MemoTheme.of(context);
    final role = config.roles[index];
    final isBuiltin = _isBuiltinRole(role.role);
    final icon = isBuiltin ? OrchestraDefaults.iconForRole(role.role) : '\u2756';
    final label = isBuiltin ? OrchestraDefaults.labelForRole(role.role) : (role.role.isEmpty ? '\u00d6zel rol' : role.role);
    final desc = isBuiltin ? (_roleDesc[role.role] ?? '') : '\u00d6zel rol';
    final currentKey = '${role.modelType}/${role.modelName}';
    final validChoice = choices.any((ch) => ch.key == currentKey);
    final assignedLabel = validChoice ? choices.firstWhere((ch) => ch.key == currentKey).label : '';
    final isExpanded = _expanded.contains(role.role);

    void updateRole(RoleConfig updated) {
      final newRoles = List<RoleConfig>.from(config.roles);
      newRoles[index] = updated;
      setState(() => _config = config.copyWith(roles: newRoles));
    }

    return Container(
      margin: const EdgeInsets.only(bottom: 10),
      decoration: BoxDecoration(
        color: c.bgPanel,
        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
        border: Border.all(color: role.enabled ? MemoTheme.accent.withValues(alpha: 0.35) : c.borderSoft),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Summary row (always visible) — tap to expand
          InkWell(
            borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
            onTap: () => setState(() => isExpanded ? _expanded.remove(role.role) : _expanded.add(role.role)),
            child: Padding(
              padding: const EdgeInsets.fromLTRB(14, 12, 12, 12),
              child: Row(
                children: [
                  Text(icon, style: TextStyle(fontSize: 16, color: role.enabled ? MemoTheme.accent : c.textDim)),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(label, style: TextStyle(fontSize: 13.5, fontWeight: FontWeight.w600, color: c.textMain)),
                        const SizedBox(height: 2),
                        Text(
                          role.enabled
                              ? (assignedLabel.isNotEmpty ? assignedLabel : '⚠ Model atanmadı')
                              : desc,
                          style: TextStyle(
                            fontSize: 11.5,
                            color: role.enabled && assignedLabel.isEmpty ? MemoTheme.warningOrange : c.textDim,
                          ),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(width: 8),
                  Switch(
                    value: role.enabled,
                    onChanged: (v) {
                      updateRole(role.copyWith(enabled: v));
                      if (v) setState(() => _expanded.add(role.role));
                    },
                    activeThumbColor: MemoTheme.accent,
                  ),
                  Icon(isExpanded ? Icons.expand_less : Icons.expand_more, size: 20, color: c.textDim),
                ],
              ),
            ),
          ),

          // Expanded detail
          if (isExpanded) ...[
            Divider(height: 1, color: c.borderSoft),
            Padding(
              padding: const EdgeInsets.fromLTRB(14, 14, 14, 14),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  if (!isBuiltin) ...[
                    _RoleNameField(
                      initialValue: role.role,
                      onChanged: (v) => updateRole(role.copyWith(role: v)),
                    ),
                    const SizedBox(height: 10),
                  ],
                  Text('Model', style: TextStyle(fontSize: 11.5, fontWeight: FontWeight.w600, color: c.textMuted)),
                  const SizedBox(height: 6),
                  DropdownButtonFormField<String>(
                    key: ValueKey('role_${index}_${choices.length}'),
                    value: validChoice ? currentKey : null,
                    isExpanded: true,
                    hint: Text('Model seç', style: TextStyle(fontSize: 12.5, color: c.textDim)),
                    decoration: InputDecoration(
                      isDense: true,
                      contentPadding: const EdgeInsets.symmetric(horizontal: 12, vertical: 12),
                      border: OutlineInputBorder(borderRadius: BorderRadius.circular(MemoTheme.radiusMd)),
                    ),
                    items: choices.map((ch) => DropdownMenuItem(value: ch.key, child: Text('${ch.icon} ${ch.label}', style: const TextStyle(fontSize: 12.5), overflow: TextOverflow.ellipsis))).toList(),
                    onChanged: (val) {
                      if (val == null) return;
                      final choice = choices.firstWhere((ch) => ch.key == val);
                      updateRole(role.copyWith(modelType: choice.type, modelName: choice.model));
                    },
                  ),
                  if (role.modelType == 'openrouter') ...[
                    const SizedBox(height: 6),
                    InkWell(
                      onTap: () => _pickModelForRole(index, role),
                      borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                      child: Container(
                        width: double.infinity,
                        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 10),
                        decoration: BoxDecoration(
                          border: Border.all(color: c.borderSoft),
                          borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                          color: c.bgElement,
                        ),
                        child: Row(
                          children: [
                            Expanded(
                              child: Text(
                                role.modelName.isNotEmpty ? role.modelName : 'OpenRouter modeli seç',
                                style: TextStyle(fontSize: 12, color: role.modelName.isNotEmpty ? c.textMain : c.textDim),
                                overflow: TextOverflow.ellipsis,
                              ),
                            ),
                            const SizedBox(width: 8),
                            Icon(Icons.search, size: 15, color: c.textDim),
                          ],
                        ),
                      ),
                    ),
                  ],
                  const SizedBox(height: 12),

                  // Advanced: system prompt (hidden by default)
                  InkWell(
                    onTap: () => setState(() => _promptOpen.contains(role.role) ? _promptOpen.remove(role.role) : _promptOpen.add(role.role)),
                    child: Row(
                      children: [
                        Icon(_promptOpen.contains(role.role) ? Icons.expand_less : Icons.expand_more, size: 16, color: c.textDim),
                        const SizedBox(width: 4),
                        Text('Gelişmiş: sistem talimatı', style: TextStyle(fontSize: 12, color: c.textMuted)),
                      ],
                    ),
                  ),
                  if (_promptOpen.contains(role.role)) ...[
                    const SizedBox(height: 8),
                    _SystemPromptField(
                      initialValue: role.systemPrompt,
                      onChanged: (v) => updateRole(role.copyWith(systemPrompt: v)),
                    ),
                  ],

                  if (!isBuiltin) ...[
                    const SizedBox(height: 4),
                    Align(
                      alignment: Alignment.centerRight,
                      child: TextButton.icon(
                        onPressed: () {
                          final newRoles = List<RoleConfig>.from(config.roles);
                          newRoles.removeAt(index);
                          setState(() => _config = config.copyWith(roles: newRoles));
                        },
                        icon: const Icon(Icons.delete_outline, size: 16),
                        label: const Text('Rolü sil'),
                        style: TextButton.styleFrom(foregroundColor: MemoTheme.red),
                      ),
                    ),
                  ],
                ],
              ),
            ),
          ],
        ],
      ),
    );
  }

  Future<void> _pickModelForRole(int index, RoleConfig role) async {
    final apiClient = ref.read(apiClientProvider);
    List<Map<String, dynamic>> models;
    try {
      final result = await apiClient.fetchOpenRouterModels('');
      if (result['status'] != 'ok') {
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text('❌ ${result['error'] ?? 'Model listesi alınamadı. Önce API Provider\'dan OpenRouter\'ı yapılandır.'}')),
          );
        }
        return;
      }
      final rawModels = result['models'];
      models = (rawModels is List) ? rawModels.cast<Map<String, dynamic>>() : <Map<String, dynamic>>[];
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Hata: $e')));
      return;
    }
    if (!mounted) return;
    final selected = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (_) => _RoleModelBrowserDialog(models: models),
    );
    if (selected != null) {
      final modelId = selected['id'] as String?;
      if (modelId == null) return;
      final newRoles = List<RoleConfig>.from(_config!.roles);
      newRoles[index] = role.copyWith(modelName: modelId);
      setState(() => _config = _config!.copyWith(roles: newRoles));
    }
  }

  Future<void> _save(OrchestraConfig config) async {
    final noModelRoles = config.roles.where((r) => r.enabled && (r.modelType.isEmpty || r.modelName.isEmpty));
    if (config.enabled) {
      if (config.chiefType.isEmpty || config.chiefModel.isEmpty) {
        if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Şef modele bir model ata')));
        return;
      }
      if (noModelRoles.isNotEmpty) {
        if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Lütfen tüm aktif rollere model ata')));
        return;
      }
    }
    setState(() => _saving = true);
    try {
      await ref.read(orchestraConfigProvider.notifier).save(config);
      if (mounted) {
        final messenger = ScaffoldMessenger.of(context);
        Navigator.of(context).pop();
        messenger.showSnackBar(const SnackBar(content: Text('Orchestra config saved')));
      }
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Save failed: $e')));
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }
}

class _RoleNameField extends StatefulWidget {
  final String initialValue;
  final ValueChanged<String> onChanged;
  const _RoleNameField({required this.initialValue, required this.onChanged});

  @override
  State<_RoleNameField> createState() => _RoleNameFieldState();
}

class _RoleNameFieldState extends State<_RoleNameField> {
  late TextEditingController _controller;

  @override
  void initState() {
    super.initState();
    _controller = TextEditingController(text: widget.initialValue);
  }

  @override
  void didUpdateWidget(_RoleNameField old) {
    super.didUpdateWidget(old);
    if (widget.initialValue != old.initialValue) {
      _controller.text = widget.initialValue;
    }
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return TextField(
      controller: _controller,
      style: TextStyle(
        fontSize: 13,
        fontWeight: FontWeight.w500,
        color: MemoTheme.of(context).textMain,
      ),
      decoration: const InputDecoration(
        isDense: true,
        border: InputBorder.none,
        contentPadding: EdgeInsets.zero,
      ),
      onChanged: widget.onChanged,
    );
  }
}

class _SystemPromptField extends StatefulWidget {
  final String initialValue;
  final ValueChanged<String> onChanged;
  const _SystemPromptField({required this.initialValue, required this.onChanged});

  @override
  State<_SystemPromptField> createState() => _SystemPromptFieldState();
}

class _SystemPromptFieldState extends State<_SystemPromptField> {
  late TextEditingController _controller;

  @override
  void initState() {
    super.initState();
    _controller = TextEditingController(text: widget.initialValue);
  }

  @override
  void didUpdateWidget(_SystemPromptField old) {
    super.didUpdateWidget(old);
    if (widget.initialValue != old.initialValue) {
      _controller.text = widget.initialValue;
    }
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return TextField(
      controller: _controller,
      style: TextStyle(
        fontSize: 11,
        color: MemoTheme.of(context).textDim,
      ),
      decoration: InputDecoration(
        hintText: 'System prompt...',
        hintStyle: TextStyle(
          fontSize: 11,
          color: MemoTheme.of(context).textDim.withValues(alpha: 0.4),
        ),
        isDense: true,
        contentPadding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
        ),
        filled: true,
        fillColor: MemoTheme.of(context).bgElement,
      ),
      minLines: 2,
      maxLines: 4,
      onChanged: widget.onChanged,
    );
  }
}

class _ModelChoice {
  final String type;
  final String model;
  final String key;
  final String icon;
  final String label;
  _ModelChoice(this.type, this.model, this.icon, this.label) : key = '$type/$model';
}

class _RoleModelBrowserDialog extends StatefulWidget {
  final List<Map<String, dynamic>> models;
  const _RoleModelBrowserDialog({required this.models});

  @override
  State<_RoleModelBrowserDialog> createState() => _RoleModelBrowserDialogState();
}

class _RoleModelBrowserDialogState extends State<_RoleModelBrowserDialog> {
  String _search = '';

  List<Map<String, dynamic>> get _filtered {
    final models = List<Map<String, dynamic>>.from(widget.models);
    models.sort((a, b) {
      final aFree = (a['pricing']?['prompt'] ?? 0.0) == 0.0;
      final bFree = (b['pricing']?['prompt'] ?? 0.0) == 0.0;
      if (aFree != bFree) return aFree ? -1 : 1;
      return (a['name'] as String? ?? '').compareTo(b['name'] as String? ?? '');
    });
    if (_search.isEmpty) return models;
    final q = _search.toLowerCase();
    return models.where((m) =>
      (m['id'] as String? ?? '').toLowerCase().contains(q) ||
      (m['name'] as String? ?? '').toLowerCase().contains(q)
    ).toList();
  }

  @override
  Widget build(BuildContext context) {
    final filtered = _filtered;
    return Dialog(
      backgroundColor: MemoTheme.of(context).bgApp,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(MemoTheme.radiusLg)),
      constraints: const BoxConstraints(maxWidth: 480, maxHeight: 520),
      child: Column(
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 16, 16, 8),
            child: TextField(
              onChanged: (v) => setState(() => _search = v),
              decoration: InputDecoration(
                hintText: 'Model ara...',
                isDense: true,
                prefixIcon: const Icon(Icons.search, size: 18),
                border: OutlineInputBorder(borderRadius: BorderRadius.circular(MemoTheme.radiusMd)),
                contentPadding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
              ),
              autofocus: true,
            ),
          ),
          const Padding(
            padding: EdgeInsets.symmetric(horizontal: 16),
            child: Row(
              children: [
                Text('🟢', style: TextStyle(fontSize: 10)),
                SizedBox(width: 4),
                Text('Ücretsiz', style: TextStyle(fontSize: 10)),
                SizedBox(width: 16),
                Text('🟡', style: TextStyle(fontSize: 10)),
                SizedBox(width: 4),
                Text('Ücretli', style: TextStyle(fontSize: 10)),
              ],
            ),
          ),
          const SizedBox(height: 4),
          Expanded(
            child: ListView.separated(
              padding: const EdgeInsets.symmetric(horizontal: 16),
              itemCount: filtered.length,
              separatorBuilder: (_, _) => Divider(height: 1, color: MemoTheme.of(context).borderSoft),
              itemBuilder: (ctx, i) {
                final m = filtered[i];
                final id = m['id'] as String? ?? '';
                final name = m['name'] as String? ?? '';
                final pricing = m['pricing'] as Map<String, dynamic>? ?? {};
                final promptVal = pricing['prompt'];
                final promptPrice = promptVal is double
                    ? promptVal
                    : double.tryParse(promptVal?.toString() ?? '0') ?? 0.0;
                final contextLen = m['context_length'] ?? 0;
                final isFree = promptPrice == 0.0;
                return ListTile(
                  dense: true,
                  title: Text(name, style: const TextStyle(fontSize: 13)),
                  subtitle: Text(id, style: TextStyle(fontSize: 10, color: MemoTheme.of(context).textDim), overflow: TextOverflow.ellipsis),
                  leading: Text(isFree ? '🟢' : '🟡', style: const TextStyle(fontSize: 16)),
                  trailing: Text(
                    '${contextLen ~/ 1000}K | \$${promptPrice.toStringAsFixed(promptPrice < 0.001 ? 7 : promptPrice < 1 ? 5 : 4)}/K',
                    style: TextStyle(fontSize: 9, color: MemoTheme.of(context).textDim),
                  ),
                  onTap: () => Navigator.pop(context, m),
                );
              },
            ),
          ),
        ],
      ),
    );
  }
}
