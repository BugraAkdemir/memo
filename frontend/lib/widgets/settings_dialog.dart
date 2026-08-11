import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_svg/flutter_svg.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../providers/settings_provider.dart';
import 'settings/tabs/general_tab.dart';
import 'settings/tabs/system_prompt_tab.dart';
import 'settings/tabs/incognito_prompt_tab.dart';
import 'settings/tabs/memory_tab.dart';
import 'settings/tabs/memory_import_tab.dart';
import 'settings/tabs/providers_tab.dart';
import 'settings/tabs/cli_connections_tab.dart';
import 'settings/tabs/orchestra_tab.dart';
import 'settings/tabs/agent_permissions_tab.dart';
import 'settings/tabs/learning_tab.dart';
import 'settings/tabs/mood_tab.dart';
import 'settings/tabs/skills_tab.dart';
import 'settings/tabs/gpu_config_tab.dart';
import 'settings/tabs/backup_restore_tab.dart';
import 'settings/tabs/remote_access_tab.dart';
import 'settings/tabs/accounts_tab.dart';
import 'settings/tabs/beta_features_tab.dart';
import 'settings/tabs/about_tab.dart';
import 'settings/tabs/taskloop_tab.dart';
import 'settings/tabs/stats_tab.dart';
import 'settings/tabs/report_bug_tab.dart';

/// Settings dialog: a searchable, grouped rail on the left, tab content on
/// the right. Redesigned (v3.3.4) from a single flat list of 20
/// identically-weighted rows — which read as cluttered and cramped no
/// matter the dialog size — into a small number of labeled groups plus a
/// search box, so scanning/finding a specific setting doesn't mean reading
/// 20 rows top to bottom every time.
class SettingsDialog extends ConsumerStatefulWidget {
  const SettingsDialog({super.key, this.initialTab = 0});

  /// Which tab to open on. Lets a caller elsewhere in the app (e.g. a
  /// "server unreachable" screen offering to change the backend URL) land
  /// the user directly on the relevant tab instead of always the first one.
  final int initialTab;

  /// Index of the Remote Access tab in [_SettingsDialogState._tabs] — kept
  /// here as a named constant so callers outside this file don't have to
  /// hardcode the position of one entry in that list.
  static const remoteAccessTabIndex = 15;

  @override
  ConsumerState<SettingsDialog> createState() => _SettingsDialogState();
}

class _SettingsDialogState extends ConsumerState<SettingsDialog> {
  late int _activeTab = widget.initialTab;
  final _searchController = TextEditingController();
  String _searchQuery = '';

  static const _tabIcons = [
    'lib/icon/slash/gear.svg',
    'lib/icon/slash/chat-text.svg',
    'lib/icon/slash/eye-slash.svg',
    'lib/icon/slash/brain.svg',
    'lib/icon/slash/translate.svg',
    'lib/icon/slash/plug.svg',
    'lib/icon/slash/code.svg', // CLI Bağlantıları
    'lib/icon/slash/music-notes.svg',
    'lib/icon/slash/shield-check.svg',
    'lib/icon/slash/lightbulb.svg',
    'lib/icon/slash/arrows-left-right.svg',
    'lib/icon/slash/puzzle-piece.svg',
    'lib/icon/slash/cpu.svg',
    'lib/icon/slash/chart-bar.svg',
    'lib/icon/slash/archive.svg',
    'lib/icon/slash/globe.svg',
    'lib/icon/slash/code.svg', // Beta features
    'lib/icon/slash/info.svg',
    'lib/icon/slash/list-checks.svg',
    'lib/icon/slash/wrench.svg',
    'lib/icon/slash/people.svg',
  ];

  /// Tab indices grouped under an eyebrow header, in sidebar display order.
  /// Every index 0..20 must appear exactly once — covered by
  /// settings_dialog_test.dart's group-coverage test.
  static const _groups = [
    ('settings_group_general', [0, 1, 2]),
    ('settings_group_providers', [5, 6, 15]),
    ('settings_group_memory', [3, 4, 9, 10]),
    ('settings_group_agents', [7, 8, 11, 18]),
    ('settings_group_system', [12, 13, 14, 20]),
    ('settings_group_other', [16, 17, 19]),
  ];

