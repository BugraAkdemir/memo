import 'package:flutter/material.dart';

import '../core/l10n.dart';
import '../core/theme.dart';

/// One line of the model picker: a section header, or a selectable row whose
/// [value] is handed back when it is tapped.
class ModelPickerEntry {
  final String? value;
  final String title;
  final String tag;
  final Widget? leading;
  final Widget? trailing;
  final bool active;
  final bool isHeader;

  const ModelPickerEntry.header(this.title, {this.leading})
      : value = null,
        tag = '',
        trailing = null,
        active = false,
        isHeader = true;

  const ModelPickerEntry.item({
    required String this.value,
    required this.title,
    this.tag = '',
    this.leading,
    this.trailing,
    this.active = false,
  }) : isHeader = false;
}

/// Opens the model picker under [anchor] (the top-bar button's rectangle) and
/// returns the chosen entry's value, or null when it is dismissed.
///
/// A panel rather than a popup menu: its height is capped to what fits below the
/// button and it scrolls inside that, so a long model list can no longer run off
/// the bottom of the window; and it can search, which a dozen-plus models need.
/// [footer] is pinned under the scrolling list (the "add provider" row).
Future<String?> showModelPicker({
  required BuildContext context,
  required Rect anchor,
  required List<ModelPickerEntry> entries,
  ModelPickerEntry? footer,
}) {
  return showGeneralDialog<String>(
    context: context,
    barrierDismissible: true,
    barrierLabel: L10n.t('switch_model'),
    barrierColor: Colors.transparent,
    transitionDuration: const Duration(milliseconds: 110),
    transitionBuilder: (_, anim, _, child) => FadeTransition(
      opacity: CurvedAnimation(parent: anim, curve: Curves.easeOut),
      child: child,
    ),
    pageBuilder: (ctx, _, _) => _ModelPickerPanel(anchor: anchor, entries: entries, footer: footer),
  );
}

class _ModelPickerPanel extends StatefulWidget {
  final Rect anchor;
  final List<ModelPickerEntry> entries;
  final ModelPickerEntry? footer;
  const _ModelPickerPanel({required this.anchor, required this.entries, this.footer});

  @override
  State<_ModelPickerPanel> createState() => _ModelPickerPanelState();
}

class _ModelPickerPanelState extends State<_ModelPickerPanel> {
  final _search = TextEditingController();
  String _q = '';

  @override
  void dispose() {
    _search.dispose();
    super.dispose();
  }

  /// Rows matching the query, with a header kept only while something under it
  /// matches.
  List<ModelPickerEntry> get _visible {
    final q = _q.trim().toLowerCase();
    if (q.isEmpty) return widget.entries;
    final out = <ModelPickerEntry>[];
    ModelPickerEntry? pendingHeader;
    for (final e in widget.entries) {
      if (e.isHeader) {
        pendingHeader = e;
        continue;
      }
      if ('${e.title} ${e.tag} ${e.value}'.toLowerCase().contains(q)) {
        if (pendingHeader != null) {
          out.add(pendingHeader);
          pendingHeader = null;
        }
        out.add(e);
      }
    }
    return out;
  }

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final size = MediaQuery.sizeOf(context);
    const margin = 12.0;
    final width = (size.width - margin * 2).clamp(240.0, 340.0);
    final left = (widget.anchor.right - width).clamp(margin, size.width - width - margin);
    final top = widget.anchor.bottom + 6;
    // Everything below the button, minus a margin — never taller than 560, never
    // so short the list is unusable.
    final maxHeight = (size.height - top - margin).clamp(220.0, 560.0);

    final selectable = widget.entries.where((e) => !e.isHeader).length;
    final visible = _visible;

