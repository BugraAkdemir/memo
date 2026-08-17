import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/theme.dart';
import '../../../core/l10n.dart';
import 'package:flutter/services.dart';
import '../../../models/gpu_info.dart';
import '../../../providers/settings_provider.dart';
import '../../../providers/chat_provider.dart';
import '../../../core/friendly_error.dart';

String _formatCompactCount(int n) {
  if (n >= 1000000) return '${(n / 1000000).toStringAsFixed(1)}M';
  if (n >= 1000) return '${(n / 1000).toStringAsFixed(1)}K';
  return '$n';
}

class MemoryTab extends ConsumerStatefulWidget {
  const MemoryTab({super.key});

  @override
  ConsumerState<MemoryTab> createState() => MemoryTabState();
}

class MemoryTabState extends ConsumerState<MemoryTab> {
  final _topKController = TextEditingController();
  final _minSimilarityController = TextEditingController();
  bool _settingsInitialized = false;
  bool _savingSettings = false;
  final _debugQueryController = TextEditingController();
  bool _debugSearching = false;
  List<MemorySearchResult> _debugResults = [];
  String? _debugError;
  bool _debugSearched = false;
  MemoryStats? _memoryStats;
  bool _statsLoading = false;

  @override
  void dispose() {
    _topKController.dispose();
    _minSimilarityController.dispose();
    _debugQueryController.dispose();
    super.dispose();
  }

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _loadStats());
  }

  Future<void> _loadStats() async {
    if (!mounted) return;
    setState(() => _statsLoading = true);
    try {
      final stats = await ref.read(apiClientProvider).getMemoryStats();
      if (mounted) setState(() => _memoryStats = stats);
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(L10n.t('memory_stats_unavailable', {'e': FriendlyError.describeGeneric(e)})),
          ),
        );
      }
    } finally {
      if (mounted) setState(() => _statsLoading = false);
    }
  }

  Future<void> _runDebugSearch() async {
    final query = _debugQueryController.text.trim();
    if (query.isEmpty) return;
    setState(() {
      _debugSearching = true;
      _debugError = null;
      _debugResults = [];
      _debugSearched = false;
    });
    try {
      final results = await ref
          .read(apiClientProvider)
          .debugMemorySearch(query);
      if (mounted) {
        setState(() {
          _debugResults = results;
          _debugSearched = true;
        });
      }
    } catch (e) {
      if (mounted) {
        setState(() => _debugError = e.toString());
      }
    } finally {
      if (mounted) {
        setState(() => _debugSearching = false);
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final memoryAsync = ref.watch(memoryFilesProvider);
    final settingsAsync = ref.watch(memorySettingsProvider);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Expanded(
          child: ListView(
            padding: EdgeInsets.all(32),
            children: [
              Text(
                L10n.t('memory'),
                style: Theme.of(context).textTheme.titleLarge?.copyWith(
                  fontWeight: FontWeight.bold,
                  color: MemoTheme.of(context).textMain,
                ),
              ),
              SizedBox(height: 8),
              Text(
                L10n.t('memory_advanced_hint'),
                style: TextStyle(
                  color: MemoTheme.of(context).textDim,
                  fontSize: 13,
                ),
              ),
              SizedBox(height: 20),
              settingsAsync.when(
                loading: () => Center(child: CircularProgressIndicator()),
                error: (e, _) => Text('${L10n.t('error')}: ${FriendlyError.describeGeneric(e)}'),
                data: (settings) {
                  if (!_settingsInitialized) {
                    _topKController.text = settings.topK.toString();
                    _minSimilarityController.text = settings.minSimilarity
                        .toStringAsFixed(2);
                    _settingsInitialized = true;
                  }

                  return Container(
                    padding: EdgeInsets.all(16),
                    decoration: BoxDecoration(
                      color: MemoTheme.of(context).bgPanel,
                      borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                      border: Border.all(
                        color: MemoTheme.of(context).borderSoft,
                      ),
                    ),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        Text(
                          L10n.t('memory_retrieval_settings'),
                          style: TextStyle(
                            fontSize: 14,
                            fontWeight: FontWeight.w600,
                            color: MemoTheme.of(context).textMain,
                          ),
                        ),
                        SizedBox(height: 14),
                        MemorySettingField(
                          label: L10n.t('memory_top_k'),
                          controller: _topKController,
                          hint: '3',
                          inputFormatters: [
                            FilteringTextInputFormatter.digitsOnly,
                          ],
                        ),
                        SizedBox(height: 12),
                        MemorySettingField(
                          label: L10n.t('memory_min_similarity'),
                          controller: _minSimilarityController,
                          hint: '0.25',
                          inputFormatters: [
                            FilteringTextInputFormatter.allow(
                              RegExp(r'^\d*\.?\d{0,2}'),
                            ),
                          ],
                        ),
                        SizedBox(height: 14),
                        Row(
                          mainAxisAlignment: MainAxisAlignment.end,
                          children: [
                            ElevatedButton(
                              onPressed: _savingSettings
                                  ? null
                                  : () async {
                                      final topK =
                                          int.tryParse(_topKController.text) ??
                                          settings.topK;
                                      final minSimilarity =
                                          double.tryParse(
                                            _minSimilarityController.text,
                                          ) ??
                                          settings.minSimilarity;
                                      final messenger = ScaffoldMessenger.of(
                                        context,
                                      );

                                      setState(() => _savingSettings = true);
                                      try {
                                        await ref
                                            .read(
                                              memorySettingsProvider.notifier,
                                            )
                                            .save(
                                              topK: topK,
                                              minSimilarity: minSimilarity,
                                            );
                                        if (mounted) {
                                          messenger.showSnackBar(
                                            SnackBar(
                                              content: Text(L10n.t('saved')),
                                            ),
                                          );
                                        }
                                      } catch (e) {
                                        if (mounted) {
                                          messenger.showSnackBar(
                                            SnackBar(
                                              content: Text(
                                                '${L10n.t('error')}: ${FriendlyError.describeGeneric(e)}',
                                              ),
                                            ),
                                          );
                                        }
                                      } finally {
                                        if (mounted) {
                                          setState(
                                            () => _savingSettings = false,
                                          );
                                        }
                                      }
                                    },
                              child: _savingSettings
                                  ? SizedBox(
                                      width: 14,
                                      height: 14,
                                      child: CircularProgressIndicator(
                                        strokeWidth: 2,
                                      ),
                                    )
                                  : Text(L10n.t('save')),
                            ),
                          ],
                        ),
                      ],
                    ),
                  );
                },
              ),
              SizedBox(height: 28),
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Text(
                    L10n.t('memory_files'),
                    style: Theme.of(context).textTheme.titleMedium?.copyWith(
                      fontWeight: FontWeight.bold,
                      color: MemoTheme.of(context).textMain,
                    ),
                  ),
                  OutlinedButton.icon(
                    icon: Icon(Icons.delete_sweep, size: 18),
                    label: Text(L10n.t('clear_memory')),
                    style: OutlinedButton.styleFrom(
                      foregroundColor: MemoTheme.red,
                      side: BorderSide(color: MemoTheme.red),
                    ),
                    onPressed: () async {
                      final confirmed = await showDialog<bool>(
                        context: context,
                        builder: (ctx) => AlertDialog(
                          backgroundColor: MemoTheme.of(context).bgPanel,
                          title: Text(L10n.t('clear_memory_title')),
                          content: Text(L10n.t('clear_memory_confirm_ext')),
                          actions: [
                            TextButton(
                              onPressed: () => Navigator.pop(ctx, false),
                              child: Text(L10n.t('cancel')),
                            ),
                            TextButton(
                              onPressed: () => Navigator.pop(ctx, true),
                              style: TextButton.styleFrom(
                                foregroundColor: MemoTheme.red,
                              ),
                              child: Text(L10n.t('clear_memory')),
                            ),
                          ],
                        ),
                      );
                      if (confirmed == true) {
                        try {
                          await ref
                              .read(memoryFilesProvider.notifier)
                              .clearAll();
                        } catch (e) {
                          if (context.mounted) {
                            ScaffoldMessenger.of(context).showSnackBar(
                              SnackBar(content: Text('${L10n.t('error')}: ${FriendlyError.describeGeneric(e)}')),
                            );
                          }
                        }
                      }
                    },
                  ),
                ],
              ),
              SizedBox(height: 12),
              memoryAsync.when(
                loading: () => Center(child: CircularProgressIndicator()),
                error: (e, _) => Center(child: Text('${L10n.t('error')}: ${FriendlyError.describeGeneric(e)}')),
                data: (files) {
                  if (files.isEmpty) {
                    return Padding(
                      padding: EdgeInsets.only(top: 40),
                      child: Center(
                        child: Text(
                          L10n.t('no_memory_files'),
                          style: TextStyle(
                            color: MemoTheme.of(context).textDim,
                          ),
                        ),
                      ),
                    );
                  }

                  // Collapsed by default, and the list itself is a bounded-
                  // height ListView.builder rather than an eager Column —
                  // a memory store with hundreds of entries used to build
                  // every ListTile/Divider up front (rebuilt on every
                  // setState in this tab, e.g. _loadStats/_runDebugSearch),
                  // which is what caused the RAM/lag complaint. Collapsed,
                  // zero row widgets exist at all; expanded, only the rows
                  // actually visible in the fixed-height viewport are built
                  // (ListView.builder's laziness only works with a bounded
                  // height — shrinkWrap inside the outer page ListView would
                  // still force building everything up front).
                  return Theme(
                    data: Theme.of(
                      context,
                    ).copyWith(dividerColor: Colors.transparent),
                    child: ExpansionTile(
                      tilePadding: EdgeInsets.zero,
                      childrenPadding: EdgeInsets.zero,
                      title: Text(
                        L10n.t('memory_files_show', {'n': '${files.length}'}),
                        style: TextStyle(
                          fontSize: 13,
                          color: MemoTheme.of(context).textMain,
                        ),
                      ),
                      children: [
                        SizedBox(
                          height: 420,
                          child: ListView.builder(
                            itemCount: files.length,
                            itemBuilder: (context, i) {
                              final file = files[i];
                              return Column(
                                children: [
                                  ListTile(
                                    contentPadding: EdgeInsets.zero,
                                    title: Text(
                                      file.name,
                                      style: TextStyle(
                                        fontWeight: FontWeight.w500,
                                      ),
                                    ),
                                    subtitle: Text(
                                      '${file.sizeKb} KB • ${file.modified}',
                                      style: TextStyle(
                                        color: MemoTheme.of(context).textDim,
                                        fontSize: 12,
                                      ),
                                    ),
                                    trailing: IconButton(
                                      icon: Icon(Icons.delete_outline),
                                      color: MemoTheme.red,
                                      onPressed: () async {
                                        try {
                                          await ref
                                              .read(
                                                memoryFilesProvider.notifier,
                                              )
                                              .deleteFile(file.path);
                                        } catch (e) {
                                          if (context.mounted) {
                                            ScaffoldMessenger.of(
                                              context,
                                            ).showSnackBar(
                                              SnackBar(
                                                content: Text(
                                                  '${L10n.t('error')}: ${FriendlyError.describeGeneric(e)}',
                                                ),
                                              ),
                                            );
                                          }
                                        }
                                      },
                                    ),
                                  ),
                                  Divider(height: 1),
                                ],
                              );
                            },
                          ),
                        ),
                      ],
                    ),
                  );
                },
              ),
              SizedBox(height: 28),
              Row(
                children: [
                  Text(
                    'Memory Analytics',
                    style: Theme.of(context).textTheme.titleMedium?.copyWith(
                      fontWeight: FontWeight.bold,
                      color: MemoTheme.of(context).textMain,
                    ),
                  ),
                  SizedBox(width: 8),
                  if (_statsLoading)
                    SizedBox(
                      width: 14,
                      height: 14,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  else
                    IconButton(
                      icon: Icon(Icons.refresh, size: 18),
                      color: MemoTheme.of(context).textDim,
                      onPressed: _loadStats,
                      tooltip: L10n.t('refresh'),
                      padding: EdgeInsets.zero,
                      constraints: BoxConstraints(),
                    ),
                ],
              ),
              SizedBox(height: 12),
              if (_memoryStats != null) ...[
                Container(
                  padding: EdgeInsets.all(16),
                  decoration: BoxDecoration(
                    color: MemoTheme.of(context).bgPanel,
                    borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                    border: Border.all(color: MemoTheme.of(context).borderSoft),
                  ),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      Row(
                        children: [
                          _StatChip(
                            label: 'Total',
                            value: '${_memoryStats!.count}',
                          ),
                          SizedBox(width: 8),
                          _StatChip(
                            label: 'Pinned',
                            value: '${_memoryStats!.explicitCount}',
                            accent: true,
                          ),
                          SizedBox(width: 8),
                          _StatChip(
                            label: 'Pinned tokens',
                            value: _formatCompactCount(_memoryStats!.pinnedTokens),
                            accent: true,
                          ),
                          SizedBox(width: 8),
                          _StatChip(
                            label: 'This week',
                            value: '+${_memoryStats!.addedThisWeek}',
                          ),
                          if (_memoryStats!.pendingDeletion > 0) ...[
                            SizedBox(width: 8),
                            _StatChip(
                              label: 'Expiring',
                              value: '${_memoryStats!.pendingDeletion}',
                              warn: true,
                            ),
                          ],
                        ],
                      ),
                      if (_memoryStats!.topRetrieved.isNotEmpty) ...[
                        SizedBox(height: 14),
                        Text(
                          'Most accessed memories',
                          style: TextStyle(
                            fontSize: 12,
                            fontWeight: FontWeight.w600,
                            color: MemoTheme.of(context).textDim,
                          ),
                        ),
                        SizedBox(height: 8),
                        ..._memoryStats!.topRetrieved.asMap().entries.map((e) {
                          final r = e.value;
                          return Padding(
                            padding: EdgeInsets.only(bottom: 6),
                            child: Row(
                              children: [
                                Container(
                                  width: 20,
                                  alignment: Alignment.center,
                                  child: Text(
                                    '#${e.key + 1}',
                                    style: TextStyle(
                                      fontSize: 11,
                                      color: MemoTheme.of(context).textDim,
                                    ),
                                  ),
                                ),
                                SizedBox(width: 8),
                                Expanded(
                                  child: Text(
                                    r.content.length > 80
                                        ? '${r.content.substring(0, 80)}…'
                                        : r.content,
                                    style: TextStyle(
                                      fontSize: 12,
                                      color: MemoTheme.of(context).textMain,
                                    ),
                                    maxLines: 1,
                                    overflow: TextOverflow.ellipsis,
                                  ),
                                ),
                                SizedBox(width: 8),
                                Text(
                                  '${r.retrieveCount}×',
                                  style: TextStyle(
                                    fontSize: 11,
                                    fontWeight: FontWeight.w600,
                                    color: MemoTheme.accent,
                                  ),
                                ),
                              ],
                            ),
                          );
                        }),
                      ],
                    ],
                  ),
                ),
                SizedBox(height: 28),
              ],
              Text(
                L10n.t('memory_debug_search'),
                style: Theme.of(context).textTheme.titleMedium?.copyWith(
                  fontWeight: FontWeight.bold,
                  color: MemoTheme.of(context).textMain,
                ),
              ),
              SizedBox(height: 8),
              Text(
                L10n.t('memory_debug_hint'),
                style: TextStyle(
                  color: MemoTheme.of(context).textDim,
                  fontSize: 13,
                ),
              ),
              SizedBox(height: 12),
              Row(
                children: [
                  Expanded(
                    child: SizedBox(
                      height: 36,
                      child: TextField(
                        controller: _debugQueryController,
                        style: TextStyle(fontSize: 13),
                        onSubmitted: (_) => _runDebugSearch(),
                        decoration: InputDecoration(
                          hintText: L10n.t('memory_debug_placeholder'),
                          contentPadding: EdgeInsets.symmetric(horizontal: 12),
                          filled: true,
                          fillColor: MemoTheme.of(context).bgApp,
                          border: OutlineInputBorder(
                            borderRadius: BorderRadius.circular(
                              MemoTheme.radiusSm,
                            ),
                            borderSide: BorderSide(
                              color: MemoTheme.of(context).borderSoft,
                            ),
                          ),
                          enabledBorder: OutlineInputBorder(
                            borderRadius: BorderRadius.circular(
                              MemoTheme.radiusSm,
                            ),
                            borderSide: BorderSide(
                              color: MemoTheme.of(context).borderSoft,
                            ),
                          ),
                          focusedBorder: OutlineInputBorder(
                            borderRadius: BorderRadius.circular(
                              MemoTheme.radiusSm,
                            ),
                            borderSide: BorderSide(color: MemoTheme.accent),
                          ),
                        ),
                      ),
                    ),
                  ),
                  SizedBox(width: 8),
                  SizedBox(
                    height: 36,
                    child: ElevatedButton(
                      onPressed: _debugSearching ? null : _runDebugSearch,
                      child: _debugSearching
                          ? SizedBox(
                              width: 14,
                              height: 14,
                              child: CircularProgressIndicator(strokeWidth: 2),
                            )
                          : Text(L10n.t('memory_debug_search_btn')),
                    ),
                  ),
                ],
              ),
              if (_debugError != null)
                Padding(
                  padding: EdgeInsets.only(top: 8),
                  child: Text(
                    _debugError!,
                    style: TextStyle(color: MemoTheme.red, fontSize: 12),
                  ),
                ),
              if (_debugSearched &&
                  _debugResults.isEmpty &&
                  _debugError == null)
                Padding(
                  padding: EdgeInsets.only(top: 12),
                  child: Text(
                    L10n.t('memory_debug_no_results'),
                    style: TextStyle(
                      color: MemoTheme.of(context).textDim,
                      fontSize: 13,
                    ),
                  ),
                ),
              if (_debugResults.isNotEmpty)
                Padding(
                  padding: EdgeInsets.only(top: 12),
                  // Bounded-height ListView.builder, not an eager Column —
                  // each result renders a decorated Container (border +
                  // rounded corners, pricier than a plain ListTile), and
                  // with a well-populated memory store this list is what
                  // caused the RAM/lag complaint alongside the Memory Files
                  // list above. Same fix: only the rows actually scrolled
                  // into view within the fixed height get built.
                  child: SizedBox(
                    height: 420,
                    child: ListView.builder(
                      itemCount: _debugResults.length,
                      itemBuilder: (context, i) {
                        final r = _debugResults[i];
                        final matchLabel = r.matchType == 'pinned'
                            ? L10n.t('memory_debug_match_pinned')
                            : r.matchType == 'vector'
                            ? L10n.t('memory_debug_match_vector')
                            : r.matchType == 'fts'
                            ? L10n.t('memory_debug_match_fts')
                            : L10n.t('memory_debug_match_hybrid');
                        return Container(
                          margin: EdgeInsets.only(bottom: 8),
                          padding: EdgeInsets.all(12),
                          decoration: BoxDecoration(
                            color: MemoTheme.of(context).bgApp,
                            borderRadius: BorderRadius.circular(
                              MemoTheme.radiusSm,
                            ),
                            border: Border.all(
                              color: MemoTheme.of(context).borderSoft,
                            ),
                          ),
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Row(
                                children: [
                                  Text(
                                    '#${i + 1}',
                                    style: TextStyle(
                                      fontWeight: FontWeight.bold,
                                      color: MemoTheme.of(context).textDim,
                                      fontSize: 12,
                                    ),
                                  ),
                                  SizedBox(width: 8),
                                  Container(
                                    padding: EdgeInsets.symmetric(
                                      horizontal: 6,
                                      vertical: 2,
                                    ),
                                    decoration: BoxDecoration(
                                      color: MemoTheme.accent.withValues(
                                        alpha: 0.15,
                                      ),
                                      borderRadius: BorderRadius.circular(4),
                                    ),
                                    child: Text(
                                      matchLabel,
                                      style: TextStyle(
                                        color: MemoTheme.accent,
                                        fontSize: 11,
                                        fontWeight: FontWeight.w600,
                                      ),
                                    ),
                                  ),
                                  const Spacer(),
                                  Text(
                                    '${L10n.t('memory_debug_score')}: ${r.similarity.toStringAsFixed(3)}',
                                    style: TextStyle(
                                      color: MemoTheme.of(context).textDim,
                                      fontSize: 11,
                                    ),
                                  ),
                                ],
                              ),
                              SizedBox(height: 6),
                              Text(
                                r.content.length > 200
                                    ? '${r.content.substring(0, 200)}…'
                                    : r.content,
                                style: TextStyle(
                                  fontSize: 12,
                                  color: MemoTheme.of(context).textMain,
                                ),
                              ),
                              SizedBox(height: 4),
                              Text(
                                r.timestamp,
                                style: TextStyle(
                                  fontSize: 11,
                                  color: MemoTheme.of(context).textDim,
                                ),
                              ),
                            ],
                          ),
                        );
                      },
                    ),
                  ),
                ),
            ],
          ),
        ),
      ],
    );
  }
}

