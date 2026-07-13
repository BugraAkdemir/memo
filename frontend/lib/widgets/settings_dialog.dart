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
import 'settings/tabs/orchestra_tab.dart';
import 'settings/tabs/agent_permissions_tab.dart';
import 'settings/tabs/learning_tab.dart';
import 'settings/tabs/mood_tab.dart';
import 'settings/tabs/skills_tab.dart';
import 'settings/tabs/gpu_config_tab.dart';
import 'settings/tabs/backup_restore_tab.dart';
import 'settings/tabs/remote_access_tab.dart';
import 'settings/tabs/about_tab.dart';
import 'settings/tabs/taskloop_tab.dart';

/// Settings dialog with vertical tabs on the left and content on the right.
class SettingsDialog extends ConsumerStatefulWidget {
  const SettingsDialog({super.key});

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
    'lib/icon/slash/translate.svg',
    'lib/icon/slash/plug.svg',
    'lib/icon/slash/music-notes.svg',
    'lib/icon/slash/shield-check.svg',
    'lib/icon/slash/lightbulb.svg',
    'lib/icon/slash/arrows-left-right.svg',
    'lib/icon/slash/puzzle-piece.svg',
    'lib/icon/slash/cpu.svg',
    'lib/icon/slash/archive.svg',
    'lib/icon/slash/globe.svg',
    'lib/icon/slash/info.svg',
    'lib/icon/slash/gears.svg',
  ];

  List<String> get _tabs => [
    L10n.t('general'),
    L10n.t('system_prompt'),
    L10n.t('incognito_prompt'),
    L10n.t('memory'),
    L10n.t('tab_memory_import'),
    L10n.t('tab_providers'),
    L10n.t('tab_orchestra'),
    L10n.t('tab_agent_permissions'),
    L10n.t('tab_learning'),
    L10n.t('tab_mood'),
    L10n.t('tab_skills'),
    L10n.t('tab_gpu_config'),
    L10n.t('backup'),
    L10n.t('remote_access'),
    L10n.t('about'),
    L10n.t('tab_taskloop'),
  ];

  @override
  Widget build(BuildContext context) {
    ref.watch(localeProvider);
    final tabs = _tabs;
    final tabIndex = _activeTab.clamp(0, tabs.length - 1);

    final screenSize = MediaQuery.of(context).size;
    final dialogWidth = (screenSize.width * 0.85).clamp(400.0, 900.0);
    final dialogHeight = (screenSize.height * 0.85).clamp(400.0, 700.0);

    return Dialog(
      backgroundColor: MemoTheme.of(context).bgApp,
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
        child: Column(
          children: [
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
            Expanded(
              child: Row(
                children: [
                  Container(
                    width: 200,
                    decoration: BoxDecoration(
                      color: MemoTheme.of(context).bgPanel.withValues(alpha: 0.5),
                      border: Border(
                        right: BorderSide(color: MemoTheme.of(context).borderSoft),
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
                            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 11),
                            decoration: BoxDecoration(
                              color: isActive ? MemoTheme.of(context).bgElement : Colors.transparent,
                              border: Border(
                                left: BorderSide(
                                  color: isActive ? MemoTheme.accent : Colors.transparent,
                                  width: 3,
                                ),
                              ),
                            ),
                            child: Row(
                              children: [
                                SizedBox(
                                  width: 18, height: 18,
                                  child: SvgPicture.asset(
                                    _tabIcons[index],
                                    colorFilter: ColorFilter.mode(iconColor, BlendMode.srcIn),
                                  ),
                                ),
                                const SizedBox(width: 10),
                                Expanded(
                                  child: Text(
                                    tabs[index],
                                    style: TextStyle(
                                      fontSize: 13,
                                      fontWeight: isActive ? FontWeight.w600 : FontWeight.w500,
                                      color: isActive ? MemoTheme.of(context).textMain : MemoTheme.of(context).textSecondary,
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
      case 0: return GeneralTab();
      case 1: return SystemPromptTab();
      case 2: return IncognitoPromptTab();
      case 3: return MemoryTab();
      case 4: return MemoryImportTab();
      case 5: return ProvidersTab();
      case 6: return OrchestraTab();
      case 7: return AgentPermissionsTab();
      case 8: return LearningTab();
      case 9: return MoodTab();
      case 10: return SkillsTab();
      case 11: return GpuConfigTab();
      case 12: return BackupRestoreTab();
      case 13: return RemoteAccessTab();
      case 14: return AboutTab();
      case 15: return TaskLoopTab();
      default: return SizedBox.shrink();
    }
  }
}
