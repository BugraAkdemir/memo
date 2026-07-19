import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/l10n.dart';
import '../../core/theme.dart';
import '../../models/curated_models.dart';
import '../../providers/chat_provider.dart';
import '../../providers/models_provider.dart';

import 'discover_item.dart';
import 'model_detail_panel.dart';

// ─── Sort mode ────────────────────────────────────────────────────

enum _SortMode { defaultOrder, mostDownloads, smallestFirst, largestFirst }

// ─── Discover tab — two-panel layout ─────────────────────────────

class DiscoverTab extends ConsumerStatefulWidget {
  final bool isActive;

  const DiscoverTab({super.key, required this.isActive});

  @override
  ConsumerState<DiscoverTab> createState() => _DiscoverTabState();
}

class _DiscoverTabState extends ConsumerState<DiscoverTab> {
  DiscoverItem? _selected;
  final _searchCtrl = TextEditingController();
  Timer? _debounce;

  // Filter + sort state
  final Set<String> _filters = {}; // 'tools', 'vision', 'code', '1-8b', '8-14b', '14b+'
  _SortMode _sort = _SortMode.defaultOrder;

  @override
  void dispose() {
    _debounce?.cancel();
    _searchCtrl.dispose();
    super.dispose();
  }

  void _onSearch(String val) {
    _debounce?.cancel();
    _debounce = Timer(const Duration(milliseconds: 400), () {
      // IndexedStack keeps this tab mounted even when the user has switched
      // to "My Models" — `mounted` alone doesn't catch that, so a pending
      // debounce fired an unnecessary HuggingFace search after the user had
      // already navigated away. widget.isActive tracks the parent's actual
      // selected tab.
      if (mounted && widget.isActive) {
        ref.read(modelSearchQueryProvider.notifier).state = val.trim();
      }
    });
  }

  /// Extract numeric param count for size filtering.
  double? _paramValue(DiscoverItem item) {
    final text = item.paramCount;
    if (text == null) return null;
    final m = RegExp(r'(\d+(?:\.\d+)?)B').firstMatch(text);
    if (m == null) return null;
    return double.tryParse(m.group(1)!);
  }

  List<DiscoverItem> _applyFiltersSort(List<DiscoverItem> raw) {
    var list = raw.toList();

    // Capability filters — OR'd together (a model matching ANY selected
    // capability passes), same as the size filters just below. Previously
    // each capability was applied as its own independent `.where`, which
    // chained into an AND: picking both "Tools" and "Vision" required a
    // single model to have both, when a user checking two capability boxes
    // almost always means "show me either kind" — since very few GGUF
    // models genuinely support both, that combination silently produced an
    // empty result list before this fix.
    final capFilterKeys = _filters.intersection({'tools', 'vision', 'code', 'embedding'});
    if (capFilterKeys.isNotEmpty) {
      list = list.where((i) {
        if (capFilterKeys.contains('tools') && i.supportsTools) return true;
        if (capFilterKeys.contains('vision') && i.supportsVision) return true;
        if (capFilterKeys.contains('code') && i.supportsCode) return true;
        if (capFilterKeys.contains('embedding') && i.isEmbedding) return true;
        return false;
      }).toList();
    }

    // Size filters
    final sizeFilters = _filters.intersection({'1-8b', '8-14b', '14b+'});
    if (sizeFilters.isNotEmpty) {
      list = list.where((i) {
        final p = _paramValue(i);
        if (p == null) return false;
        if (sizeFilters.contains('1-8b') && p <= 8) return true;
        if (sizeFilters.contains('8-14b') && p > 8 && p <= 14) return true;
        if (sizeFilters.contains('14b+') && p > 14) return true;
        return false;
      }).toList();
    }

    // Sort
    switch (_sort) {
      case _SortMode.mostDownloads:
        list.sort((a, b) => b.downloads.compareTo(a.downloads));
      case _SortMode.smallestFirst:
        list.sort((a, b) =>
            (_paramValue(a) ?? 999).compareTo(_paramValue(b) ?? 999));
      case _SortMode.largestFirst:
        list.sort((a, b) =>
            (_paramValue(b) ?? 0).compareTo(_paramValue(a) ?? 0));
      case _SortMode.defaultOrder:
        break;
    }

    return list;
  }

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final query = ref.watch(modelSearchQueryProvider);
    final resultsAsync = ref.watch(modelSearchResultsProvider);

