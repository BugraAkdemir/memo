import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/theme.dart';
import '../../../core/l10n.dart';
import '../../../models/provider_config.dart';
import '../../../providers/provider_provider.dart';
import '../../provider_config_dialog.dart';
import '../image_model_section.dart';
import '../decision_model_section.dart';
import '../section_tab_bar.dart';
import '../../../core/friendly_error.dart';

/// Filters out untouched placeholder entries — installs from before
/// defaultConfigs() stopped pre-seeding every known provider type
/// (internal/provider/config.go) still have these sitting in their
/// providers.json (no migration deletes existing data out from under a
/// user), and the API Providers list is exactly where that clutter was
/// reported: disabled cards for providers nobody ever actually added. A
/// provider counts as "actually added" if it's enabled, or has a real API
/// key, or a custom base URL — any one of those means a human configured
/// it, however briefly. Extracted as its own top-level function so this
/// filtering logic is unit-testable without pumping the whole widget tree.
List<ProviderConfig> visibleProviders(List<ProviderConfig> all) {
  return all
      .where(
        (p) =>
            p.enabled ||
            (p.apiKey != null && p.apiKey!.isNotEmpty) ||
            (p.baseUrl != null && p.baseUrl!.isNotEmpty),
      )
      .toList();
}

class ProvidersTab extends ConsumerStatefulWidget {
  const ProvidersTab({super.key});

  @override
  ConsumerState<ProvidersTab> createState() => _ProvidersTabState();
}

class _ProvidersTabState extends ConsumerState<ProvidersTab> {
  int _section = 0;

  @override
  Widget build(BuildContext context) {
    final providersAsync = ref.watch(providerListProvider);
    final activeType = ref.watch(activeProviderTypeProvider).valueOrNull ?? '';
    final theme = MemoTheme.of(context);
    return LayoutBuilder(
      builder: (context, constraints) {
        final inset = constraints.maxWidth < 600 ? 16.0 : 32.0;
        final padding = EdgeInsets.fromLTRB(inset, 20, inset, 32);
        return Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Padding(
              padding: EdgeInsets.fromLTRB(inset, 28, inset, 0),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    L10n.t('providers_title'),
                    style: Theme.of(context).textTheme.titleLarge?.copyWith(
                      fontWeight: FontWeight.bold,
                      color: theme.textMain,
                    ),
                  ),
                  const SizedBox(height: 18),
                  SectionTabBar(
                    selected: _section,
                    onSelected: (index) => setState(() => _section = index),
                    items: [
                      SectionTabItem(
                        Icons.dns_outlined,
                        L10n.t('providers_connections'),
                        count: visibleProviders(
                          providersAsync.valueOrNull ?? [],
                        ).length,
                      ),
                      SectionTabItem(
                        Icons.image_outlined,
                        L10n.t('providers_images'),
                      ),
                      SectionTabItem(
                        Icons.fact_check_outlined,
                        L10n.t('providers_decisions'),
                      ),
                    ],
                  ),
                ],
              ),
            ),
            const SizedBox(height: 14),
            Divider(height: 1, color: theme.borderSoft),
            Expanded(
              // Keep form drafts and scroll positions when changing sections.
              child: IndexedStack(
                index: _section,
                children: [
                  ListView(
                    padding: padding,
                    children: [
                      Wrap(
                        spacing: 16,
                        runSpacing: 12,
                        alignment: WrapAlignment.spaceBetween,
                        crossAxisAlignment: WrapCrossAlignment.center,
                        children: [
                          Text(
                            L10n.t('providers_connections'),
                            style: TextStyle(
                              fontSize: 14,
                              fontWeight: FontWeight.w600,
                              color: theme.textMain,
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
                            label: Text(L10n.t('add_provider')),
                          ),
                        ],
                      ),
                      const SizedBox(height: 8),
                      Text(
                        L10n.t('providers_description'),
                        style: TextStyle(color: theme.textDim, fontSize: 12),
                      ),
                      const SizedBox(height: 20),
                      providersAsync.when(
                        data: (allProviders) {
                          final providers = visibleProviders(allProviders);
                          if (providers.isEmpty) {
                            return Container(
                              padding: const EdgeInsets.all(24),
                              decoration: BoxDecoration(
                                color: theme.bgPanel,
                                border: Border.all(color: theme.borderSoft),
                                borderRadius: BorderRadius.circular(
                                  MemoTheme.radiusMd,
                                ),
                              ),
                              child: Column(
                                children: [
                                  Icon(
                                    Icons.dns_outlined,
                                    color: theme.textDim,
                                    size: 28,
                                  ),
                                  const SizedBox(height: 12),
                                  Text(
                                    L10n.t('no_providers'),
                                    style: TextStyle(color: theme.textMain),
                                  ),
                                  const SizedBox(height: 6),
                                  Text(
                                    L10n.t('add_provider_hint'),
                                    textAlign: TextAlign.center,
                                    style: TextStyle(
                                      color: theme.textDim,
                                      fontSize: 12,
                                    ),
                                  ),
                                ],
                              ),
                            );
                          }
                          return Column(
                            children: providers
                                .map(
                                  (p) => ProviderCard(
                                    p: p,
                                    isActive: p.name == activeType,
                                  ),
                                )
                                .toList(),
                          );
                        },
                        loading: () =>
                            const Center(child: CircularProgressIndicator()),
                        error: (e, _) => Text(
                          L10n.t('providers_error', {
                            'e': FriendlyError.describeGeneric(e),
                          }),
                        ),
                      ),
                    ],
                  ),
                  ListView(
                    padding: padding,
                    children: const [ImageModelSection()],
                  ),
                  ListView(
                    padding: padding,
                    children: const [DecisionModelSection()],
                  ),
                ],
              ),
            ),
          ],
        );
      },
    );
  }
}

