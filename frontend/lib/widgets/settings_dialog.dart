import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_svg/flutter_svg.dart';
import 'package:file_picker/file_picker.dart';
import 'package:url_launcher/url_launcher.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/orchestra_config.dart';
import '../models/provider_config.dart';
import '../providers/settings_provider.dart';
import '../providers/learning_provider.dart';
import '../providers/models_provider.dart';
import '../providers/chat_provider.dart';
import '../providers/orchestra_provider.dart';
import '../providers/provider_provider.dart';
import '../providers/skill_provider.dart';
import 'orchestra_config_dialog.dart';
import 'skill_config_dialog.dart';
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

  static const _tabIcons = [
    'lib/icon/slash/gear.svg',
    'lib/icon/slash/chat-text.svg',
    'lib/icon/slash/eye-slash.svg',
    'lib/icon/slash/brain.svg',
    'lib/icon/slash/plug.svg',
    'lib/icon/slash/music-notes.svg',
    'lib/icon/slash/shield-check.svg',
    'lib/icon/slash/lightbulb.svg',
    'lib/icon/slash/puzzle-piece.svg',
    'lib/icon/slash/cpu.svg',
    'lib/icon/slash/archive.svg',
    'lib/icon/slash/globe.svg',
    'lib/icon/slash/info.svg',
  ];

  List<String> get _tabs => [
    L10n.t('general'),
    L10n.t('system_prompt'),
    L10n.t('incognito_prompt'),
    L10n.t('memory'),
    L10n.t('tab_providers'),
    L10n.t('tab_orchestra'),
    L10n.t('tab_agent_permissions'),
    'Learning',
    'Skills',
    L10n.t('tab_gpu_config'),
    L10n.t('backup'),
    L10n.t('remote_access'),
    L10n.t('about'),
  ];

  @override
  Widget build(BuildContext context) {
    ref.watch(localeProvider);
    final tabs = _tabs;
    final tabIndex = _activeTab.clamp(0, tabs.length - 1);

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
                        final isActive = tabIndex == index;
                        final iconColor = isActive
                            ? MemoTheme.accent
                            : MemoTheme.of(context).textDim;
                        return InkWell(
                          onTap: () => setState(() => _activeTab = index),
                          child: Container(
                            padding: const EdgeInsets.symmetric(
                              horizontal: 16,
                              vertical: 11,
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
                            child: Row(
                              children: [
                                SizedBox(
                                  width: 18,
                                  height: 18,
                                  child: SvgPicture.asset(
                                    _tabIcons[index],
                                    colorFilter: ColorFilter.mode(
                                      iconColor,
                                      BlendMode.srcIn,
                                    ),
                                  ),
                                ),
                                const SizedBox(width: 10),
                                Expanded(
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
                                    overflow: TextOverflow.ellipsis,
                                  ),
                                ),
                              ],
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
                      child: _buildTabContent(tabIndex),
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
        return _LearningTab();
      case 8:
        return _SkillsTab();
      case 9:
        return _GpuConfigTab();
      case 10:
        return _BackupRestoreTab();
      case 11:
        return _RemoteAccessTab();
      case 12:
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
  // ── Yerel yedekleme durumu ──────────────────────────────────────
  bool _exporting = false;
  bool _importing = false;
  bool _includeModels = false;
  bool _wiping = false;
  bool _wipeConfirm1 = false;
  bool _wipeConfirm2 = false;

  // ── Bulut yedekleme durumu ──────────────────────────────────────
  bool _cloudConnected = false;
  String _cloudName = '';
  String _cloudEmail = '';
  String _cloudOp = ''; // 'connecting' | 'saving' | 'backup' | 'restore' | ''
  bool _showCredSetup = false;
  final _clientIdCtrl = TextEditingController();
  final _clientSecretCtrl = TextEditingController();
  final _passphraseCtrl = TextEditingController();
  Timer? _authPollTimer;

  @override
  void initState() {
    super.initState();
    _loadCloudStatus();
  }

  @override
  void dispose() {
    _authPollTimer?.cancel();
    _clientIdCtrl.dispose();
    _clientSecretCtrl.dispose();
    _passphraseCtrl.dispose();
    super.dispose();
  }

  // ── Bulut metodları ─────────────────────────────────────────────

  Future<void> _loadCloudStatus() async {
    try {
      final api = ref.read(apiClientProvider);
      final connected = await api.checkSyncAuth();
      if (!mounted) return;
      if (connected) {
        final account = await api.getSyncAccount();
        final settings = await api.getSyncSettings();
        if (!mounted) return;
        setState(() {
          _cloudConnected = true;
          _cloudName = account['name'] as String? ?? '';
          _cloudEmail = account['email'] as String? ?? '';
          _clientIdCtrl.text = settings['client_id'] as String? ?? '';
          _clientSecretCtrl.text = settings['client_secret'] as String? ?? '';
          _passphraseCtrl.text = settings['passphrase'] as String? ?? '';
        });
      } else {
        final settings = await api.getSyncSettings();
        if (!mounted) return;
        setState(() {
          _cloudConnected = false;
          _clientIdCtrl.text = settings['client_id'] as String? ?? '';
          _clientSecretCtrl.text = settings['client_secret'] as String? ?? '';
          _passphraseCtrl.text = settings['passphrase'] as String? ?? '';
        });
      }
    } catch (_) {}
  }

  Future<void> _saveCredentials() async {
    setState(() { _cloudOp = 'saving'; });
    try {
      await ref.read(apiClientProvider).updateSyncSettings(
        enabled: true,
        clientId: _clientIdCtrl.text.trim(),
        clientSecret: _clientSecretCtrl.text.trim(),
        passphrase: _passphraseCtrl.text.trim(),
        tokenPath: '',
        intervalMessages: 50,
      );
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Kimlik bilgileri kaydedildi')),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Kaydetme hatası: $e')),
        );
      }
    } finally {
      if (mounted) setState(() { _cloudOp = ''; });
    }
  }

  Future<void> _connectDrive() async {
    if (_clientIdCtrl.text.trim().isEmpty || _clientSecretCtrl.text.trim().isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Lütfen önce Client ID ve Client Secret girin')),
      );
      return;
    }
    setState(() { _cloudOp = 'connecting'; });
    try {
      await ref.read(apiClientProvider).updateSyncSettings(
        enabled: true,
        clientId: _clientIdCtrl.text.trim(),
        clientSecret: _clientSecretCtrl.text.trim(),
        passphrase: _passphraseCtrl.text.trim(),
        tokenPath: '',
        intervalMessages: 50,
      );
      final api = ref.read(apiClientProvider);
      final url = await api.startSyncAuth();
      if (url.isNotEmpty) {
        await launchUrl(Uri.parse(url), mode: LaunchMode.externalApplication);
      }
      // Yetkilendirme tamamlanana kadar her 3 saniyede bir sorgula (max 2 dk)
      int attempts = 0;
      _authPollTimer?.cancel();
      _authPollTimer = Timer.periodic(const Duration(seconds: 3), (t) async {
        attempts++;
        if (attempts > 40) {
          t.cancel();
          if (mounted) setState(() { _cloudOp = ''; });
          return;
        }
        try {
          final done = await ref.read(apiClientProvider).checkSyncAuth();
          if (done) {
            t.cancel();
            await _loadCloudStatus();
            if (mounted) setState(() { _cloudOp = ''; });
          }
        } catch (_) {}
      });
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Bağlantı hatası: $e')),
        );
        setState(() { _cloudOp = ''; });
      }
    }
  }

  Future<void> _backupNow() async {
    setState(() { _cloudOp = 'backup'; });
    try {
      await ref.read(apiClientProvider).triggerSync();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Drive yedeklemesi başlatıldı (arka planda çalışıyor)')),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Yedekleme hatası: $e')),
        );
      }
    } finally {
      if (mounted) setState(() { _cloudOp = ''; });
    }
  }

  Future<void> _restoreCloud() async {
    final confirm = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Buluttan Geri Yükle'),
        content: const Text(
          "Drive'daki son yedek geri yüklenecek.\n"
          'Mevcut hafıza verilerinin üzerine yazılacak.\n'
          'Devam etmek istiyor musunuz?',
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('İptal')),
          TextButton(onPressed: () => Navigator.pop(ctx, true), child: const Text('Geri Yükle')),
        ],
      ),
    );
    if (confirm != true) return;
    setState(() { _cloudOp = 'restore'; });
    try {
      await ref.read(apiClientProvider).pullSync();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Geri yükleme başlatıldı. Tamamlandığında uygulamayı yeniden başlatın.')),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Geri yükleme hatası: $e')),
        );
      }
    } finally {
      if (mounted) setState(() { _cloudOp = ''; });
    }
  }

  Future<void> _disconnectCloud() async {
    final confirm = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Drive Bağlantısını Kes'),
        content: const Text('Google Drive bağlantısı kesilecek. Yerel yedekler korunur.'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('İptal')),
          TextButton(
            onPressed: () => Navigator.pop(ctx, true),
            style: TextButton.styleFrom(foregroundColor: MemoTheme.red),
            child: const Text('Bağlantıyı Kes'),
          ),
        ],
      ),
    );
    if (confirm != true) return;
    try {
      await ref.read(apiClientProvider).disconnectSync();
      if (mounted) {
        setState(() {
          _cloudConnected = false;
          _cloudName = '';
          _cloudEmail = '';
        });
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Drive bağlantısı kesildi')),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Hata: $e')),
        );
      }
    }
  }

  // ── Yerel yedekleme metodları ───────────────────────────────────

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

        // ── Bulut Yedekleme ────────────────────────────────────────
        SizedBox(height: 40),
        Divider(color: MemoTheme.of(context).borderSoft),
        SizedBox(height: 24),

        Row(
          children: [
            Icon(Icons.cloud_outlined, color: MemoTheme.accent, size: 22),
            SizedBox(width: 10),
            Text(
              'Bulut Yedekleme (Google Drive)',
              style: TextStyle(
                fontWeight: FontWeight.bold,
                fontSize: 16,
                color: MemoTheme.of(context).textMain,
              ),
            ),
          ],
        ),
        SizedBox(height: 8),
        Text(
          'Hafıza verilerini AES-256 şifreli olarak Google Drive\'a yedekle ve '
          'farklı cihazlara geri yükle. Sadece bu uygulamanın oluşturduğu '
          'dosyalara erişim sağlanır.',
          style: TextStyle(color: MemoTheme.of(context).textDim, fontSize: 13),
        ),
        SizedBox(height: 20),

        // Bağlantı durumu kartı
        Container(
          padding: EdgeInsets.all(16),
          decoration: BoxDecoration(
            color: _cloudConnected
                ? MemoTheme.green.withValues(alpha: 0.08)
                : MemoTheme.of(context).bgPanel,
            borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
            border: Border.all(
              color: _cloudConnected
                  ? MemoTheme.green.withValues(alpha: 0.3)
                  : MemoTheme.of(context).borderSoft,
            ),
          ),
          child: Row(
            children: [
              Container(
                width: 44,
                height: 44,
                decoration: BoxDecoration(
                  color: _cloudConnected
                      ? MemoTheme.green.withValues(alpha: 0.15)
                      : MemoTheme.of(context).bgElement,
                  shape: BoxShape.circle,
                ),
                child: Icon(
                  _cloudConnected ? Icons.cloud_done : Icons.cloud_off,
                  color: _cloudConnected
                      ? MemoTheme.green
                      : MemoTheme.of(context).textDim,
                  size: 22,
                ),
              ),
              SizedBox(width: 14),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      _cloudConnected ? 'Drive Bağlı' : 'Bağlı Değil',
                      style: TextStyle(
                        fontWeight: FontWeight.w600,
                        fontSize: 14,
                        color: _cloudConnected
                            ? MemoTheme.green
                            : MemoTheme.of(context).textMain,
                      ),
                    ),
                    if (_cloudConnected && _cloudEmail.isNotEmpty)
                      Padding(
                        padding: EdgeInsets.only(top: 2),
                        child: Text(
                          '$_cloudName • $_cloudEmail',
                          style: TextStyle(
                            fontSize: 12,
                            color: MemoTheme.of(context).textDim,
                          ),
                        ),
                      ),
                    if (!_cloudConnected)
                      Padding(
                        padding: EdgeInsets.only(top: 2),
                        child: Text(
                          'Kimlik bilgilerini girin ve bağlan',
                          style: TextStyle(
                            fontSize: 12,
                            color: MemoTheme.of(context).textDim,
                          ),
                        ),
                      ),
                    if (_cloudOp == 'connecting')
                      Padding(
                        padding: EdgeInsets.only(top: 4),
                        child: Row(
                          children: [
                            SizedBox(
                              width: 12,
                              height: 12,
                              child: CircularProgressIndicator(strokeWidth: 2),
                            ),
                            SizedBox(width: 6),
                            Text(
                              'Tarayıcıda yetkilendirme bekleniyor...',
                              style: TextStyle(
                                fontSize: 11,
                                color: MemoTheme.accent,
                              ),
                            ),
                          ],
                        ),
                      ),
                  ],
                ),
              ),
              if (_cloudConnected)
                TextButton(
                  onPressed: _cloudOp.isNotEmpty ? null : _disconnectCloud,
                  style: TextButton.styleFrom(foregroundColor: MemoTheme.red),
                  child: Text('Kes', style: TextStyle(fontSize: 12)),
                ),
            ],
          ),
        ),

        // Kimlik bilgileri formu (bağlı değilse veya ayarlar açıksa)
        if (!_cloudConnected || _showCredSetup) ...[
          SizedBox(height: 16),
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                'Google OAuth Kimlik Bilgileri',
                style: TextStyle(
                  fontWeight: FontWeight.w600,
                  fontSize: 13,
                  color: MemoTheme.of(context).textMain,
                ),
              ),
              if (_cloudConnected)
                TextButton(
                  onPressed: () => setState(() => _showCredSetup = false),
                  child: Text('Kapat', style: TextStyle(fontSize: 12)),
                ),
            ],
          ),
          SizedBox(height: 4),
          Text(
            'Google Cloud Console\'dan bir OAuth 2.0 Desktop App kimlik bilgisi oluşturun.',
            style: TextStyle(fontSize: 12, color: MemoTheme.of(context).textDim),
          ),
          SizedBox(height: 12),
          _CloudTextField(
            label: 'Client ID',
            controller: _clientIdCtrl,
            hint: 'xxxx.apps.googleusercontent.com',
          ),
          SizedBox(height: 8),
          _CloudTextField(
            label: 'Client Secret',
            controller: _clientSecretCtrl,
            hint: 'GOCSPX-...',
            obscure: true,
          ),
          SizedBox(height: 8),
          _CloudTextField(
            label: 'Şifreleme Parolası',
            controller: _passphraseCtrl,
            hint: 'Opsiyonel — boş bırakırsanız cihaz kimliği kullanılır',
            obscure: true,
          ),
          SizedBox(height: 12),
          Row(
            mainAxisAlignment: MainAxisAlignment.end,
            children: [
              if (!_cloudConnected) ...[
                OutlinedButton(
                  onPressed: _cloudOp.isNotEmpty ? null : _saveCredentials,
                  child: _cloudOp == 'saving'
                      ? SizedBox(
                          width: 14,
                          height: 14,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : Text('Kaydet'),
                ),
                SizedBox(width: 8),
                FilledButton.icon(
                  onPressed: _cloudOp.isNotEmpty ? null : _connectDrive,
                  icon: _cloudOp == 'connecting'
                      ? SizedBox(
                          width: 14,
                          height: 14,
                          child: CircularProgressIndicator(
                            strokeWidth: 2,
                            color: Colors.white,
                          ),
                        )
                      : Icon(Icons.login, size: 16),
                  label: Text('Google Drive\'a Bağlan'),
                  style: FilledButton.styleFrom(backgroundColor: MemoTheme.accent),
                ),
              ] else ...[
                FilledButton(
                  onPressed: _cloudOp.isNotEmpty ? null : _saveCredentials,
                  child: _cloudOp == 'saving'
                      ? SizedBox(
                          width: 14,
                          height: 14,
                          child: CircularProgressIndicator(
                            strokeWidth: 2,
                            color: Colors.white,
                          ),
                        )
                      : Text('Kimlik Bilgilerini Güncelle'),
                  style: FilledButton.styleFrom(backgroundColor: MemoTheme.accent),
                ),
              ],
            ],
          ),
        ],

        // Bağlı iken yedekleme eylemleri
        if (_cloudConnected) ...[
          SizedBox(height: 16),
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                'Yedekleme İşlemleri',
                style: TextStyle(
                  fontWeight: FontWeight.w600,
                  fontSize: 13,
                  color: MemoTheme.of(context).textMain,
                ),
              ),
              TextButton(
                onPressed: () => setState(() => _showCredSetup = !_showCredSetup),
                child: Text(
                  _showCredSetup ? 'Ayarları Kapat' : 'Kimlik Bilgilerini Düzenle',
                  style: TextStyle(fontSize: 12),
                ),
              ),
            ],
          ),
          SizedBox(height: 8),

          Row(
            children: [
              Expanded(
                child: Card(
                  child: ListTile(
                    leading: _cloudOp == 'backup'
                        ? SizedBox(
                            width: 20,
                            height: 20,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : Icon(Icons.cloud_upload_outlined, color: MemoTheme.accent),
                    title: Text('Şimdi Yedekle', style: TextStyle(fontSize: 14)),
                    subtitle: Text(
                      'Hafızayı Drive\'a gönder',
                      style: TextStyle(fontSize: 11, color: MemoTheme.of(context).textDim),
                    ),
                    onTap: _cloudOp.isNotEmpty ? null : _backupNow,
                  ),
                ),
              ),
              SizedBox(width: 8),
              Expanded(
                child: Card(
                  child: ListTile(
                    leading: _cloudOp == 'restore'
                        ? SizedBox(
                            width: 20,
                            height: 20,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : Icon(Icons.cloud_download_outlined, color: MemoTheme.warmBrown),
                    title: Text('Geri Yükle', style: TextStyle(fontSize: 14)),
                    subtitle: Text(
                      'Son yedeği indir ve uygula',
                      style: TextStyle(fontSize: 11, color: MemoTheme.of(context).textDim),
                    ),
                    onTap: _cloudOp.isNotEmpty ? null : _restoreCloud,
                  ),
                ),
              ),
            ],
          ),
        ],
        SizedBox(height: 24),
      ],
    );
  }
}