    final List<DiscoverItem> rawItems;
    final bool isLoading;
    if (query.isEmpty) {
      rawItems = curatedModels.map(DiscoverItem.fromCurated).toList();
      isLoading = false;
    } else {
      rawItems =
          resultsAsync.valueOrNull?.map(DiscoverItem.fromHF).toList() ?? [];
      isLoading = resultsAsync.isLoading;
    }

    final items = _applyFiltersSort(rawItems);

    return Row(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        SizedBox(
          width: 340,
          child: _ModelListPanel(
            items: items,
            selected: _selected,
            isCurated: query.isEmpty,
            isLoading: isLoading,
            searchController: _searchCtrl,
            activeFilters: _filters,
            sortMode: _sort,
            onSearch: _onSearch,
            onSelect: (item) => setState(() => _selected = item),
            onFilterToggle: (key) => setState(() {
              if (_filters.contains(key)) {
                _filters.remove(key);
              } else {
                _filters.add(key);
              }
            }),
            onSortChange: (mode) => setState(() => _sort = mode),
            onFiltersClear: () => setState(() => _filters.clear()),
          ),
        ),
        VerticalDivider(width: 1, thickness: 1, color: c.borderSoft),
        Expanded(
          child: _selected == null
              ? const _EmptyDetailState()
              : ModelDetailPanel(
                  key: ValueKey(_selected!.repoId),
                  item: _selected!,
                  onSelectOther: (repoId, displayName) => setState(() {
                    _selected = DiscoverItem(
                      repoId: repoId,
                      author: repoId.contains('/')
                          ? repoId.split('/').first
                          : repoId,
                      displayName: displayName,
                      description: '',
                      supportsTools: false,
                      supportsVision: false,
                      supportsCode: false,
                      isEmbedding: false,
                      downloads: 0,
                      likes: 0,
                      tags: const [],
                      isCurated: false,
                    );
                  }),
                ),
        ),
      ],
    );
  }
}

// ─── Left panel: search + filters + model list ───────────────────

class _ModelListPanel extends ConsumerWidget {
  final List<DiscoverItem> items;
  final DiscoverItem? selected;
  final bool isCurated;
  final bool isLoading;
  final TextEditingController searchController;
  final Set<String> activeFilters;
  final _SortMode sortMode;
  final ValueChanged<String> onSearch;
  final ValueChanged<DiscoverItem> onSelect;
  final ValueChanged<String> onFilterToggle;
  final ValueChanged<_SortMode> onSortChange;
  final VoidCallback onFiltersClear;

