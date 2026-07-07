import 'dart:io';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/theme.dart';
import '../../../core/l10n.dart';
import '../../../providers/chat_provider.dart';
import '../../../providers/models_provider.dart';
import '../../../providers/settings_provider.dart';

class GeneralTab extends ConsumerWidget {
  const GeneralTab({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final locale = ref.watch(localeProvider);
    final memoryEnabledAsync = ref.watch(memoryEnabledProvider);
    final embeddingStatus = ref.watch(embeddingStatusProvider);

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
              dropdownColor: MemoTheme.of(context).bgPanel.withValues(alpha: 1.0),
              elevation: 8,
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
          L10n.t('theme'),
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
              dropdownColor: MemoTheme.of(context).bgPanel.withValues(alpha: 1.0),
              elevation: 8,
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
                activeThumbColor: MemoTheme.accent,
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
            data: (enabled) => Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
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
                      activeThumbColor: MemoTheme.accent,
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
                if (enabled) ...[
                  SizedBox(height: 10),
                  _EmbeddingStatusRow(embeddingStatus),
                ],
              ],
            ),
          ),
        ),

        SizedBox(height: 32),


        // Reset Setup Wizard
        Text(
          L10n.t('settings_setup_section'),
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
                L10n.t('settings_reset_setup'),
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
              Text(
                L10n.t('settings_reset_tour'),
                style: TextStyle(
                  fontSize: 14,
                  color: MemoTheme.of(context).textMain,
                ),
              ),
              OutlinedButton(
                onPressed: () {
                  ref.read(tourSeenProvider.notifier).resetTour();
                  Navigator.of(context).pop();
                },
                child: Text(L10n.t('reset')),
              ),
            ],
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
              Text(
                L10n.t('settings_reset_launchpad'),
                style: TextStyle(
                  fontSize: 14,
                  color: MemoTheme.of(context).textMain,
                ),
              ),
              OutlinedButton(
                onPressed: () {
                  ref.read(launchpadSeenProvider.notifier).reset();
                  Navigator.of(context).pop();
                },
                child: Text(L10n.t('reset')),
              ),
            ],
          ),
        ),

        SizedBox(height: 32),
        const _CliUninstallSection(),
      ],
    );
  }
}

class _EmbeddingStatusRow extends StatelessWidget {
  final AsyncValue<dynamic> status;
  const _EmbeddingStatusRow(this.status);

  static const _green = Color(0xFF6FA07B);

  @override
  Widget build(BuildContext context) {
    final running = status.valueOrNull?.running as bool? ?? false;
    final modelName = status.valueOrNull?.modelName as String? ?? '';
    final theme = MemoTheme.of(context);

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 7),
      decoration: BoxDecoration(
        color: running
            ? _green.withValues(alpha: 0.08)
            : theme.bgPanel,
        borderRadius: BorderRadius.circular(8),
        border: Border.all(
          color: running
              ? _green.withValues(alpha: 0.35)
              : theme.borderSoft,
        ),
      ),
      child: Row(
        children: [
          if (running) ...[
            Icon(Icons.memory_rounded, size: 14, color: _green),
            const SizedBox(width: 7),
            Expanded(
              child: Text(
                modelName.isNotEmpty ? 'Embedding: $modelName' : 'Embedding modeli aktif',
                style: TextStyle(fontSize: 12, color: _green),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
            ),
          ] else ...[
            Icon(Icons.hub_outlined, size: 14, color: theme.textDim),
            const SizedBox(width: 7),
            Expanded(
              child: Text(
                'Embedding: kapalı',
                style: TextStyle(fontSize: 12, color: theme.textDim),
              ),
            ),
          ],
        ],
      ),
    );
  }
}

/// CLI management (reinstall/remove the `memo` terminal command) and a full
/// uninstall, with an optional "keep memory" toggle. Windows has no separate
/// CLI wrapper to manage — memo.exe in {app} is the app itself — so those
/// two actions are hidden there and only the full-uninstall flow shows.
class _CliUninstallSection extends ConsumerStatefulWidget {
  const _CliUninstallSection();

  @override
  ConsumerState<_CliUninstallSection> createState() => _CliUninstallSectionState();
}

