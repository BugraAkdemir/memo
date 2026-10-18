import 'package:flutter/material.dart';

import '../core/l10n.dart';
import '../core/theme.dart';

/// The "@" file-mention dropdown above the composer: a flat list of
/// project-relative paths.
///
/// Rows have a fixed height so the list can keep the arrow-key selection in
/// view: before, the highlight moved down past the visible rows while the list
/// stayed where it was, so the person could not tell which file was selected.
/// The list scrolls with the mouse wheel and shows a scrollbar.
class FileMentionPopup extends StatefulWidget {
  final List<String> results;
  final int selectedIndex;
  final ValueChanged<int> onSelect;

  const FileMentionPopup({
    super.key,
    required this.results,
    required this.selectedIndex,
    required this.onSelect,
  });

  static const double rowHeight = 34;
  static const double maxHeight = 272;

  @override
  State<FileMentionPopup> createState() => _FileMentionPopupState();
}

class _FileMentionPopupState extends State<FileMentionPopup> {
  final ScrollController _scroll = ScrollController();

  @override
  void initState() {
    super.initState();
    _scheduleEnsureVisible();
  }

  @override
  void didUpdateWidget(FileMentionPopup old) {
    super.didUpdateWidget(old);
    if (old.selectedIndex != widget.selectedIndex ||
        old.results.length != widget.results.length) {
      _scheduleEnsureVisible();
    }
  }

  @override
  void dispose() {
    _scroll.dispose();
    super.dispose();
  }

  void _scheduleEnsureVisible() {
    WidgetsBinding.instance.addPostFrameCallback((_) => _ensureVisible());
  }

  /// Scrolls just enough that the selected row is fully inside the viewport.
  void _ensureVisible() {
    if (!mounted || !_scroll.hasClients || widget.results.isEmpty) return;
    final index = widget.selectedIndex.clamp(0, widget.results.length - 1);
    final top = index * FileMentionPopup.rowHeight;
    final bottom = top + FileMentionPopup.rowHeight;
    final viewport = _scroll.position.viewportDimension;
    final offset = _scroll.offset;
    double? target;
    if (top < offset) {
      target = top;
    } else if (bottom > offset + viewport) {
      target = bottom - viewport;
    }
    if (target != null) {
      _scroll.jumpTo(target.clamp(0.0, _scroll.position.maxScrollExtent));
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    final decoration = BoxDecoration(
      color: theme.bgApp,
      borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
      border: Border.all(color: theme.borderSoft),
      boxShadow: MemoTheme.shadowMd,
    );

    if (widget.results.isEmpty) {
      return Container(
        constraints: const BoxConstraints(maxHeight: 80),
        decoration: decoration,
        padding: const EdgeInsets.all(16),
        child: Text(
          L10n.t('file_mention_none'),
          style: TextStyle(
            fontSize: 13,
            color: theme.textDim,
            fontStyle: FontStyle.italic,
          ),
        ),
      );
    }

    final height = (widget.results.length * FileMentionPopup.rowHeight + 8)
        .clamp(0.0, FileMentionPopup.maxHeight);
    return Container(
      height: height,
      decoration: decoration,
      child: ClipRRect(
        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
        child: Padding(
          padding: const EdgeInsets.symmetric(vertical: 4),
          child: Scrollbar(
            controller: _scroll,
            thumbVisibility: widget.results.length * FileMentionPopup.rowHeight >
                height,
            child: ListView.builder(
              controller: _scroll,
              itemExtent: FileMentionPopup.rowHeight,
              itemCount: widget.results.length,
              itemBuilder: (context, index) {
                final path = widget.results[index];
                final selected = index == widget.selectedIndex;
                return InkWell(
                  onTap: () => widget.onSelect(index),
                  child: Container(
                    color: selected ? theme.bgElement : Colors.transparent,
                    padding: const EdgeInsets.symmetric(horizontal: 14),
                    alignment: Alignment.centerLeft,
                    child: Row(
                      children: [
                        Icon(
                          Icons.insert_drive_file_outlined,
                          size: 15,
                          color: selected ? MemoTheme.accent : theme.textDim,
                        ),
                        const SizedBox(width: 8),
                        Expanded(
                          child: Text(
                            path,
                            style: TextStyle(fontSize: 13, color: theme.textMain),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                          ),
                        ),
                      ],
                    ),
                  ),
                );
              },
            ),
          ),
        ),
      ),
    );
  }
}
