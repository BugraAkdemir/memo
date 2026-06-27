import 'dart:async';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:file_picker/file_picker.dart';
import 'package:path/path.dart' as p;

import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/curated_models.dart';
import '../models/gpu_info.dart';
import '../models/local_model.dart';
import '../providers/chat_provider.dart';
import '../providers/models_provider.dart';
import '../widgets/model_config_dialog.dart';

// ─── Unified discover item ────────────────────────────────────────

@immutable
class _DiscoverItem {
  final String repoId;
  final String author;
  /// Override author used for avatar/logo fetch (e.g. the original model creator
  /// when the uploader is a community quantizer).
  final String? brandAuthor;
  final String displayName;
  final String description;
  final bool supportsTools;
  final bool supportsVision;
  final bool supportsCode;
  final bool isEmbedding;
  final int downloads;
  final int likes;
  final String? lastModified;
  final List<String> tags;
  final int? approxBytes;
  final bool isCurated;

  const _DiscoverItem({
    required this.repoId,
    required this.author,
    this.brandAuthor,
    required this.displayName,
    required this.description,
    required this.supportsTools,
    required this.supportsVision,
    required this.supportsCode,
    required this.isEmbedding,
    required this.downloads,
    required this.likes,
    this.lastModified,
    required this.tags,
    this.approxBytes,
    required this.isCurated,
  });

  /// The author slug used to look up the avatar on HuggingFace.
  String get avatarAuthor => brandAuthor ?? author;

  factory _DiscoverItem.fromCurated(CuratedModel m) => _DiscoverItem(
        repoId: m.repoId,
        author: m.repoId.contains('/') ? m.repoId.split('/').first : m.repoId,
        brandAuthor: m.brandAuthor,
        displayName: m.name,
        description: m.desc,
        supportsTools: m.supportsTools,
        supportsVision: m.kind == ModelKind.vision,
        supportsCode: false,
        isEmbedding: m.kind == ModelKind.memory,
        downloads: 0,
        likes: 0,
        tags: const [],
        approxBytes: m.approxBytes,
        isCurated: true,
      );

  factory _DiscoverItem.fromHF(HFModelResult r) {
    final lowerTags = r.tags.map((t) => t.toLowerCase()).toSet();
    final isEmbed = lowerTags.intersection({
      'feature-extraction',
      'sentence-similarity',
      'sentence-transformers',
      'text-embeddings-inference',
    }).isNotEmpty;
    return _DiscoverItem(
      repoId: r.id,
      author: r.author,
      displayName: _humanizeName(r.id),
      description: '',
      supportsTools: r.supportsTools,
      supportsVision: r.supportsVision,
      supportsCode: r.supportsCode,
      isEmbedding: isEmbed,
      downloads: r.downloads,
      likes: r.likes,
      lastModified: r.lastModified,
      tags: r.tags,
      isCurated: false,
    );
  }

  String? get paramCount => _extractParams(displayName.isNotEmpty ? displayName : repoId);
  String? get arch => _detectArch(tags);

  /// True if this model likely supports tool/function calling.
  /// HF search tags rarely include 'function-calling' on GGUF repos, so we
  /// fall back to checking the model name against known tool-capable families.
  bool get likelySupportsTools {
    if (supportsTools) return true;
    if (isEmbedding) return false;
    final lower = '${displayName.toLowerCase()} ${repoId.toLowerCase()}';
    const families = [
      'llama-3', 'llama3', 'llama 3',
      'qwen2', 'qwen 2', 'qwen2.5', 'qwen3', 'qwen 3',
      'mistral', 'mixtral',
      'hermes', 'functionary', 'nexusraven', 'gorilla',
      'phi-3', 'phi-4', 'phi3', 'phi4', 'phi 3', 'phi 4',
      'gemma-2', 'gemma2', 'gemma 2', 'gemma-3', 'gemma3', 'gemma 3',
      'command-r', 'deepseek', 'internlm',
      'smollm', 'dolphin', 'openhermes',
    ];
    final hasFamily = families.any((f) => lower.contains(f));
    final isInstruct =
        lower.contains('instruct') || lower.contains('chat') || lower.contains('-it');
    return hasFamily && isInstruct;
  }

  /// True if this model likely supports vision/image input.
  bool get likelySupportsVision {
    if (supportsVision) return true;
    final lower = '${displayName.toLowerCase()} ${repoId.toLowerCase()}';
    const families = [
      'llava', 'bakllava', 'vision', 'moondream', 'minicpm-v',
      'internvl', 'cogvlm', 'qwen-vl', 'qwenvl', 'idefics',
      'pixtral', 'paligemma', 'florence',
    ];
    return families.any((f) => lower.contains(f));
  }

  /// True if this model is primarily a code model.
  bool get likelySupportsCode {
    if (supportsCode) return true;
    final lower = '${displayName.toLowerCase()} ${repoId.toLowerCase()}';
    const families = [
      'codellama', 'codegemma', 'deepseek-coder', 'starcoder',
      'wizardcoder', 'phind-codellama', 'codestral', 'qwen2.5-coder',
      'granite-code', 'opencoder',
    ];
    return families.any((f) => lower.contains(f));
  }
}

// ─── Helpers ─────────────────────────────────────────────────────

String _humanizeName(String repoId) {
  final name = repoId.contains('/') ? repoId.split('/').last : repoId;
  return name
      .replaceAll(RegExp(r'[-_]gguf', caseSensitive: false), '')
      .replaceAll(RegExp(r'[-_]'), ' ')
      .trim();
}

String? _extractParams(String text) {
  final m = RegExp(r'(\d+(?:\.\d+)?)\s*[Bb](?:\b|$)', caseSensitive: false)
      .firstMatch(text);
  if (m == null) return null;
  final val = double.tryParse(m.group(1)!);
  if (val == null || val > 2000 || val < 0.1) return null;
  if (val == val.roundToDouble()) return '${val.round()}B';
  return '${val}B';
}

String? _detectArch(List<String> tags) {
  const knownArchs = {
    'gemma', 'gemma2', 'gemma3', 'llama', 'mistral', 'qwen2', 'qwen3',
    'phi', 'phi3', 'phi4', 'falcon', 'gpt2', 'mpt', 'bloom', 'internlm2',
    'deepseek', 'deepseek2', 'cohere',
  };
  for (final t in tags) {
    if (knownArchs.contains(t.toLowerCase())) return t.toLowerCase();
  }
  return null;
}

String _timeAgo(String? iso) {
  if (iso == null || iso.isEmpty) return '';
  final dt = DateTime.tryParse(iso);
  if (dt == null) return '';
  final diff = DateTime.now().difference(dt);
  if (diff.inDays > 365) return '${(diff.inDays / 365).round()}y ago';
  if (diff.inDays > 30) return '${(diff.inDays / 30).round()}mo ago';
  if (diff.inDays > 0) return '${diff.inDays}d ago';
  if (diff.inHours > 0) return '${diff.inHours}h ago';
  return 'just now';
}

String _fmtCount(int n) {
  if (n >= 1000000) return '${(n / 1000000).toStringAsFixed(1)}M';
  if (n >= 1000) return '${(n / 1000).toStringAsFixed(0)}K';
  return '$n';
}


