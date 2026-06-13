import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:file_picker/file_picker.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/orchestra_config.dart';
import '../models/provider_config.dart';
import '../providers/settings_provider.dart';
import '../providers/models_provider.dart';
import '../providers/chat_provider.dart';
import '../providers/orchestra_provider.dart';
import '../providers/provider_provider.dart';
import 'orchestra_config_dialog.dart';
import 'provider_config_dialog.dart';
import 'agent/permission_history.dart';

/// Settings dialog with vertical tabs on the left and content on the right.
class SettingsDialog extends ConsumerStatefulWidget {
  SettingsDialog({super.key});

  @override
  ConsumerState<SettingsDialog> createState() => _SettingsDialogState();
}

class _SettingsDialogState extends ConsumerState<SettingsDialog> {
  int _activeTab = 0;

  List<String> get _tabs => [
    L10n.t('general'),
    L10n.t('system_prompt'),
    L10n.t('incognito_prompt'),
    L10n.t('memory'),
    L10n.t('tab_providers'),
    L10n.t('tab_orchestra'),
    L10n.t('tab_agent_permissions'),
    L10n.t('tab_gpu_config'),
    L10n.t('backup'),
    L10n.t('remote_access'),
    L10n.t('about'),
  ];

  @override
  Widget build(BuildContext context) {
    ref.watch(localeProvider);
    final tabs = _tabs;

    return Dialog(
      backgroundColor: MemoTheme.of(context).bgApp,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(MemoTheme.radiusLg),
      ),
      child: Container(
        width: 800,
        height: 600,
        clipBehavior: Clip.antiAlias,
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(MemoTheme.radiusLg),
        ),
        child: Column(
          children: [
            // ─── Header ─────────────────────────────────
            Container(
              height: 56,
              padding: EdgeInsets.symmetric(horizontal: 24),
              decoration: BoxDecoration(
                color: MemoTheme.of(context).bgPanel,
                border: Border(
                  bottom: BorderSide(color: MemoTheme.of(context).borderSoft),
                ),
              ),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Text(
                    L10n.t('settings'),
                    style: TextStyle(
                      fontSize: 16,
                      fontWeight: FontWeight.w600,
                      color: MemoTheme.of(context).textMain,
                    ),
                  ),
                  IconButton(
                    icon: Icon(Icons.close, size: 20),
                    onPressed: () => Navigator.of(context).pop(),
                    color: MemoTheme.of(context).textDim,
                  ),
                ],
              ),
            ),

            // ─── Body (Tabs + Content) ──────────────────
            Expanded(
              child: Row(
                children: [
                  // Tab List
                  Container(
                    width: 200,
                    decoration: BoxDecoration(
                      color: MemoTheme.of(
                        context,
                      ).bgPanel.withValues(alpha: 0.5),
                      border: Border(
                        right: BorderSide(
                          color: MemoTheme.of(context).borderSoft,
                        ),
                      ),
                    ),
                    child: ListView.builder(
                      padding: EdgeInsets.symmetric(vertical: 12),
                      itemCount: tabs.length,
                      itemBuilder: (context, index) {
                        final isActive = _activeTab == index;
                        return InkWell(
                          onTap: () => setState(() => _activeTab = index),
                          child: Container(
                            padding: EdgeInsets.symmetric(
                              horizontal: 24,
                              vertical: 12,
                            ),
                            decoration: BoxDecoration(
                              color: isActive
                                  ? MemoTheme.of(context).bgElement
                                  : Colors.transparent,
                              border: Border(
                                left: BorderSide(
                                  color: isActive
                                      ? MemoTheme.accent
                                      : Colors.transparent,
                                  width: 3,
                                ),
                              ),
                            ),
                            child: Text(
                              tabs[index],
                              style: TextStyle(
                                fontSize: 13,
                                fontWeight: isActive
                                    ? FontWeight.w600
                                    : FontWeight.w500,
                                color: isActive
                                    ? MemoTheme.of(context).textMain
                                    : MemoTheme.of(context).textSecondary,
                              ),
                            ),
                          ),
                        );
                      },
                    ),
                  ),

                  // Tab Content
                  Expanded(
                    child: Container(
                      color: MemoTheme.of(context).bgApp,
                      child: _buildTabContent(_activeTab),
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildTabContent(int index) {
    switch (index) {
      case 0:
        return _GeneralTab();
      case 1:
        return _SystemPromptTab();
      case 2:
        return _IncognitoPromptTab();
      case 3:
        return _MemoryTab();
      case 4:
        return _ProvidersTab();
      case 5:
        return _OrchestraTab();
      case 6:
        return _AgentPermissionsTab();
      case 7:
        return _GpuConfigTab();
      case 8:
        return _BackupRestoreTab();
      case 9:
        return _RemoteAccessTab();
      case 10:
        return _AboutTab();
      default:
        return SizedBox.shrink();
    }
  }
}

class _AgentPermissionsTab extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    return const Padding(
      padding: EdgeInsets.all(32),
      child: PermissionHistory(),
    );
  }
}

// ─── API Providers Tab ─────────────────────────────────────────────

class _ProvidersTab extends ConsumerWidget {
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
              children: providers.map((p) => _ProviderCard(p: p, isActive: p.type == activeType)).toList(),
            );
          },
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (e, _) => Text('Error: $e'),
        ),
      ],
    );
  }
}

class _ProviderCard extends ConsumerWidget {
  final ProviderConfig p;
  final bool isActive;

  const _ProviderCard({required this.p, this.isActive = false});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final icon = providerIcon(p.type);

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
                  Center(child: Text(icon, style: const TextStyle(fontSize: 24))),
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
                        .deleteProvider(p.type);
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
                        p.enabled ? Icons.toggle_off : Icons.toggle_on,
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

// ─── Orchestra Mode Tab ──────────────────────────────────────────

class _OrchestraTab extends ConsumerWidget {
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

class _GeneralTab extends ConsumerWidget {
  _GeneralTab();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final locale = ref.watch(localeProvider);
    final memoryEnabledAsync = ref.watch(memoryEnabledProvider);