  const _ModelListPanel({
    required this.items,
    required this.selected,
    required this.isCurated,
    required this.isLoading,
    required this.searchController,
    required this.activeFilters,
    required this.sortMode,
    required this.onSearch,
    required this.onSelect,
    required this.onFilterToggle,
    required this.onSortChange,
    required this.onFiltersClear,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = MemoTheme.of(context);

    // Filter chip definitions: key, label
    final capFilters = [
      ('tools', L10n.t('tools')),
      ('vision', L10n.t('vision')),
      ('code', L10n.t('code')),
      ('embedding', L10n.t('embedding_filter')),
    ];
    final sizeFilters = [
      ('1-8b', '1–8B'),
      ('8-14b', '8–14B'),
      ('14b+', '14B+'),
    ];

    String sortLabel(_SortMode m) => switch (m) {
          _SortMode.defaultOrder => L10n.t('default'),
          _SortMode.mostDownloads => L10n.t('most_popular'),
          _SortMode.smallestFirst => L10n.t('smallest'),
          _SortMode.largestFirst => L10n.t('largest'),
        };

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        // ── Search bar ──
        Padding(
          padding: const EdgeInsets.fromLTRB(10, 10, 10, 6),
          child: TextField(
            controller: searchController,
            onChanged: onSearch,
            style: TextStyle(fontSize: 13, color: c.textMain),
            decoration: InputDecoration(
              hintText: L10n.t('search_models_on_huggingface'),
              prefixIcon: Icon(Icons.search, size: 18, color: c.textDim),
              suffixIcon: searchController.text.isNotEmpty
                  ? IconButton(
                      icon: Icon(Icons.clear, size: 16, color: c.textDim),
                      onPressed: () {
                        searchController.clear();
                        onSearch('');
                      },
                      padding: EdgeInsets.zero,
                    )
                  : null,
              contentPadding:
                  const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
              isDense: true,
            ),
          ),
        ),

        // ── Filter chips ──
        SizedBox(
          height: 36,
          child: ListView(
            scrollDirection: Axis.horizontal,
            padding: const EdgeInsets.fromLTRB(10, 0, 10, 4),
            children: [
              // Sort popup
              _SortChip(
                label: sortLabel(sortMode),
                onSelected: (mode) => onSortChange(mode),
              ),
              const SizedBox(width: 6),
              // Separator
              Container(
                width: 1, height: 20,
                margin: const EdgeInsets.symmetric(horizontal: 4, vertical: 6),
                color: c.borderSoft,
              ),
              const SizedBox(width: 2),
              for (final (key, label) in [...capFilters, ...sizeFilters]) ...[
                _FilterChip(
                  label: label,
                  active: activeFilters.contains(key),
                  onTap: () => onFilterToggle(key),
                ),
                const SizedBox(width: 5),
              ],
            ],
          ),
        ),

        // ── Section label + active-filter indicator ──
        Padding(
          padding: const EdgeInsets.fromLTRB(14, 4, 14, 4),
          child: Row(
            children: [
              Expanded(
                child: Text(
                  isCurated && activeFilters.isEmpty
                      ? (L10n.t('featured_models'))
                      : (L10n.t('length_results', {'length': '${items.length}'})),
                  style: TextStyle(
                    fontSize: 11,
                    fontWeight: FontWeight.w600,
                    color: c.textDim,
                    letterSpacing: 0.4,
                  ),
                ),
              ),
              if (activeFilters.isNotEmpty)
                GestureDetector(
                  onTap: onFiltersClear,
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text(
                        L10n.t('filters_active_count', {'count': '${activeFilters.length}'}),
                        style: TextStyle(
                          fontSize: 11,
                          fontWeight: FontWeight.w600,
                          color: MemoTheme.accent,
                        ),
                      ),
                      const SizedBox(width: 4),
                      Icon(Icons.close_rounded, size: 13, color: MemoTheme.accent),
                    ],
                  ),
                ),
            ],
          ),
        ),

        // ── Model list ──
        Expanded(
          child: isLoading
              ? const Center(child: CircularProgressIndicator(strokeWidth: 2))
              : items.isEmpty
                  ? Center(
                      child: Padding(
                        padding: const EdgeInsets.all(24),
                        child: Text(
                          L10n.t('no_search_results'),
                          style: TextStyle(color: c.textDim, fontSize: 13),
                        ),
                      ),
                    )
                  : ListView.builder(
                      itemCount: items.length,
                      itemBuilder: (ctx, i) => _ModelListRow(
                        item: items[i],
                        isSelected: selected?.repoId == items[i].repoId,
                        onTap: () => onSelect(items[i]),
                      ),
                    ),
        ),
      ],
    );
  }
}

// ─── Filter chip ──────────────────────────────────────────────────

