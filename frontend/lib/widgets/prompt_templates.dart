import 'package:flutter/material.dart';
import 'package:flutter_svg/flutter_svg.dart';

import '../core/l10n.dart';
import '../core/theme.dart';

class PromptTemplate {
  final String key;
  final String icon;
  final String label;
  final String text;

  const PromptTemplate({
    required this.key,
    required this.icon,
    required this.label,
    required this.text,
  });
}

class PromptCommand {
  final String key;
  final String icon;
  final String label;
  final String subtitle;

  const PromptCommand({
    required this.key,
    required this.icon,
    required this.label,
    required this.subtitle,
  });
}

enum ItemType { template, command }

class PopupItem {
  final ItemType type;
  final PromptTemplate? template;
  final PromptCommand? command;

  const PopupItem.template(this.template) : type = ItemType.template, command = null;
  const PopupItem.command(this.command) : type = ItemType.command, template = null;

  String get key => template?.key ?? command!.key;
  String get icon => template?.icon ?? command!.icon;
  String get label => template?.label ?? command!.label;
  String get text => template?.text ?? '';
}

/// Built at call time so labels/text follow the active [L10n] locale.
List<PopupItem> get templates => [
  PopupItem.template(PromptTemplate(
    key: '/code',
    icon: 'lib/icon/slash/code.svg',
    label: L10n.t('template_review'),
    text: L10n.t('template_review_text'),
  )),
  PopupItem.template(PromptTemplate(
    key: '/explain',
    icon: 'lib/icon/slash/lightbulb.svg',
    label: L10n.t('template_explain'),
    text: L10n.t('template_explain_text'),
  )),
  PopupItem.template(PromptTemplate(
    key: '/fix',
    icon: 'lib/icon/slash/wrench.svg',
    label: L10n.t('template_fix'),
    text: L10n.t('template_fix_text'),
  )),
  PopupItem.template(PromptTemplate(
    key: '/plan',
    icon: 'lib/icon/slash/list-checks.svg',
    label: L10n.t('template_plan'),
    text: L10n.t('template_plan_text'),
  )),
  PopupItem.template(PromptTemplate(
    key: '/summary',
    icon: 'lib/icon/slash/article.svg',
    label: L10n.t('template_summarize'),
    text: L10n.t('template_summarize_text'),
  )),
  PopupItem.template(PromptTemplate(
    key: '/compare',
    icon: 'lib/icon/slash/arrows-left-right.svg',
    label: L10n.t('template_compare'),
    text: L10n.t('template_compare_text'),
  )),
  PopupItem.template(PromptTemplate(
    key: '/brainstorm',
    icon: 'lib/icon/slash/brain.svg',
    label: L10n.t('template_brainstorm'),
    text: L10n.t('template_brainstorm_text'),
  )),
  PopupItem.template(PromptTemplate(
    key: '/translate',
    icon: 'lib/icon/slash/translate.svg',
    label: L10n.t('template_translate'),
    text: L10n.t('template_translate_text'),
  )),
  PopupItem.command(PromptCommand(
    key: '/model',
    icon: 'lib/icon/slash/cpu.svg',
    label: L10n.t('template_switch_model'),
    subtitle: L10n.t('template_switch_model_sub'),
  )),
  PopupItem.command(PromptCommand(
    key: '/orchestra',
    icon: 'lib/icon/slash/music-notes.svg',
    label: L10n.t('template_orchestra'),
    subtitle: L10n.t('template_orchestra_sub'),
  )),
  PopupItem.command(PromptCommand(
    key: '/skill',
    icon: 'lib/icon/slash/puzzle-piece.svg',
    label: L10n.t('template_skill'),
    subtitle: L10n.t('template_skill_sub'),
  )),
];

/// Result when user picks an item.
sealed class PopupResult {}

class PopupInsertText extends PopupResult {
  final String text;
  PopupInsertText(this.text);
}