    return ListView(
      padding: EdgeInsets.all(32),
      children: [
        Text(
          L10n.t('general'),
          style: Theme.of(context).textTheme.titleLarge?.copyWith(
            fontWeight: FontWeight.bold,
            color: MemoTheme.of(context).textMain,
          ),
        ),
        SizedBox(height: 32),

        // Language Selection
        Text(
          L10n.t('language'),
          style: TextStyle(
            fontSize: 14,
            fontWeight: FontWeight.w600,
            color: MemoTheme.of(context).textMain,
          ),
        ),
        SizedBox(height: 12),
        Container(
          padding: EdgeInsets.symmetric(horizontal: 16),
          decoration: BoxDecoration(
            color: MemoTheme.of(context).bgPanel,
            borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
            border: Border.all(color: MemoTheme.of(context).borderSoft),
          ),
          child: DropdownButtonHideUnderline(
            child: DropdownButton<MemoLocale>(
              value: locale,
              isExpanded: true,
              dropdownColor: MemoTheme.of(context).bgPanel,
              icon: Icon(
                Icons.arrow_drop_down,
                color: MemoTheme.of(context).textDim,
              ),
              items: const [
                DropdownMenuItem(value: MemoLocale.tr, child: Text('Türkçe')),
                DropdownMenuItem(value: MemoLocale.en, child: Text('English')),
              ],
              onChanged: (val) {
                if (val != null) {
                  ref.read(localeProvider.notifier).setLocale(val);
                }
              },
            ),
          ),
        ),

        SizedBox(height: 32),

        // Theme Selection
        Text(
          'Tema',
          style: TextStyle(
            fontSize: 14,
            fontWeight: FontWeight.w600,
            color: MemoTheme.of(context).textMain,
          ),
        ),
        SizedBox(height: 12),
        Container(
          padding: EdgeInsets.symmetric(horizontal: 16),
          decoration: BoxDecoration(
            color: MemoTheme.of(context).bgPanel,
            borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
            border: Border.all(color: MemoTheme.of(context).borderSoft),
          ),
          child: DropdownButtonHideUnderline(
            child: DropdownButton<String>(
              value: ref.watch(themeModeProvider),
              isExpanded: true,
              dropdownColor: MemoTheme.of(context).bgPanel,
              icon: Icon(
                Icons.arrow_drop_down,
                color: MemoTheme.of(context).textDim,
              ),
              items: const [
                DropdownMenuItem(
                  value: 'system',
                  child: Text('Sistem Varsayılanı'),
                ),
                DropdownMenuItem(value: 'light', child: Text('Açık')),
                DropdownMenuItem(value: 'dark', child: Text('Koyu')),
              ],
              onChanged: (val) {
                if (val != null) {
                  ref.read(themeModeProvider.notifier).setMode(val);
                }
              },
            ),
          ),
        ),

        SizedBox(height: 32),

        // Streaming Toggle
        Text(
          'Anlık Gösterim',
          style: TextStyle(
            fontSize: 14,
            fontWeight: FontWeight.w600,
            color: MemoTheme.of(context).textMain,
          ),
        ),
        SizedBox(height: 12),
        Container(
          padding: EdgeInsets.all(16),
          decoration: BoxDecoration(
            color: MemoTheme.of(context).bgPanel,
            borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
            border: Border.all(color: MemoTheme.of(context).borderSoft),
          ),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      ref.watch(streamingEnabledProvider) ? 'Açık' : 'Kapalı',
                      style: TextStyle(
                        fontSize: 14,
                        color: MemoTheme.of(context).textMain,
                      ),
                    ),
                    SizedBox(height: 4),
                    Text(
                      'Kapalıyken yanıt tamamlandığında tek seferde gösterilir.',
                      style: TextStyle(
                        fontSize: 12,
                        color: MemoTheme.of(context).textDim,
                      ),
                    ),
                  ],
                ),
              ),
              Switch(
                value: ref.watch(streamingEnabledProvider),
                activeColor: MemoTheme.accent,
                onChanged: (_) {
                  ref.read(streamingEnabledProvider.notifier).toggle();
                },
              ),
            ],
          ),
        ),

        SizedBox(height: 32),

        // Memory Toggle
        Text(
          L10n.t('memory_section'),
          style: TextStyle(
            fontSize: 14,
            fontWeight: FontWeight.w600,
            color: MemoTheme.of(context).textMain,
          ),
        ),
        SizedBox(height: 12),
        Container(
          padding: EdgeInsets.all(16),
          decoration: BoxDecoration(
            color: MemoTheme.of(context).bgPanel,
            borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
            border: Border.all(color: MemoTheme.of(context).borderSoft),
          ),
          child: memoryEnabledAsync.when(
            loading: () => SizedBox(
              height: 24,
              child: Center(child: CircularProgressIndicator(strokeWidth: 2)),
            ),
            error: (e, _) => Text('${L10n.t('error')}: $e'),
            data: (enabled) => Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        enabled ? L10n.t('memory_active') : L10n.t('memory_disabled'),
                        style: TextStyle(
                          fontSize: 14,
                          color: MemoTheme.of(context).textMain,
                        ),
                      ),
                      SizedBox(height: 4),
                      Text(
                        L10n.t('memory_toggle_desc'),
                        style: TextStyle(
                          fontSize: 12,
                          color: MemoTheme.of(context).textDim,
                        ),
                      ),
                    ],
                  ),
                ),
                Switch(
                  value: enabled,
                  activeColor: MemoTheme.accent,
                  onChanged: (_) async {
                    try {
                      await ref.read(memoryEnabledProvider.notifier).toggle();
                    } catch (e) {
                      if (context.mounted) {
                        ScaffoldMessenger.of(context).showSnackBar(
                          SnackBar(content: Text('${L10n.t('error')}: $e')),
                        );
                      }
                    }
                  },
                ),
              ],
            ),
          ),
        ),

        SizedBox(height: 32),

        // ─── Beta Features ──────────────────────────────────────
        const SizedBox(height: 32),
        Text(
          'Beta Özellikler',
          style: TextStyle(
            fontSize: 14,
            fontWeight: FontWeight.w600,
            color: MemoTheme.of(context).textMain,
          ),
        ),
        const SizedBox(height: 12),
        Container(
          padding: const EdgeInsets.all(16),
          decoration: BoxDecoration(
            color: MemoTheme.of(context).bgPanel,
            borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
            border: Border.all(color: MemoTheme.of(context).borderSoft),
          ),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      ref.watch(betaFeaturesProvider)
                          ? 'Beta Açık'
                          : 'Beta Kapalı',
                      style: TextStyle(
                        fontSize: 14,
                        color: MemoTheme.of(context).textMain,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      'Beta özellikler (WhatsApp entegrasyonu vb.) açık/kapalı.',
                      style: TextStyle(
                        fontSize: 12,
                        color: MemoTheme.of(context).textDim,
                      ),
                    ),
                  ],
                ),
              ),
              Switch(
                value: ref.watch(betaFeaturesProvider),
                activeColor: MemoTheme.accent,
                onChanged: (value) async {
                  // If turning ON, show a warning dialog first
                  if (value) {
                    final confirmed = await showDialog<bool>(
                      context: context,
                      builder: (ctx) => AlertDialog(
                        title: const Row(
                          children: [
                            Icon(
                              Icons.warning_amber_rounded,
                              color: MemoTheme.warningOrange,
                              size: 24,
                            ),
                            SizedBox(width: 12),
                            Text('Beta Özellikler'),
                          ],
                        ),
                        content: const Text(
                          'Beta özellikler henüz tam kararlı değildir.\n\n'
                          '• WhatsApp entegrasyonu deneysel aşamadadır\n'
                          '• Beklenmeyen hatalarla karşılaşabilirsiniz\n'
                          '• Veri kaybı yaşanabilir\n'
                          '• Bazı özellikler beklendiği gibi çalışmayabilir\n\n'
                          'Devam etmek istediğinize emin misiniz?',
                        ),
                        actions: [
                          TextButton(
                            onPressed: () => Navigator.of(ctx).pop(false),
                            child: const Text('İptal'),
                          ),
                          ElevatedButton(
                            onPressed: () => Navigator.of(ctx).pop(true),
                            style: ElevatedButton.styleFrom(
                              backgroundColor: MemoTheme.warningOrange,
                              foregroundColor: Colors.white,
                            ),
                            child: const Text('Evet, Aç'),
                          ),
                        ],
                      ),
                    );
                    if (confirmed == true) {
                      ref.read(betaFeaturesProvider.notifier).toggle();
                    }
                  } else {
                    // Turning OFF — no warning needed
                    ref.read(betaFeaturesProvider.notifier).toggle();
                  }
                },
              ),
            ],
          ),
        ),

        const SizedBox(height: 32),

        // Reset Setup Wizard
        Text(
          'Kurulum',
          style: TextStyle(
            fontSize: 14,
            fontWeight: FontWeight.w600,
            color: MemoTheme.of(context).textMain,
          ),
        ),
        SizedBox(height: 12),
        Container(
          padding: EdgeInsets.all(16),
          decoration: BoxDecoration(
            color: MemoTheme.of(context).bgPanel,
            borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
            border: Border.all(color: MemoTheme.of(context).borderSoft),
          ),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                'Kurulumu Sıfırla',
                style: TextStyle(
                  fontSize: 14,
                  color: MemoTheme.of(context).textMain,
                ),
              ),
              OutlinedButton(
                onPressed: () {
                  ref.read(setupCompleteProvider.notifier).resetSetup();
                  Navigator.of(context).pop();
                },
                child: Text(L10n.t('reset')),
              ),
            ],
          ),
        ),
      ],
    );
  }
}

