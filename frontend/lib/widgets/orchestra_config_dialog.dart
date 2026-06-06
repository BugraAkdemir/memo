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
    return Container(
      padding: const EdgeInsets.all(24),
      decoration: BoxDecoration(border: Border(bottom: BorderSide(color: MemoTheme.of(context).borderSoft))),
      child: Row(
        children: [
          const Text('🎵', style: TextStyle(fontSize: 24)),
          const SizedBox(width: 12),
          Text('Orchestra Mode', style: TextStyle(fontSize: 18, fontWeight: FontWeight.w600, color: MemoTheme.of(context).textMain)),
          const Spacer(),
          Transform.scale(scale: 0.8, child: Switch(
            value: config.enabled,
            onChanged: (v) => setState(() => _config = config.copyWith(enabled: v)),
            activeColor: MemoTheme.accent,
          )),
          Text(config.enabled ? 'Aktif' : 'Pasif', style: TextStyle(fontSize: 12, color: config.enabled ? MemoTheme.accent : MemoTheme.of(context).textDim)),
        ],
      ),
    );
  }

  Widget _buildBody(BuildContext context, OrchestraConfig config, List<_ModelChoice> modelChoices) {
    return ListView(
      padding: const EdgeInsets.all(24),
      children: [
        _sectionTitle('🧙 Şef (Chief) Model'),
        const SizedBox(height: 8),
        Text('Şef, kullanıcının isteğini analiz eder, görev dağıtır ve sonuçları sentezler.', style: TextStyle(fontSize: 12, color: MemoTheme.of(context).textDim)),
        const SizedBox(height: 10),
        _buildChiefSelector(config, modelChoices),
        const SizedBox(height: 24),
        _sectionTitle('🎭 Uzman Rolleri'),
        const SizedBox(height: 8),
        Text('Her role bir model ata. Sadece açık roller orkestrasyona katılır. Özel roller ekleyip silebilirsin.', style: TextStyle(fontSize: 12, color: MemoTheme.of(context).textDim)),
        const SizedBox(height: 12),
        ...List.generate(config.roles.length, (i) => _buildRoleCard(context, config, i, modelChoices)),
        const SizedBox(height: 8),
        OutlinedButton.icon(
          onPressed: () => _addCustomRole(config),
          icon: const Icon(Icons.add, size: 16),
          label: const Text('Özel Rol Ekle', style: TextStyle(fontSize: 12)),
          style: OutlinedButton.styleFrom(
            side: BorderSide(color: MemoTheme.of(context).borderSoft),
            foregroundColor: MemoTheme.of(context).textDim,
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
          ),
        ),
      ],
    );
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
          TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('Cancel')),
          const SizedBox(width: 12),
          FilledButton(
            onPressed: _saving ? null : () => _save(config),
            style: FilledButton.styleFrom(backgroundColor: MemoTheme.accent),
            child: _saving
                ? const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2))
                : const Text('Save'),
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
          choices.add(_ModelChoice(p.type, p.model, providerIcon(p.type), '${p.name} → ${p.model}'));
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
    final role = config.roles[index];
    final isBuiltin = _isBuiltinRole(role.role);
    final icon = isBuiltin ? OrchestraDefaults.iconForRole(role.role) : '🧩';
    final label = isBuiltin ? OrchestraDefaults.labelForRole(role.role) : role.role;
    final currentKey = '${role.modelType}/${role.modelName}';
    final validChoice = choices.any((c) => c.key == currentKey);

    return Container(
      margin: const EdgeInsets.only(bottom: 12),
      decoration: BoxDecoration(
        color: MemoTheme.of(context).bgPanel,
        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
        border: Border.all(color: role.enabled ? MemoTheme.accent.withValues(alpha: 0.3) : MemoTheme.of(context).borderSoft),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(12, 12, 12, 0),
            child: Row(
              children: [
                Text(icon, style: const TextStyle(fontSize: 16)),
                const SizedBox(width: 8),
                isBuiltin
                    ? Expanded(child: Text(label, style: TextStyle(fontSize: 13, fontWeight: FontWeight.w500, color: MemoTheme.of(context).textMain)))
                    : Expanded(
                        child: _RoleNameField(
                          initialValue: role.role,
                          onChanged: (v) {
                            final newRoles = List<RoleConfig>.from(config.roles);
                            newRoles[index] = role.copyWith(role: v);
                            setState(() => _config = config.copyWith(roles: newRoles));
                          },
                        ),
                      ),
                if (!isBuiltin)
                  IconButton(
                    icon: Icon(Icons.delete_outline, size: 16, color: MemoTheme.of(context).textDim),
                    onPressed: () {
                      final newRoles = List<RoleConfig>.from(config.roles);
                      newRoles.removeAt(index);
                      setState(() => _config = config.copyWith(roles: newRoles));
                    },
                    padding: EdgeInsets.zero,
                    constraints: const BoxConstraints(),
                    tooltip: 'Rolü sil',
                  ),
                const SizedBox(width: 8),
                Transform.scale(scale: 0.7, child: Switch(
                  value: role.enabled,
                  onChanged: (v) {
                    final newRoles = List<RoleConfig>.from(config.roles);
                    newRoles[index] = role.copyWith(enabled: v);
                    setState(() => _config = config.copyWith(roles: newRoles));
                  },
                  activeColor: MemoTheme.accent,
                )),
              ],
            ),
          ),
          Padding(
            padding: const EdgeInsets.fromLTRB(12, 4, 12, 4),
            child: Column(
              children: [
                DropdownButtonFormField<String>(
                  key: ValueKey('role_${index}_${choices.length}'),
                  value: role.enabled && validChoice ? currentKey : null,
                  hint: Text(role.enabled ? 'Provider seç' : 'Önce rolü aç', style: TextStyle(fontSize: 12, color: MemoTheme.of(context).textDim)),
                  decoration: InputDecoration(
                    isDense: true,
                    contentPadding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
                    border: OutlineInputBorder(borderRadius: BorderRadius.circular(MemoTheme.radiusMd)),
                    filled: true,
                    fillColor: role.enabled ? Colors.transparent : MemoTheme.of(context).bgElement,
                  ),
                  items: choices.map((c) => DropdownMenuItem(value: c.key, child: Text('${c.icon} ${c.label}', style: const TextStyle(fontSize: 12)))).toList(),
                  onChanged: role.enabled ? (val) {
                    if (val == null) return;
                    final choice = choices.firstWhere((c) => c.key == val);
                    final newRoles = List<RoleConfig>.from(config.roles);
                    newRoles[index] = role.copyWith(modelType: choice.type, modelName: choice.model);
                    setState(() => _config = config.copyWith(roles: newRoles));
                  } : null,
                ),
                if (role.modelType == 'openrouter') ...[
                  const SizedBox(height: 6),
                  InkWell(
                    onTap: () => _pickModelForRole(index, role),
                    child: Container(
                      width: double.infinity,
                      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
                      decoration: BoxDecoration(
                        border: Border.all(color: MemoTheme.of(context).borderSoft),
                        borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                        color: MemoTheme.of(context).bgElement,
                      ),
                      child: Row(
                        children: [
                          Expanded(
                            child: Text(
                              role.modelName.isNotEmpty ? role.modelName : 'Model seçmek için tıkla',
                              style: TextStyle(
                                fontSize: 12,
                                color: role.modelName.isNotEmpty ? MemoTheme.of(context).textMain : MemoTheme.of(context).textDim,
                              ),
                            ),
                          ),
                          const SizedBox(width: 8),
                          Icon(Icons.search, size: 14, color: MemoTheme.of(context).textDim),
                        ],
                      ),
                    ),
                  ),
                ],
              ],
            ),
          ),
          Padding(
            padding: const EdgeInsets.fromLTRB(12, 0, 12, 12),
            child: _SystemPromptField(
              initialValue: role.systemPrompt,
              onChanged: (v) {
                final newRoles = List<RoleConfig>.from(config.roles);
                newRoles[index] = role.copyWith(systemPrompt: v);
                setState(() => _config = config.copyWith(roles: newRoles));
              },
            ),
          ),
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
      models = (result['models'] as List).cast<Map<String, dynamic>>();
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
      final modelId = selected['id'] as String;
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
              separatorBuilder: (_, __) => Divider(height: 1, color: MemoTheme.of(context).borderSoft),
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