Color _authorColor(String author) {
  const colors = [
    Color(0xFF4B7BEC),
    Color(0xFF7C6FEE),
    Color(0xFF26C6DA),
    Color(0xFF50C878),
    Color(0xFFFF7043),
    Color(0xFFE91E8C),
    Color(0xFFF9CA24),
  ];
  if (author.isEmpty) return colors[0];
  final idx = author.codeUnits.fold(0, (s, c) => s + c) % colors.length;
  return colors[idx];
}

// ─── Root screen ─────────────────────────────────────────────────

class ModelStoreScreen extends ConsumerStatefulWidget {
  const ModelStoreScreen({super.key});

  @override
  ConsumerState<ModelStoreScreen> createState() => _ModelStoreScreenState();
}

class _ModelStoreScreenState extends ConsumerState<ModelStoreScreen> {
  int _tab = 0;

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    return Container(
      color: c.bgApp,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          _Header(tab: _tab, onTab: (t) => setState(() => _tab = t)),
          const _DownloadBanner(),
          Expanded(
            child: IndexedStack(
              index: _tab,
              children: const [_DiscoverTab(), _MyModelsTab()],
            ),
          ),
        ],
      ),
    );
  }
}

// ─── Header ──────────────────────────────────────────────────────

class _Header extends ConsumerWidget {
  final int tab;
  final ValueChanged<int> onTab;
  const _Header({required this.tab, required this.onTab});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = MemoTheme.of(context);
    return Container(
      padding: const EdgeInsets.fromLTRB(24, 16, 24, 0),
      decoration: BoxDecoration(
        border: Border(bottom: BorderSide(color: c.borderSoft)),
      ),
      child: Row(
        children: [
          Text(
            L10n.t('nav_models'),
            style: Theme.of(context).textTheme.titleLarge?.copyWith(
                  fontWeight: FontWeight.w700,
                  color: c.textMain,
                ),
          ),
          const SizedBox(width: 24),
          _TabItem(
            label: L10n.t('tab_discover'),
            active: tab == 0,
            onTap: () => onTab(0),
          ),
          const SizedBox(width: 4),
          _TabItem(
            label: L10n.t('tab_my_models'),
            active: tab == 1,
            onTap: () => onTab(1),
          ),
          const Spacer(),
          const Flexible(child: _HardwareChip()),
        ],
      ),
    );
  }
}

class _TabItem extends StatelessWidget {
  final String label;
  final bool active;
  final VoidCallback onTap;
  const _TabItem({required this.label, required this.active, required this.onTap});

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    return MouseRegion(
      cursor: SystemMouseCursors.click,
      child: GestureDetector(
        onTap: onTap,
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
          decoration: BoxDecoration(
            border: Border(
              bottom: BorderSide(
                color: active ? MemoTheme.accent : Colors.transparent,
                width: 2,
              ),
            ),
          ),
          child: Text(
            label,
            style: TextStyle(
              fontSize: 13,
              fontWeight: active ? FontWeight.w600 : FontWeight.w500,
              color: active ? MemoTheme.accent : c.textMuted,
            ),
          ),
        ),
      ),
    );
  }
}

class _HardwareChip extends ConsumerWidget {
  const _HardwareChip();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = MemoTheme.of(context);
    final gpu = ref.watch(gpuInfoProvider).valueOrNull ?? const GPUInfo();
    final parts = <String>[];
    if (gpu.ramTotalMb > 0) parts.add('${gpu.ramFormatted} RAM');
    if (gpu.hasGpu) {
      final name = _shortGpuName(gpu.name);
      parts.add(gpu.vramMb > 0 ? '$name ${gpu.vramFormatted}' : name);
    } else {
      parts.add('CPU');
    }
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
      decoration: BoxDecoration(
        color: c.bgElement,
        borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
        border: Border.all(color: c.borderSoft),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(gpu.hasGpu ? Icons.memory : Icons.developer_board,
              size: 13, color: MemoTheme.accent),
          const SizedBox(width: 6),
          Flexible(
            child: Text(
              parts.join(' · '),
              maxLines: 1,
              softWrap: false,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(fontSize: 11, color: c.textMuted),
            ),
          ),
        ],
      ),
    );
  }
}

/// Trims verbose lspci-style GPU descriptions to a readable short name.
/// e.g. "VGA compatible controller: Advanced Micro Devices, Inc. [AMD/ATI]
/// Cezanne [Radeon Vega Series / Radeon Vega Mobile Gfx]" → "Radeon Vega
/// Series / Radeon Vega Mobile Gfx". Clean names (NVIDIA/Windows) pass through.
String _shortGpuName(String name) {
  final bracket = RegExp(r'\[([^\[\]]+)\]\s*$').firstMatch(name);
  if (bracket != null) return bracket.group(1)!.trim();
  return name.replaceFirst(RegExp(r'^.*controller:\s*'), '').trim();
}

// ─── Sort mode ────────────────────────────────────────────────────

enum _SortMode { defaultOrder, mostDownloads, smallestFirst, largestFirst }

// ─── Discover tab — two-panel layout ─────────────────────────────

class _DiscoverTab extends ConsumerStatefulWidget {
  const _DiscoverTab();

  @override
  ConsumerState<_DiscoverTab> createState() => _DiscoverTabState();
}

