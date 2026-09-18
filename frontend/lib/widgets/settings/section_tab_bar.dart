import 'package:flutter/material.dart';
import '../../core/theme.dart';

String _formatCompactCount(int n) {
  if (n >= 1000000) return '${(n / 1000000).toStringAsFixed(1)}M';
  if (n >= 1000) return '${(n / 1000).toStringAsFixed(1)}K';
  return '$n';
}

/// One pill's spec for [SectionTabBar]. [count], when positive, renders as a
/// small badge on the pill — used for a live size hint (e.g. how many
/// pinned facts a section holds) so the number is visible before switching
/// to that section. Omit or pass null/0 for sections with nothing to count.
class SectionTabItem {
  final IconData icon;
  final String label;
  final int? count;
  const SectionTabItem(this.icon, this.label, {this.count});
}

/// Horizontal pill sub-navigation shared by settings tabs that would
/// otherwise stack every one of their sections into a single long scroll
/// (see memory_tab.dart, the first tab redesigned this way, and its git
/// history for the "her şey alt alta" complaint this exists to fix).
/// Renders [items] as pills and reports taps via [onSelected]; the caller
/// owns which section index is active and swaps the content pane below.
class SectionTabBar extends StatelessWidget {
  final int selected;
  final ValueChanged<int> onSelected;
  final List<SectionTabItem> items;

  const SectionTabBar({
    super.key,
    required this.selected,
    required this.onSelected,
    required this.items,
  });

  @override
  Widget build(BuildContext context) {
    return SingleChildScrollView(
      scrollDirection: Axis.horizontal,
      child: Row(
        children: [
          for (var i = 0; i < items.length; i++) ...[
            if (i > 0) const SizedBox(width: 6),
            _SectionPill(
              icon: items[i].icon,
              label: items[i].label,
              count: items[i].count,
              selected: selected == i,
              onTap: () => onSelected(i),
            ),
          ],
        ],
      ),
    );
  }
}

class _SectionPill extends StatelessWidget {
  final IconData icon;
  final String label;
  final int? count;
  final bool selected;
  final VoidCallback onTap;

  const _SectionPill({
    required this.icon,
    required this.label,
    required this.count,
    required this.selected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    final fg = selected ? theme.textInverse : theme.textMain;
    return Material(
      color: Colors.transparent,
      child: InkWell(
        borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
        onTap: onTap,
        child: AnimatedContainer(
          duration: const Duration(milliseconds: 140),
          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 9),
          decoration: BoxDecoration(
            color: selected ? MemoTheme.accent : theme.bgElement,
            borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
            border: Border.all(
              color: selected ? MemoTheme.accent : theme.borderSoft,
            ),
          ),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(icon, size: 16, color: fg),
              const SizedBox(width: 6),
              Text(
                label,
                style: TextStyle(
                  fontSize: 13,
                  fontWeight: FontWeight.w600,
                  color: fg,
                ),
              ),
              if (count != null && count! > 0) ...[
                const SizedBox(width: 6),
                Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 6,
                    vertical: 1,
                  ),
                  decoration: BoxDecoration(
                    color: selected
                        ? theme.textInverse.withValues(alpha: 0.2)
                        : MemoTheme.accent.withValues(alpha: 0.15),
                    borderRadius: BorderRadius.circular(999),
                  ),
                  child: Text(
                    _formatCompactCount(count!),
                    style: TextStyle(
                      fontSize: 10.5,
                      fontWeight: FontWeight.w700,
                      color: selected ? fg : MemoTheme.accent,
                    ),
                  ),
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }
}