class _FilterChip extends StatelessWidget {
  final String label;
  final bool active;
  final VoidCallback onTap;
  const _FilterChip({required this.label, required this.active, required this.onTap});

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    return MouseRegion(
      cursor: SystemMouseCursors.click,
      child: GestureDetector(
        onTap: onTap,
        child: AnimatedContainer(
          duration: const Duration(milliseconds: 150),
          padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
          decoration: BoxDecoration(
            color: active
                ? MemoTheme.accent.withValues(alpha: 0.15)
                : c.bgElement,
            borderRadius: BorderRadius.circular(20),
            border: Border.all(
              color: active
                  ? MemoTheme.accent.withValues(alpha: 0.6)
                  : c.borderSoft,
            ),
          ),
          child: Text(
            label,
            style: TextStyle(
              fontSize: 11,
              fontWeight: FontWeight.w600,
              color: active ? MemoTheme.accent : c.textMuted,
            ),
          ),
        ),
      ),
    );
  }
}

// ─── Sort chip (popup) ────────────────────────────────────────────

class _SortChip extends StatelessWidget {
  final String label;
  final ValueChanged<_SortMode> onSelected;
  const _SortChip({required this.label, required this.onSelected});

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    return MouseRegion(
      cursor: SystemMouseCursors.click,
      child: GestureDetector(
        onTapDown: (details) {
          final pos = details.globalPosition;
          showMenu<_SortMode>(
            context: context,
            position: RelativeRect.fromLTRB(pos.dx, pos.dy, pos.dx + 1, pos.dy + 1),
            items: [
              PopupMenuItem(
                value: _SortMode.defaultOrder,
                child: Text(L10n.t('default')),
              ),
              PopupMenuItem(
                value: _SortMode.mostDownloads,
                child: Text(L10n.t('most_popular')),
              ),
              PopupMenuItem(
                value: _SortMode.smallestFirst,
                child: Text(L10n.t('smallest_first')),
              ),
              PopupMenuItem(
                value: _SortMode.largestFirst,
                child: Text(L10n.t('largest_first')),
              ),
            ],
          ).then((mode) {
            if (mode != null) onSelected(mode);
          });
        },
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
          decoration: BoxDecoration(
            color: c.bgElement,
            borderRadius: BorderRadius.circular(20),
            border: Border.all(color: c.borderSoft),
          ),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(Icons.sort, size: 13, color: c.textMuted),
              const SizedBox(width: 4),
              Text(
                label,
                style: TextStyle(
                    fontSize: 11,
                    fontWeight: FontWeight.w600,
                    color: c.textMuted),
              ),
              const SizedBox(width: 2),
              Icon(Icons.arrow_drop_down, size: 14, color: c.textDim),
            ],
          ),
        ),
      ),
    );
  }
}


// ─── Model list row ───────────────────────────────────────────────

class _ModelListRow extends ConsumerWidget {
  final DiscoverItem item;
  final bool isSelected;
  final VoidCallback onTap;

