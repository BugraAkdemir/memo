import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../providers/settings_provider.dart';
import '../widgets/settings_dialog.dart';
import '../widgets/llama_installer_view.dart';
import '../widgets/setup_wizard_view.dart';
import '../widgets/version_banner.dart';
import '../widgets/engine_strip.dart';
import 'chat_screen.dart';
import 'agent_screen.dart';
import 'model_store_screen.dart';
import 'whatsapp_screen.dart';

/// Main app shell — NavRail + content area.
class AppShell extends ConsumerStatefulWidget {
  AppShell({super.key});

  @override
  ConsumerState<AppShell> createState() => _AppShellState();
}

class _AppShellState extends ConsumerState<AppShell> {
  int _currentIndex = 0; // 0=chat 1=agent 2=models 3=whatsapp

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
                child: Column(
                  children: [
                    Expanded(
                      child: IndexedStack(
                        index: _currentIndex,
                        children: [
                          ChatScreen(key: ValueKey('chat_$locale')),
                          const AgentScreen(),
                          ModelStoreScreen(key: ValueKey('models_$locale')),
                          const WhatsAppScreen(),
                        ],
                      ),
                    ),
                    EngineStrip(
                      onOpenModels: () => setState(() => _currentIndex = 2),
                    ),
                  ],
                ),
              ),
            ],
          ),
          SetupWizardOverlay(),
          LlamaInstallerOverlay(),
          const VersionBanner(),
        ],
      ),
    );
  }

  Widget _buildNavRail() {
    final c = MemoTheme.of(context);
    return Container(
      width: 64,
      decoration: BoxDecoration(
        color: c.bgPanel,
        border: Border(right: BorderSide(color: c.borderSoft)),
      ),
      child: Column(
        children: [
          const SizedBox(height: 16),

          // ─── Logo ────────────────────────────────────
          ClipRRect(
            borderRadius: BorderRadius.circular(12),
            child: Image.asset(
              'lib/icon/memo.png',
              width: 40,
              height: 40,
              fit: BoxFit.cover,
            ),
          ),

          const SizedBox(height: 24),

          _NavRailButton(
            icon: Icons.chat_bubble_outline,
            activeIcon: Icons.chat_bubble,
            label: L10n.t('chats'),
            isActive: _currentIndex == 0,
            onTap: () => setState(() => _currentIndex = 0),
          ),

          const SizedBox(height: 8),

          _NavRailButton(
            icon: Icons.smart_toy_outlined,
            activeIcon: Icons.smart_toy,
            label: 'Ajan',
            isActive: _currentIndex == 1,
            onTap: () => setState(() => _currentIndex = 1),
          ),

          const SizedBox(height: 8),

          _NavRailButton(
            icon: Icons.memory_outlined,
            activeIcon: Icons.memory,
            label: L10n.t('model_store'),
            isActive: _currentIndex == 2,
            onTap: () => setState(() => _currentIndex = 2),
          ),

          const SizedBox(height: 8),

          _NavRailButton(
            icon: Icons.message_outlined,
            activeIcon: Icons.message,
            label: 'WhatsApp',
            isActive: _currentIndex == 3,
            onTap: () => setState(() => _currentIndex = 3),
          ),

          const Spacer(),

          _NavRailButton(
            icon: Icons.settings_outlined,
            activeIcon: Icons.settings,
            label: L10n.t('settings'),
            isActive: false,
            onTap: () => showDialog(
              context: context,
              builder: (context) => SettingsDialog(),
            ),
          ),

          const SizedBox(height: 16),
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

  const _NavRailButton({
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
          duration: const Duration(milliseconds: 150),
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