class _SystemPromptTab extends ConsumerStatefulWidget {
  _SystemPromptTab();

  @override
  ConsumerState<_SystemPromptTab> createState() => _SystemPromptTabState();
}

class _SystemPromptTabState extends ConsumerState<_SystemPromptTab> {
  final _controller = TextEditingController();

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final asyncPrompt = ref.watch(systemPromptProvider);

    return ListView(
      padding: EdgeInsets.all(32),
      children: [
        Text(
          L10n.t('system_prompt'),
          style: Theme.of(context).textTheme.titleLarge?.copyWith(
            fontWeight: FontWeight.bold,
            color: MemoTheme.of(context).textMain,
          ),
        ),
        SizedBox(height: 12),
        Text(
          'Modelin temel davranışını, kimliğini ve sınırlarını belirleyen ana yönerge.',
          style: TextStyle(color: MemoTheme.of(context).textDim, fontSize: 13),
        ),
        SizedBox(height: 24),

        asyncPrompt.when(
          loading: () => Center(child: CircularProgressIndicator()),
          error: (e, _) => Text('${L10n.t('error')}: $e'),
          data: (prompt) {
            if (_controller.text != prompt) {
              _controller.text = prompt;
            }
            return Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                TextField(
                  controller: _controller,
                  maxLines: 12,
                  style: TextStyle(fontSize: 13, fontFamily: 'JetBrains Mono'),
                  decoration: InputDecoration(alignLabelWithHint: true),
                ),
                SizedBox(height: 16),
                Row(
                  mainAxisAlignment: MainAxisAlignment.end,
                  children: [
                    TextButton(
                      onPressed: () {
                        ref.read(systemPromptProvider.notifier).reset();
                      },
                      child: Text(L10n.t('reset_prompt')),
                    ),
                    SizedBox(width: 12),
                    ElevatedButton(
                      onPressed: () {
                        ref
                            .read(systemPromptProvider.notifier)
                            .save(_controller.text);
                        ScaffoldMessenger.of(context).showSnackBar(
                          SnackBar(content: Text('${L10n.t('save')} başarılı')),
                        );
                      },
                      child: Text(L10n.t('save')),
                    ),
                  ],
                ),
              ],
            );
          },
        ),
      ],
    );
  }
}

class _IncognitoPromptTab extends ConsumerStatefulWidget {
  _IncognitoPromptTab();

  @override
  ConsumerState<_IncognitoPromptTab> createState() =>
      _IncognitoPromptTabState();
}

class _IncognitoPromptTabState extends ConsumerState<_IncognitoPromptTab> {
  final _controller = TextEditingController();
  bool _initialized = false;

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final asyncPrompt = ref.watch(incognitoPromptProvider);

    return ListView(
      padding: EdgeInsets.all(32),
      children: [
        Text(
          L10n.t('incognito_prompt'),
          style: Theme.of(context).textTheme.titleLarge?.copyWith(
            fontWeight: FontWeight.bold,
            color: MemoTheme.of(context).textMain,
          ),
        ),
        SizedBox(height: 12),
        Text(
          'Gizli moddayken modelin hafızaya erişmeden nasıl davranması gerektiğini belirten yönerge.',
          style: TextStyle(color: MemoTheme.of(context).textDim, fontSize: 13),
        ),
        SizedBox(height: 24),

        asyncPrompt.when(
          loading: () => Center(child: CircularProgressIndicator()),
          error: (e, _) => Text('${L10n.t('error')}: $e'),
          data: (prompt) {
            // Always update controller text; _initialized prevents overwriting user edits on rebuild
            if (!_initialized || _controller.text.isEmpty) {
              _controller.text = prompt;
              _initialized = true;
            }
            return Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                TextField(
                  controller: _controller,
                  maxLines: 12,
                  style: TextStyle(fontSize: 13, fontFamily: 'JetBrains Mono'),
                ),
                SizedBox(height: 16),
                Row(
                  mainAxisAlignment: MainAxisAlignment.end,
                  children: [
                    ElevatedButton(
                      onPressed: () {
                        ref
                            .read(incognitoPromptProvider.notifier)
                            .save(_controller.text);
                        ScaffoldMessenger.of(context).showSnackBar(
                          SnackBar(content: Text('${L10n.t('save')} başarılı')),
                        );
                      },
                      child: Text(L10n.t('save')),
                    ),
                  ],
                ),
              ],
            );
          },
        ),
      ],
    );
  }
}

class _MemoryTab extends ConsumerStatefulWidget {
  _MemoryTab();

  @override
  ConsumerState<_MemoryTab> createState() => _MemoryTabState();
}

class _MemoryTabState extends ConsumerState<_MemoryTab> {
  final _topKController = TextEditingController();
  final _minSimilarityController = TextEditingController();
  bool _settingsInitialized = false;
  bool _savingSettings = false;

