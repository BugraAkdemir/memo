import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../providers/settings_provider.dart';

/// Settings dialog with vertical tabs on the left and content on the right.
class SettingsDialog extends ConsumerStatefulWidget {
  const SettingsDialog({super.key});

  @override
  ConsumerState<SettingsDialog> createState() => _SettingsDialogState();
}

class _SettingsDialogState extends ConsumerState<SettingsDialog> {
  int _activeTab = 0;

  final List<String> _tabs = [
    L10n.t('general'),
    L10n.t('system_prompt'),
    L10n.t('incognito_prompt'),
    L10n.t('memory'),
    L10n.t('cloud_sync'),
    L10n.t('remote_access'),
    L10n.t('about'),
  ];

  @override
  Widget build(BuildContext context) {
    return Dialog(
      backgroundColor: MemoTheme.bgApp,
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
              padding: const EdgeInsets.symmetric(horizontal: 24),
              decoration: BoxDecoration(
                color: MemoTheme.bgPanel,
                border: Border(bottom: BorderSide(color: MemoTheme.borderSoft)),
              ),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Text(
                    L10n.t('settings'),
                    style: const TextStyle(
                      fontSize: 16,
                      fontWeight: FontWeight.w600,
                      color: MemoTheme.textMain,
                    ),
                  ),
                  IconButton(
                    icon: const Icon(Icons.close, size: 20),
                    onPressed: () => Navigator.of(context).pop(),
                    color: MemoTheme.textDim,
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
                      color: MemoTheme.bgPanel.withValues(alpha: 0.5),
                      border: Border(
                          right: BorderSide(color: MemoTheme.borderSoft)),
                    ),
                    child: ListView.builder(
                      padding: const EdgeInsets.symmetric(vertical: 12),
                      itemCount: _tabs.length,
                      itemBuilder: (context, index) {
                        final isActive = _activeTab == index;
                        return InkWell(
                          onTap: () => setState(() => _activeTab = index),
                          child: Container(
                            padding: const EdgeInsets.symmetric(
                                horizontal: 24, vertical: 12),
                            decoration: BoxDecoration(
                              color: isActive
                                  ? MemoTheme.bgElement
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
                              _tabs[index],
                              style: TextStyle(
                                fontSize: 13,
                                fontWeight: isActive
                                    ? FontWeight.w600
                                    : FontWeight.w500,
                                color: isActive
                                    ? MemoTheme.textMain
                                    : MemoTheme.textSecondary,
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
                      color: MemoTheme.bgApp,
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
    // 4: Cloud Sync
    // 5: Remote Access
    // 6: About
    switch (index) {
      case 0:
        return const _GeneralTab();
      case 1:
        return const _SystemPromptTab();
      case 2:
        return const _IncognitoPromptTab();
      case 3:
        return const _MemoryTab();
      case 4:
        return const _CloudSyncTab();
      case 5:
        return const _RemoteAccessTab();
      case 6:
        return const _AboutTab();
      default:
        return const SizedBox.shrink();
    }
  }
}

class _GeneralTab extends ConsumerWidget {
  const _GeneralTab();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return ListView(
      padding: const EdgeInsets.all(32),
      children: [
        Text(
          L10n.t('general'),
          style: Theme.of(context).textTheme.titleLarge?.copyWith(
                fontWeight: FontWeight.bold,
                color: MemoTheme.textMain,
              ),
        ),
        const SizedBox(height: 32),

        // Language Selection
        Text(
          L10n.t('language'),
          style: const TextStyle(
            fontSize: 14,
            fontWeight: FontWeight.w600,
            color: MemoTheme.textMain,
          ),
        ),
        const SizedBox(height: 12),
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 16),
          decoration: BoxDecoration(
            color: MemoTheme.bgPanel,
            borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
            border: Border.all(color: MemoTheme.borderSoft),
          ),
          child: DropdownButtonHideUnderline(
            child: DropdownButton<MemoLocale>(
              value: L10n.locale,
              isExpanded: true,
              dropdownColor: MemoTheme.bgPanel,
              icon: const Icon(Icons.arrow_drop_down, color: MemoTheme.textDim),
              items: const [
                DropdownMenuItem(
                  value: MemoLocale.tr,
                  child: Text('Türkçe'),
                ),
                DropdownMenuItem(
                  value: MemoLocale.en,
                  child: Text('English'),
                ),
              ],
              onChanged: (val) {
                if (val != null) {
                  ref.read(localeProvider.notifier).setLocale(val);
                }
              },
            ),
          ),
        ),

        const SizedBox(height: 32),

        // Reset Setup Wizard
        Text(
          'Kurulum',
          style: const TextStyle(
            fontSize: 14,
            fontWeight: FontWeight.w600,
            color: MemoTheme.textMain,
          ),
        ),
        const SizedBox(height: 12),
        Container(
          padding: const EdgeInsets.all(16),
          decoration: BoxDecoration(
            color: MemoTheme.bgPanel,
            borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
            border: Border.all(color: MemoTheme.borderSoft),
          ),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                'Kurulumu Sıfırla',
                style: const TextStyle(
                  fontSize: 14,
                  color: MemoTheme.textMain,
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
  const _SystemPromptTab();

  @override
  ConsumerState<_SystemPromptTab> createState() => _SystemPromptTabState();
}

class _SystemPromptTabState extends ConsumerState<_SystemPromptTab> {
  final _controller = TextEditingController();
  bool _initialized = false;

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final asyncPrompt = ref.watch(systemPromptProvider);

    return ListView(
      padding: const EdgeInsets.all(32),
      children: [
        Text(
          L10n.t('system_prompt'),
          style: Theme.of(context).textTheme.titleLarge?.copyWith(
                fontWeight: FontWeight.bold,
                color: MemoTheme.textMain,
              ),
        ),
        const SizedBox(height: 12),
        Text(
          'Modelin temel davranışını, kimliğini ve sınırlarını belirleyen ana yönerge.',
          style: TextStyle(color: MemoTheme.textDim, fontSize: 13),
        ),
        const SizedBox(height: 24),

        asyncPrompt.when(
          loading: () => const Center(child: CircularProgressIndicator()),
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
                  style: const TextStyle(
                      fontSize: 13, fontFamily: 'JetBrains Mono'),
                  decoration: const InputDecoration(
                    alignLabelWithHint: true,
                  ),
                ),
                const SizedBox(height: 16),
                Row(
                  mainAxisAlignment: MainAxisAlignment.end,
                  children: [
                    TextButton(
                      onPressed: () {
                        ref.read(systemPromptProvider.notifier).reset();
                        _initialized = false;
                      },
                      child: Text(L10n.t('reset_prompt')),
                    ),
                    const SizedBox(width: 12),
                    ElevatedButton(
                      onPressed: () {
                        ref
                            .read(systemPromptProvider.notifier)
                            .save(_controller.text);
                        ScaffoldMessenger.of(context).showSnackBar(
                          SnackBar(content: Text(L10n.t('save') + ' başarılı')),
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
  const _IncognitoPromptTab();

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
      padding: const EdgeInsets.all(32),
      children: [
        Text(
          L10n.t('incognito_prompt'),
          style: Theme.of(context).textTheme.titleLarge?.copyWith(
                fontWeight: FontWeight.bold,
                color: MemoTheme.textMain,
              ),
        ),
        const SizedBox(height: 12),
        Text(
          'Gizli moddayken modelin hafızaya erişmeden nasıl davranması gerektiğini belirten yönerge.',
          style: TextStyle(color: MemoTheme.textDim, fontSize: 13),
        ),
        const SizedBox(height: 24),

        asyncPrompt.when(
          loading: () => const Center(child: CircularProgressIndicator()),
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
                  style: const TextStyle(
                      fontSize: 13, fontFamily: 'JetBrains Mono'),
                ),
                const SizedBox(height: 16),
                Row(
                  mainAxisAlignment: MainAxisAlignment.end,
                  children: [
                    ElevatedButton(
                      onPressed: () {
                        ref
                            .read(incognitoPromptProvider.notifier)
                            .save(_controller.text);
                        ScaffoldMessenger.of(context).showSnackBar(
                          SnackBar(content: Text(L10n.t('save') + ' başarılı')),
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

class _MemoryTab extends ConsumerWidget {
  const _MemoryTab();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final memoryAsync = ref.watch(memoryFilesProvider);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Padding(
          padding: const EdgeInsets.all(32).copyWith(bottom: 16),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                L10n.t('memory_files'),
                style: Theme.of(context).textTheme.titleLarge?.copyWith(
                      fontWeight: FontWeight.bold,
                      color: MemoTheme.textMain,
                    ),
              ),
              OutlinedButton.icon(
                icon: const Icon(Icons.delete_sweep, size: 18),
                label: Text(L10n.t('clear_memory')),
                style: OutlinedButton.styleFrom(
                  foregroundColor: MemoTheme.red,
                  side: const BorderSide(color: MemoTheme.red),
                ),
                onPressed: () {
                  // TODO: Confirm dialog
                  ref.read(memoryFilesProvider.notifier).clearAll();
                },
              ),
            ],
          ),
        ),
        Expanded(
          child: memoryAsync.when(
            loading: () => const Center(child: CircularProgressIndicator()),
            error: (e, _) => Center(child: Text('${L10n.t('error')}: $e')),
            data: (files) {
              if (files.isEmpty) {
                return Center(
                  child: Text(
                    L10n.t('no_memory_files'),
                    style: TextStyle(color: MemoTheme.textDim),
                  ),
                );
              }

              return ListView.separated(
                padding: const EdgeInsets.symmetric(horizontal: 32),
                itemCount: files.length,
                separatorBuilder: (context, index) => const Divider(),
                itemBuilder: (context, index) {
                  final file = files[index];
                  return ListTile(
                    contentPadding: EdgeInsets.zero,
                    title: Text(file.name,
                        style: const TextStyle(fontWeight: FontWeight.w500)),
                    subtitle: Text('${file.sizeKb} KB • ${file.modified}',
                        style: TextStyle(color: MemoTheme.textDim, fontSize: 12)),
                    trailing: IconButton(
                      icon: const Icon(Icons.delete_outline),
                      color: MemoTheme.red,
                      onPressed: () {
                        ref.read(memoryFilesProvider.notifier).deleteFile(file.path);
                      },
                    ),
                  );
                },
              );
            },
          ),
        ),
      ],
    );
  }
}

class _CloudSyncTab extends ConsumerWidget {
  const _CloudSyncTab();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.cloud_sync_outlined, size: 48, color: MemoTheme.textDim),
          const SizedBox(height: 16),
          Text(
            L10n.t('cloud_sync'),
            style: Theme.of(context).textTheme.titleLarge?.copyWith(
                  color: MemoTheme.textMuted,
                ),
          ),
          const SizedBox(height: 8),
          Text(
            'Google Drive entegrasyonu yapım aşamasında...',
            style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                  color: MemoTheme.textDim,
                ),
          ),
        ],
      ),
    );
  }
}

class _RemoteAccessTab extends ConsumerWidget {
  const _RemoteAccessTab();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.wifi_tethering, size: 48, color: MemoTheme.textDim),
          const SizedBox(height: 16),
          Text(
            L10n.t('remote_access'),
            style: Theme.of(context).textTheme.titleLarge?.copyWith(
                  color: MemoTheme.textMuted,
                ),
          ),
          const SizedBox(height: 8),
          Text(
            'Yapım aşamasında...',
            style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                  color: MemoTheme.textDim,
                ),
          ),
        ],
      ),
    );
  }
}

class _AboutTab extends ConsumerWidget {
  const _AboutTab();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final versionAsync = ref.watch(appVersionProvider);

    return ListView(
      padding: const EdgeInsets.all(32),
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
              child: const Center(
                child: Text('M',
                    style: TextStyle(
                        fontSize: 28,
                        fontWeight: FontWeight.bold,
                        color: MemoTheme.accent)),
              ),
            ),
            const SizedBox(width: 24),
            Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  L10n.t('app_title'),
                  style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                        fontWeight: FontWeight.bold,
                        color: MemoTheme.textMain,
                      ),
                ),
                versionAsync.when(
                  loading: () => const Text('...'),
                  error: (_, __) => const SizedBox(),
                  data: (v) => Text(v,
                      style: TextStyle(color: MemoTheme.textDim)),
                ),
              ],
            ),
          ],
        ),
        const SizedBox(height: 48),
        Text('Vizyon ve Misyon',
            style: TextStyle(fontWeight: FontWeight.bold, fontSize: 16)),
        const SizedBox(height: 8),
        Text(
            'Memo, tamamen yerel bilgisayarınızda çalışan, sizin konuşmalarınızı ve tercihlerinizi zamanla öğrenip kalıcı hafızasına kazıyan özel bir yapay zeka asistanıdır. Asıl amaç, bulut teknolojilere muhtaç kalmadan, özgürce ve güvenle kendi bilgisayarında barındırabileceğiniz akıllı bir asistan yaratmaktır.',
            style: TextStyle(height: 1.6, color: MemoTheme.textMuted)),
        const SizedBox(height: 32),
        Text('Açık Kaynak (MIT Lisansı)',
            style: TextStyle(fontWeight: FontWeight.bold, fontSize: 16)),
        const SizedBox(height: 8),
        Text(
            'Bu yazılım MIT lisansı ile açık kaynak olarak sunulmaktadır. Geliştirici: Buğra Akdemir',
            style: TextStyle(height: 1.6, color: MemoTheme.textMuted)),
      ],
    );
  }
}
