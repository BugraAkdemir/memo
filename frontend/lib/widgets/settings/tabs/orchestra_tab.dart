import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/theme.dart';
import '../../../providers/provider_provider.dart';
import '../../../models/orchestra_config.dart';
import '../../../providers/orchestra_provider.dart';
import '../../orchestra_config_dialog.dart';

class OrchestraTab extends ConsumerWidget {
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final configAsync = ref.watch(orchestraConfigProvider);

    return ListView(
      padding: const EdgeInsets.all(32),
      children: [
        // Header
        Row(
          children: [
            const Text('🎵', style: TextStyle(fontSize: 24)),
            const SizedBox(width: 12),
            Text(
              'Orchestra Mode',
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
          'Birden çok modeli aynı anda bir ekip olarak çalıştır. '
          'Bir Şef (Chief) model kullanıcının isteğini analiz eder, '
          'alt görevlere böler ve her görevi uzmanlaşmış modele atar.',
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
                    child: Text(
                      config.enabled ? '🎵' : '⏸️',
                      style: const TextStyle(fontSize: 20),
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
                            ? 'Orchestra Mode Aktif'
                            : 'Orchestra Mode Pasif',
                        style: TextStyle(
                          fontSize: 14,
                          fontWeight: FontWeight.w600,
                          color: MemoTheme.of(context).textMain,
                        ),
                      ),
                      const SizedBox(height: 4),
                      Text(
                        config.enabled
                            ? 'Şef: ${config.chiefType}/${config.chiefModel} • ${config.roles.where((r) => r.enabled).length} rol aktif'
                            : 'Aktifleştirmek için aç/kapa yap',
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
                    activeColor: MemoTheme.accent,
                  ),
                ),
              ],
            ),
          ),
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (e, _) => Center(child: Text('Error: $e')),
        ),
        const SizedBox(height: 20),

        // Configure button
        FilledButton.icon(
          onPressed: () => showDialog(
            context: context,
            builder: (_) => const OrchestraConfigDialog(),
          ),
          icon: const Icon(Icons.tune, size: 18),
          label: const Text('Rolleri ve Modelleri Yapılandır'),
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
                  '🎭 Aktif Roller',
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
          error: (_, __) => const SizedBox.shrink(),
        ),
      ],
    );
  }
}