  @override
  void dispose() {
    _topKController.dispose();
    _minSimilarityController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final memoryAsync = ref.watch(memoryFilesProvider);
    final settingsAsync = ref.watch(memorySettingsProvider);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Expanded(
          child: ListView(
            padding: EdgeInsets.all(32),
            children: [
              Text(
                L10n.t('memory'),
                style: Theme.of(context).textTheme.titleLarge?.copyWith(
                  fontWeight: FontWeight.bold,
                  color: MemoTheme.of(context).textMain,
                ),
              ),
              SizedBox(height: 8),
              Text(
                L10n.t('memory_advanced_hint'),
                style: TextStyle(
                  color: MemoTheme.of(context).textDim,
                  fontSize: 13,
                ),
              ),
              SizedBox(height: 20),
              settingsAsync.when(
                loading: () => Center(child: CircularProgressIndicator()),
                error: (e, _) => Text('${L10n.t('error')}: $e'),
                data: (settings) {
                  if (!_settingsInitialized) {
                    _topKController.text = settings.topK.toString();
                    _minSimilarityController.text = settings.minSimilarity
                        .toStringAsFixed(2);
                    _settingsInitialized = true;
                  }

                  return Container(
                    padding: EdgeInsets.all(16),
                    decoration: BoxDecoration(
                      color: MemoTheme.of(context).bgPanel,
                      borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                      border: Border.all(
                        color: MemoTheme.of(context).borderSoft,
                      ),
                    ),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        Text(
                          L10n.t('memory_retrieval_settings'),
                          style: TextStyle(
                            fontSize: 14,
                            fontWeight: FontWeight.w600,
                            color: MemoTheme.of(context).textMain,
                          ),
                        ),
                        SizedBox(height: 14),
                        _MemorySettingField(
                          label: L10n.t('memory_top_k'),
                          controller: _topKController,
                          hint: '3',
                          inputFormatters: [
                            FilteringTextInputFormatter.digitsOnly,
                          ],
                        ),
                        SizedBox(height: 12),
                        _MemorySettingField(
                          label: L10n.t('memory_min_similarity'),
                          controller: _minSimilarityController,
                          hint: '0.25',
                          inputFormatters: [
                            FilteringTextInputFormatter.allow(
                              RegExp(r'^\d*\.?\d{0,2}'),
                            ),
                          ],
                        ),
                        SizedBox(height: 14),
                        Row(
                          mainAxisAlignment: MainAxisAlignment.end,
                          children: [
                            ElevatedButton(
                              onPressed: _savingSettings
                                  ? null
                                  : () async {
                                      final topK =
                                          int.tryParse(_topKController.text) ??
                                          settings.topK;
                                      final minSimilarity =
                                          double.tryParse(
                                            _minSimilarityController.text,
                                          ) ??
                                          settings.minSimilarity;
                                      final messenger = ScaffoldMessenger.of(
                                        context,
                                      );

                                      setState(() => _savingSettings = true);
                                      try {
                                        await ref
                                            .read(
                                              memorySettingsProvider.notifier,
                                            )
                                            .save(
                                              topK: topK,
                                              minSimilarity: minSimilarity,
                                            );
                                        if (mounted) {
                                          messenger.showSnackBar(
                                            SnackBar(
                                              content: Text(L10n.t('saved')),
                                            ),
                                          );
                                        }
                                      } catch (e) {
                                        if (mounted) {
                                          messenger.showSnackBar(
                                            SnackBar(
                                              content: Text(
                                                '${L10n.t('error')}: $e',
                                              ),
                                            ),
                                          );
                                        }
                                      } finally {
                                        if (mounted) {
                                          setState(
                                            () => _savingSettings = false,
                                          );
                                        }
                                      }
                                    },
                              child: _savingSettings
                                  ? SizedBox(
                                      width: 14,
                                      height: 14,
                                      child: CircularProgressIndicator(
                                        strokeWidth: 2,
                                      ),
                                    )
                                  : Text(L10n.t('save')),
                            ),
                          ],
                        ),
                      ],
                    ),
                  );
                },
              ),
              SizedBox(height: 28),
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Text(
                    L10n.t('memory_files'),
                    style: Theme.of(context).textTheme.titleMedium?.copyWith(
                      fontWeight: FontWeight.bold,
                      color: MemoTheme.of(context).textMain,
                    ),
                  ),
                  OutlinedButton.icon(
                    icon: Icon(Icons.delete_sweep, size: 18),
                    label: Text(L10n.t('clear_memory')),
                    style: OutlinedButton.styleFrom(
                      foregroundColor: MemoTheme.red,
                      side: BorderSide(color: MemoTheme.red),
                    ),
                    onPressed: () async {
                      final confirmed = await showDialog<bool>(
                        context: context,
                        builder: (ctx) => AlertDialog(
                          backgroundColor: MemoTheme.of(context).bgPanel,
                          title: Text(L10n.t('clear_memory_title')),
                          content: Text(
                            L10n.t('clear_memory_confirm_ext'),
                          ),
                          actions: [
                            TextButton(
                              onPressed: () => Navigator.pop(ctx, false),
                              child: Text(L10n.t('cancel')),
                            ),
                            TextButton(
                              onPressed: () => Navigator.pop(ctx, true),
                              style: TextButton.styleFrom(
                                foregroundColor: MemoTheme.red,
                              ),
                              child: Text(L10n.t('clear_memory')),
                            ),
                          ],
                        ),
                      );
                      if (confirmed == true) {
                        try {
                          await ref.read(memoryFilesProvider.notifier).clearAll();
                        } catch (e) {
                          if (context.mounted) {
                            ScaffoldMessenger.of(context).showSnackBar(
                              SnackBar(content: Text('${L10n.t('error')}: $e')),
                            );
                          }
                        }
                      }
                    },
                  ),
                ],
              ),
              SizedBox(height: 12),
              memoryAsync.when(
                loading: () => Center(child: CircularProgressIndicator()),
                error: (e, _) => Center(child: Text('${L10n.t('error')}: $e')),
                data: (files) {
                  if (files.isEmpty) {
                    return Padding(
                      padding: EdgeInsets.only(top: 40),
                      child: Center(
                        child: Text(
                          L10n.t('no_memory_files'),
                          style: TextStyle(
                            color: MemoTheme.of(context).textDim,
                          ),
                        ),
                      ),
                    );
                  }

                  return Column(
                    children: files.map((file) {
                      return Column(
                        children: [
                          ListTile(
                            contentPadding: EdgeInsets.zero,
                            title: Text(
                              file.name,
                              style: TextStyle(fontWeight: FontWeight.w500),
                            ),
                            subtitle: Text(
                              '${file.sizeKb} KB • ${file.modified}',
                              style: TextStyle(
                                color: MemoTheme.of(context).textDim,
                                fontSize: 12,
                              ),
                            ),
                            trailing: IconButton(
                              icon: Icon(Icons.delete_outline),
                              color: MemoTheme.red,
                              onPressed: () async {
                                try {
                                  await ref
                                      .read(memoryFilesProvider.notifier)
                                      .deleteFile(file.path);
                                } catch (e) {
                                  if (context.mounted) {
                                    ScaffoldMessenger.of(context).showSnackBar(
                                      SnackBar(content: Text('${L10n.t('error')}: $e')),
                                    );
                                  }
                                }
                              },
                            ),
                          ),
                          Divider(),
                        ],
                      );
                    }).toList(),
                  );
                },
              ),
            ],
          ),
        ),
      ],
    );
  }
}

class _MemorySettingField extends StatelessWidget {
  final String label;
  final TextEditingController controller;
  final String hint;
  final List<TextInputFormatter> inputFormatters;