  List<String> get _tabs => [
    L10n.t('general'),
    L10n.t('system_prompt'),
    L10n.t('incognito_prompt'),
    L10n.t('memory'),
    L10n.t('tab_memory_import'),
    L10n.t('tab_providers'),
    L10n.t('tab_cli_connections'),
    L10n.t('tab_orchestra'),
    L10n.t('tab_agent_permissions'),
    L10n.t('tab_learning'),
    L10n.t('tab_mood'),
    L10n.t('tab_skills'),
    L10n.t('tab_gpu_config'),
    L10n.t('tab_stats'),
    L10n.t('backup'),
    L10n.t('remote_access'),
    L10n.t('tab_beta_features'),
    L10n.t('about'),
    L10n.t('tab_taskloop'),
    L10n.t('tab_report_bug'),
    L10n.t('tab_accounts'),
  ];

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    ref.watch(localeProvider);
    final tabs = _tabs;
    final tabIndex = _activeTab.clamp(0, tabs.length - 1);
    final theme = MemoTheme.of(context);

    final screenSize = MediaQuery.of(context).size;
    // Fixed once computed from the viewport, so the Row below (sidebar +
    // content) always lays out against a stable width regardless of how
    // small the actual window gets — the inner layout can't RenderFlex
    // overflow from a shrinking screen, only the dialog's own size adapts.
    final dialogWidth = (screenSize.width * 0.85).clamp(400.0, 1040.0);
    final dialogHeight = (screenSize.height * 0.85).clamp(440.0, 760.0);

