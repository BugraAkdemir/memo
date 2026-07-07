import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/theme.dart';
import '../../../models/provider_config.dart';
import '../../../providers/provider_provider.dart';
import '../../provider_config_dialog.dart';

class ProvidersTab extends ConsumerWidget {
  const ProvidersTab({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final providersAsync = ref.watch(providerListProvider);
    final activeType = ref.watch(activeProviderTypeProvider).valueOrNull ?? '';

    return ListView(
      padding: const EdgeInsets.all(32),
      children: [
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            Text(
              'API Providers',
              style: Theme.of(context).textTheme.titleLarge?.copyWith(
                fontWeight: FontWeight.bold,
                color: MemoTheme.of(context).textMain,
              ),
            ),
            FilledButton.icon(
              onPressed: () async {
                final result = await showDialog<bool>(
                  context: context,
                  builder: (_) => const ProviderConfigDialog(),
                );
                if (result == true) {
                  ref.invalidate(providerListProvider);
                }
              },
              icon: const Icon(Icons.add, size: 18),
              label: const Text('Add Provider'),
            ),
          ],
        ),
        const SizedBox(height: 8),
        Text(
          'Configure external LLM providers (OpenAI, Claude, Gemini, etc.)',
          style: TextStyle(color: MemoTheme.of(context).textDim, fontSize: 13),
        ),
        const SizedBox(height: 24),

        providersAsync.when(
          data: (providers) {
            if (providers.isEmpty) {
              return Container(
                padding: const EdgeInsets.all(32),
                decoration: BoxDecoration(
                  border: Border.all(color: MemoTheme.of(context).borderSoft),
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Column(
                  children: [
                    const SizedBox(height: 8),
                    Text(
                      'No providers configured yet.',
                      style: TextStyle(color: MemoTheme.of(context).textDim),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      'Click "Add Provider" to get started.',
                      style: TextStyle(
                        color: MemoTheme.of(context).textDim,
                        fontSize: 12,
                      ),
                    ),
                  ],
                ),
              );
            }

            return Column(
              children: providers.map((p) => ProviderCard(p: p, isActive: p.name == activeType)).toList(),
            );
          },
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (e, _) => Text('Error: $e'),
        ),
      ],
    );
  }
}

class ProviderCard extends ConsumerWidget {
  final ProviderConfig p;
  final bool isActive;

