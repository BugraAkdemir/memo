import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../core/friendly_error.dart';
import '../../core/l10n.dart';
import '../../providers/chat_provider.dart';
import '../../providers/subagent_provider.dart';

class SubagentSettings extends ConsumerStatefulWidget {
  const SubagentSettings({super.key});
  @override
  ConsumerState<SubagentSettings> createState() => _SubagentSettingsState();
}

class _SubagentSettingsState extends ConsumerState<SubagentSettings> {
  bool _saving = false;
  String? _error;
  Future<void> _save(int n) async {
    setState(() {
      _saving = true;
      _error = null;
    });
    try {
      await ref.read(apiClientProvider).setSubagentParallelism(n);
      if (mounted) ref.invalidate(subagentParallelismProvider);
    } catch (e) {
      if (mounted) setState(() => _error = FriendlyError.describeGeneric(e));
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  @override
  Widget build(BuildContext context) => ref
      .watch(subagentParallelismProvider)
      .when(
        loading: () => const LinearProgressIndicator(),
        error: (e, _) => TextButton(
          onPressed: () => ref.invalidate(subagentParallelismProvider),
          child: Text(L10n.t('subagent_load_error')),
        ),
        data: (n) => Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              L10n.t('subagent_settings'),
              style: Theme.of(context).textTheme.titleMedium,
            ),
            const SizedBox(height: 8),
            Text(L10n.t('subagent_parallel_help')),
            const SizedBox(height: 8),
            DropdownButtonFormField<int>(
              key: ValueKey('subagent-parallel-$n'),
              initialValue: n,
              decoration: InputDecoration(
                labelText: L10n.t('subagent_parallel'),
              ),
              items: [
                for (var i = 1; i <= 16; i++)
                  DropdownMenuItem(value: i, child: Text('$i')),
              ],
              onChanged: _saving
                  ? null
                  : (v) {
                      if (v != null) _save(v);
                    },
            ),
            if (_error != null) Text(_error!),
          ],
        ),
      );
}
