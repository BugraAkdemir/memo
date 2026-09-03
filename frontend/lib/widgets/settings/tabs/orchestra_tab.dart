import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/theme.dart';
import '../../../core/l10n.dart';
import '../../../providers/provider_provider.dart';
import '../../../models/orchestra_config.dart';
import '../../../providers/orchestra_provider.dart';
import '../../orchestra_config_dialog.dart';
import '../../svg_icon.dart';
import '../../../core/friendly_error.dart';

class OrchestraTab extends ConsumerWidget {
  const OrchestraTab({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final configAsync = ref.watch(orchestraConfigProvider);

    return ListView(
      padding: const EdgeInsets.all(32),
      children: [
        // Header
        Row(
          children: [
            SvgIcon('music-notes', size: 24, color: MemoTheme.of(context).textMain),
            const SizedBox(width: 12),
            Text(
              L10n.t('orchestra_title'),
              style: TextStyle(
                fontSize: 20,
                fontWeight: FontWeight.w600,
                color: MemoTheme.of(context).textMain,
              ),
            ),
          ],
        ),
        const SizedBox(height: 8),
        Text(
          L10n.t('orchestra_desc'),
          style: TextStyle(
            fontSize: 13,
            color: MemoTheme.of(context).textSecondary,
          ),
        ),
        const SizedBox(height: 24),

        // Status card
        configAsync.when(
          data: (config) => Container(
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(
              color: config.enabled
                  ? MemoTheme.accent.withValues(alpha: 0.1)
                  : MemoTheme.of(context).bgPanel,
              borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
              border: Border.all(
                color: config.enabled
                    ? MemoTheme.accent.withValues(alpha: 0.3)
                    : MemoTheme.of(context).borderSoft,
              ),
            ),
            child: Row(
              children: [
                Container(
                  width: 40,
                  height: 40,
                  decoration: BoxDecoration(
                    color: config.enabled
                        ? MemoTheme.accent.withValues(alpha: 0.2)
                        : MemoTheme.of(context).bgElement,
                    borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                  ),
                  child: Center(
                    child: SvgIcon(
                      config.enabled ? 'music-notes' : 'pause',
                      size: 20,
                      color: config.enabled
                          ? MemoTheme.accent
                          : MemoTheme.of(context).textDim,
                    ),
                  ),
                ),
                const SizedBox(width: 16),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        config.enabled
                            ? L10n.t('orchestra_active')
                            : L10n.t('orchestra_inactive'),
                        style: TextStyle(
                          fontSize: 14,
                          fontWeight: FontWeight.w600,
                          color: MemoTheme.of(context).textMain,
                        ),
                      ),
                      const SizedBox(height: 4),
                      Text(
                        config.enabled
                            ? L10n.t('orchestra_status', {
                                'chief': config.chiefType,
                                'model': config.chiefModel,
                                'count': '${config.roles.where((r) => r.enabled).length}',
                              })
                            : L10n.t('orchestra_hint'),
                        style: TextStyle(
                          fontSize: 12,
                          color: MemoTheme.of(context).textDim,
                        ),
                      ),
                    ],
                  ),
                ),
                const SizedBox(width: 12),
                Transform.scale(
                  scale: 0.8,
                  child: Switch(
                    value: config.enabled,
                    onChanged: (v) =>
                        ref.read(orchestraConfigProvider.notifier).toggle(v),
                    activeThumbColor: MemoTheme.accent,
                  ),
                ),
              ],
            ),
          ),
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (e, _) => Center(child: Text(L10n.t('orchestra_error', {'e': FriendlyError.describeGeneric(e)}))),
        ),
        const SizedBox(height: 20),

        // Configure button
        FilledButton.icon(
          onPressed: () => showDialog(
            context: context,
            builder: (_) => const OrchestraConfigDialog(),
          ),
          icon: const Icon(Icons.tune, size: 18),
          label: Text(L10n.t('configure_roles')),
          style: FilledButton.styleFrom(backgroundColor: MemoTheme.accent),
        ),
        const SizedBox(height: 32),

        // Role summary
        configAsync.when(
          data: (config) {
            if (!config.enabled) return const SizedBox.shrink();
            return Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  L10n.t('active_roles'),
                  style: TextStyle(
                    fontSize: 14,
                    fontWeight: FontWeight.w600,
                    color: MemoTheme.of(context).textMain,
                  ),
                ),
                const SizedBox(height: 12),
                ...config.roles
                    .where((r) => r.enabled)
                    .map(
                      (role) => Container(
                        margin: const EdgeInsets.only(bottom: 8),
                        padding: const EdgeInsets.all(12),
                        decoration: BoxDecoration(
                          color: MemoTheme.of(context).bgPanel,
                          borderRadius: BorderRadius.circular(
                            MemoTheme.radiusMd,
                          ),
                          border: Border.all(
                            color: MemoTheme.of(context).borderSoft,
                          ),
                        ),
                        child: Row(
                          children: [
                            Text(
                              OrchestraDefaults.iconForRole(role.role),
                              style: const TextStyle(fontSize: 16),
                            ),
                            const SizedBox(width: 12),
                            Expanded(
                              child: Column(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  Text(
                                    OrchestraDefaults.labelForRole(role.role),
                                    style: TextStyle(
                                      fontSize: 13,
                                      fontWeight: FontWeight.w500,
                                      color: MemoTheme.of(context).textMain,
                                    ),
                                  ),
                                  const SizedBox(height: 2),
                                  Text(
                                    '${providerIcon(role.modelType)} ${role.modelType} → ${role.modelName}',
                                    style: TextStyle(
                                      fontSize: 11,
                                      color: MemoTheme.of(context).textDim,
                                    ),
                                  ),
                                ],
                              ),
                            ),
                          ],
                        ),
                      ),
                    ),
              ],
            );
          },
          loading: () => const SizedBox.shrink(),
          error: (_, _) => const SizedBox.shrink(),
        ),
      ],
    );
  }
}
