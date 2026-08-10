import 'dart:io';
import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/theme.dart';
import '../../../core/l10n.dart';
import '../../../providers/chat_provider.dart';
import '../../../providers/models_provider.dart';
import '../../../providers/settings_provider.dart';
import '../../../core/friendly_error.dart';

class GeneralTab extends ConsumerWidget {
  const GeneralTab({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final locale = ref.watch(localeProvider);
    final memoryEnabledAsync = ref.watch(memoryEnabledProvider);
    final embeddingStatus = ref.watch(embeddingStatusProvider);
    final minimalModeAsync = ref.watch(minimalModeProvider);

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
              items: [
                DropdownMenuItem(value: MemoLocale.tr, child: Text(L10n.t('lang_turkish'))),
                DropdownMenuItem(value: MemoLocale.en, child: Text(L10n.t('lang_english'))),
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
              items: [
                DropdownMenuItem(
                  value: 'system',
                  child: Text(L10n.t('theme_system')),
                ),
                DropdownMenuItem(value: 'light', child: Text(L10n.t('theme_light'))),
                DropdownMenuItem(value: 'dark', child: Text(L10n.t('theme_dark'))),
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
          L10n.t('streaming'),
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
                      ref.watch(streamingEnabledProvider) ? L10n.t('on') : L10n.t('off'),
                      style: TextStyle(
                        fontSize: 14,
                        color: MemoTheme.of(context).textMain,
                      ),
                    ),
                    SizedBox(height: 4),
                    Text(
                      L10n.t('streaming_off_desc'),
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
            error: (e, _) => Text('${L10n.t('error')}: ${FriendlyError.describeGeneric(e)}'),
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
                              SnackBar(content: Text('${L10n.t('error')}: ${FriendlyError.describeGeneric(e)}')),
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

        // Minimal Mode Toggle
        Text(
          L10n.t('minimal_mode_section'),
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
          child: minimalModeAsync.when(
            loading: () => SizedBox(
              height: 24,
              child: Center(child: CircularProgressIndicator(strokeWidth: 2)),
            ),
            error: (e, _) => Text('${L10n.t('error')}: ${FriendlyError.describeGeneric(e)}'),
            data: (enabled) => Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        enabled
                            ? L10n.t('minimal_mode_active')
                            : L10n.t('minimal_mode_disabled'),
                        style: TextStyle(
                          fontSize: 14,
                          color: MemoTheme.of(context).textMain,
                        ),
                      ),
                      SizedBox(height: 4),
                      Text(
                        L10n.t('minimal_mode_toggle_desc'),
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
                      await ref.read(minimalModeProvider.notifier).toggle();
                    } catch (e) {
                      if (context.mounted) {
                        ScaffoldMessenger.of(context).showSnackBar(
                          SnackBar(content: Text('${L10n.t('error')}: ${FriendlyError.describeGeneric(e)}')),
                        );
                      }
                    }
                  },
                ),
              ],
            ),
          ),
        ),
        if (minimalModeAsync.valueOrNull == true) ...[
          SizedBox(height: 8),
          const _MinimalModeOverridesDropdown(),
        ],

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

/// Collapsible "keep these on anyway" breakdown, shown only while Minimal
/// Mode itself is on — each checkbox re-enables one category without
/// requiring Minimal Mode to be turned off entirely (see
/// minimalModeOverridesProvider's doc comment).
class _MinimalModeOverridesDropdown extends ConsumerWidget {
  const _MinimalModeOverridesDropdown();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = MemoTheme.of(context);
    final overridesAsync = ref.watch(minimalModeOverridesProvider);