  _MemorySettingField({
    required this.label,
    required this.controller,
    required this.hint,
    required this.inputFormatters,
  });

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        SizedBox(
          width: 170,
          child: Text(
            label,
            style: TextStyle(fontWeight: FontWeight.w500, fontSize: 13),
          ),
        ),
        Expanded(
          child: SizedBox(
            height: 36,
            child: TextField(
              controller: controller,
              keyboardType: TextInputType.numberWithOptions(decimal: true),
              inputFormatters: inputFormatters,
              style: TextStyle(fontSize: 13),
              decoration: InputDecoration(
                hintText: hint,
                contentPadding: EdgeInsets.symmetric(horizontal: 12),
                filled: true,
                fillColor: MemoTheme.of(context).bgApp,
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                  borderSide: BorderSide(
                    color: MemoTheme.of(context).borderSoft,
                  ),
                ),
                enabledBorder: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                  borderSide: BorderSide(
                    color: MemoTheme.of(context).borderSoft,
                  ),
                ),
                focusedBorder: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                  borderSide: BorderSide(color: MemoTheme.accent),
                ),
              ),
            ),
          ),
        ),
      ],
    );
  }
}

class _BackupRestoreTab extends ConsumerStatefulWidget {
  _BackupRestoreTab();

  @override
  ConsumerState<_BackupRestoreTab> createState() => _BackupRestoreTabState();
}

class _BackupRestoreTabState extends ConsumerState<_BackupRestoreTab> {
  bool _exporting = false;
  bool _importing = false;
  bool _includeModels = false;
  bool _wiping = false;
  bool _wipeConfirm1 = false;
  bool _wipeConfirm2 = false;

  Future<void> _export() async {
    if (_exporting) return;
    setState(() => _exporting = true);
    try {
      final api = ref.read(apiClientProvider);
      final data = await api.exportData(includeModels: _includeModels);
      if (!mounted) return;

      final path = await FilePicker.platform.saveFile(
        dialogTitle: 'Memo Yedekle',
        fileName: 'memo_backup.memo',
        type: FileType.any,
      );
      if (path != null) {
        await File(path).writeAsBytes(data);
        if (mounted) {
          ScaffoldMessenger.of(
            context,
          ).showSnackBar(SnackBar(content: Text('Yedek kaydedildi: $path')));
        }
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('Dışa aktarma hatası: $e')));
      }
    } finally {
      if (mounted) setState(() => _exporting = false);
    }
  }

  Future<void> _import() async {
    if (_importing) return;
    setState(() => _importing = true);
    try {
      final result = await FilePicker.platform.pickFiles(
        dialogTitle: 'Memo Yedek İçe Aktar',
        type: FileType.any,
      );
      if (result == null || result.files.isEmpty) return;

      final bytes = result.files.first.bytes;
      if (bytes == null) {
        final path = result.files.first.path;
        if (path == null) return;
        final file = File(path);
        if (!await file.exists()) return;
        final data = await file.readAsBytes();
        await ref.read(apiClientProvider).importData(data);
      } else {
        await ref.read(apiClientProvider).importData(bytes);
      }

      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(
              'Yedek başarıyla içe aktarıldı. Uygulamayı yeniden başlatın.',
            ),
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('İçe aktarma hatası: $e')));
      }
    } finally {
      if (mounted) setState(() => _importing = false);
    }
  }

  Future<void> _wipe() async {
    if (_wiping) return;
    setState(() => _wiping = true);
    try {
      await ref.read(apiClientProvider).wipeData();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Tüm veriler silindi. Uygulamayı yeniden başlatın.'),
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('Silme hatası: $e')));
      }
    } finally {
      if (mounted)
        setState(() {
          _wiping = false;
          _wipeConfirm1 = false;
          _wipeConfirm2 = false;
        });
    }
  }

  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: EdgeInsets.all(32),
      children: [
        Text(
          'Yedekleme',
          style: Theme.of(context).textTheme.titleLarge?.copyWith(
            fontWeight: FontWeight.bold,
            color: MemoTheme.of(context).textMain,
          ),
        ),
        SizedBox(height: 12),
        Text(
          'Tüm sohbet geçmişi, yapılandırma ve WhatsApp mesajlarınızı .memo dosyasına aktarın veya geri yükleyin.',
          style: TextStyle(color: MemoTheme.of(context).textDim, fontSize: 13),
        ),
        SizedBox(height: 24),

        // Include models toggle
        Card(
          child: SwitchListTile(
            title: Text('Modelleri dahil et'),
            subtitle: Text(
              'GGUF modelleri (büyük boyut)',
              style: TextStyle(
                fontSize: 12,
                color: MemoTheme.of(context).textDim,
              ),
            ),
            value: _includeModels,
            onChanged: (v) => setState(() => _includeModels = v),
            secondary: Icon(Icons.model_training, color: MemoTheme.accent),
          ),
        ),
        SizedBox(height: 12),

        // Export
        Card(
          child: ListTile(
            leading: Icon(Icons.file_upload_outlined, color: MemoTheme.accent),
            title: Text('Dışa Aktar'),
            subtitle: Text(
              'Tüm verileri .memo dosyasına kaydeder',
              style: TextStyle(
                fontSize: 12,
                color: MemoTheme.of(context).textDim,
              ),
            ),
            trailing: _exporting
                ? SizedBox(
                    width: 20,
                    height: 20,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                : Icon(Icons.download, color: MemoTheme.of(context).textDim),
            onTap: _exporting ? null : _export,
          ),
        ),
        SizedBox(height: 12),

        // Import
        Card(
          child: ListTile(
            leading: Icon(
              Icons.file_download_outlined,
              color: MemoTheme.warmBrown,
            ),
            title: Text('İçe Aktar'),
            subtitle: Text(
              '.memo dosyasından verileri geri yükler',
              style: TextStyle(
                fontSize: 12,
                color: MemoTheme.of(context).textDim,
              ),
            ),
            trailing: _importing
                ? SizedBox(
                    width: 20,
                    height: 20,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                : Icon(Icons.upload, color: MemoTheme.of(context).textDim),
            onTap: _importing ? null : _import,
          ),
        ),
        SizedBox(height: 32),

        // Wipe All Data
        Text(
          'Tüm Verileri Sil',
          style: TextStyle(
            fontWeight: FontWeight.bold,
            fontSize: 16,
            color: MemoTheme.warmBrown,
          ),
        ),
        SizedBox(height: 8),
        Text(
          'Sohbet geçmişi, WhatsApp mesajları, hafıza ve yapılandırma kalıcı olarak silinir.',
          style: TextStyle(color: MemoTheme.of(context).textDim, fontSize: 13),
        ),
        SizedBox(height: 12),
        if (!_wipeConfirm1)
          Card(
            child: ListTile(
              leading: Icon(Icons.delete_forever, color: MemoTheme.warmBrown),
              title: Text(
                'Tüm Verileri Sil',
                style: TextStyle(color: MemoTheme.warmBrown),
              ),
              subtitle: Text(
                'Bu işlem geri alınamaz',
                style: TextStyle(
                  fontSize: 12,
                  color: MemoTheme.of(context).textDim,
                ),
              ),
              trailing: Icon(Icons.warning_amber, color: MemoTheme.warmBrown),
              onTap: () => setState(() => _wipeConfirm1 = true),
            ),
          ),
        if (_wipeConfirm1 && !_wipeConfirm2)
          Card(
            color: MemoTheme.warmBrown.withValues(alpha: 0.08),
            child: ListTile(
              leading: Icon(Icons.delete_forever, color: Colors.redAccent),
              title: Text(
                'Emin misiniz?',
                style: TextStyle(color: Colors.redAccent),
              ),
              subtitle: Text(
                'Tüm verileriniz silinecek. Onaylamak için tekrar tıklayın.',
                style: TextStyle(
                  fontSize: 12,
                  color: MemoTheme.of(context).textDim,
                ),
              ),
              trailing: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  IconButton(
                    icon: Icon(
                      Icons.close,
                      color: MemoTheme.of(context).textDim,
                    ),
                    onPressed: () => setState(() {
                      _wipeConfirm1 = false;
                      _wipeConfirm2 = false;
                    }),
                  ),
                  Icon(Icons.warning, color: Colors.redAccent),
                ],
              ),
              onTap: () => setState(() => _wipeConfirm2 = true),
            ),
          ),
        if (_wipeConfirm2)
          Card(
            color: MemoTheme.red.withValues(alpha: 0.12),
            child: ListTile(
              leading: _wiping
                  ? SizedBox(
                      width: 20,
                      height: 20,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : Icon(Icons.delete_sweep, color: MemoTheme.red),
              title: Text(
                'Sil',
                style: TextStyle(
                  color: MemoTheme.red,
                  fontWeight: FontWeight.bold,
                ),
              ),
              subtitle: Text(
                'Bu işlem geri alınamaz. Tüm veriler silinecek.',
                style: TextStyle(
                  fontSize: 12,
                  color: MemoTheme.of(context).textDim,
                ),
              ),
              trailing: IconButton(
                icon: Icon(Icons.close, color: MemoTheme.of(context).textDim),
                onPressed: () => setState(() {
                  _wipeConfirm1 = false;
                  _wipeConfirm2 = false;
                }),
              ),
              onTap: _wiping ? null : _wipe,
            ),
          ),
      ],
    );
  }
}