class _RemoteAccessTab extends ConsumerStatefulWidget {
  @override
  _RemoteAccessTabState createState() => _RemoteAccessTabState();
}

class _RemoteAccessTabState extends ConsumerState<_RemoteAccessTab> {
  final _ngrokTokenCtrl = TextEditingController();
  final _tsKeyCtrl = TextEditingController();
  final _tsHostCtrl = TextEditingController();
  bool _tsFunnel = false;
  bool _tsBusy = false;
  bool _enabling = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.invalidate(remoteAccessProvider);
    });
  }

  @override
  void dispose() {
    _ngrokTokenCtrl.dispose();
    _tsKeyCtrl.dispose();
    _tsHostCtrl.dispose();
    super.dispose();
  }

  Future<void> _enableTailscale() async {
    setState(() => _tsBusy = true);
    try {
      await ref.read(apiClientProvider).setTailscaleMode(
            true,
            8090,
            authKey: _tsKeyCtrl.text.trim(),
            hostname: _tsHostCtrl.text.trim(),
            funnel: _tsFunnel,
          );
      await Future.delayed(const Duration(seconds: 2));
      ref.invalidate(remoteAccessProvider);
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text('Tailscale hatası: $e')));
      }
    } finally {
      if (mounted) setState(() => _tsBusy = false);
    }
  }

  Future<void> _disableTailscale() async {
    setState(() => _tsBusy = true);
    try {
      await ref.read(apiClientProvider).setTailscaleMode(false, 8090);
      ref.invalidate(remoteAccessProvider);
    } catch (_) {
    } finally {
      if (mounted) setState(() => _tsBusy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    final raAsync = ref.watch(remoteAccessProvider);

    return raAsync.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (err, _) => Center(
        child: Padding(
          padding: const EdgeInsets.all(32),
          child: Text('Failed to load: $err',
              style: TextStyle(color: MemoTheme.red)),
        ),
      ),
      data: (data) => _buildStatus(context, theme, data),
    );
  }

  Widget _buildStatus(BuildContext context, ThemeColors theme, Map<String, dynamic> data) {
    final enabled = data['enabled'] as bool? ?? false;
    final running = data['running'] as bool? ?? false;
    final token = data['token'] as String? ?? '';
    final ngrokUrl = data['ngrok_url'] as String? ?? '';
    final ngrokError = data['ngrok_error'] as String? ?? '';
    final addresses = (data['addresses'] as List?)?.cast<String>() ?? [];
    final savedNgrokToken = data['ngrok_token'] as String? ?? '';
    final ngrokAutoStart = data['ngrok_auto_start'] as bool? ?? false;
    if (_ngrokTokenCtrl.text.isEmpty && savedNgrokToken.isNotEmpty) {
      _ngrokTokenCtrl.text = savedNgrokToken;
    }

    return ListView(
      padding: const EdgeInsets.all(32),
      children: [
        Row(
          children: [
            Container(
              width: 12,
              height: 12,
              decoration: BoxDecoration(
                shape: BoxShape.circle,
                color: running ? MemoTheme.green : theme.textDim,
              ),
            ),
            const SizedBox(width: 10),
            Text(
              running ? 'Remote access active' : 'Remote access off',
              style: TextStyle(
                fontSize: 15,
                fontWeight: FontWeight.w600,
                color: theme.textMain,
              ),
            ),
          ],
        ),
        const SizedBox(height: 24),

        if (token.isNotEmpty) ...[
          _label('Access Token'),
          const SizedBox(height: 6),
          _valueBox(
            child: Row(
              children: [
                Expanded(
                  child: Text(token,
                      style: TextStyle(
                          fontFamily: 'JetBrainsMono',
                          fontSize: 13,
                          color: MemoTheme.accent)),
                ),
                GestureDetector(
                  onTap: () {
                    Clipboard.setData(ClipboardData(text: token));
                    ScaffoldMessenger.of(context).showSnackBar(
                      const SnackBar(content: Text('Token copied')),
                    );
                  },
                  child: Icon(Icons.copy_rounded, size: 18, color: theme.textDim),
                ),
              ],
            ),
          ),
          const SizedBox(height: 20),
        ],

        // ── Beta features toggle ──────────────────────────────────────
        SwitchListTile(
          title: Text('Beta Özellikler',
              style: TextStyle(fontSize: 13, color: theme.textMain)),
          subtitle: Text(
            'Deneysel özellikleri aç (örn. Tailscale tüneli)',
            style: TextStyle(fontSize: 11, color: theme.textDim),
          ),
          value: data['beta'] as bool? ?? false,
          onChanged: (v) async {
            await ref.read(apiClientProvider).setBeta(v);
            ref.invalidate(remoteAccessProvider);
          },
          dense: true,
          contentPadding: EdgeInsets.zero,
          activeColor: MemoTheme.accent,
        ),
        const SizedBox(height: 12),

        // ── Tailscale (embedded, stable URL) — beta only ──────────────
        if (data['beta'] as bool? ?? false) ...[
          _buildTailscaleSection(context, theme, data),
          const SizedBox(height: 24),
        ],

        if (ngrokUrl.isNotEmpty) ...[
          _label('Ngrok Tunnel URL'),
          const SizedBox(height: 6),
          _valueBox(
            borderColor: MemoTheme.accent,
            child: Row(
              children: [
                Icon(Icons.public_rounded, size: 16, color: MemoTheme.accent),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(ngrokUrl,
                      style: TextStyle(
                          fontFamily: 'JetBrainsMono',
                          fontSize: 13,
                          color: MemoTheme.accentLight)),
                ),
                GestureDetector(
                  onTap: () {
                    Clipboard.setData(ClipboardData(text: ngrokUrl));
                    ScaffoldMessenger.of(context).showSnackBar(
                      const SnackBar(content: Text('URL copied')),
                    );
                  },
                  child: Icon(Icons.copy_rounded, size: 18, color: theme.textDim),
                ),
              ],
            ),
          ),
          const SizedBox(height: 20),
        ],
        if (savedNgrokToken.isNotEmpty) ...[
          _label('Ngrok Auth Token (saved)'),
          const SizedBox(height: 6),
          _valueBox(
            child: Row(
              children: [
                Icon(Icons.vpn_key_outlined, size: 16, color: theme.textDim),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(savedNgrokToken,
                      style: TextStyle(
                          fontFamily: 'JetBrainsMono',
                          fontSize: 13,
                          color: theme.textMain)),
                ),
              ],
            ),
          ),
          const SizedBox(height: 20),
        ],

        if (ngrokError.isNotEmpty) ...[
          Container(
            padding: const EdgeInsets.all(14),
            decoration: BoxDecoration(
              color: MemoTheme.red.withValues(alpha: 0.10),
              borderRadius: BorderRadius.circular(10),
              border: Border.all(color: MemoTheme.red.withValues(alpha: 0.35)),
            ),
            child: Row(
              children: [
                Icon(Icons.error_outline_rounded, size: 18, color: MemoTheme.red),
                const SizedBox(width: 10),
                Expanded(
                  child: Text(ngrokError,
                      style: TextStyle(fontSize: 13, color: MemoTheme.red)),
                ),
              ],
            ),
          ),
          const SizedBox(height: 20),
        ],

        if (addresses.isNotEmpty) ...[
          _label('Local Addresses'),
          const SizedBox(height: 6),
          ...addresses.map((addr) => Padding(
                padding: const EdgeInsets.only(bottom: 4),
                child: Text(addr,
                    style: TextStyle(
                        fontFamily: 'JetBrainsMono',
                        fontSize: 12,
                        color: theme.textDim)),
              )),
          const SizedBox(height: 20),
        ],

        _label('Auto-Start on Backend Launch'),
        const SizedBox(height: 6),
        SwitchListTile(
          title: Text('Start ngrok tunnel automatically',
              style: TextStyle(fontSize: 13, color: theme.textMain)),
          subtitle: Text(
            ngrokAutoStart
                ? 'Will start on next backend launch'
                : 'Start manually from this panel',
            style: TextStyle(fontSize: 11, color: theme.textDim),
          ),
          value: ngrokAutoStart,
          onChanged: (v) => _setAutoStart(v),
          dense: true,
          contentPadding: EdgeInsets.zero,
        ),
        const SizedBox(height: 20),
        _label('Configure Remote Access'),
        const SizedBox(height: 8),
        Row(
          children: [
            Expanded(
              child: TextField(
                controller: _ngrokTokenCtrl,
                decoration: const InputDecoration(
                  labelText: 'Ngrok Auth Token',
                  hintText: '2hP2x...',
                  prefixIcon: Icon(Icons.vpn_key_outlined, size: 20),
                ),
                style: TextStyle(
                    fontFamily: 'JetBrainsMono',
                    fontSize: 14,
                    color: theme.textMain),
              ),
            ),
            const SizedBox(width: 12),
            if (enabled)
              FilledButton.tonalIcon(
                onPressed: _enabling ? null : () => _disable(),
                icon: _enabling
                    ? const SizedBox(
                        width: 16,
                        height: 16,
                        child: CircularProgressIndicator(strokeWidth: 2))
                    : const Icon(Icons.power_off_rounded, size: 18),
                label: Text(_enabling ? '...' : 'Disable'),
                style: FilledButton.styleFrom(
                  backgroundColor: MemoTheme.red,
                  foregroundColor: theme.textInverse,
                  padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 14),
                ),
              )
            else
              FilledButton.icon(
                onPressed: _enabling ? null : () => _enable(),
                icon: _enabling
                    ? const SizedBox(
                        width: 16,
                        height: 16,
                        child: CircularProgressIndicator(strokeWidth: 2))
                    : const Icon(Icons.power_settings_new_rounded, size: 18),
                label: Text(_enabling ? '...' : 'Enable & Start'),
                style: FilledButton.styleFrom(
                  backgroundColor: MemoTheme.accent,
                  foregroundColor: theme.textInverse,
                  padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 14),
                ),
              ),
          ],
        ),

        if (!enabled) ...[
          const SizedBox(height: 12),
          Text(
            'Enter your ngrok auth token to start a public tunnel.\n'
            'Get it from https://dashboard.ngrok.com',
            style: TextStyle(fontSize: 12, color: theme.textDim),
          ),
        ],
      ],
    );
  }

  Widget _valueBox({Widget? child, Color? borderColor}) {
    final theme = MemoTheme.of(context);
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
      decoration: BoxDecoration(
        color: theme.bgElement,
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: borderColor ?? theme.borderSoft),
      ),
      child: child,
    );
  }

  Widget _buildTailscaleSection(
      BuildContext context, ThemeColors theme, Map<String, dynamic> data) {
    final tsUrl = data['tailscale_url'] as String? ?? '';
    final tsIp = data['tailscale_ip'] as String? ?? '';
    final tsRunning = data['tailscale_running'] as bool? ?? false;
    final tsError = data['tailscale_error'] as String? ?? '';
    final savedHost = data['tailscale_hostname'] as String? ?? 'memo';
    if (_tsHostCtrl.text.isEmpty) _tsHostCtrl.text = savedHost;

    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: theme.bgElement,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
            color: tsRunning ? MemoTheme.accent : theme.borderSoft),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(Icons.hub_outlined,
                  size: 18,
                  color: tsRunning ? MemoTheme.accent : theme.textDim),
              const SizedBox(width: 8),
              Text('Tailscale (sabit URL, gömülü)',
                  style: TextStyle(
                      fontWeight: FontWeight.w600, color: theme.textMain)),
              const Spacer(),
              if (tsRunning)
                Container(
                  width: 10,
                  height: 10,
                  decoration: const BoxDecoration(
                      shape: BoxShape.circle, color: MemoTheme.green),
                ),
            ],
          ),
          const SizedBox(height: 4),
          Text(
            'ngrok\'un aksine URL hiç değişmez ve ayrı binary indirmez. '
            'Tek seferlik bir auth key gerekir (login.tailscale.com → Settings → Keys).',
            style: TextStyle(fontSize: 12, color: theme.textDim),
          ),
          const SizedBox(height: 12),

          if (tsUrl.isNotEmpty) ...[
            _valueBox(
              borderColor: MemoTheme.accent,
              child: Row(
                children: [
                  Icon(Icons.public_rounded, size: 16, color: MemoTheme.accent),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(tsUrl,
                        style: TextStyle(
                            fontFamily: 'JetBrainsMono',
                            fontSize: 13,
                            color: MemoTheme.accent)),
                  ),
                  GestureDetector(
                    onTap: () {
                      Clipboard.setData(ClipboardData(text: tsUrl));
                      ScaffoldMessenger.of(context).showSnackBar(
                        const SnackBar(content: Text('URL kopyalandı')),
                      );
                    },
                    child:
                        Icon(Icons.copy_rounded, size: 18, color: theme.textDim),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 8),
          ],

          if (tsIp.isNotEmpty) ...[
            _valueBox(
              child: Row(
                children: [
                  Icon(Icons.lan_outlined, size: 16, color: theme.textDim),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text('$tsIp  (MagicDNS kapalıysa bunu kullan)',
                        style: TextStyle(
                            fontFamily: 'JetBrainsMono',
                            fontSize: 12.5,
                            color: theme.textMain)),
                  ),
                  GestureDetector(
                    onTap: () {
                      Clipboard.setData(ClipboardData(text: tsIp));
                      ScaffoldMessenger.of(context).showSnackBar(
                        const SnackBar(content: Text('IP kopyalandı')),
                      );
                    },
                    child:
                        Icon(Icons.copy_rounded, size: 18, color: theme.textDim),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 12),
          ],

          if (tsError.isNotEmpty) ...[
            Text('Hata: $tsError',
                style: TextStyle(fontSize: 12, color: MemoTheme.red)),
            const SizedBox(height: 12),
          ],

          if (!tsRunning) ...[
            TextField(
              controller: _tsKeyCtrl,
              decoration: const InputDecoration(
                labelText: 'Tailscale Auth Key',
                hintText: 'tskey-auth-...',
                prefixIcon: Icon(Icons.vpn_key_outlined, size: 20),
                isDense: true,
              ),
              style: TextStyle(
                  fontFamily: 'JetBrainsMono', fontSize: 13, color: theme.textMain),
            ),
            const SizedBox(height: 10),
            Row(
              children: [
                Expanded(
                  child: TextField(
                    controller: _tsHostCtrl,
                    decoration: const InputDecoration(
                      labelText: 'Cihaz adı',
                      hintText: 'memo',
                      isDense: true,
                    ),
                    style: TextStyle(fontSize: 13, color: theme.textMain),
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: SwitchListTile(
                    title: Text('Funnel (public)',
                        style: TextStyle(fontSize: 12, color: theme.textMain)),
                    subtitle: Text('Telefona kurulum gerekmez',
                        style: TextStyle(fontSize: 10, color: theme.textDim)),
                    value: _tsFunnel,
                    onChanged: (v) => setState(() => _tsFunnel = v),
                    dense: true,
                    contentPadding: EdgeInsets.zero,
                    activeColor: MemoTheme.accent,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 12),
            FilledButton.icon(
              onPressed: _tsBusy ? null : _enableTailscale,
              icon: _tsBusy
                  ? const SizedBox(
                      width: 16,
                      height: 16,
                      child: CircularProgressIndicator(strokeWidth: 2))
                  : const Icon(Icons.power_settings_new_rounded, size: 18),
              label: Text(_tsBusy ? 'Başlatılıyor...' : 'Tailscale ile Başlat'),
              style: FilledButton.styleFrom(
                backgroundColor: MemoTheme.accent,
                foregroundColor: theme.textInverse,
                padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 12),
              ),
            ),
          ] else
            FilledButton.tonalIcon(
              onPressed: _tsBusy ? null : _disableTailscale,
              icon: _tsBusy
                  ? const SizedBox(
                      width: 16,
                      height: 16,
                      child: CircularProgressIndicator(strokeWidth: 2))
                  : const Icon(Icons.power_off_rounded, size: 18),
              label: const Text('Tailscale Durdur'),
              style: FilledButton.styleFrom(
                backgroundColor: MemoTheme.red,
                foregroundColor: theme.textInverse,
              ),
            ),
        ],
      ),
    );
  }

  void _setAutoStart(bool v) async {
    try {
      await ref.read(apiClientProvider).setRemoteAccessAutoStart(v);
      // If enabling, poll until ngrok URL appears
      if (v) {
        for (int i = 0; i < 30; i++) {
          await Future.delayed(const Duration(seconds: 1));
          try {
            final status = await ref.read(apiClientProvider).getRemoteAccess();
            final url = status['ngrok_url'] as String? ?? '';
            final err = status['ngrok_error'] as String? ?? '';
            if (url.isNotEmpty || err.isNotEmpty) break;
          } catch (_) {}
        }
      }
      ref.invalidate(remoteAccessProvider);
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text('Failed: $e')));
      }
    }
  }

  void _enable() async {
    final token = _ngrokTokenCtrl.text.trim();
    if (token.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Enter ngrok auth token first')),
      );
      return;
    }
    setState(() => _enabling = true);
    try {
      await ref.read(apiClientProvider).setRemoteAccess(true, 8090,
          ngrokMode: true, ngrokToken: token);
      // Poll until ngrok URL appears or error (takes a few seconds)
      for (int i = 0; i < 30; i++) {
        await Future.delayed(const Duration(seconds: 1));
        try {
          final status = await ref.read(apiClientProvider).getRemoteAccess();
          final url = status['ngrok_url'] as String? ?? '';
          final err = status['ngrok_error'] as String? ?? '';
          if (url.isNotEmpty || err.isNotEmpty) break;
        } catch (_) {}
      }
      ref.invalidate(remoteAccessProvider);
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text('Failed: $e')));
      }
    } finally {
      if (mounted) setState(() => _enabling = false);
    }
  }

  void _disable() async {
    setState(() => _enabling = true);
    try {
      await ref.read(apiClientProvider).setRemoteAccess(false, 8090);
      ref.invalidate(remoteAccessProvider);
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Failed: $e')),
        );
      }
    } finally {
      if (mounted) setState(() => _enabling = false);
    }
  }

  Widget _label(String s) {
    return Text(s,
        style: TextStyle(
            fontSize: 11,
            fontWeight: FontWeight.w600,
            color: MemoTheme.of(context).textDim,
            letterSpacing: 1.2));
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
            ClipRRect(
              borderRadius: BorderRadius.circular(16),
              child: Image.asset(
                'lib/icon/memo.png',
                width: 64,
                height: 64,
                fit: BoxFit.cover,
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
  final _focusNode = FocusNode();
  bool _isEditing = false;

  @override
  void initState() {
    super.initState();
    _controller = TextEditingController(
      text: widget.value == 0 && widget.min == 0
          ? '0'
          : widget.value.toString(),
    );
    _focusNode.addListener(() {
      _isEditing = _focusNode.hasFocus;
    });
  }

  @override
  void didUpdateWidget(_ParamIntInput old) {
    super.didUpdateWidget(old);
    if (widget.value != old.value && !_isEditing) {
      _controller.text = widget.value == 0 && widget.min == 0
          ? '0'
          : widget.value.toString();
    }
  }

  @override
  void dispose() {
    _controller.dispose();
    _focusNode.dispose();
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
            focusNode: _focusNode,
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

// ─── Learning Tab ───────────────────────────────────────────

class _LearningTab extends ConsumerWidget {
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final settingsAsync = ref.watch(learningSettingsProvider);
    final patternsAsync = ref.watch(learningPatternsProvider);
    final theme = MemoTheme.of(context);

    // The whole tab scrolls — this keeps the layout overflow-proof no matter
    // how much content (settings cards + a long patterns list) is present.
    return ListView(
      padding: const EdgeInsets.all(20),
      children: [
        Row(
          children: [
            Text(
              'Learning Profile',
              style: TextStyle(
                fontSize: 18,
                fontWeight: FontWeight.w600,
                color: theme.textMain,
              ),
            ),
          ],
        ),
        const SizedBox(height: 8),
        Text(
          'Memo kullanim aliskanliklarini ogrenir ve proaktif olarak yardim teklif eder.',
          style: TextStyle(color: theme.textDim, fontSize: 13),
        ),
        const SizedBox(height: 16),

        // Settings card
        settingsAsync.when(
          loading: () => const CircularProgressIndicator(),
          error: (e, _) => Text('Hata: $e', style: TextStyle(color: MemoTheme.red)),
          data: (settings) => _SettingsCard(settings: settings, ref: ref),
        ),
        const SizedBox(height: 12),

        // Single model mode + calendar reminder.
        const _ModelRoutingCard(),
        const SizedBox(height: 16),

        // Patterns header
        Row(
          children: [
            Text(
              'Ogrenilen Patternler',
              style: TextStyle(fontSize: 15, fontWeight: FontWeight.w600, color: theme.textMain),
            ),
            const Spacer(),
            if (patternsAsync.valueOrNull?.isNotEmpty ?? false)
              TextButton.icon(
                onPressed: () => _clearAll(context, ref),
                icon: Icon(Icons.delete_sweep_outlined, size: 16, color: MemoTheme.red),
                label: Text('Tumunu Sil', style: TextStyle(fontSize: 12, color: MemoTheme.red)),
                style: TextButton.styleFrom(visualDensity: VisualDensity.compact),
              ),
          ],
        ),
        const SizedBox(height: 8),

        // Patterns list (rendered inline; the outer ListView handles scrolling).
        patternsAsync.when(
          loading: () => const Center(child: Padding(
            padding: EdgeInsets.all(24),
            child: CircularProgressIndicator(),
          )),
          error: (e, _) => Center(
            child: Text('Patternler yuklenemedi: $e', style: TextStyle(color: theme.textDim)),
          ),
          data: (patterns) {
            if (patterns.isEmpty) {
              return Padding(
                padding: const EdgeInsets.symmetric(vertical: 24),
                child: Center(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(Icons.auto_awesome_outlined, size: 48, color: theme.textDim),
                      const SizedBox(height: 12),
                      Text(
                        'Henuz pattern yok.',
                        style: TextStyle(color: theme.textDim, fontSize: 14),
                      ),
                      const SizedBox(height: 4),
                      Text(
                        'Memo sadece gozlem yapiyor.\nBir kac hafta icinde aliskanliklarinizi ogrenir.',
                        style: TextStyle(color: theme.textDim, fontSize: 12),
                        textAlign: TextAlign.center,
                      ),
                    ],
                  ),
                ),
              );
            }
            return Column(
              children: [
                for (int i = 0; i < patterns.length; i++) ...[
                  if (i > 0) Divider(height: 1, color: theme.borderSoft),
                  _PatternCard(pattern: patterns[i]),
                ],
              ],
            );
          },
        ),
      ],
    );
  }

  Future<void> _clearAll(BuildContext context, WidgetRef ref) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Tum Ogrenme Verilerini Sil'),
        content: const Text(
          'Tum gozlemler ve ogrenilen patternler kalici olarak silinecek. Bu islem geri alinamaz.',
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('İptal')),
          TextButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('Hepsini Sil', style: TextStyle(color: Colors.red)),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    await ref.read(apiClientProvider).clearLearningData();
    ref.invalidate(learningPatternsProvider);
  }
}