  const ProviderCard({super.key, required this.p, this.isActive = false});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Card(
      margin: const EdgeInsets.only(bottom: 12),
      child: Container(
        decoration: isActive ? BoxDecoration(
          border: Border.all(color: MemoTheme.accent, width: 2),
          borderRadius: BorderRadius.circular(12),
        ) : null,
        child: Padding(
        padding: const EdgeInsets.all(16),
        child: Row(
          children: [
            // Icon
            Container(
              width: 48,
              height: 48,
              decoration: BoxDecoration(
                color: p.enabled
                    ? MemoTheme.accent.withValues(alpha: 0.1)
                    : MemoTheme.of(context).borderSoft,
                borderRadius: BorderRadius.circular(12),
              ),
              child: Stack(
                children: [
                  Center(child: providerLogoWidget(p.type, size: 28)),
                  if (isActive)
                    Positioned(
                      top: -2,
                      right: -2,
                      child: Container(
                        width: 14,
                        height: 14,
                        decoration: const BoxDecoration(
                          color: MemoTheme.green,
                          shape: BoxShape.circle,
                        ),
                        child: const Icon(Icons.check, size: 10, color: Colors.white),
                      ),
                    ),
                ],
              ),
            ),
            const SizedBox(width: 16),

            // Info
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Text(
                        p.name,
                        style: const TextStyle(
                          fontWeight: FontWeight.w600,
                          fontSize: 14,
                        ),
                      ),
                      if (isActive) ...[
                        const SizedBox(width: 8),
                        Container(
                          padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 1),
                          decoration: BoxDecoration(
                            color: MemoTheme.accent.withValues(alpha: 0.15),
                            borderRadius: BorderRadius.circular(4),
                          ),
                          child: const Text(
                            'ACTIVE',
                            style: TextStyle(
                              fontSize: 9,
                              fontWeight: FontWeight.bold,
                              color: MemoTheme.accent,
                            ),
                          ),
                        ),
                      ],
                    ],
                  ),
                  const SizedBox(height: 2),
                  Text(
                    p.model,
                    style: TextStyle(
                      fontSize: 12,
                      color: MemoTheme.of(context).textDim,
                    ),
                  ),
                  const SizedBox(height: 4),
                  Row(
                    children: [
                      Container(
                        padding: const EdgeInsets.symmetric(
                          horizontal: 6,
                          vertical: 2,
                        ),
                        decoration: BoxDecoration(
                          color: p.enabled
                              ? MemoTheme.green.withValues(alpha: 0.1)
                              : MemoTheme.of(context).textDim.withValues(alpha: 0.1),
                          borderRadius: BorderRadius.circular(4),
                        ),
                        child: Text(
                          p.enabled ? 'Enabled' : 'Disabled',
                          style: TextStyle(
                            fontSize: 11,
                            color: p.enabled ? MemoTheme.green : MemoTheme.of(context).textDim,
                            fontWeight: FontWeight.w500,
                          ),
                        ),
                      ),
                      const SizedBox(width: 8),
                      if (p.connected)
                        const Row(
                          children: [
                            Icon(
                              Icons.check_circle,
                              size: 14,
                              color: MemoTheme.green,
                            ),
                            SizedBox(width: 4),
                            Text(
                              'Connected',
                              style: TextStyle(
                                fontSize: 11,
                                color: MemoTheme.green,
                              ),
                            ),
                          ],
                        ),
                    ],
                  ),
                ],
              ),
            ),

            // Actions
            PopupMenuButton<String>(
              onSelected: (value) async {
                if (value == 'configure') {
                  final result = await showDialog<bool>(
                    context: context,
                    builder: (_) => ProviderConfigDialog(existing: p),
                  );
                  if (result == true) {
                    ref.invalidate(providerListProvider);
                  }
                } else if (value == 'delete') {
                  final confirm = await showDialog<bool>(
                    context: context,
                    builder: (ctx) => AlertDialog(
                      title: const Text('Delete Provider'),
                      content: Text('Delete ${p.name} configuration?'),
                      actions: [
                        TextButton(
                          onPressed: () => Navigator.of(ctx).pop(false),
                          child: const Text('Cancel'),
                        ),
                        TextButton(
                          onPressed: () => Navigator.of(ctx).pop(true),
                          child: const Text('Delete'),
                        ),
                      ],
                    ),
                  );
                  if (confirm == true) {
                    await ref
                        .read(providerListProvider.notifier)
                        .deleteProvider(p.type, name: p.name);
                  }
                } else if (value == 'toggle') {
                  await ref
                      .read(providerListProvider.notifier)
                      .updateProvider(p.copyWith(enabled: !p.enabled));
                }
              },
              itemBuilder: (_) => [
                PopupMenuItem(
                  value: 'configure',
                  child: const Row(
                    children: [
                      Icon(Icons.edit, size: 18),
                      SizedBox(width: 8),
                      Text('Configure'),
                    ],
                  ),
                ),
                PopupMenuItem(
                  value: 'toggle',
                  child: Row(
                    children: [
                      Icon(
                        p.enabled ? Icons.toggle_on : Icons.toggle_off,
                        size: 18,
                      ),
                      const SizedBox(width: 8),
                      Text(p.enabled ? 'Disable' : 'Enable'),
                    ],
                  ),
                ),
                const PopupMenuDivider(),
                PopupMenuItem(
                  value: 'delete',
                  child: const Row(
                    children: [
                      Icon(Icons.delete, size: 18, color: MemoTheme.red),
                      SizedBox(width: 8),
                      Text('Delete', style: TextStyle(color: MemoTheme.red)),
                    ],
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    ),
  );
  }
}