class _RemoteAccessTab extends ConsumerWidget {
  _RemoteAccessTab();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(
            Icons.wifi_tethering,
            size: 48,
            color: MemoTheme.of(context).textDim,
          ),
          SizedBox(height: 16),
          Text(
            L10n.t('remote_access'),
            style: Theme.of(context).textTheme.titleLarge?.copyWith(
              color: MemoTheme.of(context).textMuted,
            ),
          ),
          SizedBox(height: 8),
          Text(
            'Bu özellik v3.0.0\'da devre dışı bırakılmıştır. Gelecek bir sürümde tekrar eklenecek.',
            style: Theme.of(context).textTheme.bodyMedium?.copyWith(
              color: MemoTheme.of(context).textDim,
            ),
            textAlign: TextAlign.center,
          ),
        ],
      ),
    );
  }
}

class _AboutTab extends ConsumerWidget {
  _AboutTab();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final versionAsync = ref.watch(appVersionProvider);

    return ListView(
      padding: EdgeInsets.all(32),
      children: [
        Row(
          children: [
            Container(
              width: 64,
              height: 64,
              decoration: BoxDecoration(
                color: MemoTheme.accentPale,
                borderRadius: BorderRadius.circular(16),
                border: Border.all(color: MemoTheme.accent, width: 2),
              ),
              child: Center(
                child: Text(
                  'M',
                  style: TextStyle(
                    fontSize: 28,
                    fontWeight: FontWeight.bold,
                    color: MemoTheme.accent,
                  ),
                ),
              ),
            ),
            SizedBox(width: 24),
            Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  L10n.t('app_title'),
                  style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                    fontWeight: FontWeight.bold,
                    color: MemoTheme.of(context).textMain,
                  ),
                ),
                versionAsync.when(
                  loading: () => Text('...'),
                  error: (_, _) => SizedBox(),
                  data: (v) => Text(
                    v,
                    style: TextStyle(color: MemoTheme.of(context).textDim),
                  ),
                ),
              ],
            ),
          ],
        ),
        SizedBox(height: 32),
        Text(
          L10n.t('about_vision'),
          style: TextStyle(fontWeight: FontWeight.bold, fontSize: 16),
        ),
        SizedBox(height: 8),
        Text(
          'Memo, tamamen yerel bilgisayarınızda çalışan, gizlilik odaklı bir yapay zeka asistanıdır. '
          'Konuşmalarınızı ve tercihlerinizi zamanla öğrenip kalıcı hafızasına kazır. '
          'Üçüncü taraf sunuculara ihtiyaç duymadan, kendi bilgisayarınızda çalışır — '
          'verileriniz tamamen sizde kalır. İsteğe bağlı olarak harici API sağlayıcıları '
          'veya yerel llama.cpp modelleri ile kullanılabilir. '
          'WhatsApp entegrasyonu, RAG hafıza ve E2E şifreli bulut senkronizasyonu destekler.',
          style: TextStyle(height: 1.6, color: MemoTheme.of(context).textMuted),
        ),
        SizedBox(height: 24),
        Text(
          'Lisans',
          style: TextStyle(fontWeight: FontWeight.bold, fontSize: 16),
        ),
        SizedBox(height: 8),
        Text(
          'Bu yazılım GNU Affero Genel Kamu Lisansı v3 (AGPL-3.0) ile lisanslanmıştır. '
          'Geliştirici: Buğra Akdemir. Kaynak kod: github.com/BugraAkdemir/memo',
          style: TextStyle(height: 1.6, color: MemoTheme.of(context).textMuted),
        ),
        SizedBox(height: 24),
        Text(
          'Teknolojiler',
          style: TextStyle(fontWeight: FontWeight.bold, fontSize: 16),
        ),
        SizedBox(height: 8),
        Text(
          'Go 1.25 + Flutter 3.10 | SQLite + sqlite-vec (vektör arama) | '
          'whatsmeow (WhatsApp Web) | llama.cpp | Riverpod | Dio',
          style: TextStyle(height: 1.6, color: MemoTheme.of(context).textMuted),
        ),
      ],
    );
  }
}

class _GpuConfigTab extends ConsumerStatefulWidget {
  _GpuConfigTab();

  @override
  ConsumerState<_GpuConfigTab> createState() => _GpuConfigTabState();
}

class _GpuConfigTabState extends ConsumerState<_GpuConfigTab> {
  bool _installing = false;
  String _error = '';