class _SettingsCard extends StatelessWidget {
  final Map<String, dynamic> settings;
  final WidgetRef ref;
  const _SettingsCard({required this.settings, required this.ref});

  @override
  Widget build(BuildContext context) {
    final enabled = settings['enabled'] as bool? ?? false;
    final level = settings['level'] as String? ?? 'off';
    final theme = MemoTheme.of(context);

    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: theme.bgElement,
        borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
        border: Border.all(color: theme.borderSoft),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Text('Proactive Learning', style: TextStyle(fontWeight: FontWeight.w500, color: theme.textMain)),
              const Spacer(),
              Switch(
                value: enabled,
                onChanged: (v) {
                  final notifier = ref.read(learningSettingsProvider.notifier);
                  // Enabling with a still-"off" level would leave the engine
                  // dormant, so default to "normal" when turning it on.
                  if (v) {
                    notifier.update(true, (level == 'off' || level.isEmpty) ? 'normal' : level);
                  } else {
                    notifier.update(false, level);
                  }
                },
                activeColor: MemoTheme.accent,
              ),
            ],
          ),
          if (enabled) ...[
            const SizedBox(height: 8),
            Text('Seviye:', style: TextStyle(fontSize: 12, color: theme.textDim)),
            const SizedBox(height: 4),
            Wrap(
              spacing: 6,
              children: ['subtle', 'normal', 'assertive'].map((l) {
                final selected = level == l;
                return ChoiceChip(
                  label: Text(l, style: TextStyle(fontSize: 11, color: selected ? Colors.white : theme.textDim)),
                  selected: selected,
                  selectedColor: MemoTheme.accent,
                  onSelected: selected ? null : (v) => ref.read(learningSettingsProvider.notifier).update(enabled, l),
                  visualDensity: VisualDensity.compact,
                );
              }).toList(),
            ),
          ],
        ],
      ),
    );
  }
}

