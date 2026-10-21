import 'package:flutter/material.dart';
import 'package:flutter_svg/flutter_svg.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../providers/skill_provider.dart';

/// What a row of the "/" menu does when it is picked.
enum SlashKind {
  /// An installed skill: inserts "/name " so the message calls it.
  skill,

  /// A command the app handles itself that takes text after it ("/remember …").
  insert,

  /// Opens a picker right away ("/model", "/orchestra").
  action,
}

class SlashItem {
  final SlashKind kind;

  /// What is shown and inserted, e.g. "/pair-programming".
  final String key;
  final String icon;
  final String label;
  final String subtitle;

  /// A skill this chat has already called.
  final bool active;

  const SlashItem({
    required this.kind,
    required this.key,
    required this.icon,
    required this.label,
    this.subtitle = '',
    this.active = false,
  });
}

/// The rows of the "/" menu: every installed skill, then the commands that do
/// something real. [atMessageStart] is false when the "/" was typed in the
/// middle of a sentence — only a skill can be called from there, so the
/// commands that act on the whole message are left out.
List<SlashItem> buildSlashItems({
  required List<SkillDefinition> skills,
  Set<String> activeSkills = const {},
  bool atMessageStart = true,
}) {
  final sorted = [...skills]
    ..sort((a, b) => a.name.toLowerCase().compareTo(b.name.toLowerCase()));
  return [
    for (final s in sorted)
      SlashItem(
        kind: SlashKind.skill,
        key: '/${skillSlug(s.name)}',
        icon: 'lib/icon/slash/puzzle-piece.svg',
        label: s.name,
        subtitle: s.description,
        active: activeSkills.contains(s.name),
      ),
    if (atMessageStart) ...[
      SlashItem(
        kind: SlashKind.insert,
        key: '/remember',
        icon: 'lib/icon/slash/brain.svg',
        label: L10n.t('slash_remember'),
        subtitle: L10n.t('slash_remember_sub'),
      ),
      SlashItem(
        kind: SlashKind.insert,
        key: '/forget',
        icon: 'lib/icon/slash/eye-slash.svg',
        label: L10n.t('slash_forget'),
        subtitle: L10n.t('slash_forget_sub'),
      ),
      SlashItem(
        kind: SlashKind.action,
        key: '/model',
        icon: 'lib/icon/slash/cpu.svg',
        label: L10n.t('template_switch_model'),
        subtitle: L10n.t('template_switch_model_sub'),
      ),
      SlashItem(
        kind: SlashKind.action,
        key: '/orchestra',
        icon: 'lib/icon/slash/music-notes.svg',
        label: L10n.t('template_orchestra'),
        subtitle: L10n.t('template_orchestra_sub'),
      ),
    ],
  ];
}

/// The rows that match [query], best first: the command typed exactly, then
/// commands starting with it, then containing it, then a name, then a
/// description. Typing "/model" must open the model picker even when a hundred
/// skills mention "model" in their description; within one rank the menu's own
/// order (skills by name, then commands) is kept.
List<SlashItem> filterSlashItems(List<SlashItem> items, String query) {
  final q = query.trim().toLowerCase();
  if (q.isEmpty) return items;
  final key = '/$q';
  int rank(SlashItem i) {
    final k = i.key.toLowerCase();
    if (k == key) return 0;
    if (k.startsWith(key)) return 1;
    if (k.contains(q)) return 2;
    if (i.label.toLowerCase().contains(q)) return 3;
    if (i.subtitle.toLowerCase().contains(q)) return 4;
    return -1;
  }

  final ranked = <(int, int, SlashItem)>[];
  for (var n = 0; n < items.length; n++) {
    final r = rank(items[n]);
    if (r >= 0) ranked.add((r, n, items[n]));
  }
  ranked.sort((a, b) => a.$1 != b.$1 ? a.$1.compareTo(b.$1) : a.$2.compareTo(b.$2));
  return [for (final r in ranked) r.$3];
}

/// Popup that appears above the input when the user types "/". Shows the
/// [items] it is given (already filtered) and reports the picked one.
class SlashMenuPopup extends StatefulWidget {
  final List<SlashItem> items;
  final ValueChanged<SlashItem> onSelect;
  final int selectedIndex;

  /// Skills are still being loaded: say so instead of "nothing matches".
  final bool loading;

  /// The skill list could not be loaded: say so, instead of showing only the
  /// commands (or "nothing matches") as if no skills were installed.
  final bool loadFailed;

  const SlashMenuPopup({
    super.key,
    required this.items,
    required this.onSelect,
    this.selectedIndex = 0,
    this.loading = false,
    this.loadFailed = false,
  });

  @override
  State<SlashMenuPopup> createState() => _SlashMenuPopupState();
}

class _SlashMenuPopupState extends State<SlashMenuPopup> {
  final ScrollController _scroll = ScrollController();

  double get _rowHeight => 60 * MediaQuery.textScalerOf(context).scale(13) / 13;

  @override
  void initState() {
    super.initState();
    _scheduleEnsureVisible();
  }