    return Container(
      decoration: BoxDecoration(
        color: theme.bgPanel,
        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
        border: Border.all(color: theme.borderSoft),
      ),
      child: overridesAsync.when(
        loading: () => Padding(
          padding: EdgeInsets.all(16),
          child: SizedBox(
            height: 20,
            child: Center(child: CircularProgressIndicator(strokeWidth: 2)),
          ),
        ),
        error: (e, _) => Padding(
          padding: EdgeInsets.all(16),
          child: Text('${L10n.t('error')}: ${FriendlyError.describeGeneric(e)}'),
        ),
        data: (overrides) => Theme(
          // ExpansionTile's default divider clashes with this panel's own
          // border — flatten it so the collapsed/expanded states look like
          // one continuous card, matching the Minimal Mode container above.
          data: Theme.of(context).copyWith(dividerColor: Colors.transparent),
          child: ExpansionTile(
            tilePadding: EdgeInsets.symmetric(horizontal: 16),
            childrenPadding: EdgeInsets.only(left: 16, right: 16, bottom: 12),
            title: Text(
              L10n.t('minimal_mode_overrides_title'),
              style: TextStyle(fontSize: 13, color: theme.textMain),
            ),
            subtitle: Text(
              L10n.t('minimal_mode_overrides_desc'),
              style: TextStyle(fontSize: 11, color: theme.textDim),
            ),
            children: [
              _OverrideRow(
                title: L10n.t('minimal_mode_keep_persona'),
                desc: L10n.t('minimal_mode_keep_persona_desc'),
                value: overrides.keepPersona,
                onChanged: (v) => ref
                    .read(minimalModeOverridesProvider.notifier)
                    .save(overrides.copyWith(keepPersona: v)),
              ),
              _OverrideRow(
                title: L10n.t('minimal_mode_keep_capabilities'),
                desc: L10n.t('minimal_mode_keep_capabilities_desc'),
                value: overrides.keepCapabilities,
                onChanged: (v) => ref
                    .read(minimalModeOverridesProvider.notifier)
                    .save(overrides.copyWith(keepCapabilities: v)),
              ),
              _OverrideRow(
                title: L10n.t('minimal_mode_keep_passive'),
                desc: L10n.t('minimal_mode_keep_passive_desc'),
                value: overrides.keepPassive,
                onChanged: (v) => ref
                    .read(minimalModeOverridesProvider.notifier)
                    .save(overrides.copyWith(keepPassive: v)),
              ),
              _OverrideRow(
                title: L10n.t('minimal_mode_keep_proactive'),
                desc: L10n.t('minimal_mode_keep_proactive_desc'),
                value: overrides.keepProactive,
                onChanged: (v) => ref
                    .read(minimalModeOverridesProvider.notifier)
                    .save(overrides.copyWith(keepProactive: v)),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _OverrideRow extends StatelessWidget {
  final String title;
  final String desc;
  final bool value;
  final ValueChanged<bool> onChanged;

  const _OverrideRow({
    required this.title,
    required this.desc,
    required this.value,
    required this.onChanged,
  });

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    return Padding(
      padding: EdgeInsets.symmetric(vertical: 4),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(title, style: TextStyle(fontSize: 12.5, color: theme.textMain)),
                SizedBox(height: 2),
                Text(desc, style: TextStyle(fontSize: 11, color: theme.textDim)),
              ],
            ),
          ),
          SizedBox(
            height: 32,
            child: Switch(
              value: value,
              activeThumbColor: MemoTheme.accent,
              onChanged: (v) => onChanged(v),
            ),
          ),
        ],
      ),
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
                modelName.isNotEmpty ? L10n.t('embedding_active_named', {'model': modelName}) : L10n.t('embedding_active_generic'),
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
                L10n.t('embedding_off'),
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
          SnackBar(content: Text(L10n.t('cli_reinstalled_msg'))),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(L10n.t('cli_error', {'e': FriendlyError.describeGeneric(e)}))),
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
        title: Text(L10n.t('cli_remove_title')),
        content: Text(L10n.t('cli_remove_confirm_body')),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: Text(L10n.t('cancel'))),
          TextButton(onPressed: () => Navigator.pop(ctx, true), child: Text(L10n.t('cli_remove_btn'))),
        ],
      ),
    );
    if (confirm != true) return;

    setState(() => _removingCli = true);
    try {
      await ref.read(apiClientProvider).removeCli();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(L10n.t('cli_removed_msg'))),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(L10n.t('cli_error', {'e': FriendlyError.describeGeneric(e)}))),
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
          SnackBar(content: Text(L10n.t('uninstall_error', {'e': FriendlyError.describeGeneric(e)}))),
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
          L10n.t('uninstall_done_title'),
          style: TextStyle(color: theme.textMain, fontWeight: FontWeight.bold),
        ),
        content: Text(
          _keepMemory
              ? L10n.t('uninstall_done_body_keep')
              : L10n.t('uninstall_done_body_all'),
          style: TextStyle(color: theme.textDim, fontSize: 13),
        ),
        actions: [
          TextButton(
            onPressed: () async {
              await ref.read(apiClientProvider).shutdown();
              exit(0);
            },
            child: Text(
              L10n.t('close'),
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
    final showCliActions = !kIsWeb && !Platform.isWindows;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          L10n.t('cli_section_title'),
          style: TextStyle(fontSize: 14, fontWeight: FontWeight.w600, color: theme.textMain),
        ),
        SizedBox(height: 12),

        if (showCliActions) ...[
          Card(
            child: ListTile(
              leading: _reinstalling
                  ? const SizedBox(width: 20, height: 20, child: CircularProgressIndicator(strokeWidth: 2))
                  : Icon(Icons.terminal, color: MemoTheme.accent),
              title: Text(L10n.t('cli_reinstall_title')),
              subtitle: Text(
                L10n.t('cli_reinstall_desc'),
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
              title: Text(L10n.t('cli_remove_title')),
              subtitle: Text(
                L10n.t('cli_remove_desc'),
                style: TextStyle(fontSize: 12, color: theme.textDim),
              ),
              onTap: _removingCli ? null : _removeCli,
            ),
          ),
          const SizedBox(height: 24),
        ] else ...[
          Text(
            L10n.t('cli_windows_note'),
            style: TextStyle(fontSize: 12, color: theme.textDim),
          ),
          const SizedBox(height: 24),
        ],

        Text(
          L10n.t('uninstall_section_title'),
          style: TextStyle(fontWeight: FontWeight.bold, fontSize: 16, color: MemoTheme.warmBrown),
        ),
        SizedBox(height: 8),
        Text(
          L10n.t('uninstall_section_desc'),
          style: TextStyle(color: theme.textDim, fontSize: 13),
        ),
        SizedBox(height: 12),

        Card(
          child: SwitchListTile(
            title: Text(L10n.t('uninstall_keep_memory_title')),
            subtitle: Text(
              L10n.t('uninstall_keep_memory_desc'),
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
              title: Text(L10n.t('uninstall_section_title'), style: TextStyle(color: MemoTheme.warmBrown)),
              subtitle: Text(
                L10n.t('backup_wipe_irreversible'),
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
              title: Text(L10n.t('backup_wipe_confirm_title'), style: TextStyle(color: Colors.redAccent)),
              subtitle: Text(
                _keepMemory
                    ? L10n.t('uninstall_confirm2_body_keep')
                    : L10n.t('uninstall_confirm2_body_all'),
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
              title: Text(L10n.t('cli_remove_btn'), style: TextStyle(color: MemoTheme.red, fontWeight: FontWeight.bold)),
              subtitle: Text(
                L10n.t('uninstall_final_irreversible'),
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