// Single model mode + calendar reminder lead time. Loads/saves directly via
// the API client; both belong to the learning system's model routing.
class _ModelRoutingCard extends ConsumerStatefulWidget {
  const _ModelRoutingCard();

  @override
  ConsumerState<_ModelRoutingCard> createState() => _ModelRoutingCardState();
}

class _ModelRoutingCardState extends ConsumerState<_ModelRoutingCard> {
  bool _loading = true;
  String? _error;
  bool _singleModel = false;
  final _modelCtrl = TextEditingController();
  int _reminderLead = 30;
  bool _guessTime = true;
  bool _saving = false;

  static const _leadOptions = [10, 15, 30, 60, 120];

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    _modelCtrl.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    try {
      final api = ref.read(apiClientProvider);
      final learning = await api.getLearningSettings();
      final calendar = await api.getCalendarSettings();
      if (!mounted) return;
      setState(() {
        _singleModel = learning['single_model_enabled'] as bool? ?? false;
        _modelCtrl.text = learning['model_id'] as String? ?? '';
        final lead = calendar['reminder_lead_minutes'] as int? ?? 30;
        _reminderLead = _leadOptions.contains(lead) ? lead : 30;
        _guessTime = !(calendar['disable_time_guess'] as bool? ?? false);
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = '$e';
        _loading = false;
      });
    }
  }

  Future<void> _save() async {
    setState(() => _saving = true);
    try {
      final api = ref.read(apiClientProvider);
      await api.updateLearningSettings(_singleModel, _modelCtrl.text.trim());
      await api.updateCalendarSettings(_reminderLead, disableTimeGuess: !_guessTime);
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(const SnackBar(content: Text('Öğrenme ayarları kaydedildi')));
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text('Hata: $e')));
      }
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    if (_loading) {
      return const Padding(
        padding: EdgeInsets.all(8),
        child: SizedBox(height: 18, width: 18, child: CircularProgressIndicator(strokeWidth: 2)),
      );
    }
    if (_error != null) {
      return Text('Hata: $_error', style: TextStyle(color: MemoTheme.red, fontSize: 12));
    }

    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: theme.bgElement,
        borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
        border: Border.all(color: theme.borderSoft),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Text('Tek Model Modu',
                  style: TextStyle(fontWeight: FontWeight.w500, color: theme.textMain)),
              const Spacer(),
              Switch(
                value: _singleModel,
                activeColor: MemoTheme.accent,
                onChanged: (v) => setState(() => _singleModel = v),
              ),
            ],
          ),
          Text(
            'Niyet analizi ve proaktif kararlar Orchestra yerine tek modeli kullanır.',
            style: TextStyle(fontSize: 12, color: theme.textDim),
          ),
          if (_singleModel) ...[
            const SizedBox(height: 8),
            TextField(
              controller: _modelCtrl,
              style: TextStyle(color: theme.textMain, fontSize: 13),
              decoration: InputDecoration(
                isDense: true,
                hintText: 'Model ID (ör. gpt-4o-mini)',
                hintStyle: TextStyle(color: theme.textDim, fontSize: 13),
                border: const OutlineInputBorder(),
              ),
            ),
          ],
          const SizedBox(height: 14),
          Row(
            children: [
              Text('Takvim hatırlatma:',
                  style: TextStyle(fontSize: 13, color: theme.textMain)),
              const Spacer(),
              DropdownButton<int>(
                value: _reminderLead,
                underline: const SizedBox.shrink(),
                style: TextStyle(fontSize: 13, color: theme.textMain),
                items: _leadOptions
                    .map((m) => DropdownMenuItem(
                          value: m,
                          child: Text(m < 60 ? '$m dk önce' : '${m ~/ 60} saat önce'),
                        ))
                    .toList(),
                onChanged: (v) => setState(() => _reminderLead = v ?? 30),
              ),
            ],
          ),
          const SizedBox(height: 6),
          SwitchListTile(
            title: Text('Belirsiz saatleri tahmin et',
                style: TextStyle(fontSize: 13, color: theme.textMain)),
            subtitle: Text(
              '"yarın dışarı çıkalım" gibi saatsiz planlara saat ata',
              style: TextStyle(fontSize: 11, color: theme.textDim),
            ),
            value: _guessTime,
            onChanged: (v) => setState(() => _guessTime = v),
            dense: true,
            contentPadding: EdgeInsets.zero,
            activeColor: MemoTheme.accent,
          ),
          const SizedBox(height: 10),
          Align(
            alignment: Alignment.centerRight,
            child: TextButton(
              onPressed: _saving ? null : _save,
              child: _saving
                  ? const SizedBox(height: 16, width: 16, child: CircularProgressIndicator(strokeWidth: 2))
                  : const Text('Kaydet'),
            ),
          ),
        ],
      ),
    );
  }
}