    // ScaffoldMessenger + Scaffold so tab SnackBars (ScaffoldMessenger.of)
    // render ON TOP of this modal Dialog instead of behind it on the root
    // Scaffold (BUG-M2). MemoryImportTab already uses an inline banner for the
    // same reason; this covers every other tab without rewriting each one.
    return Dialog(
      backgroundColor: theme.bgApp,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(MemoTheme.radiusLg),
      ),
      child: Container(
        width: dialogWidth,
        height: dialogHeight,
        clipBehavior: Clip.antiAlias,
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(MemoTheme.radiusLg),
        ),
        child: ScaffoldMessenger(
          child: Scaffold(
            backgroundColor: theme.bgApp,
            body: Column(
              children: [
                Container(
                  height: 56,
                  padding: const EdgeInsets.symmetric(horizontal: 24),
                  decoration: BoxDecoration(
                    color: theme.bgPanel,
                    border: Border(bottom: BorderSide(color: theme.borderSoft)),
                  ),
                  child: Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      Text(
                        L10n.t('settings'),
                        style: TextStyle(
                          fontSize: 16,
                          fontWeight: FontWeight.w600,
                          color: theme.textMain,
                        ),
                      ),
                      IconButton(
                        icon: const Icon(Icons.close, size: 20),
                        onPressed: () => Navigator.of(context).pop(),
                        color: theme.textDim,
                      ),
                    ],
                  ),
                ),
                Expanded(
                  child: Row(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      SizedBox(
                        width: 216,
                        child: Container(
                          decoration: BoxDecoration(
                            color: theme.bgPanel.withValues(alpha: 0.5),
                            border: Border(right: BorderSide(color: theme.borderSoft)),
                          ),
                          child: Column(
                            children: [
                              _SettingsSearchField(
                                controller: _searchController,
                                onChanged: (v) => setState(() => _searchQuery = v),
                              ),
                              Expanded(
                                child: _SettingsRail(
                                  tabs: tabs,
                                  tabIcons: _tabIcons,
                                  groups: _groups,
                                  query: _searchQuery,
                                  activeIndex: tabIndex,
                                  onSelect: (i) => setState(() => _activeTab = i),
                                ),
                              ),
                            ],
                          ),
                        ),
                      ),
                      Expanded(
                        child: Container(
                          color: theme.bgApp,
                          child: _buildTabContent(tabIndex),
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildTabContent(int index) {
    switch (index) {
      case 0: return GeneralTab();
      case 1: return SystemPromptTab();
      case 2: return IncognitoPromptTab();
      case 3: return MemoryTab();
      case 4: return MemoryImportTab();
      case 5: return ProvidersTab();
      case 6: return CLIConnectionsTab();
      case 7: return OrchestraTab();
      case 8: return AgentPermissionsTab();
      case 9: return LearningTab();
      case 10: return MoodTab();
      case 11: return SkillsTab();
      case 12: return GpuConfigTab();
      case 13: return StatsTab();
      case 14: return BackupRestoreTab();
      case 15: return RemoteAccessTab();
      case 16: return BetaFeaturesTab();
      case 17: return AboutTab();
      case 18: return TaskLoopTab();
      case 19: return ReportBugTab();
      case 20: return AccountsTab();
      default: return const SizedBox.shrink();
    }
  }
}

/// Compact search box pinned above the grouped rail — the redesign's answer
/// to 20 rows with no way to jump straight to one whose name you remember
/// (or don't: filtering by what you can see still narrows the scan).
class _SettingsSearchField extends StatelessWidget {
  final TextEditingController controller;
  final ValueChanged<String> onChanged;

  const _SettingsSearchField({required this.controller, required this.onChanged});

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    return Padding(
      padding: const EdgeInsets.fromLTRB(12, 12, 12, 8),
      child: TextField(
        controller: controller,
        onChanged: onChanged,
        style: TextStyle(fontSize: 13, color: theme.textMain),
        decoration: InputDecoration(
          isDense: true,
          hintText: L10n.t('settings_search_hint'),
          hintStyle: TextStyle(fontSize: 13, color: theme.textDim),
          prefixIcon: Icon(Icons.search_rounded, size: 16, color: theme.textDim),
          prefixIconConstraints: const BoxConstraints(minWidth: 32, minHeight: 16),
          contentPadding: const EdgeInsets.symmetric(vertical: 9),
          filled: true,
          fillColor: theme.bgElement,
          border: OutlineInputBorder(
            borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
            borderSide: BorderSide.none,
          ),
          enabledBorder: OutlineInputBorder(
            borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
            borderSide: BorderSide.none,
          ),
          focusedBorder: OutlineInputBorder(
            borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
            borderSide: const BorderSide(color: MemoTheme.accent, width: 1.5),
          ),
        ),
      ),
    );
  }
}

/// The grouped, filterable list of settings tabs — group headers are plain
/// labels (not tappable, not numbered: there is no sequence here, order is
/// just topical clustering) so the eye can skip straight to a cluster
/// instead of reading 20 same-weight rows in a row.
class _SettingsRail extends StatelessWidget {
  final List<String> tabs;
  final List<String> tabIcons;
  final List<(String, List<int>)> groups;
  final String query;
  final int activeIndex;
  final ValueChanged<int> onSelect;

  const _SettingsRail({
    required this.tabs,
    required this.tabIcons,
    required this.groups,
    required this.query,
    required this.activeIndex,
    required this.onSelect,
  });

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    final needle = query.trim().toLowerCase();

    final children = <Widget>[];
    for (final (titleKey, indices) in groups) {
      final visible = needle.isEmpty
          ? indices
          : indices.where((i) => tabs[i].toLowerCase().contains(needle)).toList();
      if (visible.isEmpty) continue;
      children.add(_GroupHeader(L10n.t(titleKey)));
      for (final i in visible) {
        children.add(_SettingsTabRow(
          label: tabs[i],
          icon: tabIcons[i],
          isActive: activeIndex == i,
          onTap: () => onSelect(i),
        ));
      }
      children.add(const SizedBox(height: 10));
    }

    if (children.isEmpty) {
      return Padding(
        padding: const EdgeInsets.all(20),
        child: Text(
          L10n.t('settings_search_no_results'),
          style: TextStyle(fontSize: 12, color: theme.textDim),
        ),
      );
    }

    return ListView(
      key: const Key('settingsRailList'),
      padding: const EdgeInsets.only(bottom: 12),
      children: children,
    );
  }
}

class _GroupHeader extends StatelessWidget {
  final String label;
  const _GroupHeader(this.label);

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 12, 16, 6),
      child: Text(
        label,
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
        style: TextStyle(
          fontSize: 10.5,
          fontWeight: FontWeight.w700,
          letterSpacing: 0.6,
          color: theme.textDim,
        ),
      ),
    );
  }
}

class _SettingsTabRow extends StatelessWidget {
  final String label;
  final String icon;
  final bool isActive;
  final VoidCallback onTap;

  const _SettingsTabRow({
    required this.label,
    required this.icon,
    required this.isActive,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    final iconColor = isActive ? MemoTheme.accent : theme.textDim;
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 1),
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: onTap,
          borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
          child: Container(
            padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 9),
            decoration: BoxDecoration(
              color: isActive ? MemoTheme.accentMuted : Colors.transparent,
              borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
            ),
            child: Row(
              children: [
                SizedBox(
                  width: 17,
                  height: 17,
                  child: SvgPicture.asset(
                    icon,
                    colorFilter: ColorFilter.mode(iconColor, BlendMode.srcIn),
                  ),
                ),
                const SizedBox(width: 10),
                Expanded(
                  child: Text(
                    label,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(
                      fontSize: 13,
                      fontWeight: isActive ? FontWeight.w600 : FontWeight.w500,
                      color: isActive ? theme.textMain : theme.textSecondary,
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