class _DiscoverTabState extends ConsumerState<_DiscoverTab> {
  _DiscoverItem? _selected;
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
      if (mounted) {
        ref.read(modelSearchQueryProvider.notifier).state = val.trim();
      }
    });
  }

  /// Extract numeric param count for size filtering.
  double? _paramValue(_DiscoverItem item) {
    final text = item.paramCount;
    if (text == null) return null;
    final m = RegExp(r'(\d+(?:\.\d+)?)B').firstMatch(text);
    if (m == null) return null;
    return double.tryParse(m.group(1)!);
  }

  List<_DiscoverItem> _applyFiltersSort(List<_DiscoverItem> raw) {
    var list = raw.toList();

    // Capability filters (OR within same type)
    if (_filters.contains('tools')) {
      list = list.where((i) => i.likelySupportsTools).toList();
    }
    if (_filters.contains('vision')) {
      list = list.where((i) => i.likelySupportsVision).toList();
    }
    if (_filters.contains('code')) {
      list = list.where((i) => i.likelySupportsCode).toList();
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

    final List<_DiscoverItem> rawItems;
    final bool isLoading;
    if (query.isEmpty) {
      rawItems = curatedModels.map(_DiscoverItem.fromCurated).toList();
      isLoading = false;
    } else {
      rawItems =
          resultsAsync.valueOrNull?.map(_DiscoverItem.fromHF).toList() ?? [];
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
          ),
        ),
        VerticalDivider(width: 1, thickness: 1, color: c.borderSoft),
        Expanded(
          child: _selected == null
              ? const _EmptyDetailState()
              : _ModelDetailPanel(
                  key: ValueKey(_selected!.repoId),
                  item: _selected!,
                  onSelectOther: (repoId, displayName) => setState(() {
                    _selected = _DiscoverItem(
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
  final List<_DiscoverItem> items;
  final _DiscoverItem? selected;
  final bool isCurated;
  final bool isLoading;
  final TextEditingController searchController;
  final Set<String> activeFilters;
  final _SortMode sortMode;
  final ValueChanged<String> onSearch;
  final ValueChanged<_DiscoverItem> onSelect;
  final ValueChanged<String> onFilterToggle;
  final ValueChanged<_SortMode> onSortChange;

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
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = MemoTheme.of(context);
    final tr = L10n.locale == MemoLocale.tr;

    // Filter chip definitions: key, label
    final capFilters = [
      ('tools', tr ? '🔧 Araç' : '🔧 Tools'),
      ('vision', tr ? '👁 Vision' : '👁 Vision'),
      ('code', tr ? '💻 Kod' : '💻 Code'),
    ];
    final sizeFilters = [
      ('1-8b', '1–8B'),
      ('8-14b', '8–14B'),
      ('14b+', '14B+'),
    ];

    String sortLabel(_SortMode m) => switch (m) {
          _SortMode.defaultOrder => tr ? 'Varsayılan' : 'Default',
          _SortMode.mostDownloads => tr ? 'En popüler' : 'Most popular',
          _SortMode.smallestFirst => tr ? 'En küçük' : 'Smallest',
          _SortMode.largestFirst => tr ? 'En büyük' : 'Largest',
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
              hintText: tr
                  ? 'HuggingFace\'te model ara...'
                  : 'Search models on HuggingFace...',
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

        // ── Section label ──
        Padding(
          padding: const EdgeInsets.fromLTRB(14, 4, 14, 4),
          child: Text(
            isCurated && activeFilters.isEmpty
                ? (tr ? 'Önerilen modeller' : 'Featured models')
                : (tr ? '${items.length} sonuç' : '${items.length} results'),
            style: TextStyle(
              fontSize: 11,
              fontWeight: FontWeight.w600,
              color: c.textDim,
              letterSpacing: 0.4,
            ),
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
                          tr ? 'Sonuç bulunamadı' : 'No results found',
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
    final tr = L10n.locale == MemoLocale.tr;
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
                child: Text(tr ? 'Varsayılan' : 'Default'),
              ),
              PopupMenuItem(
                value: _SortMode.mostDownloads,
                child: Text(tr ? 'En popüler' : 'Most popular'),
              ),
              PopupMenuItem(
                value: _SortMode.smallestFirst,
                child: Text(tr ? 'En küçükten büyüğe' : 'Smallest first'),
              ),
              PopupMenuItem(
                value: _SortMode.largestFirst,
                child: Text(tr ? 'En büyükten küçüğe' : 'Largest first'),
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

// ─── Author avatar (HF logo with letter fallback) ────────────────

class _AuthorAvatar extends StatefulWidget {
  final String author;
  final Dio dio;
  const _AuthorAvatar({required this.author, required this.dio});

  @override
  State<_AuthorAvatar> createState() => _AuthorAvatarState();
}

class _AuthorAvatarState extends State<_AuthorAvatar> {
  // Shared across all instances — one fetch per unique author per session.
  static final _cache = <String, String?>{};
  String? _avatarUrl;
  bool _resolved = false;

  @override
  void initState() {
    super.initState();
    _resolve();
  }

  @override
  void didUpdateWidget(_AuthorAvatar old) {
    super.didUpdateWidget(old);
    if (old.author != widget.author) _resolve();
  }

  Future<void> _resolve() async {
    final a = widget.author;
    if (_cache.containsKey(a)) {
      if (mounted) setState(() { _avatarUrl = _cache[a]; _resolved = true; });
      return;
    }
    String? url;
    for (final endpoint in [
      'https://huggingface.co/api/organizations/$a',
      'https://huggingface.co/api/users/$a',
    ]) {
      try {
        final r = await widget.dio.get<Map<String, dynamic>>(
          endpoint,
          options: Options(
            receiveTimeout: const Duration(seconds: 6),
            sendTimeout: const Duration(seconds: 6),
          ),
        );
        final u = r.data?['avatarUrl'] as String?;
        if (u != null && u.isNotEmpty) { url = u; break; }
      } catch (_) {}
    }
    _cache[a] = url;
    if (mounted) setState(() { _avatarUrl = url; _resolved = true; });
  }

  @override
  Widget build(BuildContext context) {
    if (_resolved && _avatarUrl != null) {
      return ClipRRect(
        borderRadius: BorderRadius.circular(8),
        child: Image.network(
          _avatarUrl!,
          width: 34,
          height: 34,
          fit: BoxFit.cover,
          errorBuilder: (context, error, stackTrace) => _LetterAvatar(author: widget.author),
        ),
      );
    }
    return _LetterAvatar(author: widget.author);
  }
}

class _LetterAvatar extends StatelessWidget {
  final String author;
  const _LetterAvatar({required this.author});

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 34,
      height: 34,
      decoration: BoxDecoration(
        color: _authorColor(author),
        borderRadius: BorderRadius.circular(8),
      ),
      alignment: Alignment.center,
      child: Text(
        author.isNotEmpty ? author[0].toUpperCase() : '?',
        style: const TextStyle(
          color: Colors.white,
          fontSize: 14,
          fontWeight: FontWeight.w700,
        ),
      ),
    );
  }
}

// ─── Model list row ───────────────────────────────────────────────

class _ModelListRow extends ConsumerWidget {
  final _DiscoverItem item;
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

    final timeAgo = _timeAgo(item.lastModified);
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
                _AuthorAvatar(author: item.avatarAuthor, dio: ref.read(apiClientProvider).dio),
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
                          if (item.likelySupportsTools)
                            _CapIcon(
                              icon: Icons.build_outlined,
                              color: MemoTheme.accent,
                              tooltip: 'Tool Use',
                            ),
                          if (item.likelySupportsVision) ...[
                            if (item.likelySupportsTools)
                              const SizedBox(width: 4),
                            _CapIcon(
                              icon: Icons.visibility_outlined,
                              color: const Color(0xFF50C878),
                              tooltip: 'Vision',
                            ),
                          ],
                          if (item.likelySupportsCode) ...[
                            if (item.likelySupportsTools || item.likelySupportsVision)
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
                              '↓ ${_fmtCount(item.downloads)}',
                              style:
                                  TextStyle(fontSize: 10, color: c.textDim),
                            )
                          else if (timeAgo.isNotEmpty)
                            Text(
                              timeAgo,
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
            L10n.locale == MemoLocale.tr
                ? 'Detayları görmek için bir model seç'
                : 'Select a model to see details',
            style: TextStyle(fontSize: 14, color: c.textDim),
          ),
        ],
      ),
    );
  }
}

// ─── Right panel: model detail ────────────────────────────────────

class _ModelDetailPanel extends ConsumerStatefulWidget {
  final _DiscoverItem item;
  final void Function(String repoId, String displayName)? onSelectOther;

  const _ModelDetailPanel({
    super.key,
    required this.item,
    this.onSelectOther,
  });

  @override
  ConsumerState<_ModelDetailPanel> createState() => _ModelDetailPanelState();
}

class _ModelDetailPanelState extends ConsumerState<_ModelDetailPanel> {
  // Files / selection
  List<GGUFFile>? _files;
  String? _filesError;
  GGUFFile? _selectedFile;

  // Dropdown state
  bool _dropdownOpen = false;

  // README
  String? _readme;
  bool _readmeLoading = false;

  // More from author
  List<Map<String, dynamic>>? _moreModels;

  @override
  void initState() {
    super.initState();
    _loadFiles();
    _loadReadme();
    _loadMoreModels();
  }

  Future<void> _loadFiles() async {
    try {
      final files =
          await ref.read(apiClientProvider).getModelFiles(widget.item.repoId);
      files.sort((a, b) =>
          quantInfo(b.filename).priority.compareTo(quantInfo(a.filename).priority));
      if (mounted) {
        setState(() {
          _files = files;
          _selectedFile = pickRecommendedFile(files);
        });
      }
    } catch (e) {
      if (mounted) setState(() => _filesError = e.toString());
    }
  }

  Future<void> _loadReadme() async {
    if (mounted) setState(() => _readmeLoading = true);
    try {
      final url =
          'https://huggingface.co/${widget.item.repoId}/raw/main/README.md';
      final resp = await ref.read(apiClientProvider).dio.get<String>(
        url,
        options: Options(responseType: ResponseType.plain),
      );
      if (mounted) {
        setState(() {
          _readme = _stripFrontmatter(resp.data ?? '');
          _readmeLoading = false;
        });
      }
    } catch (_) {
      if (mounted) setState(() => _readmeLoading = false);
    }
  }

  String _stripFrontmatter(String content) {
    if (!content.startsWith('---\n')) return content;
    final end = content.indexOf('\n---\n', 4);
    if (end == -1) return content;
    return content.substring(end + 5);
  }

  Future<void> _loadMoreModels() async {
    try {
      final author = widget.item.author;
      if (author.isEmpty) return;
      final url =
          'https://huggingface.co/api/models?author=$author&filter=gguf&sort=downloads&direction=-1&limit=6';
      final resp = await ref.read(apiClientProvider).dio.get<dynamic>(url);
      final data = resp.data;
      if (data is List && mounted) {
        final results = data
            .whereType<Map<String, dynamic>>()
            .where((m) => (m['id'] as String? ?? '') != widget.item.repoId)
            .take(5)
            .map((m) => {
                  'id': m['id'] as String? ?? '',
                  'downloads': m['downloads'] as int? ?? 0,
                })
            .toList();
        setState(() => _moreModels = results);
      }
    } catch (_) {
      // silently ignore
    }
  }

  Future<void> _download() async {
    final file = _selectedFile;
    if (file == null) return;
    final messenger = ScaffoldMessenger.of(context);
    try {
      await ref
          .read(apiClientProvider)
          .downloadModel(widget.item.repoId, file.filename);
      if (mounted) {
        messenger.showSnackBar(SnackBar(
          content: Text(L10n.t('download_started', {'file': file.filename})),
          backgroundColor: MemoTheme.green,
        ));
      }
    } catch (e) {
      if (mounted) {
        messenger.showSnackBar(SnackBar(
            content: Text(L10n.t('import_error', {'e': e.toString()}))));
      }
    }
  }

  /// Extracts a short quant code from the filename (e.g. "Q4_K_M").
  String _quantCode(String filename) {
    final m = RegExp(r'[Qq]\d[^.]*').firstMatch(filename);
    if (m != null) return m.group(0)!.toUpperCase();
    if (filename.toLowerCase().contains('fp16') ||
        filename.toLowerCase().contains('f16')) {
      return 'FP16';
    }
    return 'GGUF';
  }

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final gpu = ref.watch(gpuInfoProvider).valueOrNull ?? const GPUInfo();
    final localModels = ref.watch(localModelsProvider).valueOrNull ?? [];
    final downloadingNow =
        ref.watch(downloadProgressProvider).valueOrNull?.active ?? false;
    final item = widget.item;

    return SingleChildScrollView(
      padding: const EdgeInsets.fromLTRB(28, 24, 28, 32),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // ── Header: Repo ID + copy ──
          Row(
            children: [
              Expanded(
                child: Text(
                  item.repoId,
                  style: TextStyle(
                    fontSize: 15,
                    fontWeight: FontWeight.w700,
                    color: c.textMain,
                  ),
                ),
              ),
              IconButton(
                icon: Icon(Icons.copy_outlined, size: 15, color: c.textDim),
                tooltip: 'Copy repo ID',
                onPressed: () =>
                    Clipboard.setData(ClipboardData(text: item.repoId)),
                padding: EdgeInsets.zero,
                constraints: const BoxConstraints(),
              ),
            ],
          ),
          const SizedBox(height: 6),

          // ── Stats row ──
          Wrap(
            spacing: 12,
            children: [
              if (item.downloads > 0)
                _StatChip(
                    icon: Icons.download_rounded,
                    text: _fmtCount(item.downloads)),
              if (item.likes > 0)
                _StatChip(
                    icon: Icons.star_border_rounded, text: '${item.likes}'),
              if (_timeAgo(item.lastModified).isNotEmpty)
                _StatChip(
                    icon: Icons.schedule_outlined,
                    text: _timeAgo(item.lastModified)),
            ],
          ),

          // ── Description ──
          if (item.description.isNotEmpty) ...[
            const SizedBox(height: 14),
            Container(
              padding: const EdgeInsets.all(14),
              decoration: BoxDecoration(
                color: c.bgPanel,
                borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                border: Border.all(color: c.borderSoft),
              ),
              child: Text(
                item.description,
                style:
                    TextStyle(fontSize: 13, color: c.textMain, height: 1.5),
              ),
            ),
          ],

          const SizedBox(height: 14),

          // ── Metadata tags ──
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              if (item.paramCount != null)
                _InfoTag(
                    label:
                        L10n.locale == MemoLocale.tr ? 'Parametre' : 'Params',
                    value: item.paramCount!),
              if (item.arch != null)
                _InfoTag(
                    label: L10n.locale == MemoLocale.tr ? 'Mimari' : 'Arch',
                    value: item.arch!),
              _InfoTag(
                  label: L10n.locale == MemoLocale.tr ? 'Format' : 'Format',
                  value: 'GGUF',
                  accent: true),
              if (item.isEmbedding)
                _InfoTag(
                    label: L10n.locale == MemoLocale.tr ? 'Tür' : 'Domain',
                    value: 'Embedding'),
            ],
          ),

          // ── Capabilities ──
          if (item.likelySupportsTools ||
              item.likelySupportsVision ||
              item.likelySupportsCode) ...[
            const SizedBox(height: 12),
            Row(
              children: [
                Text(
                  L10n.locale == MemoLocale.tr
                      ? 'Yetenekler: '
                      : 'Capabilities: ',
                  style: TextStyle(
                      fontSize: 12,
                      fontWeight: FontWeight.w600,
                      color: c.textMuted),
                ),
                const SizedBox(width: 4),
                Flexible(
                  child: Wrap(
                    spacing: 8,
                    runSpacing: 6,
                    children: [
                      if (item.likelySupportsVision)
                        _CapabilityPill(
                          icon: Icons.visibility_outlined,
                          label: L10n.locale == MemoLocale.tr
                              ? 'Görüntü'
                              : 'Vision',
                          color: const Color(0xFF50C878),
                        ),
                      if (item.likelySupportsTools)
                        _CapabilityPill(
                          icon: Icons.build_outlined,
                          label: L10n.locale == MemoLocale.tr
                              ? 'Araç'
                              : 'Tool Use',
                          color: MemoTheme.accent,
                        ),
                      if (item.likelySupportsCode)
                        _CapabilityPill(
                          icon: Icons.code,
                          label:
                              L10n.locale == MemoLocale.tr ? 'Kod' : 'Code',
                          color: const Color(0xFF7C6FEE),
                        ),
                    ],
                  ),
                ),
              ],
            ),
          ],

          const SizedBox(height: 20),

          // ── Download Options section header ──
          Text(
            L10n.locale == MemoLocale.tr
                ? 'İndirme Seçenekleri'
                : 'Download Options',
            style: TextStyle(
                fontSize: 12,
                fontWeight: FontWeight.w700,
                color: c.textMuted,
                letterSpacing: 0.4),
          ),
          const SizedBox(height: 10),

          // ── Download Options body ──
          if (_filesError != null)
            Text('${L10n.t('error')}: $_filesError',
                style: const TextStyle(color: MemoTheme.red, fontSize: 13))
          else if (_files == null)
            const Center(
              child: Padding(
                padding: EdgeInsets.symmetric(vertical: 12),
                child: CircularProgressIndicator(strokeWidth: 2),
              ),
            )
          else if (_files!.isEmpty)
            Text(L10n.t('no_files_found'),
                style: TextStyle(color: c.textDim, fontSize: 13))
          else ...[
            // ── Collapsible dropdown ──
            _buildDropdown(c, gpu, localModels),
            const SizedBox(height: 10),

            // ── Download button + hardware fit chip ──
            if (_selectedFile != null)
              _buildDownloadRow(c, gpu, localModels, downloadingNow),
          ],

          // ── README section ──
          if (_readmeLoading || (_readme != null && _readme!.isNotEmpty)) ...[
            const SizedBox(height: 28),
            Row(
              children: [
                Text(
                  'README',
                  style: TextStyle(
                      fontSize: 12,
                      fontWeight: FontWeight.w700,
                      color: c.textMuted,
                      letterSpacing: 0.4),
                ),
                const SizedBox(width: 8),
                if (_readmeLoading)
                  const SizedBox(
                    width: 12,
                    height: 12,
                    child: CircularProgressIndicator(strokeWidth: 1.5),
                  ),
              ],
            ),
            if (!_readmeLoading && _readme != null && _readme!.isNotEmpty) ...[
              const SizedBox(height: 10),
              Container(
                padding: const EdgeInsets.all(16),
                decoration: BoxDecoration(
                  color: c.bgPanel,
                  borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                  border: Border.all(color: c.borderSoft),
                ),
                child: MarkdownBody(
                  data: _readme!,
                  shrinkWrap: true,
                  styleSheet: MarkdownStyleSheet.fromTheme(
                    Theme.of(context),
                  ).copyWith(
                    p: TextStyle(
                        fontSize: 13, color: c.textMain, height: 1.6),
                    code: TextStyle(
                        fontSize: 12,
                        color: MemoTheme.accent,
                        backgroundColor:
                            MemoTheme.accent.withValues(alpha: 0.08)),
                    h1: TextStyle(
                        fontSize: 17,
                        fontWeight: FontWeight.w700,
                        color: c.textMain),
                    h2: TextStyle(
                        fontSize: 15,
                        fontWeight: FontWeight.w700,
                        color: c.textMain),
                    h3: TextStyle(
                        fontSize: 13,
                        fontWeight: FontWeight.w700,
                        color: c.textMain),
                  ),
                ),
              ),
            ],
          ],

          // ── More from [author] ──
          if (_moreModels != null && _moreModels!.isNotEmpty) ...[
            const SizedBox(height: 28),
            Text(
              L10n.locale == MemoLocale.tr
                  ? '${widget.item.author} tarafından diğerleri'
                  : 'More from ${widget.item.author}',
              style: TextStyle(
                  fontSize: 12,
                  fontWeight: FontWeight.w700,
                  color: c.textMuted,
                  letterSpacing: 0.4),
            ),
            const SizedBox(height: 8),
            Container(
              decoration: BoxDecoration(
                color: c.bgPanel,
                borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                border: Border.all(color: c.borderSoft),
              ),
              child: Column(
                children: [
                  for (int i = 0; i < _moreModels!.length; i++) ...[
                    if (i > 0)
                      Divider(height: 1, thickness: 1, color: c.borderSoft),
                    _MoreModelRow(
                      id: _moreModels![i]['id'] as String? ?? '',
                      downloads: (_moreModels![i]['downloads'] as int?) ?? 0,
                      onTap: () {
                        final id = _moreModels![i]['id'] as String?;
                        widget.onSelectOther
                            ?.call(id, _humanizeName(id));
                      },
                    ),
                  ],
                ],
              ),
            ),
          ],
        ],
      ),
    );
  }

  Widget _buildDropdown(
      ThemeColors c, GPUInfo gpu, List<LocalModel> localModels) {
    final files = _files!;
    final selected = _selectedFile;

    // Hardware fit badge widget
    Widget fitBadge(GGUFFile file) {
      final fit = hardwareFit(file.size, gpu);
      final (color, icon, label) = switch (fit.level) {
        FitLevel.good => (
            MemoTheme.green,
            Icons.check_circle_outline,
            L10n.locale == MemoLocale.tr ? 'Uygun' : 'Fits',
          ),
        FitLevel.ok => (
            MemoTheme.accent,
            Icons.check_circle_outline,
            L10n.locale == MemoLocale.tr ? 'CPU\'da çalışır' : 'CPU OK',
          ),
        FitLevel.warn => (
            MemoTheme.warningOrange,
            Icons.warning_amber_rounded,
            L10n.locale == MemoLocale.tr ? '× Çok büyük' : '× Too large',
          ),
      };
      return Container(
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
        decoration: BoxDecoration(
          color: color.withValues(alpha: 0.12),
          borderRadius: BorderRadius.circular(5),
          border: Border.all(color: color.withValues(alpha: 0.35)),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(icon, size: 11, color: color),
            const SizedBox(width: 4),
            Text(label,
                style: TextStyle(
                    fontSize: 11, fontWeight: FontWeight.w600, color: color)),
          ],
        ),
      );
    }

    // Collapsed header row
    Widget headerRow(GGUFFile? file) {
      if (file == null) {
        return Text(
          L10n.locale == MemoLocale.tr ? 'Dosya seç...' : 'Select a file...',
          style: TextStyle(fontSize: 13, color: c.textDim),
        );
      }
      final q = quantInfo(file.filename);
      final qCode = _quantCode(file.filename);
      return Row(
        children: [
          _GgufBadge(),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              q.label, // user-friendly: "Dengeli kalite", "Yüksek kalite"...
              style: TextStyle(
                  fontSize: 13, fontWeight: FontWeight.w500, color: c.textMain),
              overflow: TextOverflow.ellipsis,
              maxLines: 1,
            ),
          ),
          const SizedBox(width: 6),
          Text(qCode,
              style: TextStyle(
                  fontSize: 11,
                  fontWeight: FontWeight.w600,
                  color: c.textDim,
                  fontFamily: 'monospace')),
          const SizedBox(width: 10),
          fitBadge(file),
          const SizedBox(width: 10),
          Text(file.sizeFormatted,
              style: TextStyle(fontSize: 12, color: c.textMuted)),
          const SizedBox(width: 6),
          Icon(
            _dropdownOpen ? Icons.keyboard_arrow_up : Icons.keyboard_arrow_down,
            size: 18,
            color: c.textDim,
          ),
        ],
      );
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        // Collapsed trigger
        GestureDetector(
          onTap: () => setState(() => _dropdownOpen = !_dropdownOpen),
          child: MouseRegion(
            cursor: SystemMouseCursors.click,
            child: AnimatedContainer(
              duration: const Duration(milliseconds: 150),
              padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
              decoration: BoxDecoration(
                color: c.bgPanel,
                borderRadius: _dropdownOpen
                    ? const BorderRadius.vertical(top: Radius.circular(8))
                    : BorderRadius.circular(8),
                border: Border.all(
                  color: _dropdownOpen
                      ? MemoTheme.accent.withValues(alpha: 0.5)
                      : c.borderSoft,
                ),
              ),
              child: headerRow(selected),
            ),
          ),
        ),

        // Expanded list
        if (_dropdownOpen)
          Container(
            decoration: BoxDecoration(
              color: c.bgPanel,
              borderRadius:
                  const BorderRadius.vertical(bottom: Radius.circular(8)),
              border: Border(
                left: BorderSide(color: MemoTheme.accent.withValues(alpha: 0.4)),
                right: BorderSide(color: MemoTheme.accent.withValues(alpha: 0.4)),
                bottom: BorderSide(color: MemoTheme.accent.withValues(alpha: 0.4)),
              ),
            ),
            child: Column(
              children: files.map((file) {
                final isSelected = selected?.filename == file.filename;
                final isDownloaded =
                    localModels.any((m) => m.filename == file.filename);
                final q = quantInfo(file.filename);
                final qCode = _quantCode(file.filename);
                return MouseRegion(
                  cursor: SystemMouseCursors.click,
                  child: GestureDetector(
                    onTap: () => setState(() {
                      _selectedFile = file;
                      _dropdownOpen = false;
                    }),
                    child: AnimatedContainer(
                      duration: const Duration(milliseconds: 100),
                      padding: const EdgeInsets.symmetric(
                          horizontal: 14, vertical: 11),
                      decoration: BoxDecoration(
                        color: isSelected
                            ? MemoTheme.accent.withValues(alpha: 0.08)
                            : Colors.transparent,
                        border:
                            Border(top: BorderSide(color: c.borderSoft)),
                      ),
                      child: Row(
                        children: [
                          // checkmark / placeholder
                          SizedBox(
                            width: 18,
                            child: isSelected
                                ? Icon(Icons.check,
                                    size: 14, color: MemoTheme.accent)
                                : null,
                          ),
                          const SizedBox(width: 4),
                          _GgufBadge(),
                          const SizedBox(width: 8),
                          // Friendly quant label
                          Expanded(
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Text(
                                  q.label,
                                  style: TextStyle(
                                    fontSize: 13,
                                    fontWeight: isSelected
                                        ? FontWeight.w600
                                        : FontWeight.w500,
                                    color: isSelected
                                        ? MemoTheme.accent
                                        : c.textMain,
                                  ),
                                ),
                                Text(
                                  qCode,
                                  style: TextStyle(
                                      fontSize: 10,
                                      color: c.textDim,
                                      fontFamily: 'monospace'),
                                ),
                              ],
                            ),
                          ),
                          const SizedBox(width: 8),
                          // Hardware fit badge
                          fitBadge(file),
                          const SizedBox(width: 10),
                          // Size
                          Text(file.sizeFormatted,
                              style: TextStyle(
                                  fontSize: 12, color: c.textMuted)),
                          if (isDownloaded) ...[
                            const SizedBox(width: 8),
                            Icon(Icons.check_circle,
                                size: 14, color: MemoTheme.green),
                          ],
                        ],
                      ),
                    ),
                  ),
                );
              }).toList(),
            ),
          ),
      ],
    );
  }

  Widget _buildDownloadRow(ThemeColors c, GPUInfo gpu,
      List<LocalModel> localModels, bool downloadingNow) {
    final file = _selectedFile!;
    final fit = hardwareFit(file.size, gpu);
    final isFileDownloaded =
        localModels.any((m) => m.filename == file.filename);

    final (fitColor, _) = switch (fit.level) {
      FitLevel.good => (MemoTheme.green, Icons.check_circle_outline),
      FitLevel.ok => (MemoTheme.accent, Icons.check_circle_outline),
      FitLevel.warn => (MemoTheme.warningOrange, Icons.warning_amber_rounded),
    };

    Widget button;
    if (isFileDownloaded) {
      final localModel =
          localModels.firstWhere((m) => m.filename == file.filename);
      button = Expanded(
        child: ElevatedButton.icon(
          onPressed: () => showDialog(
            context: context,
            builder: (_) => ModelConfigDialog(model: localModel),
          ),
          icon: const Icon(Icons.play_arrow_rounded, size: 18),
          label: Text(L10n.locale == MemoLocale.tr
              ? 'Başlat · ${file.sizeFormatted}'
              : 'Start · ${file.sizeFormatted}'),
          style: ElevatedButton.styleFrom(
              padding: const EdgeInsets.symmetric(vertical: 14)),
        ),
      );
    } else if (downloadingNow) {
      button = Expanded(
        child: ElevatedButton.icon(
          onPressed: () => ref.read(apiClientProvider).cancelDownload(),
          icon: const Icon(Icons.stop_rounded, size: 18),
          label: Text(
            L10n.locale == MemoLocale.tr ? 'İptal Et' : 'Cancel',
          ),
          style: ElevatedButton.styleFrom(
            padding: const EdgeInsets.symmetric(vertical: 14),
            backgroundColor: MemoTheme.red.withValues(alpha: 0.12),
            foregroundColor: MemoTheme.red,
          ),
        ),
      );
    } else {
      button = Expanded(
        child: ElevatedButton.icon(
          onPressed: _download,
          icon: const Icon(Icons.download_rounded, size: 18),
          label: Text('${L10n.t('download')} · ${file.sizeFormatted}'),
          style: ElevatedButton.styleFrom(
              padding: const EdgeInsets.symmetric(vertical: 14)),
        ),
      );
    }

    return Row(
      crossAxisAlignment: CrossAxisAlignment.center,
      children: [
        button,
        const SizedBox(width: 10),
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 6),
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(6),
            border: Border.all(color: fitColor.withValues(alpha: 0.5)),
          ),
          child: Text(
            fit.label,
            style: TextStyle(
                fontSize: 11,
                fontWeight: FontWeight.w600,
                color: fitColor),
          ),
        ),
      ],
    );
  }
}

