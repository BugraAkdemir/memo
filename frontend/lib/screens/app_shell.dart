import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../providers/settings_provider.dart';
import '../widgets/settings_dialog.dart';
import '../widgets/llama_installer_view.dart';
import '../widgets/setup_wizard_view.dart';
import 'chat_screen.dart';
import 'agent_screen.dart';
import 'model_store_screen.dart';

/// Main app shell — NavRail + content area.
class AppShell extends ConsumerStatefulWidget {
   AppShell({super.key});

  @override
  ConsumerState<AppShell> createState() => _AppShellState();
}

class _AppShellState extends ConsumerState<AppShell> {
  int _currentIndex = 0; // 0 = chat, 1 = agent, 2 = models

  @override
  Widget build(BuildContext context) {
    final locale = ref.watch(localeProvider);
    L10n.setLocale(locale);

    return Scaffold(
      backgroundColor: MemoTheme.of(context).bgApp,
      body: Stack(
        children: [
          Row(
            children: [
              _buildNavRail(),
              Expanded(
                child: IndexedStack(
                  index: _currentIndex,
                  children: [
                    ChatScreen(key: ValueKey('chat_$locale')),
                    const AgentScreen(),
                    ModelStoreScreen(key: ValueKey('models_$locale')),
                  ],
                ),
              ),
            ],
          ),
          SetupWizardOverlay(),
          LlamaInstallerOverlay(),
        ],
      ),
    );
  }

  Widget _buildNavRail() {
    return Container(
      width: 64,
      decoration: BoxDecoration(
        color: MemoTheme.of(context).bgPanel,
        border: Border(right: BorderSide(color: MemoTheme.of(context).borderSoft)),
      ),
      child: Column(
        children: [
           SizedBox(height: 16),

          // ─── Logo ──────────────────────────────────
          Container(
            width: 40,
            height: 40,
            decoration: BoxDecoration(
              color: MemoTheme.accentPale,
              borderRadius: BorderRadius.circular(12),
              border: Border.all(color: MemoTheme.accent, width: 1.5),
            ),
            child:  Center(
              child: Text(
                'M',
                style: TextStyle(
                  fontSize: 20,
                  fontWeight: FontWeight.bold,
                  color: MemoTheme.accent,
                ),
              ),
            ),
          ),

           SizedBox(height: 24),

           // ─── Chat Tab ────────────────────────────────
           _NavRailButton(
            icon: Icons.chat_bubble_outline,
            activeIcon: Icons.chat_bubble,
            label: L10n.t('chats'),
            isActive: _currentIndex == 0,
            onTap: () => setState(() => _currentIndex = 0),
          ),

           SizedBox(height: 8),

           // ─── Agent Tab ──────────────────────────────
          _NavRailButton(
            icon: Icons.smart_toy_outlined,
            activeIcon: Icons.smart_toy,
            label: 'Ajan',
            isActive: _currentIndex == 1,
            onTap: () => setState(() => _currentIndex = 1),
          ),

           SizedBox(height: 8),

           // ─── Models Tab ──────────────────────────────
          _NavRailButton(
            icon: Icons.memory_outlined,
            activeIcon: Icons.memory,
            label: L10n.t('model_store'),
            isActive: _currentIndex == 2,
            onTap: () => setState(() => _currentIndex = 2),
          ),

           Spacer(),

          // ─── Settings Button ─────────────────────────
          _NavRailButton(
            icon: Icons.settings_outlined,
            activeIcon: Icons.settings,
            label: L10n.t('settings'),
            isActive: false,
            onTap: () {
              showDialog(
                context: context,
                builder: (context) =>  SettingsDialog(),
              );
            },
          ),

           SizedBox(height: 16),
        ],
      ),
    );
  }
}

class _NavRailButton extends StatelessWidget {
  final IconData icon;
  final IconData activeIcon;
  final String label;
  final bool isActive;
  final VoidCallback onTap;

   _NavRailButton({
    required this.icon,
    required this.activeIcon,
    required this.label,
    required this.isActive,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Tooltip(
      message: label,
      preferBelow: false,
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
        child: AnimatedContainer(
          duration:  Duration(milliseconds: 150),
          width: 44,
          height: 44,
          decoration: BoxDecoration(
            color: isActive ? MemoTheme.accentMuted : Colors.transparent,
            borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
          ),
          child: Icon(
            isActive ? activeIcon : icon,
            size: 22,
            color: isActive ? MemoTheme.accent : MemoTheme.of(context).textDim,
          ),
        ),
      ),
    );
  }
}