class _PatternCard extends ConsumerWidget {
  final LearnedPattern pattern;
  const _PatternCard({required this.pattern});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = MemoTheme.of(context);
    final confPct = (pattern.confidence * 100).round();

    Color confColor;
    if (confPct >= 70) {
      confColor = MemoTheme.green;
    } else if (confPct >= 40) {
      confColor = MemoTheme.accent;
    } else {
      confColor = theme.textDim;
    }

    return Container(
      padding: const EdgeInsets.all(12),
      margin: const EdgeInsets.symmetric(vertical: 4),
      decoration: BoxDecoration(
        color: theme.bgElement,
        borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
        border: Border.all(color: theme.borderSoft),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                decoration: BoxDecoration(
                  color: confColor.withValues(alpha: 0.15),
                  borderRadius: BorderRadius.circular(4),
                ),
                child: Text(
                  confPct >= 70 ? '🔥 ${confPct}%' : confPct >= 40 ? '📈 ${confPct}%' : '📊 ${confPct}%',
                  style: TextStyle(fontSize: 11, fontWeight: FontWeight.w600, color: confColor, fontFamily: 'JetBrains Mono'),
                ),
              ),
              const SizedBox(width: 8),
              Text(
                pattern.activityType,
                style: TextStyle(fontWeight: FontWeight.w600, fontSize: 13, color: theme.textMain),
              ),
              const Spacer(),
              IconButton(
                icon: Icon(Icons.delete_outline, size: 16, color: theme.textDim),
                onPressed: () => _forget(context, ref),
                visualDensity: VisualDensity.compact,
                padding: EdgeInsets.zero,
                constraints: const BoxConstraints(),
              ),
            ],
          ),
          const SizedBox(height: 6),
          Row(
            children: [
              _infoChip(Icons.schedule, pattern.timeDisplay, theme),
              const SizedBox(width: 8),
              _infoChip(Icons.timer_outlined, pattern.stdDisplay, theme),
              const SizedBox(width: 8),
              _infoChip(Icons.calendar_view_week, pattern.daysDisplay, theme),
              if (pattern.totalCount > 0) ...[
                const SizedBox(width: 8),
                _infoChip(Icons.repeat, '${pattern.totalCount}x', theme),
              ],
            ],
          ),
        ],
      ),
    );
  }

  Widget _infoChip(IconData icon, String text, ThemeColors theme) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 5, vertical: 2),
      decoration: BoxDecoration(
        color: theme.bgApp,
        borderRadius: BorderRadius.circular(3),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 10, color: theme.textDim),
          const SizedBox(width: 3),
          Text(text, style: TextStyle(fontSize: 10, color: theme.textDim, fontFamily: 'JetBrains Mono')),
        ],
      ),
    );
  }

  Future<void> _forget(BuildContext context, WidgetRef ref) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Pattern\'i Unut'),
        content: Text('"${pattern.activityType}" pattern\'ini silmek istedigine emin misin?'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('İptal')),
          TextButton(onPressed: () => Navigator.pop(ctx, true), child: const Text('Unut', style: TextStyle(color: Colors.red))),
        ],
      ),
    );
    if (confirmed != true) return;
    final api = ref.read(apiClientProvider);
    await api.forgetPattern(pattern.id);
    ref.invalidate(learningPatternsProvider);
  }
}