// ─── GGUF badge (reused across dropdown header + rows) ───────────

class _GgufBadge extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      decoration: BoxDecoration(
        color: MemoTheme.accent.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(4),
        border: Border.all(color: MemoTheme.accent.withValues(alpha: 0.3)),
      ),
      child: Text(
        'GGUF',
        style: TextStyle(
            fontSize: 10, fontWeight: FontWeight.w700, color: MemoTheme.accent),
      ),
    );
  }
}

// ─── More-from-author compact row ─────────────────────────────────

class _MoreModelRow extends StatelessWidget {
  final String id;
  final int downloads;
  final VoidCallback onTap;

  const _MoreModelRow({
    required this.id,
    required this.downloads,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    return MouseRegion(
      cursor: SystemMouseCursors.click,
      child: GestureDetector(
        onTap: onTap,
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 11),
          child: Row(
            children: [
              Expanded(
                child: Text(
                  _humanizeName(id),
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w500,
                    color: c.textMain,
                  ),
                  overflow: TextOverflow.ellipsis,
                  maxLines: 1,
                ),
              ),
              const SizedBox(width: 8),
              Text(
                '↓ ${_fmtCount(downloads)}',
                style: TextStyle(fontSize: 12, color: c.textDim),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _StatChip extends StatelessWidget {
  final IconData icon;
  final String text;
  const _StatChip({required this.icon, required this.text});

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(icon, size: 13, color: c.textDim),
        const SizedBox(width: 4),
        Text(text, style: TextStyle(fontSize: 12, color: c.textDim)),
      ],
    );
  }
}

class _InfoTag extends StatelessWidget {
  final String label;
  final String value;
  final bool accent;
  const _InfoTag(
      {required this.label, required this.value, this.accent = false});

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final tagColor = accent ? MemoTheme.accent : c.textMuted;
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(
          '$label ',
          style: TextStyle(fontSize: 12, color: c.textDim),
        ),
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
          decoration: BoxDecoration(
            color: tagColor.withValues(alpha: accent ? 0.15 : 0.08),
            borderRadius: BorderRadius.circular(5),
            border: accent
                ? Border.all(color: tagColor.withValues(alpha: 0.4))
                : null,
          ),
          child: Text(
            value,
            style: TextStyle(
              fontSize: 12,
              fontWeight: FontWeight.w600,
              color: tagColor,
            ),
          ),
        ),
      ],
    );
  }
}

