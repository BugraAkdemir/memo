import 'package:flutter/material.dart';

import '../core/theme.dart';

/// Prompt template data.
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

const _templates = [
  _PromptTemplate(
    key: '/code',
    icon: '💻',
    label: 'Kod Review',
    text:
        'Aşağıdaki kodu incele, hataları ve iyileştirme önerilerini açıkla:\n\n```\n\n```',
  ),
  _PromptTemplate(
    key: '/explain',
    icon: '📖',
    label: 'Açıkla',
    text: 'Aşağıdaki kavramı basit ve anlaşılır bir şekilde açıkla:\n\n',
  ),
  _PromptTemplate(
    key: '/fix',
    icon: '🔧',
    label: 'Hata Düzelt',
    text: 'Bu hata mesajını analiz et ve nasıl düzelteceğimi göster:\n\n',
  ),
  _PromptTemplate(
    key: '/plan',
    icon: '🗺️',
    label: 'Plan Yap',
    text:
        'Aşağıdaki görev için adım adım bir uygulama planı oluştur:\n\n',
  ),
  _PromptTemplate(
    key: '/summary',
    icon: '📝',
    label: 'Özetle',
    text: 'Aşağıdaki metni kısa ve öz şekilde özetle:\n\n',
  ),
  _PromptTemplate(
    key: '/compare',
    icon: '⚖️',
    label: 'Karşılaştır',
    text:
        'Şu iki seçeneği karşılaştır, artı ve eksilerini listele:\n\n1. \n2. ',
  ),
  _PromptTemplate(
    key: '/brainstorm',
    icon: '💡',
    label: 'Beyin Fırtınası',
    text: 'Şu konu hakkında yaratıcı fikirler üret:\n\n',
  ),
  _PromptTemplate(
    key: '/translate',
    icon: '🌐',
    label: 'Çevir (EN→TR)',
    text: 'Aşağıdaki metni Türkçeye çevir:\n\n',
  ),
];

/// Popup that appears above the input when user types "/".
class PromptTemplatesPopup extends StatelessWidget {
  final void Function(String templateText) onSelect;
  final VoidCallback onDismiss;

  const PromptTemplatesPopup({
    super.key,
    required this.onSelect,
    required this.onDismiss,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.symmetric(horizontal: 16),
      constraints: const BoxConstraints(maxHeight: 320),
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
          itemCount: _templates.length,
          itemBuilder: (context, index) {
            final tpl = _templates[index];
            return _TemplateItem(
              template: tpl,
              onTap: () => onSelect(tpl.text),
            );
          },
        ),
      ),
    );
  }
}

class _TemplateItem extends StatefulWidget {
  final _PromptTemplate template;
  final VoidCallback onTap;

  const _TemplateItem({required this.template, required this.onTap});

  @override
  State<_TemplateItem> createState() => _TemplateItemState();
}

class _TemplateItemState extends State<_TemplateItem> {
  bool _hovering = false;

  @override
  Widget build(BuildContext context) {
    return MouseRegion(
      onEnter: (_) => setState(() => _hovering = true),
      onExit: (_) => setState(() => _hovering = false),
      child: GestureDetector(
        onTap: widget.onTap,
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
          color: _hovering ? MemoTheme.of(context).bgElement : Colors.transparent,
          child: Row(
            children: [
              Text(widget.template.icon, style: const TextStyle(fontSize: 18)),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      widget.template.label,
                      style: TextStyle(
                        fontSize: 13,
                        fontWeight: FontWeight.w500,
                        color: MemoTheme.of(context).textMain,
                      ),
                    ),
                    Text(
                      widget.template.key,
                      style: TextStyle(
                        fontSize: 11,
                        color: MemoTheme.of(context).textDim,
                        fontFamily: 'JetBrains Mono',
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
