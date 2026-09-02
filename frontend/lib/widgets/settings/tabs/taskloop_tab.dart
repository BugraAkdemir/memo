import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/l10n.dart';
import '../../../core/theme.dart';
import '../../../providers/provider_provider.dart';
import '../../../providers/taskloop_settings_provider.dart';
import '../../../providers/chat_provider.dart'
    show apiClientProvider, errorMessageProvider;
import '../../../core/friendly_error.dart';

class TaskLoopTab extends ConsumerStatefulWidget {
  const TaskLoopTab({super.key});

  @override
  ConsumerState<TaskLoopTab> createState() => _TaskLoopTabState();
}

class _TaskLoopTabState extends ConsumerState<TaskLoopTab> {
  Map<String, dynamic> _s = {};
  bool _loaded = false;
  bool _saving = false;

  static const _unset = '__unset__';
  static const _local = 'local';

  Future<void> _save(Map<String, dynamic> patch) async {
    final next = {..._s, ...patch};
    setState(() {
      _s = next;
      _saving = true;
    });
    try {
      final api = ref.read(apiClientProvider);
      _s = await api.updateTaskLoopSettings(next);
    } catch (e) {
      if (mounted) {
        ref.read(errorMessageProvider.notifier).state =
            '${L10n.t('error')}: ${FriendlyError.describeGeneric(e)}';
      }
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  List<String> _modelOptions(List<dynamic> providers) {
    final out = <String>[_unset, _local];
    for (final p in providers) {
      if (p.enabled != true) continue;
      final model = (p.model as String?)?.trim() ?? '';
      if (model.isNotEmpty && !out.contains(model)) out.add(model);
    }
    return out;
  }

  String _modelLabel(String v) {
    if (v == _unset) return L10n.t('taskloop_model_unset');
    if (v == _local) return L10n.t('taskloop_local_model');
    return v;
  }

  Widget _modelDropdown(
      ThemeColors c, String labelKey, String key, List<dynamic> providers) {
    final opts = _modelOptions(providers);
    final current = (_s[key] as String?)?.trim() ?? '';
    final value = current.isEmpty ? _unset : (opts.contains(current) ? current : _unset);
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: DropdownButtonFormField<String>(
        initialValue: value,
        decoration: InputDecoration(
          labelText: L10n.t(labelKey),
          border: const OutlineInputBorder(),
          isDense: true,
        ),
        items: opts
            .map((o) => DropdownMenuItem(value: o, child: Text(_modelLabel(o))))
            .toList(),
        onChanged: _saving
            ? null
            : (v) => _save({key: v == _unset ? '' : v}),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final settingsAsync = ref.watch(taskLoopSettingsProvider);
    final providersAsync = ref.watch(providerListProvider);

    settingsAsync.whenData((data) {
      if (!_loaded) {
        _s = Map<String, dynamic>.from(data);
        _loaded = true;
      }
    });

    return SingleChildScrollView(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            L10n.t('taskloop_settings'),
            style: TextStyle(
                fontSize: 18, fontWeight: FontWeight.w600, color: c.textMain),
          ),
          const SizedBox(height: 8),
          Text(L10n.t('taskloop_description'),
              style: TextStyle(fontSize: 13, color: c.textSecondary)),
          const SizedBox(height: 24),
          if (settingsAsync.isLoading && !_loaded)
            const Center(child: CircularProgressIndicator())
          else
            _panel(
              c,
              icon: Icons.alt_route,
              title: L10n.t('tasklist_mode_planner'),
              children: [
                providersAsync.when(
                  data: (providers) => Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      _modelDropdown(
                          c, 'taskloop_planner_model', 'planner_model', providers),
                      _modelDropdown(
                          c, 'taskloop_coder_model', 'coder_model', providers),
                      _modelDropdown(c, 'taskloop_verifier_model',
                          'verifier_model', providers),
                    ],
                  ),
                  loading: () => const LinearProgressIndicator(),
                  error: (e, _) => Text(
                    '${L10n.t('error')}: ${FriendlyError.describeGeneric(e)}',
                    style: const TextStyle(fontSize: 12, color: MemoTheme.red),
                  ),
                ),
                const SizedBox(height: 8),
                _granularity(c),
                const SizedBox(height: 4),
                SwitchListTile(
                  contentPadding: EdgeInsets.zero,
                  title: Text(L10n.t('taskloop_auto_approve')),
                  subtitle: Text(L10n.t('taskloop_auto_approve_desc'),
                      style: TextStyle(fontSize: 11, color: c.textDim)),
                  value: _s['auto_approve_plan'] == true,
                  onChanged:
                      _saving ? null : (v) => _save({'auto_approve_plan': v}),
                ),
                SwitchListTile(
                  contentPadding: EdgeInsets.zero,
                  title: Text(L10n.t('taskloop_task_memory')),
                  subtitle: Text(L10n.t('taskloop_task_memory_desc'),
                      style: TextStyle(fontSize: 11, color: c.textDim)),
                  value: _s['task_memory'] == true,
                  onChanged: _saving ? null : (v) => _save({'task_memory': v}),
                ),
                const SizedBox(height: 8),
                _intField(c, 'taskloop_max_parallel', 'max_parallel_steps'),
                _intField(c, 'taskloop_max_attempts', 'max_executor_attempts'),
                _intField(c, 'taskloop_state_budget', 'handoff_state_max_tokens'),
              ],
            ),
        ],
      ),
    );
  }

  Widget _granularity(ThemeColors c) {
    final v = (_s['step_granularity'] as String?) ?? 'hybrid';
    const opts = {
      'intent': 'taskloop_gran_intent',
      'literal': 'taskloop_gran_literal',
      'hybrid': 'taskloop_gran_hybrid',
    };
    return Row(
      children: [
        Text(L10n.t('taskloop_granularity'),
            style: TextStyle(fontSize: 13, color: c.textSecondary)),
        const SizedBox(width: 12),
        ...opts.entries.map((e) => Padding(
              padding: const EdgeInsets.only(right: 6),
              child: ChoiceChip(
                label: Text(L10n.t(e.value)),
                selected: v == e.key,
                onSelected: _saving
                    ? null
                    : (_) => _save({'step_granularity': e.key}),
              ),
            )),
      ],
    );
  }

  Widget _intField(ThemeColors c, String labelKey, String key) {
    final ctrl = TextEditingController(text: '${_s[key] ?? ''}');
    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: TextField(
        controller: ctrl,
        keyboardType: TextInputType.number,
        decoration: InputDecoration(
          labelText: L10n.t(labelKey),
          border: const OutlineInputBorder(),
          isDense: true,
        ),
        onSubmitted: _saving
            ? null
            : (raw) {
                final n = int.tryParse(raw.trim());
                if (n != null && n > 0) _save({key: n});
              },
      ),
    );
  }

  Widget _panel(ThemeColors c,
      {required IconData icon,
      required String title,
      required List<Widget> children}) {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: c.bgPanel,
        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
        border: Border.all(color: c.borderSoft),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(children: [
            Icon(icon, size: 20, color: c.textSecondary),
            const SizedBox(width: 8),
            Text(title,
                style: TextStyle(
                    fontSize: 14,
                    fontWeight: FontWeight.w600,
                    color: c.textMain)),
          ]),
          const SizedBox(height: 12),
          ...children,
        ],
      ),
    );
  }
}