class _CapabilityPill extends StatelessWidget {
  final IconData icon;
  final String label;
  final Color color;
  const _CapabilityPill(
      {required this.icon, required this.label, required this.color});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(6),
        border: Border.all(color: color.withValues(alpha: 0.35)),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 13, color: color),
          const SizedBox(width: 5),
          Text(
            label,
            style: TextStyle(
                fontSize: 12, fontWeight: FontWeight.w600, color: color),
          ),
        ],
      ),
    );
  }
}

// ─── Pill widget (reused in My Models) ───────────────────────────

class _Pill extends StatelessWidget {
  final String text;
  final bool highlight;
  const _Pill({required this.text, this.highlight = false});

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(
        color: highlight ? MemoTheme.accent.withValues(alpha: 0.12) : c.bgElement,
        borderRadius: BorderRadius.circular(6),
        border: highlight
            ? Border.all(color: MemoTheme.accent.withValues(alpha: 0.4))
            : null,
      ),
      child: Text(
        text,
        style: TextStyle(
          fontSize: 11,
          fontWeight: FontWeight.w500,
          color: highlight ? MemoTheme.accent : c.textMuted,
        ),
      ),
    );
  }
}

// ─── My Models tab (unchanged design) ────────────────────────────

class _MyModelsTab extends ConsumerWidget {
  const _MyModelsTab();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = MemoTheme.of(context);
    final localAsync = ref.watch(localModelsProvider);