class ProviderCard extends ConsumerWidget {
  final ProviderConfig p;
  final bool isActive;

  const ProviderCard({super.key, required this.p, this.isActive = false});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Container(
      margin: const EdgeInsets.only(bottom: 12),
      decoration: BoxDecoration(
        color: MemoTheme.of(context).bgPanel,
        border: Border.all(
          color: isActive ? MemoTheme.accent : MemoTheme.of(context).borderSoft,
        ),
        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
      ),
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
                        child: const Icon(
                          Icons.check,
                          size: 10,
                          color: Colors.white,
                        ),
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
                      Flexible(
                        child: Text(
                          p.name,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(
                            fontWeight: FontWeight.w600,
                            fontSize: 14,
                          ),
                        ),
                      ),
                      if (isActive) ...[
                        const SizedBox(width: 8),
                        Container(
                          padding: const EdgeInsets.symmetric(
                            horizontal: 6,
                            vertical: 1,
                          ),
                          decoration: BoxDecoration(
                            color: MemoTheme.accent.withValues(alpha: 0.15),
                            borderRadius: BorderRadius.circular(4),
                          ),
                          child: Text(
                            L10n.t('provider_active_badge'),
                            style: const TextStyle(
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
                    p.configuredModels.map((m) => m.id).join(' · '),
                    maxLines: 3,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(
                      fontSize: 12,
                      color: MemoTheme.of(context).textDim,
                    ),
                  ),
                  const SizedBox(height: 4),
                  Wrap(
                    spacing: 8,
                    runSpacing: 4,
                    children: [
                      Container(
                        padding: const EdgeInsets.symmetric(
                          horizontal: 6,
                          vertical: 2,
                        ),
                        decoration: BoxDecoration(
                          color: p.enabled
                              ? MemoTheme.green.withValues(alpha: 0.1)
                              : MemoTheme.of(
                                  context,
                                ).textDim.withValues(alpha: 0.1),
                          borderRadius: BorderRadius.circular(4),
                        ),
                        child: Text(
                          p.enabled
                              ? L10n.t('provider_enabled_badge')
                              : L10n.t('provider_disabled_badge'),
                          style: TextStyle(
                            fontSize: 11,
                            color: p.enabled
                                ? MemoTheme.green
                                : MemoTheme.of(context).textDim,
                            fontWeight: FontWeight.w500,
                          ),
                        ),
                      ),
                      if (p.connected)
                        Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            const Icon(
                              Icons.check_circle,
                              size: 14,
                              color: MemoTheme.green,
                            ),
                            const SizedBox(width: 4),
                            Text(
                              L10n.t('connected'),
                              style: const TextStyle(
                                fontSize: 11,
                                color: MemoTheme.green,
                              ),
                            ),
                          ],
                        ),
                    ],
                  ),
                  Align(
                    alignment: Alignment.centerLeft,
                    child: TextButton(
                      onPressed: () async {
                        final result = await showDialog<bool>(
                          context: context,
                          builder: (_) => ProviderConfigDialog(existing: p),
                        );
                        if (result == true) {
                          ref.invalidate(providerListProvider);
                        }
                      },
                      child: Text(
                        L10n.t('provider_manage_models'),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
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
                      title: Text(L10n.t('delete_provider')),
                      content: Text(
                        L10n.t('delete_provider_confirm', {'name': p.name}),
                      ),
                      actions: [
                        TextButton(
                          onPressed: () => Navigator.of(ctx).pop(false),
                          child: Text(L10n.t('cancel')),
                        ),
                        TextButton(
                          onPressed: () => Navigator.of(ctx).pop(true),
                          child: Text(L10n.t('delete')),
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
                  child: Row(
                    children: [
                      const Icon(Icons.edit, size: 18),
                      const SizedBox(width: 8),
                      Text(L10n.t('configure')),
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
                      Text(p.enabled ? L10n.t('disable') : L10n.t('enable')),
                    ],
                  ),
                ),
                const PopupMenuDivider(),
                PopupMenuItem(
                  value: 'delete',
                  child: Row(
                    children: [
                      const Icon(Icons.delete, size: 18, color: MemoTheme.red),
                      const SizedBox(width: 8),
                      Text(
                        L10n.t('delete'),
                        style: const TextStyle(color: MemoTheme.red),
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
