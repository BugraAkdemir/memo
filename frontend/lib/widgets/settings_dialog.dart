import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

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
    'API Providers',
    '🎵 Orchestra',
    'Ekran Kartı Config',
    L10n.t('cloud_sync'),
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
              padding:  EdgeInsets.symmetric(horizontal: 24),
              decoration: BoxDecoration(
                color: MemoTheme.of(context).bgPanel,
                border: Border(bottom: BorderSide(color: MemoTheme.of(context).borderSoft)),
              ),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Text(
                    L10n.t('settings'),
                    style:  TextStyle(
                      fontSize: 16,
                      fontWeight: FontWeight.w600,
                      color: MemoTheme.of(context).textMain,
                    ),
                  ),
                  IconButton(
                    icon:  Icon(Icons.close, size: 20),
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
                      color: MemoTheme.of(context).bgPanel.withValues(alpha: 0.5),
                      border: Border(
                        right: BorderSide(color: MemoTheme.of(context).borderSoft),
                      ),
                    ),
                    child: ListView.builder(
                      padding:  EdgeInsets.symmetric(vertical: 12),
                      itemCount: tabs.length,
                      itemBuilder: (context, index) {
                        final isActive = _activeTab == index;
                        return InkWell(
                          onTap: () => setState(() => _activeTab = index),
                          child: Container(
                            padding:  EdgeInsets.symmetric(
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
    // 0: General
    // 1: System Prompt
    // 2: Incognito Prompt
    // 3: Memory
    // 4: API Providers
    // 5: GPU Config
    // 6: Cloud Sync
    // 7: Remote Access
    // 8: About
    switch (index) {
      case 0:
        return  _GeneralTab();
      case 1:
        return  _SystemPromptTab();
      case 2:
        return  _IncognitoPromptTab();
      case 3:
        return  _MemoryTab();
      case 4:
        return  _ProvidersTab();
      case 5:
        return  _OrchestraTab();
      case 6:
        return  _GpuConfigTab();
      case 7:
        return  _CloudSyncTab();
      case 8:
        return  _RemoteAccessTab();
      case 9:
        return  _AboutTab();
      default:
        return  SizedBox.shrink();
    }
  }
}

// ─── API Providers Tab ─────────────────────────────────────────────

class _ProvidersTab extends ConsumerWidget {
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final providersAsync = ref.watch(providerListProvider);

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
          style: TextStyle(
            color: MemoTheme.of(context).textDim,
            fontSize: 13,
          ),
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
              children: providers.map((p) => _ProviderCard(p: p)).toList(),
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

  const _ProviderCard({required this.p});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final icon = providerIcon(p.type);

    return Card(
      margin: const EdgeInsets.only(bottom: 12),
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
              child: Center(
                child: Text(icon, style: const TextStyle(fontSize: 24)),
              ),
            ),
            const SizedBox(width: 16),

            // Info
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    p.name,
                    style: const TextStyle(
                      fontWeight: FontWeight.w600,
                      fontSize: 14,
                    ),
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
                              ? Colors.green.withValues(alpha: 0.1)
                              : Colors.grey.withValues(alpha: 0.1),
                          borderRadius: BorderRadius.circular(4),
                        ),
                        child: Text(
                          p.enabled ? 'Enabled' : 'Disabled',
                          style: TextStyle(
                            fontSize: 11,
                            color: p.enabled ? Colors.green : Colors.grey,
                            fontWeight: FontWeight.w500,
                          ),
                        ),
                      ),
                      const SizedBox(width: 8),
                      if (p.connected)
                        const Row(
                          children: [
                            Icon(Icons.check_circle,
                                size: 14, color: Colors.green),
                            SizedBox(width: 4),
                            Text(
                              'Connected',
                              style: TextStyle(
                                fontSize: 11,
                                color: Colors.green,
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
                  await ref.read(providerListProvider.notifier).updateProvider(
                    p.copyWith(enabled: !p.enabled),
                  );
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
                      Icon(Icons.delete, size: 18, color: Colors.red),
                      SizedBox(width: 8),
                      Text('Delete', style: TextStyle(color: Colors.red)),
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
              style: TextStyle(fontSize: 20, fontWeight: FontWeight.w600, color: MemoTheme.of(context).textMain),
            ),
          ],
        ),
        const SizedBox(height: 8),
        Text(
          'Birden çok modeli aynı anda bir ekip olarak çalıştır. '
          'Bir Şef (Chief) model kullanıcının isteğini analiz eder, '
          'alt görevlere böler ve her görevi uzmanlaşmış modele atar.',
          style: TextStyle(fontSize: 13, color: MemoTheme.of(context).textSecondary),
        ),
        const SizedBox(height: 24),

        // Status card
        configAsync.when(
          data: (config) => Container(
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(
              color: config.enabled ? MemoTheme.accent.withValues(alpha: 0.1) : MemoTheme.of(context).bgPanel,
              borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
              border: Border.all(
                color: config.enabled ? MemoTheme.accent.withValues(alpha: 0.3) : MemoTheme.of(context).borderSoft,
              ),
            ),
            child: Row(
              children: [
                Container(
                  width: 40, height: 40,
                  decoration: BoxDecoration(
                    color: config.enabled ? MemoTheme.accent.withValues(alpha: 0.2) : MemoTheme.of(context).bgElement,
                    borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                  ),
                  child: Center(
                    child: Text(config.enabled ? '🎵' : '⏸️', style: const TextStyle(fontSize: 20)),
                  ),
                ),
                const SizedBox(width: 16),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        config.enabled ? 'Orchestra Mode Aktif' : 'Orchestra Mode Pasif',
                        style: TextStyle(fontSize: 14, fontWeight: FontWeight.w600, color: MemoTheme.of(context).textMain),
                      ),
                      const SizedBox(height: 4),
                      Text(
                        config.enabled
                            ? 'Şef: ${config.chiefType}/${config.chiefModel} • ${config.roles.where((r) => r.enabled).length} rol aktif'
                            : 'Aktifleştirmek için aç/kapa yap',
                        style: TextStyle(fontSize: 12, color: MemoTheme.of(context).textDim),
                      ),
                    ],
                  ),
                ),
                const SizedBox(width: 12),
                Transform.scale(
                  scale: 0.8,
                  child: Switch(
                    value: config.enabled,
                    onChanged: (v) => ref.read(orchestraConfigProvider.notifier).toggle(v),
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
                  style: TextStyle(fontSize: 14, fontWeight: FontWeight.w600, color: MemoTheme.of(context).textMain),
                ),
                const SizedBox(height: 12),
                ...config.roles.where((r) => r.enabled).map((role) => Container(
                  margin: const EdgeInsets.only(bottom: 8),
                  padding: const EdgeInsets.all(12),
                  decoration: BoxDecoration(
                    color: MemoTheme.of(context).bgPanel,
                    borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                    border: Border.all(color: MemoTheme.of(context).borderSoft),
                  ),
                  child: Row(
                    children: [
                      Text(OrchestraDefaults.iconForRole(role.role), style: const TextStyle(fontSize: 16)),
                      const SizedBox(width: 12),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              OrchestraDefaults.labelForRole(role.role),
                              style: TextStyle(fontSize: 13, fontWeight: FontWeight.w500, color: MemoTheme.of(context).textMain),
                            ),
                            const SizedBox(height: 2),
                            Text(
                              '${providerIcon(role.modelType)} ${role.modelType} → ${role.modelName}',
                              style: TextStyle(fontSize: 11, color: MemoTheme.of(context).textDim),
                            ),
                          ],
                        ),
                      ),
                    ],
                  ),
                )),
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
      padding:  EdgeInsets.all(32),
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
          style:  TextStyle(
            fontSize: 14,
            fontWeight: FontWeight.w600,
            color: MemoTheme.of(context).textMain,
          ),
        ),
         SizedBox(height: 12),
        Container(
          padding:  EdgeInsets.symmetric(horizontal: 16),
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
              icon:  Icon(Icons.arrow_drop_down, color: MemoTheme.of(context).textDim),
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
          style:  TextStyle(
            fontSize: 14,
            fontWeight: FontWeight.w600,
            color: MemoTheme.of(context).textMain,
          ),
        ),
         SizedBox(height: 12),
        Container(
          padding:  EdgeInsets.symmetric(horizontal: 16),
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
              icon:  Icon(Icons.arrow_drop_down, color: MemoTheme.of(context).textDim),
              items: const [
                DropdownMenuItem(value: 'system', child: Text('Sistem Varsayılanı')),
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
          style:  TextStyle(
            fontSize: 14,
            fontWeight: FontWeight.w600,
            color: MemoTheme.of(context).textMain,
          ),
        ),
         SizedBox(height: 12),
        Container(
          padding:  EdgeInsets.all(16),
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
                      style:  TextStyle(
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
          'Hafıza',
          style:  TextStyle(
            fontSize: 14,
            fontWeight: FontWeight.w600,
            color: MemoTheme.of(context).textMain,
          ),
        ),
         SizedBox(height: 12),
        Container(
          padding:  EdgeInsets.all(16),
          decoration: BoxDecoration(
            color: MemoTheme.of(context).bgPanel,
            borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
            border: Border.all(color: MemoTheme.of(context).borderSoft),
          ),
          child: memoryEnabledAsync.when(
            loading: () =>  SizedBox(
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
                        enabled ? 'Hafıza Aktif' : 'Hafıza Kapalı',
                        style:  TextStyle(
                          fontSize: 14,
                          color: MemoTheme.of(context).textMain,
                        ),
                      ),
                       SizedBox(height: 4),
                      Text(
                        'Kapalıyken hafıza sorgulanmaz ve yeni anı kaydedilmez. '
                        'Model %100 ham performansla çalışır.',
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
                  onChanged: (_) {
                    ref.read(memoryEnabledProvider.notifier).toggle();
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
          style:  TextStyle(
            fontSize: 14,
            fontWeight: FontWeight.w600,
            color: MemoTheme.of(context).textMain,
          ),
        ),
         SizedBox(height: 12),
        Container(
          padding:  EdgeInsets.all(16),
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
                style:  TextStyle(fontSize: 14, color: MemoTheme.of(context).textMain),
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
      padding:  EdgeInsets.all(32),
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
          loading: () =>  Center(child: CircularProgressIndicator()),
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
                  style:  TextStyle(
                    fontSize: 13,
                    fontFamily: 'JetBrains Mono',
                  ),
                  decoration:  InputDecoration(alignLabelWithHint: true),
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
      padding:  EdgeInsets.all(32),
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
          loading: () =>  Center(child: CircularProgressIndicator()),
          error: (e, _) => Text('${L10n.t('error')}: $e'),
          data: (prompt) {
            if (!_initialized) {
              _controller.text = prompt;
              _initialized = true;
            }
            return Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                TextField(
                  controller: _controller,
                  maxLines: 12,
                  style:  TextStyle(
                    fontSize: 13,
                    fontFamily: 'JetBrains Mono',
                  ),
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
            padding:  EdgeInsets.all(32),
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
                style: TextStyle(color: MemoTheme.of(context).textDim, fontSize: 13),
              ),
               SizedBox(height: 20),
              settingsAsync.when(
                loading: () =>  Center(child: CircularProgressIndicator()),
                error: (e, _) => Text('${L10n.t('error')}: $e'),
                data: (settings) {
                  if (!_settingsInitialized) {
                    _topKController.text = settings.topK.toString();
                    _minSimilarityController.text = settings.minSimilarity
                        .toStringAsFixed(2);
                    _settingsInitialized = true;
                  }

                  return Container(
                    padding:  EdgeInsets.all(16),
                    decoration: BoxDecoration(
                      color: MemoTheme.of(context).bgPanel,
                      borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                      border: Border.all(color: MemoTheme.of(context).borderSoft),
                    ),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        Text(
                          L10n.t('memory_retrieval_settings'),
                          style:  TextStyle(
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
                                  ?  SizedBox(
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
                    icon:  Icon(Icons.delete_sweep, size: 18),
                    label: Text(L10n.t('clear_memory')),
                    style: OutlinedButton.styleFrom(
                      foregroundColor: MemoTheme.red,
                      side:  BorderSide(color: MemoTheme.red),
                    ),
                    onPressed: () async {
                      final confirmed = await showDialog<bool>(
                        context: context,
                        builder: (ctx) => AlertDialog(
                          backgroundColor: MemoTheme.of(context).bgPanel,
                          title:  Text('Hafızayı Temizle'),
                          content:  Text(
                            'Tüm hafıza dosyaları silinecek. Emin misin?',
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
                              child:  Text('Temizle'),
                            ),
                          ],
                        ),
                      );
                      if (confirmed == true) {
                        ref.read(memoryFilesProvider.notifier).clearAll();
                      }
                    },
                  ),
                ],
              ),
               SizedBox(height: 12),
              memoryAsync.when(
                loading: () =>  Center(child: CircularProgressIndicator()),
                error: (e, _) => Center(child: Text('${L10n.t('error')}: $e')),
                data: (files) {
                  if (files.isEmpty) {
                    return Padding(
                      padding:  EdgeInsets.only(top: 40),
                      child: Center(
                        child: Text(
                          L10n.t('no_memory_files'),
                          style: TextStyle(color: MemoTheme.of(context).textDim),
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
                              style:  TextStyle(
                                fontWeight: FontWeight.w500,
                              ),
                            ),
                            subtitle: Text(
                              '${file.sizeKb} KB • ${file.modified}',
                              style: TextStyle(
                                color: MemoTheme.of(context).textDim,
                                fontSize: 12,
                              ),
                            ),
                            trailing: IconButton(
                              icon:  Icon(Icons.delete_outline),
                              color: MemoTheme.red,
                              onPressed: () {
                                ref
                                    .read(memoryFilesProvider.notifier)
                                    .deleteFile(file.path);
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
            style:  TextStyle(fontWeight: FontWeight.w500, fontSize: 13),
          ),
        ),
        Expanded(
          child: SizedBox(
            height: 36,
            child: TextField(
              controller: controller,
              keyboardType:  TextInputType.numberWithOptions(
                decimal: true,
              ),
              inputFormatters: inputFormatters,
              style:  TextStyle(fontSize: 13),
              decoration: InputDecoration(
                hintText: hint,
                contentPadding:  EdgeInsets.symmetric(horizontal: 12),
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
                  borderSide:  BorderSide(color: MemoTheme.accent),
                ),
              ),
            ),
          ),
        ),
      ],
    );
  }
}

class _CloudSyncTab extends ConsumerStatefulWidget {
   _CloudSyncTab();

  @override
  ConsumerState<_CloudSyncTab> createState() => _CloudSyncTabState();
}

class _CloudSyncTabState extends ConsumerState<_CloudSyncTab> {
  Future<void> _startAuth() async {
    try {
      final api = ref.read(apiClientProvider);
      final url = await api.startSyncAuth();
      if (url.isNotEmpty && mounted) {
        // Copy URL to clipboard — user opens in browser.
        await Clipboard.setData(ClipboardData(text: url));
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('OAuth URL kopyalandı: $url')));
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('$e')));
      }
    }
  }

  Future<void> _syncNow() async {
    try {
      await ref.read(apiClientProvider).syncNow();
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('Senkronizasyon başlatıldı')));
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('$e')));
      }
    }
  }

  Future<void> _disconnect() async {
    try {
      await ref.read(apiClientProvider).disconnectSync();
      if (mounted) {
        ref.invalidate(syncAuthProvider);
        ref.invalidate(syncAccountProvider);
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('Bağlantı kesildi')));
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('$e')));
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final authAsync = ref.watch(syncAuthProvider);
    final accountAsync = ref.watch(syncAccountProvider);

    return ListView(
      padding:  EdgeInsets.all(32),
      children: [
        Text(
          L10n.t('cloud_sync'),
          style: Theme.of(context).textTheme.titleLarge?.copyWith(
            fontWeight: FontWeight.bold,
            color: MemoTheme.of(context).textMain,
          ),
        ),
         SizedBox(height: 12),
        Text(
          'Google Drive üzerinde otomatik yedekleme. Her 50 mesajda bir senkronizasyon yapılır.',
          style: TextStyle(color: MemoTheme.of(context).textDim, fontSize: 13),
        ),
         SizedBox(height: 24),

        // Auth status
        authAsync.when(
          loading: () =>  Center(child: CircularProgressIndicator()),
          error: (e, _) => Text('${L10n.t('error')}: $e'),
          data: (authenticated) => Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Row(
                children: [
                  Icon(
                    authenticated ? Icons.check_circle : Icons.cloud_off,
                    color: authenticated ? MemoTheme.green : MemoTheme.red,
                    size: 20,
                  ),
                   SizedBox(width: 8),
                  Text(
                    authenticated ? 'Google Drive bağlı' : 'Bağlı değil',
                    style: TextStyle(
                      fontWeight: FontWeight.w500,
                      color: authenticated
                          ? MemoTheme.green
                          : MemoTheme.of(context).textMuted,
                    ),
                  ),
                ],
              ),
              if (authenticated) ...[
                 SizedBox(height: 8),
                accountAsync.when(
                  loading: () =>  SizedBox.shrink(),
                  error: (_, __) =>  SizedBox.shrink(),
                  data: (account) {
                    final email = account['email'] as String?;
                    if (email != null && email.isNotEmpty) {
                      return Padding(
                        padding:  EdgeInsets.only(left: 28),
                        child: Text(
                          email,
                          style: TextStyle(
                            color: MemoTheme.of(context).textDim,
                            fontSize: 13,
                          ),
                        ),
                      );
                    }
                    return  SizedBox.shrink();
                  },
                ),
              ],
               SizedBox(height: 20),
              Row(
                children: [
                  if (!authenticated)
                    ElevatedButton.icon(
                      onPressed: _startAuth,
                      icon:  Icon(Icons.login, size: 16),
                      label: Text('Google ile bağlan'),
                      style: ElevatedButton.styleFrom(
                        backgroundColor: MemoTheme.accent,
                        foregroundColor: MemoTheme.of(context).textInverse,
                      ),
                    ),
                  if (authenticated) ...[
                    OutlinedButton.icon(
                      onPressed: _syncNow,
                      icon:  Icon(Icons.sync, size: 16),
                      label: Text('Şimdi senkronize et'),
                    ),
                     SizedBox(width: 8),
                    OutlinedButton.icon(
                      onPressed: _disconnect,
                      icon:  Icon(Icons.link_off, size: 16),
                      label: Text('Bağlantıyı kes'),
                      style: OutlinedButton.styleFrom(
                        foregroundColor: MemoTheme.red,
                        side:  BorderSide(color: MemoTheme.red),
                      ),
                    ),
                  ],
                ],
              ),
            ],
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
          Icon(Icons.wifi_tethering, size: 48, color: MemoTheme.of(context).textDim),
           SizedBox(height: 16),
          Text(
            L10n.t('remote_access'),
            style: Theme.of(
              context,
            ).textTheme.titleLarge?.copyWith(color: MemoTheme.of(context).textMuted),
          ),
           SizedBox(height: 8),
          Text(
            'Bu özellik v3.0.0\'da devre dışı bırakılmıştır. Gelecek bir sürümde tekrar eklenecek.',
            style: Theme.of(
              context,
            ).textTheme.bodyMedium?.copyWith(color: MemoTheme.of(context).textDim),
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
      padding:  EdgeInsets.all(32),
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
              child:  Center(
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
                  loading: () =>  Text('...'),
                  error: (_, _) =>  SizedBox(),
                  data: (v) =>
                      Text(v, style: TextStyle(color: MemoTheme.of(context).textDim)),
                ),
              ],
            ),
          ],
        ),
         SizedBox(height: 48),
        Text(
          'Vizyon ve Misyon',
          style: TextStyle(fontWeight: FontWeight.bold, fontSize: 16),
        ),
         SizedBox(height: 8),
        Text(
          'Memo, tamamen yerel bilgisayarınızda çalışan, sizin konuşmalarınızı ve tercihlerinizi zamanla öğrenip kalıcı hafızasına kazıyan özel bir yapay zeka asistanıdır. Asıl amaç, bulut teknolojilere muhtaç kalmadan, özgürce ve güvenle kendi bilgisayarında barındırabileceğiniz akıllı bir asistan yaratmaktır.',
          style: TextStyle(height: 1.6, color: MemoTheme.of(context).textMuted),
        ),
         SizedBox(height: 32),
        Text(
          'Açık Kaynak (MIT Lisansı)',
          style: TextStyle(fontWeight: FontWeight.bold, fontSize: 16),
        ),
         SizedBox(height: 8),
        Text(
          'Bu yazılım MIT lisansı ile açık kaynak olarak sunulmaktadır. Geliştirici: Buğra Akdemir',
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
      padding:  EdgeInsets.all(32),
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
          padding:  EdgeInsets.all(20),
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
                    color: hasGpu ? MemoTheme.accent : MemoTheme.of(context).textDim,
                  ),
                   SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      hasGpu
                          ? 'Algılanan Ekran Kartı: $gpuName'
                          : 'Sadece İşlemci (CPU) algılandı veya GPU desteklenmiyor.',
                      style: TextStyle(fontSize: 14, color: MemoTheme.of(context).textMain),
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
         SizedBox(height: 24),

        installedAsync.when(
          loading: () =>  Center(child: CircularProgressIndicator()),
          error: (e, _) => Text('${L10n.t('error')}: $e'),
          data: (installed) {
            final llamaSettings = ref.watch(llamaSettingsProvider);
            return Column(
              children: [
                // Engine Mode Selection
                Container(
                  padding:  EdgeInsets.all(20),
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
                        loading: () =>  CircularProgressIndicator(),
                        error: (e, _) => Text('Hata: $e'),
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
                                ref.read(llamaSettingsProvider.notifier).save(LlamaSettings(
                                  engineMode: mode, binaryPath: cur.binaryPath,
                                  port: cur.port, ctxSize: cur.ctxSize,
                                  temperature: cur.temperature, topP: cur.topP, maxTokens: cur.maxTokens,
                                ));
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
                  padding:  EdgeInsets.all(20),
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
                            color: installed ? MemoTheme.green : Colors.orange,
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
                          padding:  EdgeInsets.only(bottom: 16),
                          child: Text(
                            _error,
                            style:  TextStyle(
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
                              ?  SizedBox(
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
                                  style:  TextStyle(
                                    fontWeight: FontWeight.w600,
                                  ),
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

  @override
  Widget build(BuildContext context) {
    final llamaSettings = ref.watch(llamaSettingsProvider);
    return llamaSettings.when(
      loading: () =>  SizedBox.shrink(),
      error: (e, _) => Text('Hata: $e'),
      data: (settings) {
        if (!_loaded) {
          _temperature = settings.temperature;
          _topP = settings.topP;
          _maxTokens = settings.maxTokens;
          _ctxSize = settings.ctxSize;
          _loaded = true;
        }
        return Container(
          padding:  EdgeInsets.all(20),
          decoration: BoxDecoration(
            color: MemoTheme.of(context).bgPanel,
            borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
            border: Border.all(color: MemoTheme.of(context).borderSoft),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
               Text('Model Parametreleri',
                style: TextStyle(
                  fontSize: 14, fontWeight: FontWeight.w600, color: MemoTheme.of(context).textMain,
                ),
              ),
               SizedBox(height: 16),
              _ParamSlider(
                label: 'Temperature', value: _temperature,
                min: 0.0, max: 2.0, divisions: 40,
                displayValue: _temperature.toStringAsFixed(2),
                onChanged: (v) => setState(() => _temperature = v),
              ),
               SizedBox(height: 12),
              _ParamSlider(
                label: 'Top P', value: _topP,
                min: 0.0, max: 1.0, divisions: 20,
                displayValue: _topP.toStringAsFixed(2),
                onChanged: (v) => setState(() => _topP = v),
              ),
               SizedBox(height: 12),
              _ParamIntInput(
                label: 'Max Tokens', value: _maxTokens,
                min: 0, max: 65536, step: 256,
                displaySuffix: '0 = limitsiz',
                onChanged: (v) => setState(() => _maxTokens = v),
              ),
               SizedBox(height: 12),
              _ParamIntInput(
                label: 'Context Size', value: _ctxSize,
                min: 512, max: 65536, step: 512,
                onChanged: (v) => setState(() => _ctxSize = v),
              ),
               SizedBox(height: 16),
              SizedBox(
                width: double.infinity, height: 44,
                child: ElevatedButton(
                  onPressed: () async {
                    await ref.read(llamaSettingsProvider.notifier).save(
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
                    if (context.mounted) {
                      ScaffoldMessenger.of(context).showSnackBar(
                        SnackBar(
                          content:  Text('Parametreler kaydedildi.'),
                          duration:  Duration(seconds: 2),
                          backgroundColor: MemoTheme.green,
                        ),
                      );
                    }
                  },
                  style: ElevatedButton.styleFrom(
                    backgroundColor: MemoTheme.accent,
                    foregroundColor: MemoTheme.of(context).textInverse,
                  ),
                  child:  Text('Uygula', style: TextStyle(fontWeight: FontWeight.w600)),
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
    required this.label, required this.value,
    required this.min, required this.max, required this.divisions,
    required this.displayValue, required this.onChanged,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            Text(label, style:  TextStyle(fontSize: 13, color: MemoTheme.of(context).textMain)),
            Text(displayValue, style:  TextStyle(fontSize: 13, color: MemoTheme.accent, fontWeight: FontWeight.w600)),
          ],
        ),
        Slider(
          value: value, min: min, max: max,
          divisions: divisions,
          activeColor: MemoTheme.accent,
          inactiveColor: MemoTheme.of(context).borderSoft,
          onChanged: onChanged,
        ),
      ],
    );
  }
}

class _ParamIntInput extends StatelessWidget {
  final String label;
  final int value;
  final int min;
  final int max;
  final int step;
  final String? displaySuffix;
  final ValueChanged<int> onChanged;

   _ParamIntInput({
    required this.label, required this.value,
    required this.min, required this.max, required this.step,
    this.displaySuffix, required this.onChanged,
  });

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        SizedBox(
          width: 140,
          child: Text(label, style:  TextStyle(fontSize: 13, color: MemoTheme.of(context).textMain)),
        ),
        SizedBox(
          width: 120,
          child: TextField(
            keyboardType: TextInputType.number,
            decoration:  InputDecoration(
              isDense: true,
              contentPadding: EdgeInsets.symmetric(horizontal: 8, vertical: 6),
              border: OutlineInputBorder(borderRadius: BorderRadius.all(Radius.circular(4))),
            ),
            controller: TextEditingController(text: value == 0 && min == 0 ? '0' : value.toString()),
            onChanged: (v) {
              final parsed = int.tryParse(v);
              if (parsed != null) {
                onChanged(parsed.clamp(min, max));
              }
            },
          ),
        ),
        if (displaySuffix != null) ...[
           SizedBox(width: 8),
          Text(displaySuffix!, style:  TextStyle(fontSize: 11, color: MemoTheme.of(context).textDim)),
        ],
      ],
    );
  }
}