    return localAsync.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (e, _) => Center(child: Text('${L10n.t('error')}: $e')),
      data: (models) => Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(28, 20, 28, 8),
            child: Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Text(
                  L10n.t('local_models'),
                  style: TextStyle(
                      fontSize: 15,
                      fontWeight: FontWeight.w600,
                      color: c.textMain),
                ),
                TextButton.icon(
                  icon: const Icon(Icons.file_upload_outlined, size: 16),
                  label: Text(L10n.t('import_model')),
                  onPressed: () => _import(context, ref),
                ),
              ],
            ),
          ),
          Expanded(
            child: models.isEmpty
                ? const _EmptyModels()
                : ListView.separated(
                    padding: const EdgeInsets.fromLTRB(28, 8, 28, 28),
                    itemCount: models.length,
                    separatorBuilder: (_, i) => const SizedBox(height: 10),
                    itemBuilder: (ctx, i) =>
                        _LocalModelCard(model: models[i]),
                  ),
          ),
        ],
      ),
    );
  }

  Future<void> _import(BuildContext context, WidgetRef ref) async {
    final messenger = ScaffoldMessenger.of(context);
    final result = await FilePicker.platform.pickFiles(type: FileType.any);
    if (result == null || result.files.single.path == null) return;
    messenger.showSnackBar(SnackBar(content: Text(L10n.t('importing_model'))));
    try {
      await ref.read(apiClientProvider).importModel(result.files.single.path!);
      ref.invalidate(localModelsProvider);
      messenger
          .showSnackBar(SnackBar(content: Text(L10n.t('import_success'))));
    } catch (e) {
      messenger.showSnackBar(SnackBar(
          content:
              Text(L10n.t('import_error', {'e': e.toString()}))));
    }
  }
}