class _CliUninstallSectionState extends ConsumerState<_CliUninstallSection> {
  bool _reinstalling = false;
  bool _removingCli = false;
  bool _uninstalling = false;
  bool _keepMemory = true;
  bool _uninstallConfirm1 = false;
  bool _uninstallConfirm2 = false;

  Future<void> _reinstallCli() async {
    setState(() => _reinstalling = true);
    try {
      await ref.read(apiClientProvider).reinstallCli();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('CLI yeniden yüklendi. Yeni bir terminal aç ve "memo" yaz.')),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Hata: $e')),
        );
      }
    } finally {
      if (mounted) setState(() => _reinstalling = false);
    }
  }

  Future<void> _removeCli() async {
    final confirm = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('CLI\'ı Kaldır'),
        content: const Text(
          'Terminalden "memo" komutu kaldırılacak. Masaüstü uygulaman ve '
          'verilerin etkilenmez.',
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('İptal')),
          TextButton(onPressed: () => Navigator.pop(ctx, true), child: const Text('Kaldır')),
        ],
      ),
    );
    if (confirm != true) return;

    setState(() => _removingCli = true);
    try {
      await ref.read(apiClientProvider).removeCli();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('CLI kaldırıldı.')),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Hata: $e')),
        );
      }
    } finally {
      if (mounted) setState(() => _removingCli = false);
    }
  }

  Future<void> _uninstall() async {
    setState(() => _uninstalling = true);
    try {
      await ref.read(apiClientProvider).uninstallMemo(keepMemory: _keepMemory);
      if (mounted) {
        await _showUninstalledDialog();
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Kaldırma hatası: $e')),
        );
      }
    } finally {
      if (mounted) {
        setState(() {
          _uninstalling = false;
          _uninstallConfirm1 = false;
          _uninstallConfirm2 = false;
        });
      }
    }
  }

  /// Memo's own working files (config/data, and on Linux/macOS the CLI +
  /// engine binaries) were just removed out from under this running
  /// process — the only sane next step is to close it. Signals the same
  /// way the wipe-data flow's restart dialog does.
  Future<void> _showUninstalledDialog() async {
    final theme = MemoTheme.of(context);
    await showDialog<void>(
      context: context,
      barrierDismissible: false,
      builder: (ctx) => AlertDialog(
        backgroundColor: theme.bgPanel,
        title: Text(
          'Memo kaldırıldı',
          style: TextStyle(color: theme.textMain, fontWeight: FontWeight.bold),
        ),
        content: Text(
          _keepMemory
              ? 'Hafızan ~/memo-memory-backup içine yedeklendi. Uygulama şimdi kapanacak.'
              : 'Tüm veriler silindi. Uygulama şimdi kapanacak.',
          style: TextStyle(color: theme.textDim, fontSize: 13),
        ),
        actions: [
          TextButton(
            onPressed: () async {
              await ref.read(apiClientProvider).shutdown();
              exit(0);
            },
            child: Text(
              'Kapat',
              style: TextStyle(color: MemoTheme.accent, fontWeight: FontWeight.bold),
            ),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    final showCliActions = !Platform.isWindows;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'CLI ve Kaldırma',
          style: TextStyle(fontSize: 14, fontWeight: FontWeight.w600, color: theme.textMain),
        ),
        SizedBox(height: 12),

        if (showCliActions) ...[
          Card(
            child: ListTile(
              leading: _reinstalling
                  ? const SizedBox(width: 20, height: 20, child: CircularProgressIndicator(strokeWidth: 2))
                  : Icon(Icons.terminal, color: MemoTheme.accent),
              title: const Text('CLI\'ı Yeniden Yükle'),
              subtitle: Text(
                'Terminaldeki "memo" komutunu bu sürümle günceller — yeni bir build sonrası eski komutta takılı kalmamak için.',
                style: TextStyle(fontSize: 12, color: theme.textDim),
              ),
              onTap: _reinstalling ? null : _reinstallCli,
            ),
          ),
          const SizedBox(height: 12),
          Card(
            child: ListTile(
              leading: _removingCli
                  ? const SizedBox(width: 20, height: 20, child: CircularProgressIndicator(strokeWidth: 2))
                  : Icon(Icons.terminal_outlined, color: theme.textDim),
              title: const Text('CLI\'ı Kaldır'),
              subtitle: Text(
                'Sadece terminaldeki "memo" komutunu kaldırır. Verilerin ve masaüstü uygulaman etkilenmez.',
                style: TextStyle(fontSize: 12, color: theme.textDim),
              ),
              onTap: _removingCli ? null : _removeCli,
            ),
          ),
          const SizedBox(height: 24),
        ] else ...[
          Text(
            'Windows\'ta terminal komutu ayrı bir kurulum değil — memo.exe uygulamanın kendisi. Kaldırmak için Windows Ayarlar > Uygulamalar\'ı kullan.',
            style: TextStyle(fontSize: 12, color: theme.textDim),
          ),
          const SizedBox(height: 24),
        ],

        Text(
          'Memo\'yu Kaldır',
          style: TextStyle(fontWeight: FontWeight.bold, fontSize: 16, color: MemoTheme.warmBrown),
        ),
        SizedBox(height: 8),
        Text(
          'CLI, yapılandırma, sohbet geçmişi ve indirilen motor dosyaları dahil her şey silinir.',
          style: TextStyle(color: theme.textDim, fontSize: 13),
        ),
        SizedBox(height: 12),

        Card(
          child: SwitchListTile(
            title: const Text('Hafızayı koru'),
            subtitle: Text(
              'Kaldırmadan önce hafızayı ~/memo-memory-backup içine yedekler.',
              style: TextStyle(fontSize: 12, color: theme.textDim),
            ),
            value: _keepMemory,
            onChanged: (v) => setState(() => _keepMemory = v),
            secondary: Icon(Icons.memory_rounded, color: MemoTheme.accent),
          ),
        ),
        SizedBox(height: 12),

        if (!_uninstallConfirm1)
          Card(
            child: ListTile(
              leading: Icon(Icons.delete_forever, color: MemoTheme.warmBrown),
              title: Text('Memo\'yu Kaldır', style: TextStyle(color: MemoTheme.warmBrown)),
              subtitle: Text(
                'Bu işlem geri alınamaz',
                style: TextStyle(fontSize: 12, color: theme.textDim),
              ),
              trailing: Icon(Icons.warning_amber, color: MemoTheme.warmBrown),
              onTap: () => setState(() => _uninstallConfirm1 = true),
            ),
          ),
        if (_uninstallConfirm1 && !_uninstallConfirm2)
          Card(
            color: MemoTheme.warmBrown.withValues(alpha: 0.08),
            child: ListTile(
              leading: Icon(Icons.delete_forever, color: Colors.redAccent),
              title: Text('Emin misiniz?', style: TextStyle(color: Colors.redAccent)),
              subtitle: Text(
                _keepMemory
                    ? 'Hafıza dışında her şey silinecek. Onaylamak için tekrar tıklayın.'
                    : 'Hafıza dahil her şey silinecek. Onaylamak için tekrar tıklayın.',
                style: TextStyle(fontSize: 12, color: theme.textDim),
              ),
              trailing: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  IconButton(
                    icon: Icon(Icons.close, color: theme.textDim),
                    onPressed: () => setState(() {
                      _uninstallConfirm1 = false;
                      _uninstallConfirm2 = false;
                    }),
                  ),
                  Icon(Icons.warning, color: Colors.redAccent),
                ],
              ),
              onTap: () => setState(() => _uninstallConfirm2 = true),
            ),
          ),
        if (_uninstallConfirm2)
          Card(
            color: MemoTheme.red.withValues(alpha: 0.12),
            child: ListTile(
              leading: _uninstalling
                  ? const SizedBox(width: 20, height: 20, child: CircularProgressIndicator(strokeWidth: 2))
                  : Icon(Icons.delete_sweep, color: MemoTheme.red),
              title: Text('Kaldır', style: TextStyle(color: MemoTheme.red, fontWeight: FontWeight.bold)),
              subtitle: Text(
                'Bu işlem geri alınamaz.',
                style: TextStyle(fontSize: 12, color: theme.textDim),
              ),
              trailing: IconButton(
                icon: Icon(Icons.close, color: theme.textDim),
                onPressed: () => setState(() {
                  _uninstallConfirm1 = false;
                  _uninstallConfirm2 = false;
                }),
              ),
              onTap: _uninstalling ? null : _uninstall,
            ),
          ),
      ],
    );
  }
}