class _StatChip extends StatelessWidget {
  final String label;
  final String value;
  final bool accent;
  final bool warn;

  const _StatChip({
    required this.label,
    required this.value,
    this.accent = false,
    this.warn = false,
  });

  @override
  Widget build(BuildContext context) {
    final color = warn
        ? MemoTheme.red
        : accent
        ? MemoTheme.accent
        : MemoTheme.of(context).textMain;
    return Container(
      padding: EdgeInsets.symmetric(horizontal: 10, vertical: 6),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
        border: Border.all(color: color.withValues(alpha: 0.25)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            value,
            style: TextStyle(
              fontSize: 18,
              fontWeight: FontWeight.bold,
              color: color,
            ),
          ),
          Text(
            label,
            style: TextStyle(
              fontSize: 11,
              color: MemoTheme.of(context).textDim,
            ),
          ),
        ],
      ),
    );
  }
}

class MemorySettingField extends StatelessWidget {
  final String label;
  final TextEditingController controller;
  final String hint;
  final List<TextInputFormatter> inputFormatters;

  const MemorySettingField({
    super.key,
    required this.label,
    required this.controller,
    required this.hint,
    required this.inputFormatters,
  });

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        SizedBox(
          width: 170,
          child: Text(
            label,
            style: TextStyle(fontWeight: FontWeight.w500, fontSize: 13),
          ),
        ),
        Expanded(
          child: SizedBox(
            height: 36,
            child: TextField(
              controller: controller,
              keyboardType: TextInputType.numberWithOptions(decimal: true),
              inputFormatters: inputFormatters,
              style: TextStyle(fontSize: 13),
              decoration: InputDecoration(
                hintText: hint,
                contentPadding: EdgeInsets.symmetric(horizontal: 12),
                filled: true,
                fillColor: MemoTheme.of(context).bgApp,
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                  borderSide: BorderSide(
                    color: MemoTheme.of(context).borderSoft,
                  ),
                ),
                enabledBorder: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                  borderSide: BorderSide(
                    color: MemoTheme.of(context).borderSoft,
                  ),
                ),
                focusedBorder: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                  borderSide: BorderSide(color: MemoTheme.accent),
                ),
              ),
            ),
          ),
        ),
      ],
    );
  }
}