class _EmptyModels extends StatelessWidget {
  const _EmptyModels();

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    Widget step(int n, String text) => Padding(
          padding: const EdgeInsets.symmetric(vertical: 6),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Container(
                width: 24,
                height: 24,
                alignment: Alignment.center,
                decoration: const BoxDecoration(
                  color: MemoTheme.accentMuted,
                  shape: BoxShape.circle,
                ),
                child: Text('$n',
                    style: const TextStyle(
                        fontSize: 12,
                        fontWeight: FontWeight.w700,
                        color: MemoTheme.accent)),
              ),
              const SizedBox(width: 12),
              Text(text, style: TextStyle(fontSize: 14, color: c.textMuted)),
            ],
          ),
        );

    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(Icons.inventory_2_outlined, size: 40, color: c.textDim),
          const SizedBox(height: 16),
          Text(L10n.t('empty_models_intro'),
              style: TextStyle(
                  fontSize: 16,
                  fontWeight: FontWeight.w600,
                  color: c.textMain)),
          const SizedBox(height: 12),
          step(1, L10n.t('empty_step_download')),
          step(2, L10n.t('empty_step_start')),
          step(3, L10n.t('empty_step_chat')),
        ],
      ),
    );
  }
}

class _LocalModelCard extends ConsumerWidget {
  final LocalModel model;
  const _LocalModelCard({required this.model});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = MemoTheme.of(context);
    final status = ref.watch(modelStatusProvider).valueOrNull;
    final embStatus = ref.watch(embeddingStatusProvider).valueOrNull;
    final running = (status?.running == true &&
            p.basename(status?.modelPath ?? '') == model.filename) ||
        (embStatus?.running == true &&
            p.basename(embStatus?.modelPath ?? '') == model.filename);
    final installed =
        ref.watch(llamaInstalledProvider).valueOrNull ?? false;

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: c.bgPanel,
        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
        border: Border.all(
            color:
                running ? MemoTheme.accent.withValues(alpha: 0.5) : c.borderSoft),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  model.filename,
                  style: TextStyle(
                      fontWeight: FontWeight.w600,
                      fontSize: 14,
                      color: c.textMain),
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              if (running) ...[
                _RunningDot(),
                const SizedBox(width: 8),
              ],
              IconButton(
                icon: const Icon(Icons.delete_outline, size: 18),
                color: MemoTheme.red,
                padding: EdgeInsets.zero,
                constraints: const BoxConstraints(),
                onPressed: () => _confirmDelete(context, ref),
              ),
            ],
          ),
          const SizedBox(height: 2),
          Text(
            model.repoId.isNotEmpty ? model.repoId : model.path,
            style: TextStyle(fontSize: 11, color: c.textDim),
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
          ),
          const SizedBox(height: 10),
          Row(
            crossAxisAlignment: CrossAxisAlignment.center,
            children: [
              Flexible(
                child: Wrap(
                  spacing: 6,
                  runSpacing: 4,
                  crossAxisAlignment: WrapCrossAlignment.center,
                  children: [
                    _Pill(
                      text: model.isEmbedding
                          ? L10n.t('kind_memory')
                          : (model.isVision || model.supportsVision
                              ? L10n.t('kind_vision')
                              : L10n.t('kind_chat')),
                    ),
                    _Pill(text: model.sizeFormatted),
                    if (model.likelySupportsTools)
                      _Pill(
                        text: L10n.locale == MemoLocale.tr
                            ? '🔧 Araç'
                            : '🔧 Tools',
                        highlight: true,
                      ),
                    if (model.supportsVision && !model.isVision)
                      _Pill(
                        text: L10n.locale == MemoLocale.tr
                            ? '👁 Görüntü'
                            : '👁 Vision',
                        highlight: true,
                      ),
                    if (model.supportsCode)
                      _Pill(
                        text: L10n.locale == MemoLocale.tr
                            ? '💻 Kod'
                            : '💻 Code',
                        highlight: true,
                      ),
                  ],
                ),
              ),
              const SizedBox(width: 12),
              running
                  ? OutlinedButton(
                      onPressed: () async {
                        final api = ref.read(apiClientProvider);
                        if (model.isEmbedding) {
                          await api.stopEmbeddingModel();
                          ref.invalidate(embeddingStatusProvider);
                        } else {
                          await api.stopModel();
                          ref.invalidate(modelStatusProvider);
                        }
                      },
                      style: OutlinedButton.styleFrom(
                        foregroundColor: MemoTheme.red,
                        side: const BorderSide(color: MemoTheme.red),
                        minimumSize: const Size(0, 34),
                        padding: const EdgeInsets.symmetric(horizontal: 16),
                      ),
                      child: Text(L10n.t('stop_model'),
                          style: const TextStyle(fontSize: 12)),
                    )
                  : ElevatedButton(
                      onPressed: !installed
                          ? null
                          : () => showDialog(
                                context: context,
                                builder: (_) =>
                                    ModelConfigDialog(model: model),
                              ),
                      style: ElevatedButton.styleFrom(
                        minimumSize: const Size(0, 34),
                        padding: const EdgeInsets.symmetric(horizontal: 16),
                      ),
                      child: Text(
                        model.isEmbedding
                            ? L10n.t('start_embedding')
                            : L10n.t('start_model'),
                        style: const TextStyle(fontSize: 12),
                      ),
                    ),
            ],
          ),
        ],
      ),
    );
  }

  Future<void> _confirmDelete(BuildContext context, WidgetRef ref) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(L10n.t('delete_model_title')),
        content: Text(
            L10n.t('delete_model_confirm_name', {'name': model.filename})),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(ctx, false),
              child: Text(L10n.t('cancel'))),
          TextButton(
            onPressed: () => Navigator.pop(ctx, true),
            style: TextButton.styleFrom(foregroundColor: MemoTheme.red),
            child: Text(L10n.t('delete')),
          ),
        ],
      ),
    );
    if (confirmed == true) {
      ref.read(localModelsProvider.notifier).deleteModel(model.path);
    }
  }
}