  Future<void> _startInstall() async {
    setState(() {
      _installing = true;
      _error = '';
    });

    try {
      await ref.read(apiClientProvider).installLlamaServer();
      ref.invalidate(llamaInstalledProvider);
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = e.toString();
      });
    } finally {
      if (mounted) {
        setState(() {
          _installing = false;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final installedAsync = ref.watch(llamaInstalledProvider);
    final gpuAsync = ref.watch(gpuInfoProvider);
    final hasGpu = gpuAsync.whenOrNull(data: (g) => g.hasGpu) ?? false;
    final gpuName = gpuAsync.whenOrNull(data: (g) => g.name) ?? '';

    return ListView(
      padding: EdgeInsets.all(32),
      children: [
        Text(
          'Ekran Kartı (GPU) / Llama Motoru',
          style: Theme.of(context).textTheme.titleLarge?.copyWith(
            fontWeight: FontWeight.bold,
            color: MemoTheme.of(context).textMain,
          ),
        ),
        SizedBox(height: 12),
        Text(
          'Yapay zeka modellerini çalıştıran Llama.cpp motorunun kurulum ve ekran kartı ayarları.',
          style: TextStyle(color: MemoTheme.of(context).textDim, fontSize: 13),
        ),
        SizedBox(height: 32),

        Container(
          padding: EdgeInsets.all(20),
          decoration: BoxDecoration(
            color: MemoTheme.of(context).bgPanel,
            borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
            border: Border.all(color: MemoTheme.of(context).borderSoft),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                'Sistem Donanım Durumu',
                style: TextStyle(
                  fontSize: 14,
                  fontWeight: FontWeight.w600,
                  color: MemoTheme.of(context).textMain,
                ),
              ),
              SizedBox(height: 12),
              Row(
                children: [
                  Icon(
                    hasGpu ? Icons.memory : Icons.developer_board,
                    size: 20,
                    color: hasGpu
                        ? MemoTheme.accent
                        : MemoTheme.of(context).textDim,
                  ),
                  SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      hasGpu
                          ? 'Algılanan Ekran Kartı: $gpuName'
                          : 'Sadece İşlemci (CPU) algılandı veya GPU desteklenmiyor.',
                      style: TextStyle(
                        fontSize: 14,
                        color: MemoTheme.of(context).textMain,
                      ),
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
        SizedBox(height: 24),

        installedAsync.when(
          loading: () => Center(child: CircularProgressIndicator()),
          error: (e, _) => Text('${L10n.t('error')}: $e'),
          data: (installed) {
            final llamaSettings = ref.watch(llamaSettingsProvider);
            return Column(
              children: [
                // Engine Mode Selection
                Container(
                  padding: EdgeInsets.all(20),
                  decoration: BoxDecoration(
                    color: MemoTheme.of(context).bgPanel,
                    borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                    border: Border.all(color: MemoTheme.of(context).borderSoft),
                  ),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        'Motor Modu',
                        style: TextStyle(
                          fontSize: 14,
                          fontWeight: FontWeight.w600,
                          color: MemoTheme.of(context).textMain,
                        ),
                      ),
                      SizedBox(height: 12),
                      llamaSettings.when(
                        loading: () => CircularProgressIndicator(),
                        error: (e, _) => Text('${L10n.t('error')}: $e'),
                        data: (settings) => DropdownButton<String>(
                          value: settings.engineMode,
                          isExpanded: true,
                          dropdownColor: MemoTheme.of(context).bgPanel,
                          items: const [
                            DropdownMenuItem(
                              value: 'auto',
                              child: Text('Otomatik (Önerilen)'),
                            ),
                            DropdownMenuItem(
                              value: 'cpu',
                              child: Text('Sadece İşlemci (CPU)'),
                            ),
                            DropdownMenuItem(
                              value: 'nvidia',
                              child: Text('NVIDIA (CUDA)'),
                            ),
                            DropdownMenuItem(
                              value: 'amd',
                              child: Text('AMD (ROCm/Vulkan)'),
                            ),
                          ],
                          onChanged: (mode) {
                            if (mode != null) {
                              final cur = llamaSettings.valueOrNull;
                              if (cur != null) {
                                ref
                                    .read(llamaSettingsProvider.notifier)
                                    .save(
                                      LlamaSettings(
                                        engineMode: mode,
                                        binaryPath: cur.binaryPath,
                                        port: cur.port,
                                        ctxSize: cur.ctxSize,
                                        temperature: cur.temperature,
                                        topP: cur.topP,
                                        maxTokens: cur.maxTokens,
                                      ),
                                    );
                              }
                            }
                          },
                        ),
                      ),
                    ],
                  ),
                ),
                SizedBox(height: 24),
                _ModelParametersCard(),
                SizedBox(height: 24),
                // Installation Status Card
                Container(
                  padding: EdgeInsets.all(20),
                  decoration: BoxDecoration(
                    color: MemoTheme.of(context).bgPanel,
                    borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                    border: Border.all(color: MemoTheme.of(context).borderSoft),
                  ),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          Icon(
                            installed
                                ? Icons.check_circle
                                : Icons.warning_amber_rounded,
                            color: installed ? MemoTheme.green : MemoTheme.warningOrange,
                            size: 24,
                          ),
                          SizedBox(width: 12),
                          Text(
                            installed
                                ? 'Llama Motoru Yüklü'
                                : 'Llama Motoru Yüklü Değil',
                            style: TextStyle(
                              fontSize: 16,
                              fontWeight: FontWeight.bold,
                              color: MemoTheme.of(context).textMain,
                            ),
                          ),
                        ],
                      ),
                      SizedBox(height: 12),
                      Text(
                        installed
                            ? 'Uygulama arka planda modelleri sorunsuz çalıştırabilir.'
                            : 'Modellerin çalışabilmesi için Llama.cpp motorunun (ve varsa GPU sürücülerinin) yüklenmesi gerekmektedir.',
                        style: TextStyle(
                          color: MemoTheme.of(context).textDim,
                          fontSize: 13,
                        ),
                      ),
                      SizedBox(height: 20),
                      if (_error.isNotEmpty)
                        Padding(
                          padding: EdgeInsets.only(bottom: 16),
                          child: Text(
                            _error,
                            style: TextStyle(
                              color: MemoTheme.red,
                              fontSize: 13,
                            ),
                          ),
                        ),
                      SizedBox(
                        width: double.infinity,
                        height: 44,
                        child: ElevatedButton(
                          onPressed: _installing ? null : _startInstall,
                          style: ElevatedButton.styleFrom(
                            backgroundColor: MemoTheme.accent,
                            foregroundColor: MemoTheme.of(context).textInverse,
                          ),
                          child: _installing
                              ? SizedBox(
                                  width: 20,
                                  height: 20,
                                  child: CircularProgressIndicator(
                                    strokeWidth: 2,
                                    color: MemoTheme.of(context).textInverse,
                                  ),
                                )
                              : Text(
                                  installed
                                      ? 'Motoru Yeniden Kur / Onar'
                                      : (hasGpu
                                            ? 'Ekran Kartı İçin Kur (Önerilen)'
                                            : 'Motoru İndir ve Kur'),
                                  style: TextStyle(fontWeight: FontWeight.w600),
                                ),
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            );
          },
        ),
      ],
    );
  }
}

class _ModelParametersCard extends ConsumerStatefulWidget {
  @override
  ConsumerState<_ModelParametersCard> createState() =>
      _ModelParametersCardState();
}

class _ModelParametersCardState extends ConsumerState<_ModelParametersCard> {
  late double _temperature;
  late double _topP;
  late int _maxTokens;
  late int _ctxSize;
  bool _loaded = false;
  bool _dirty = false;

  @override
  Widget build(BuildContext context) {
    final llamaSettings = ref.watch(llamaSettingsProvider);
    return llamaSettings.when(
      loading: () => SizedBox.shrink(),
      error: (e, _) => Text('${L10n.t('error')}: $e'),
      data: (settings) {
        if (!_loaded) {
          _temperature = settings.temperature;
          _topP = settings.topP;
          _maxTokens = settings.maxTokens;
          _ctxSize = settings.ctxSize;
          _loaded = true;
        } else if (!_dirty && (_temperature != settings.temperature ||
            _topP != settings.topP ||
            _maxTokens != settings.maxTokens ||
            _ctxSize != settings.ctxSize)) {
          _temperature = settings.temperature;
          _topP = settings.topP;
          _maxTokens = settings.maxTokens;
          _ctxSize = settings.ctxSize;
        }
        return Container(
          padding: EdgeInsets.all(20),
          decoration: BoxDecoration(
            color: MemoTheme.of(context).bgPanel,
            borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
            border: Border.all(color: MemoTheme.of(context).borderSoft),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                L10n.t('model_params'),
                style: TextStyle(
                  fontSize: 14,
                  fontWeight: FontWeight.w600,
                  color: MemoTheme.of(context).textMain,
                ),
              ),
              SizedBox(height: 16),
              _ParamSlider(
                label: 'Temperature',
                value: _temperature,
                min: 0.0,
                max: 2.0,
                divisions: 40,
                displayValue: _temperature.toStringAsFixed(2),
                onChanged: (v) => setState(() { _temperature = v; _dirty = true; }),
              ),
              SizedBox(height: 12),
              _ParamSlider(
                label: 'Top P',
                value: _topP,
                min: 0.0,
                max: 1.0,
                divisions: 20,
                displayValue: _topP.toStringAsFixed(2),
                onChanged: (v) => setState(() { _topP = v; _dirty = true; }),
              ),
              SizedBox(height: 12),
              _ParamIntInput(
                label: 'Max Tokens',
                value: _maxTokens,
                min: 0,
                max: 65536,
                step: 256,
                displaySuffix: L10n.t('param_unlimited'),
                onChanged: (v) => setState(() { _maxTokens = v; _dirty = true; }),
              ),
              SizedBox(height: 12),
              _ParamIntInput(
                label: 'Context Size',
                value: _ctxSize,
                min: 512,
                max: 65536,
                step: 512,
                onChanged: (v) => setState(() { _ctxSize = v; _dirty = true; }),
              ),
              SizedBox(height: 16),
              SizedBox(
                width: double.infinity,
                height: 44,
                child: ElevatedButton(
                  onPressed: () async {
                    await ref
                        .read(llamaSettingsProvider.notifier)
                        .save(
                          LlamaSettings(
                            engineMode: settings.engineMode,
                            binaryPath: settings.binaryPath,
                            port: settings.port,
                            ctxSize: _ctxSize,
                            temperature: _temperature,
                            topP: _topP,
                            maxTokens: _maxTokens,
                          ),
                        );
                    setState(() => _dirty = false);
                    if (context.mounted) {
                      ScaffoldMessenger.of(context).showSnackBar(
                        SnackBar(
                          content: Text(L10n.t('params_saved')),
                          duration: Duration(seconds: 2),
                          backgroundColor: MemoTheme.green,
                        ),
                      );
                    }
                  },
                  style: ElevatedButton.styleFrom(
                    backgroundColor: MemoTheme.accent,
                    foregroundColor: MemoTheme.of(context).textInverse,
                  ),
                  child: Text(
                    L10n.t('apply'),
                    style: TextStyle(fontWeight: FontWeight.w600),
                  ),
                ),
              ),
            ],
          ),
        );
      },
    );
  }
}

class _ParamSlider extends StatelessWidget {
  final String label;
  final double value;
  final double min;
  final double max;
  final int divisions;
  final String displayValue;
  final ValueChanged<double> onChanged;

  _ParamSlider({
    required this.label,
    required this.value,
    required this.min,
    required this.max,
    required this.divisions,
    required this.displayValue,
    required this.onChanged,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            Text(
              label,
              style: TextStyle(
                fontSize: 13,
                color: MemoTheme.of(context).textMain,
              ),
            ),
            Text(
              displayValue,
              style: TextStyle(
                fontSize: 13,
                color: MemoTheme.accent,
                fontWeight: FontWeight.w600,
              ),
            ),
          ],
        ),
        Slider(
          value: value,
          min: min,
          max: max,
          divisions: divisions,
          activeColor: MemoTheme.accent,
          inactiveColor: MemoTheme.of(context).borderSoft,
          onChanged: onChanged,
        ),
      ],
    );
  }
}

