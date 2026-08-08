import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/l10n.dart';
import '../../core/theme.dart';
import '../../providers/agent_provider.dart';
import '../../utils/tool_names.dart';
import '../../core/friendly_error.dart';

class PermissionHistory extends ConsumerWidget {
  const PermissionHistory({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final permissionsAsync = ref.watch(agentPermissionsProvider);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            Text(
              L10n.t('permanent_permissions'),
              style: const TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
            ),
            TextButton.icon(
              onPressed: () {
                showDialog(
                  context: context,
                  builder: (context) => AlertDialog(
                    title: Text(L10n.t('clear_all_permissions')),
                    content: Text(L10n.t('clear_permissions_confirm')),
                    actions: [
                      TextButton(
                          onPressed: () => Navigator.pop(context),
                          child: Text(L10n.t('cancel'))),
                      TextButton(
                        onPressed: () {
                          ref.read(agentPermissionsProvider.notifier).clearAll();
                          Navigator.pop(context);
                        },
                        style: TextButton.styleFrom(foregroundColor: MemoTheme.red),
                        child: Text(L10n.t('clear')),
                      ),
                    ],
                  ),
                );
              },
              icon: const Icon(Icons.delete_sweep, color: MemoTheme.red),
              label: Text(L10n.t('clear_all'),
                  style: const TextStyle(color: MemoTheme.red)),
            ),
          ],
        ),
        const SizedBox(height: 8),
        Text(
          L10n.t('permissions_desc'),
          style: const TextStyle(color: Colors.grey),
        ),
        const SizedBox(height: 16),
        Expanded(
          child: permissionsAsync.when(
            data: (permissions) {
              if (permissions.isEmpty) {
                return Center(
                  child: Text(L10n.t('no_permissions')),
                );
              }
              return ListView.builder(
                itemCount: permissions.length,
                itemBuilder: (context, index) {
                  final p = permissions[index];
                  final isAllowed = p.policy == 'allow_forever';
                  return Card(
                    margin: const EdgeInsets.only(bottom: 8),
                    child: ListTile(
                      leading: Icon(
                        isAllowed ? Icons.check_circle : Icons.block,
                        color: isAllowed ? MemoTheme.green : MemoTheme.red,
                      ),
                      title: Text(ToolNames.displayName(p.toolName),
                          style: const TextStyle(fontWeight: FontWeight.bold)),
                      subtitle: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          if (ToolNames.description(p.toolName).isNotEmpty)
                            Text(ToolNames.description(p.toolName),
                                style: TextStyle(
                                    fontSize: 12,
                                    color: Theme.of(context)
                                        .colorScheme
                                        .onSurfaceVariant)),
                          Text(L10n.t('permission_date', {'date': p.updatedAt}),
                              style: const TextStyle(fontSize: 11)),
                        ],
                      ),
                      isThreeLine: true,
                      trailing: IconButton(
                        icon: const Icon(Icons.delete_outline),
                        tooltip: L10n.t('revoke_permission'),
                        onPressed: () {
                          ref.read(agentPermissionsProvider.notifier).revoke(p.id);
                        },
                      ),
                    ),
                  );
                },
              );
            },
            loading: () => const Center(child: CircularProgressIndicator()),
            error: (e, st) =>
                Center(child: Text(L10n.t('engine_error', {'e': FriendlyError.describeGeneric(e)}))),
          ),
        ),
      ],
    );
  }
}