class _RunningDot extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
      decoration: BoxDecoration(
        color: MemoTheme.green.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(12),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(
            width: 6,
            height: 6,
            decoration: const BoxDecoration(
                color: MemoTheme.green, shape: BoxShape.circle),
          ),
          const SizedBox(width: 5),
          Text(L10n.t('running_now'),
              style: const TextStyle(
                  fontSize: 10,
                  fontWeight: FontWeight.w600,
                  color: MemoTheme.green)),
        ],
      ),
    );
  }
}

// ─── Download banner ──────────────────────────────────────────────

class _DownloadBanner extends ConsumerWidget {
  const _DownloadBanner();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = MemoTheme.of(context);
    final progress = ref.watch(downloadProgressProvider).valueOrNull;

    ref.listen(downloadProgressProvider, (prev, next) {
      final was = prev?.valueOrNull?.active ?? false;
      final now = next.valueOrNull?.active ?? false;
      if (was && !now) ref.invalidate(localModelsProvider);
    });

    if (progress == null || !progress.active) return const SizedBox.shrink();

    return Container(
      padding: const EdgeInsets.fromLTRB(28, 12, 28, 12),
      decoration: BoxDecoration(
        color: c.bgPanel,
        border: Border(bottom: BorderSide(color: c.borderSoft)),
      ),
      child: Row(
        children: [
          Icon(Icons.cloud_download_outlined,
              size: 18, color: MemoTheme.accent),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Expanded(
                      child: Text(
                        progress.filename,
                        style: TextStyle(
                            fontSize: 13,
                            fontWeight: FontWeight.w600,
                            color: c.textMain),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                    const SizedBox(width: 12),
                    Text(
                      '${progress.percent.toStringAsFixed(0)}%${progress.speed.isNotEmpty ? ' · ${progress.speed}' : ''}',
                      style: TextStyle(
                          fontSize: 12,
                          fontWeight: FontWeight.w600,
                          color: MemoTheme.accent),
                    ),
                  ],
                ),
                const SizedBox(height: 6),
                ClipRRect(
                  borderRadius: BorderRadius.circular(999),
                  child: LinearProgressIndicator(
                    value: progress.percent / 100.0,
                    minHeight: 4,
                    backgroundColor: c.bgElement,
                    valueColor:
                        const AlwaysStoppedAnimation(MemoTheme.accent),
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(width: 8),
          IconButton(
            onPressed: () => ref.read(apiClientProvider).cancelDownload(),
            icon: const Icon(Icons.close_rounded, size: 18),
            tooltip: L10n.locale == MemoLocale.tr ? 'İndirmeyi iptal et' : 'Cancel download',
            color: c.textDim,
            style: IconButton.styleFrom(
              minimumSize: const Size(32, 32),
              padding: EdgeInsets.zero,
            ),
          ),
        ],
      ),
    );
  }
}