class _ParamIntInput extends StatefulWidget {
  final String label;
  final int value;
  final int min;
  final int max;
  final int step;
  final String? displaySuffix;
  final ValueChanged<int> onChanged;

  _ParamIntInput({
    required this.label,
    required this.value,
    required this.min,
    required this.max,
    required this.step,
    this.displaySuffix,
    required this.onChanged,
  });

  @override
  State<_ParamIntInput> createState() => _ParamIntInputState();
}

class _ParamIntInputState extends State<_ParamIntInput> {
  late TextEditingController _controller;

  @override
  void initState() {
    super.initState();
    _controller = TextEditingController(
      text: widget.value == 0 && widget.min == 0
          ? '0'
          : widget.value.toString(),
    );
  }

  @override
  void didUpdateWidget(_ParamIntInput old) {
    super.didUpdateWidget(old);
    if (widget.value != old.value) {
      _controller.text = widget.value == 0 && widget.min == 0
          ? '0'
          : widget.value.toString();
    }
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        SizedBox(
          width: 140,
          child: Text(
            widget.label,
            style: TextStyle(
              fontSize: 13,
              color: MemoTheme.of(context).textMain,
            ),
          ),
        ),
        SizedBox(
          width: 120,
          child: TextField(
            controller: _controller,
            keyboardType: TextInputType.number,
            decoration: InputDecoration(
              isDense: true,
              contentPadding: EdgeInsets.symmetric(horizontal: 8, vertical: 6),
              border: OutlineInputBorder(
                borderRadius: BorderRadius.all(Radius.circular(4)),
              ),
            ),
            onChanged: (v) {
              final parsed = int.tryParse(v);
              if (parsed != null) {
                widget.onChanged(parsed.clamp(widget.min, widget.max));
              }
            },
          ),
        ),
        if (widget.displaySuffix != null) ...[
          SizedBox(width: 8),
          Text(
            widget.displaySuffix!,
            style: TextStyle(
              fontSize: 11,
              color: MemoTheme.of(context).textDim,
            ),
          ),
        ],
      ],
    );
  }
}
