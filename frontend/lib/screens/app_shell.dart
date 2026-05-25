import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../providers/settings_provider.dart';
import '../widgets/settings_dialog.dart';
import '../widgets/llama_installer_view.dart';
import '../widgets/setup_wizard_view.dart';
import 'chat_screen.dart';
import 'model_store_screen.dart';

/// Main app shell — sidebar (chat list) + content area.
/// Navigation between Chat and Model Store via bottom of sidebar.
class AppShell extends ConsumerStatefulWidget {
  const AppShell({super.key});

  @override
  ConsumerState<AppShell> createState() => _AppShellState();
}

class _AppShellState extends ConsumerState<AppShell> {
  int _currentIndex = 0; // 0 = chat, 1 = models (future)

  @override
  Widget build(BuildContext context) {
    final locale = ref.watch(localeProvider);
    L10n.setLocale(locale);

    return Scaffold(
      backgroundColor: MemoTheme.bgApp,
      body: Stack(
        children: [
          Row(
            children: [
              // ─── Nav Rail ─────────────────────────────────
              _buildNavRail(),

              // ─── Content ─────────────────────────────────
              Expanded(
                child: _currentIndex == 0
                    ? ChatScreen(key: ValueKey('chat_$locale'))
                    : ModelStoreScreen(key: ValueKey('models_$locale')),
              ),
            ],
          ),

          // ─── Overlays ───────────────────────────────────────────────
          // Order matters: SetupWizard renders on top (setup must complete first),
          // then LlamaInstaller shows if llama.cpp is missing after setup.
          const SetupWizardOverlay(),
          const LlamaInstallerOverlay(),
        ],
      ),
    );
  }

  Widget _buildNavRail() {
    return Container(
      width: 64,
      decoration: BoxDecoration(
        color: MemoTheme.bgPanel,
        border: Border(right: BorderSide(color: MemoTheme.borderSoft)),
      ),
      child: Column(
        children: [
          const SizedBox(height: 16),

          // ─── Logo ──────────────────────────────────
          Container(
            width: 40,
            height: 40,
            decoration: BoxDecoration(
              color: MemoTheme.accentPale,
              borderRadius: BorderRadius.circular(12),
              border: Border.all(color: MemoTheme.accent, width: 1.5),
            ),
            child: const Center(
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

          const SizedBox(height: 24),

          // ─── Chat Tab ────────────────────────────────
          _NavRailButton(
            icon: Icons.chat_bubble_outline,
            activeIcon: Icons.chat_bubble,
            label: L10n.t('chats'),
            isActive: _currentIndex == 0,
            onTap: () => setState(() => _currentIndex = 0),
          ),

          const SizedBox(height: 8),

          // ─── Models Tab ──────────────────────────────
          _NavRailButton(
            icon: Icons.memory_outlined,
            activeIcon: Icons.memory,
            label: L10n.t('model_store'),
            isActive: _currentIndex == 1,
            onTap: () => setState(() => _currentIndex = 1),
          ),

          const Spacer(),

          // ─── Settings Button ─────────────────────────
          _NavRailButton(
            icon: Icons.settings_outlined,
            activeIcon: Icons.settings,
            label: L10n.t('settings'),
            isActive: false,
            onTap: () {
              showDialog(
                context: context,
                builder: (context) => const SettingsDialog(),
              );
            },
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
            color: isActive ? MemoTheme.accent : MemoTheme.textDim,
          ),
        ),
      ),
    );
  }
}