// ─── Cloud Backup Text Field ──────────────────────────────────────
class _CloudTextField extends StatelessWidget {
  final String label;
  final TextEditingController controller;
  final String hint;
  final bool obscure;

  const _CloudTextField({
    required this.label,
    required this.controller,
    required this.hint,
    this.obscure = false,
  });

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        SizedBox(
          width: 160,
          child: Text(
            label,
            style: TextStyle(fontSize: 13, color: MemoTheme.of(context).textMain),
          ),
        ),
        Expanded(
          child: SizedBox(
            height: 36,
            child: TextField(
              controller: controller,
              obscureText: obscure,
              style: TextStyle(fontSize: 13),
              decoration: InputDecoration(
                hintText: hint,
                hintStyle: TextStyle(fontSize: 12),
                contentPadding: EdgeInsets.symmetric(horizontal: 12),
                filled: true,
                fillColor: MemoTheme.of(context).bgApp,
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                  borderSide: BorderSide(color: MemoTheme.of(context).borderSoft),
                ),
                enabledBorder: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                  borderSide: BorderSide(color: MemoTheme.of(context).borderSoft),
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

// ─── Skills Tab ────────────────────────────────────────────

class _SkillsTab extends ConsumerWidget {
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final skillsAsync = ref.watch(skillListProvider);
    final theme = MemoTheme.of(context);

    return Padding(
      padding: const EdgeInsets.all(20),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Text('🧩', style: TextStyle(fontSize: 24)),
              const SizedBox(width: 10),
              Text(
                'Skills',
                style: TextStyle(
                  fontSize: 18,
                  fontWeight: FontWeight.w600,
                  color: theme.textMain,
                ),
              ),
              const Spacer(),
              TextButton.icon(
                onPressed: () => _showSkillManager(context, ref),
                icon: Icon(Icons.add, size: 16),
                label: const Text('Skill Yönetimi'),
              ),
            ],
          ),
          const SizedBox(height: 8),
          Text(
            'Skill\'ler agent\'a ek talimatlar ve araçlar kazandırır.',
            style: TextStyle(color: theme.textDim, fontSize: 13),
          ),
          const SizedBox(height: 20),
          Expanded(
            child: skillsAsync.when(
              loading: () => const Center(child: CircularProgressIndicator()),
              error: (e, _) => Center(
                child: Text('Yüklenemedi: $e', style: TextStyle(color: theme.textDim)),
              ),
              data: (skills) {
                if (skills.isEmpty) {
                  return Center(
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(Icons.extension_off, size: 48, color: theme.textDim),
                        const SizedBox(height: 12),
                        Text(
                          'Henüz skill yüklenmemiş.',
                          style: TextStyle(color: theme.textDim, fontSize: 14),
                        ),
                        const SizedBox(height: 4),
                        Text(
                          'data/skills/ klasörüne SKILL.md dosyası ekleyin veya\n"Skill Yönetimi" butonundan yükleyin.',
                          style: TextStyle(color: theme.textDim, fontSize: 12),
                          textAlign: TextAlign.center,
                        ),
                      ],
                    ),
                  );
                }
                return ListView.separated(
                  itemCount: skills.length,
                  separatorBuilder: (_, __) => Divider(height: 1, color: theme.borderSoft),
                  itemBuilder: (_, i) {
                    final s = skills[i];
                    return ListTile(
                      leading: Text(s.isActive ? '✅' : '🧩', style: const TextStyle(fontSize: 24)),
                      title: Text(
                        s.name,
                        style: TextStyle(
                          fontWeight: FontWeight.w500,
                          color: theme.textMain,
                          fontFamily: 'JetBrains Mono',
                          fontSize: 13,
                        ),
                      ),
                      subtitle: Text(
                        s.description,
                        style: TextStyle(color: theme.textDim, fontSize: 12),
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                      ),
                      trailing: Switch(
                        value: s.isActive,
                        onChanged: (v) => _toggleSkill(ref, s.name, v),
                        activeColor: MemoTheme.accent,
                      ),
                    );
                  },
                );
              },
            ),
          ),
        ],
      ),
    );
  }

  void _showSkillManager(BuildContext context, WidgetRef ref) {
    showDialog(
      context: context,
      builder: (_) => const SkillConfigDialog(),
    ).then((_) => ref.invalidate(skillListProvider));
  }

  Future<void> _toggleSkill(WidgetRef ref, String name, bool active) async {
    final notifier = ref.read(skillListProvider.notifier);
    await notifier.toggleSkill(name, active);
  }
}