    return Stack(
      children: [
        Positioned(
          left: left,
          top: top,
          width: width,
          child: Material(
            color: c.bgPanel,
            elevation: 8,
            shadowColor: Colors.black54,
            borderRadius: BorderRadius.circular(12),
            clipBehavior: Clip.antiAlias,
            child: Container(
              constraints: BoxConstraints(maxHeight: maxHeight),
              decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(12),
                border: Border.all(color: c.borderSoft),
              ),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  if (selectable > 7)
                    Padding(
                      padding: const EdgeInsets.fromLTRB(10, 10, 10, 6),
                      child: SizedBox(
                        height: 34,
                        child: TextField(
                          controller: _search,
                          autofocus: true,
                          style: TextStyle(fontSize: 13, color: c.textMain),
                          onChanged: (v) => setState(() => _q = v),
                          onSubmitted: (_) {
                            final first = _visible.where((e) => !e.isHeader).firstOrNull;
                            if (first != null) Navigator.of(context).pop(first.value);
                          },
                          decoration: InputDecoration(
                            isDense: true,
                            hintText: L10n.t('search'),
                            hintStyle: TextStyle(fontSize: 13, color: c.textDim),
                            prefixIcon: Icon(Icons.search, size: 16, color: c.textDim),
                            prefixIconConstraints: const BoxConstraints(minWidth: 34),
                            filled: true,
                            fillColor: c.bgElement,
                            contentPadding: const EdgeInsets.symmetric(vertical: 8),
                            border: OutlineInputBorder(
                              borderRadius: BorderRadius.circular(8),
                              borderSide: BorderSide.none,
                            ),
                          ),
                        ),
                      ),
                    ),
                  Flexible(
                    child: ListView(
                      shrinkWrap: true,
                      padding: const EdgeInsets.symmetric(vertical: 6),
                      children: [for (final e in visible) e.isHeader ? _header(c, e) : _row(context, c, e)],
                    ),
                  ),
                  if (widget.footer != null) ...[
                    Divider(height: 1, color: c.borderSoft),
                    _row(context, c, widget.footer!, accent: true),
                  ],
                ],
              ),
            ),
          ),
        ),
      ],
    );
  }

  Widget _header(ThemeColors c, ModelPickerEntry e) => Padding(
        padding: const EdgeInsets.fromLTRB(14, 10, 14, 4),
        child: Row(
          children: [
            if (e.leading != null) ...[e.leading!, const SizedBox(width: 8)],
            Expanded(
              child: Text(
                e.title.toUpperCase(),
                overflow: TextOverflow.ellipsis,
                style: TextStyle(fontSize: 10.5, fontWeight: FontWeight.w700, letterSpacing: 0.6, color: c.textDim),
              ),
            ),
          ],
        ),
      );

  Widget _row(BuildContext context, ThemeColors c, ModelPickerEntry e, {bool accent = false}) {
    final color = accent ? MemoTheme.accent : c.textMain;
    return InkWell(
      onTap: () => Navigator.of(context).pop(e.value),
      child: Container(
        height: 38,
        margin: const EdgeInsets.symmetric(horizontal: 6),
        padding: const EdgeInsets.symmetric(horizontal: 8),
        decoration: BoxDecoration(
          color: e.active ? MemoTheme.accent.withValues(alpha: 0.12) : null,
          borderRadius: BorderRadius.circular(8),
        ),
        child: Row(
          children: [
            SizedBox(width: 20, child: Center(child: e.leading)),
            const SizedBox(width: 10),
            // Name + tag take ALL the room left of the right-hand cluster, so the
            // percentage / check columns line up across rows whether or not a row
            // has a tag, and a long name is cut only when it truly cannot fit.
            Expanded(
              child: Row(
                children: [
                  Flexible(
                    child: Text(
                      e.title,
                      overflow: TextOverflow.ellipsis,
                      maxLines: 1,
                      style: TextStyle(
                        fontSize: 13,
                        fontWeight: e.active ? FontWeight.w700 : FontWeight.w500,
                        color: color,
                      ),
                    ),
                  ),
                  if (e.tag.isNotEmpty) ...[
                    const SizedBox(width: 7),
                    Container(
                      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 1.5),
                      decoration: BoxDecoration(color: c.bgHover, borderRadius: BorderRadius.circular(5)),
                      child: Text(e.tag, style: TextStyle(fontSize: 10, fontWeight: FontWeight.w600, color: c.textDim)),
                    ),
                  ],
                ],
              ),
            ),
            if (e.trailing != null) ...[const SizedBox(width: 8), e.trailing!],
            // Fixed slot, so a row that is not the active one does not shift the
            // percentage column left by the width of the check.
            const SizedBox(width: 8),
            SizedBox(
              width: 16,
              child: e.active ? Icon(Icons.check, size: 16, color: MemoTheme.accent) : null,
            ),
          ],
        ),
      ),
    );
  }
}
