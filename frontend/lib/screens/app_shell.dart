import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import 'chat_screen.dart';

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
    return Scaffold(
      backgroundColor: MemoTheme.bgApp,
      body: Row(
        children: [
          // ─── Nav Rail ─────────────────────────────────
          _buildNavRail(),

          // ─── Content ─────────────────────────────────
          Expanded(
            child: _currentIndex == 0
                ? const ChatScreen()
                : const _ModelStorePlaceholder(),
          ),
        ],
      ),
    );
  }

  Widget _buildNavRail() {
    return Container(
      width: 64,
      decoration: BoxDecoration(
        color: MemoTheme.bgPanel,
        border: Border(
          right: BorderSide(color: MemoTheme.borderSoft),
        ),
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
              // TODO: open settings dialog (Faz 5)
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

/// Placeholder for Model Store — will be built in Faz 6.
class _ModelStorePlaceholder extends StatelessWidget {
  const _ModelStorePlaceholder();

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.memory, size: 48, color: MemoTheme.textDim),
          const SizedBox(height: 16),
          Text(
            L10n.t('model_store'),
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