  @override
  void didUpdateWidget(SlashMenuPopup oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.selectedIndex != widget.selectedIndex ||
        oldWidget.items.length != widget.items.length) {
      _scheduleEnsureVisible();
    }
  }

  void _scheduleEnsureVisible() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted || !_scroll.hasClients || widget.items.isEmpty) return;
      final index = widget.selectedIndex.clamp(0, widget.items.length - 1);
      final top = 4 + index * _rowHeight;
      final bottom = top + _rowHeight;
      final position = _scroll.position;
      // Move only on selection/query changes so wheel scrolling remains free.
      double? target;
      if (top < position.pixels) {
        target = index == 0 ? 0 : top;
      } else if (bottom > position.pixels + position.viewportDimension) {
        target = bottom - position.viewportDimension;
      }
      if (target != null) {
        _scroll.jumpTo(target.clamp(0.0, position.maxScrollExtent));
      }
    });
  }

  @override
  void dispose() {
    _scroll.dispose();
    super.dispose();
  }

  BoxDecoration _decoration(BuildContext context) => BoxDecoration(
        color: MemoTheme.of(context).bgApp,
        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
        border: Border.all(color: MemoTheme.of(context).borderSoft),
        boxShadow: MemoTheme.shadowMd,
      );

  @override
  Widget build(BuildContext context) {
    final items = widget.items;
    if (items.isEmpty) {
      return Container(
        margin: const EdgeInsets.symmetric(horizontal: 16),
        constraints: const BoxConstraints(maxHeight: 120),
        decoration: _decoration(context),
        padding: const EdgeInsets.all(16),
        child: Text(
          L10n.t(widget.loadFailed
              ? 'slash_load_failed'
              : widget.loading
                  ? 'slash_loading'
                  : 'no_matching_command'),
          style: TextStyle(
            fontSize: 13,
            color: MemoTheme.of(context).textDim,
            fontStyle: FontStyle.italic,
          ),
        ),
      );
    }

    return Container(
      margin: const EdgeInsets.symmetric(horizontal: 16),
      constraints: const BoxConstraints(maxHeight: 280),
      decoration: _decoration(context),
      child: ClipRRect(
        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (widget.loadFailed)
              Container(
                width: double.infinity,
                padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
                color: MemoTheme.of(context).bgElement,
                child: Text(
                  L10n.t('slash_load_failed'),
                  style: TextStyle(fontSize: 12, color: MemoTheme.of(context).textDim),
                ),
              ),
            Flexible(
              child: Scrollbar(
          controller: _scroll,
          child: ListView.builder(
            controller: _scroll,
            primary: false,
            itemExtent: _rowHeight,
            padding: const EdgeInsets.symmetric(vertical: 4),
            shrinkWrap: true,
            itemCount: items.length,
            itemBuilder: (context, index) {
              final item = items[index];
              return _SlashRow(
                item: item,
                isSelected: index == widget.selectedIndex,
                onTap: () => widget.onSelect(item),
              );
            },
          ),
        ),
            ),
          ],
        ),
      ),
    );
  }
}

class _SlashRow extends StatefulWidget {
  final SlashItem item;
  final VoidCallback onTap;
  final bool isSelected;

  const _SlashRow({
    required this.item,
    required this.onTap,
    this.isSelected = false,
  });

  @override
  State<_SlashRow> createState() => _SlashRowState();
}

class _SlashRowState extends State<_SlashRow> {
  bool _hovering = false;

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final item = widget.item;
    final highlight = widget.isSelected || _hovering;
    final iconColor = highlight ? MemoTheme.accent : c.textMuted;

    return MouseRegion(
      onEnter: (_) => setState(() => _hovering = true),
      onExit: (_) => setState(() => _hovering = false),
      child: GestureDetector(
        onTap: widget.onTap,
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
          color: highlight ? c.bgElement : Colors.transparent,
          child: Row(
            children: [
              SizedBox(
                width: 22,
                height: 22,
                child: SvgPicture.asset(
                  item.icon,
                  colorFilter: ColorFilter.mode(iconColor, BlendMode.srcIn),
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  mainAxisAlignment: MainAxisAlignment.center,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      item.label,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        fontSize: 13,
                        fontWeight: FontWeight.w500,
                        color: c.textMain,
                      ),
                    ),
                    Text(
                      item.key,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        fontSize: 11,
                        color: c.textDim,
                        fontFamily: 'JetBrains Mono',
                      ),
                    ),
                  ],
                ),
              ),
              if (item.kind == SlashKind.action)
                _Badge(text: L10n.t('action_badge'), color: MemoTheme.accent),
              if (item.kind == SlashKind.skill && item.active)
                _Badge(text: L10n.t('slash_skill_active'), color: MemoTheme.green),
            ],
          ),
        ),
      ),
    );
  }
}

class _Badge extends StatelessWidget {
  final String text;
  final Color color;
  const _Badge({required this.text, required this.color});

  @override
  Widget build(BuildContext context) => Container(
        margin: const EdgeInsets.only(left: 8),
        padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
        decoration: BoxDecoration(
          color: color.withValues(alpha: 0.1),
          borderRadius: BorderRadius.circular(4),
        ),
        child: Text(
          text,
          style: TextStyle(fontSize: 9, color: color, fontWeight: FontWeight.w600),
        ),
      );
}