class PopupModelSwitch extends PopupResult {}

class PopupOrchestraSwitch extends PopupResult {}

class PopupSkillSelect extends PopupResult {}

/// Popup that appears above the input when user types "/".
class PromptTemplatesPopup extends StatelessWidget {
  final void Function(PopupResult result) onSelect;
  final VoidCallback onDismiss;
  final String query;
  final int selectedIndex;

  const PromptTemplatesPopup({
    super.key,
    required this.onSelect,
    required this.onDismiss,
    this.query = '',
    this.selectedIndex = 0,
  });

  List<PopupItem> get _filteredItems {
    if (query.isEmpty) return templates;
    final q = query.toLowerCase();
    return templates.where((item) {
      return item.key.toLowerCase().contains(q) ||
          item.label.toLowerCase().contains(q);
    }).toList();
  }

  @override
  Widget build(BuildContext context) {
    final items = _filteredItems;
    if (items.isEmpty) {
      return Container(
        margin: const EdgeInsets.symmetric(horizontal: 16),
        constraints: const BoxConstraints(maxHeight: 120),
        decoration: BoxDecoration(
          color: MemoTheme.of(context).bgApp,
          borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
          border: Border.all(color: MemoTheme.of(context).borderSoft),
          boxShadow: MemoTheme.shadowMd,
        ),
        padding: const EdgeInsets.all(16),
        child: Text(
          L10n.t('no_matching_command'),
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
      decoration: BoxDecoration(
        color: MemoTheme.of(context).bgApp,
        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
        border: Border.all(color: MemoTheme.of(context).borderSoft),
        boxShadow: MemoTheme.shadowMd,
      ),
      child: ClipRRect(
        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
        child: ListView.builder(
          padding: const EdgeInsets.symmetric(vertical: 4),
          shrinkWrap: true,
          itemCount: items.length,
          itemBuilder: (context, index) {
            final item = items[index];
            final isSelected = index == selectedIndex;
            return _PopupItemWidget(
              item: item,
              isSelected: isSelected,
              onTap: () {
                if (item.type == ItemType.template) {
                  onSelect(PopupInsertText(item.text));
                } else if (item.key == '/model') {
                  onSelect(PopupModelSwitch());
                } else if (item.key == '/orchestra') {
                  onSelect(PopupOrchestraSwitch());
                } else if (item.key == '/skill') {
                  onSelect(PopupSkillSelect());
                }
              },
            );
          },
        ),
      ),
    );
  }
}

class _PopupItemWidget extends StatefulWidget {
  final PopupItem item;
  final VoidCallback onTap;
  final bool isSelected;

  const _PopupItemWidget({
    required this.item,
    required this.onTap,
    this.isSelected = false,
  });

  @override
  State<_PopupItemWidget> createState() => _PopupItemWidgetState();
}

class _PopupItemWidgetState extends State<_PopupItemWidget> {
  bool _hovering = false;

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
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
                  widget.item.icon,
                  colorFilter: ColorFilter.mode(iconColor, BlendMode.srcIn),
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      widget.item.label,
                      style: TextStyle(
                        fontSize: 13,
                        fontWeight: FontWeight.w500,
                        color: c.textMain,
                      ),
                    ),
                    Text(
                      widget.item.key,
                      style: TextStyle(
                        fontSize: 11,
                        color: c.textDim,
                        fontFamily: 'JetBrains Mono',
                      ),
                    ),
                  ],
                ),
              ),
              if (widget.item.type == ItemType.command)
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                  decoration: BoxDecoration(
                    color: MemoTheme.accent.withValues(alpha: 0.1),
                    borderRadius: BorderRadius.circular(4),
                  ),
                  child: Text(
                    L10n.t('action_badge'),
                    style: TextStyle(
                      fontSize: 9,
                      color: MemoTheme.accent,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ),
            ],
          ),
        ),
      ),
    );
  }
}
