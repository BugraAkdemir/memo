import 'package:flutter/material.dart';

import '../core/theme.dart';

class _PromptTemplate {
  final String key;
  final String icon;
  final String label;
  final String text;

  const _PromptTemplate({
    required this.key,
    required this.icon,
    required this.label,
    required this.text,
  });
}

class _PromptCommand {
  final String key;
  final String icon;
  final String label;
  final String subtitle;

  const _PromptCommand({
    required this.key,
    required this.icon,
    required this.label,
    required this.subtitle,
  });
}

enum ItemType { template, command }

class PopupItem {
  final ItemType type;
  final _PromptTemplate? template;
  final _PromptCommand? command;

  const PopupItem.template(this.template) : type = ItemType.template, command = null;
  const PopupItem.command(this.command) : type = ItemType.command, template = null;

  String get key => template?.key ?? command!.key;
  String get icon => template?.icon ?? command!.icon;
  String get label => template?.label ?? command!.label;
  String get text => template?.text ?? '';
}

const List<PopupItem> templates = [
  PopupItem.template(_PromptTemplate(
    key: '/code',
    icon: '💻',
    label: 'Kod Review',
    text:
        'Aşağıdaki kodu incele, hataları ve iyileştirme önerilerini açıkla:\n\n```\n\n```',
  )),
  PopupItem.template(_PromptTemplate(
    key: '/explain',
    icon: '📖',
    label: 'Açıkla',
    text: 'Aşağıdaki kavramı basit ve anlaşılır bir şekilde açıkla:\n\n',
  )),
  PopupItem.template(_PromptTemplate(
    key: '/fix',
    icon: '🔧',
    label: 'Hata Düzelt',
    text: 'Bu hata mesajını analiz et ve nasıl düzelteceğimi göster:\n\n',
  )),
  PopupItem.template(_PromptTemplate(
    key: '/plan',
    icon: '🗺️',
    label: 'Plan Yap',
    text: 'Aşağıdaki görev için adım adım bir uygulama planı oluştur:\n\n',
  )),
  PopupItem.template(_PromptTemplate(
    key: '/summary',
    icon: '📝',
    label: 'Özetle',
    text: 'Aşağıdaki metni kısa ve öz şekilde özetle:\n\n',
  )),
  PopupItem.template(_PromptTemplate(
    key: '/compare',
    icon: '⚖️',
    label: 'Karşılaştır',
    text: 'Şu iki seçeneği karşılaştır, artı ve eksilerini listele:\n\n1. \n2. ',
  )),
  PopupItem.template(_PromptTemplate(
    key: '/brainstorm',
    icon: '💡',
    label: 'Beyin Fırtınası',
    text: 'Şu konu hakkında yaratıcı fikirler üret:\n\n',
  )),
  PopupItem.template(_PromptTemplate(
    key: '/translate',
    icon: '🌐',
    label: 'Çevir (EN->TR)',
    text: 'Aşağıdaki metni Türkçeye çevir:\n\n',
  )),
  PopupItem.command(_PromptCommand(
    key: '/model',
    icon: '🧠',
    label: 'Model Değiştir',
    subtitle: 'Local / API arasında geçiş',
  )),
  PopupItem.command(_PromptCommand(
    key: '/orchestra',
    icon: '🎵',
    label: 'Orchestra Mode',
    subtitle: 'Çoklu model orkestrasyonu',
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
          'No matching command',
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
      constraints: const BoxConstraints(maxHeight: 360),
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
    final highlight = widget.isSelected || _hovering;
    return MouseRegion(
      onEnter: (_) => setState(() => _hovering = true),
      onExit: (_) => setState(() => _hovering = false),
      child: GestureDetector(
        onTap: widget.onTap,
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
          color: highlight
              ? MemoTheme.of(context).bgElement
              : Colors.transparent,
          child: Row(
            children: [
              Text(widget.item.icon, style: const TextStyle(fontSize: 18)),
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
                        color: MemoTheme.of(context).textMain,
                      ),
                    ),
                    Text(
                      widget.item.key,
                      style: TextStyle(
                        fontSize: 11,
                        color: MemoTheme.of(context).textDim,
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
                    'action',
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