  const _ModelListRow({
    required this.item,
    required this.isSelected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = MemoTheme.of(context);
    final localModels = ref.watch(localModelsProvider).valueOrNull ?? [];
    final isDownloaded = localModels.any(
      (m) =>
          m.repoId.toLowerCase() == item.repoId.toLowerCase() ||
          m.filename.toLowerCase().contains(
              item.repoId.split('/').lastOrNull?.toLowerCase() ?? ''),
    );

    final timeAgoText = timeAgo(item.lastModified);
    final params = item.paramCount;

    return MouseRegion(
      cursor: SystemMouseCursors.click,
      child: GestureDetector(
        onTap: onTap,
        child: AnimatedContainer(
          duration: const Duration(milliseconds: 150),
          color: isSelected
              ? MemoTheme.accent.withValues(alpha: 0.12)
              : Colors.transparent,
          child: Container(
            decoration: BoxDecoration(
              border: Border(
                left: BorderSide(
                  color: isSelected ? MemoTheme.accent : Colors.transparent,
                  width: 2,
                ),
              ),
            ),
            padding: const EdgeInsets.fromLTRB(14, 10, 12, 10),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                // Avatar
                AuthorAvatar(author: item.avatarAuthor, dio: ref.read(apiClientProvider).dio),
                const SizedBox(width: 10),
                // Content
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          Expanded(
                            child: Text(
                              item.displayName,
                              style: TextStyle(
                                fontSize: 13,
                                fontWeight: FontWeight.w600,
                                color: isSelected
                                    ? MemoTheme.accent
                                    : c.textMain,
                              ),
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                            ),
                          ),
                          if (params != null) ...[
                            const SizedBox(width: 6),
                            _SmallBadge(text: params, color: c.textDim),
                          ],
                        ],
                      ),
                      if (item.description.isNotEmpty) ...[
                        const SizedBox(height: 2),
                        Text(
                          item.description,
                          style: TextStyle(fontSize: 11, color: c.textMuted),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ],
                      const SizedBox(height: 4),
                      Row(
                        children: [
                          // Capability icons
                          if (item.supportsTools)
                            _CapIcon(
                              icon: Icons.build_outlined,
                              color: MemoTheme.accent,
                              tooltip: 'Tool Use',
                            ),
                          if (item.supportsVision) ...[
                            if (item.supportsTools)
                              const SizedBox(width: 4),
                            _CapIcon(
                              icon: Icons.visibility_outlined,
                              color: const Color(0xFF50C878),
                              tooltip: 'Vision',
                            ),
                          ],
                          if (item.supportsCode) ...[
                            if (item.supportsTools || item.supportsVision)
                              const SizedBox(width: 4),
                            _CapIcon(
                              icon: Icons.code,
                              color: const Color(0xFF7C6FEE),
                              tooltip: 'Code',
                            ),
                          ],
                          if (item.isEmbedding) ...[
                            _CapIcon(
                              icon: Icons.hub_outlined,
                              color: const Color(0xFF26C6DA),
                              tooltip: 'Embedding',
                            ),
                          ],
                          const Spacer(),
                          if (isDownloaded) ...[
                            Icon(Icons.check_circle,
                                size: 12, color: MemoTheme.green),
                            const SizedBox(width: 4),
                          ],
                          if (item.downloads > 0)
                            Text(
                              '↓ ${fmtCount(item.downloads)}',
                              style:
                                  TextStyle(fontSize: 10, color: c.textDim),
                            )
                          else if (timeAgoText.isNotEmpty)
                            Text(
                              timeAgoText,
                              style:
                                  TextStyle(fontSize: 10, color: c.textDim),
                            ),
                        ],
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _CapIcon extends StatelessWidget {
  final IconData icon;
  final Color color;
  final String tooltip;
  const _CapIcon(
      {required this.icon, required this.color, required this.tooltip});

  @override
  Widget build(BuildContext context) {
    return Tooltip(
      message: tooltip,
      child: Container(
        width: 20,
        height: 20,
        decoration: BoxDecoration(
          color: color.withValues(alpha: 0.12),
          borderRadius: BorderRadius.circular(4),
        ),
        child: Icon(icon, size: 12, color: color),
      ),
    );
  }
}

class _SmallBadge extends StatelessWidget {
  final String text;
  final Color color;
  const _SmallBadge({required this.text, required this.color});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 5, vertical: 2),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(4),
        border: Border.all(color: color.withValues(alpha: 0.3)),
      ),
      child: Text(
        text,
        style: TextStyle(fontSize: 10, fontWeight: FontWeight.w600, color: color),
      ),
    );
  }
}

// ─── Empty detail state ───────────────────────────────────────────

class _EmptyDetailState extends StatelessWidget {
  const _EmptyDetailState();

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.touch_app_outlined, size: 40, color: c.textDim),
          const SizedBox(height: 12),
          Text(
            L10n.t('select_a_model_to_see_details'),
            style: TextStyle(fontSize: 14, color: c.textDim),
          ),
        ],
      ),
    );
  }
}
