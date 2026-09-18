import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/theme.dart';
import '../../../core/l10n.dart';
import 'package:flutter/services.dart';
import '../../../models/gpu_info.dart';
import '../../../providers/settings_provider.dart';
import '../../../providers/chat_provider.dart';
import '../../../core/friendly_error.dart';
import '../section_tab_bar.dart';

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
  int _activeSection = 0;
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
  List<MemorySearchResult> _knownFacts = [];
  bool _knownFactsLoading = false;
  String? _knownFactsError;
  final Set<String> _selectedFactIds = {};
  bool _factsBusy = false;

  static const _convPageSize = 30;
  List<MemorySearchResult> _convMemories = [];
  int _convTotal = 0;
  int _convOffset = 0;
  bool _convLoading = false;
  String? _convError;
  final Set<String> _selectedConvIds = {};
  bool _convBusy = false;

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
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _loadStats();
      _loadKnownFacts();
      _loadConversationMemories();
    });
  }

  Future<void> _loadKnownFacts() async {
    if (!mounted) return;
    setState(() {
      _knownFactsLoading = true;
      _knownFactsError = null;
    });
    try {
      final facts = await ref.read(apiClientProvider).getKnownFacts();
      if (mounted) setState(() => _knownFacts = facts);
    } catch (e) {
      if (mounted) {
        setState(
          () => _knownFactsError = L10n.t('memory_known_facts_error', {
            'e': FriendlyError.describeGeneric(e),
          }),
        );
      }
    } finally {
      if (mounted) setState(() => _knownFactsLoading = false);
    }
  }

  Future<void> _loadConversationMemories({bool reset = true}) async {
    if (!mounted) return;
    setState(() {
      _convLoading = true;
      _convError = null;
      if (reset) {
        _convMemories = [];
        _convOffset = 0;
        _selectedConvIds.clear();
      }
    });
    try {
      final page = await ref
          .read(apiClientProvider)
          .listConversationMemories(limit: _convPageSize, offset: _convOffset);
      if (mounted) {
        setState(() {
          _convMemories = reset
              ? page.results
              : [..._convMemories, ...page.results];
          _convTotal = page.total;
          _convOffset = _convMemories.length;
        });
      }
    } catch (e) {
      if (mounted) {
        setState(
          () => _convError = L10n.t('memory_conversation_error', {
            'e': FriendlyError.describeGeneric(e),
          }),
        );
      }
    } finally {
      if (mounted) setState(() => _convLoading = false);
    }
  }

  Future<bool> _confirmDelete(int count) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        backgroundColor: MemoTheme.of(context).bgPanel,
        title: Text(L10n.t('memory_delete_confirm_title')),
        content: Text(
          count == 1
              ? L10n.t('memory_delete_confirm_body_one')
              : L10n.t('memory_delete_confirm_body_many', {'n': '$count'}),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: Text(L10n.t('cancel')),
          ),
          TextButton(
            onPressed: () => Navigator.pop(ctx, true),
            style: TextButton.styleFrom(foregroundColor: MemoTheme.red),
            child: Text(L10n.t('delete')),
          ),
        ],
      ),
    );
    return confirmed == true;
  }

  /// Shared executor for both the pinned-facts list and the
  /// conversation-history list: confirm, call the exact-id bulk delete,
  /// then reconcile local state from the server's actual `deleted` count
  /// rather than assuming every requested id was removed (a stale
  /// selection can legitimately delete fewer than requested — see
  /// Store.DeleteByUUIDs' doc comment).
  Future<void> _deleteIds(List<String> ids, {required bool isFacts}) async {
    if (ids.isEmpty) return;
    if (!await _confirmDelete(ids.length)) return;
    if (!mounted) return;
    final messenger = ScaffoldMessenger.of(context);
    setState(() => isFacts ? _factsBusy = true : _convBusy = true);
    try {
      final deleted = await ref
          .read(apiClientProvider)
          .deleteMemoriesByIds(ids);
      if (mounted) {
        messenger.showSnackBar(
          SnackBar(
            content: Text(
              L10n.t('memory_deleted_success', {'n': '$deleted'}),
            ),
          ),
        );
        setState(() {
          if (isFacts) {
            _knownFacts.removeWhere((f) => ids.contains(f.id));
            _selectedFactIds.removeAll(ids);
          } else {
            _convMemories.removeWhere((f) => ids.contains(f.id));
            _selectedConvIds.removeAll(ids);
            _convTotal = (_convTotal - deleted).clamp(0, _convTotal);
            _convOffset = _convMemories.length;
          }
        });
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
        setState(() => isFacts ? _factsBusy = false : _convBusy = false);
      }
    }
  }

  Future<void> _showAddFactDialog() async {
    final controller = TextEditingController();
    final result = await showDialog<String>(
      context: context,
      builder: (ctx) => AlertDialog(
        backgroundColor: MemoTheme.of(context).bgPanel,
        title: Text(L10n.t('memory_known_facts_add_title')),
        content: TextField(
          controller: controller,
          autofocus: true,
          maxLines: 3,
          decoration: InputDecoration(
            hintText: L10n.t('memory_known_facts_add_hint'),
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: Text(L10n.t('cancel')),
          ),
          TextButton(
            onPressed: () => Navigator.pop(ctx, controller.text.trim()),
            child: Text(L10n.t('save')),
          ),
        ],
      ),
    );
    if (result == null || result.isEmpty || !mounted) return;
    final messenger = ScaffoldMessenger.of(context);
    try {
      await ref.read(apiClientProvider).saveExplicitMemory(result);
      if (mounted) {
        messenger.showSnackBar(
          SnackBar(content: Text(L10n.t('memory_known_facts_add_success'))),
        );
      }
      await _loadKnownFacts();
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
    }
  }

  Future<void> _showEditFactDialog(MemorySearchResult fact) async {
    final controller = TextEditingController(text: fact.content);
    final result = await showDialog<String>(
      context: context,
      builder: (ctx) => AlertDialog(
        backgroundColor: MemoTheme.of(context).bgPanel,
        title: Text(L10n.t('memory_known_facts_edit_title')),
        content: TextField(
          controller: controller,
          autofocus: true,
          maxLines: 3,
          decoration: InputDecoration(
            hintText: L10n.t('memory_known_facts_add_hint'),
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: Text(L10n.t('cancel')),
          ),
          TextButton(
            onPressed: () => Navigator.pop(ctx, controller.text.trim()),
            child: Text(L10n.t('save')),
          ),
        ],
      ),
    );
    if (result == null ||
        result.isEmpty ||
        result == fact.content ||
        !mounted) {
      return;
    }
    final messenger = ScaffoldMessenger.of(context);
    try {
      await ref.read(apiClientProvider).updatePinnedFact(fact.id, result);
      if (mounted) {
        messenger.showSnackBar(
          SnackBar(content: Text(L10n.t('memory_known_facts_edit_success'))),
        );
      }
      await _loadKnownFacts();
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
    }
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
    final theme = MemoTheme.of(context);
    final memoryAsync = ref.watch(memoryFilesProvider);
    final settingsAsync = ref.watch(memorySettingsProvider);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(32, 28, 32, 0),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                L10n.t('memory'),
                style: Theme.of(context).textTheme.titleLarge?.copyWith(
                  fontWeight: FontWeight.bold,
                  color: theme.textMain,
                ),
              ),
              const SizedBox(height: 6),
              Text(
                L10n.t('memory_advanced_hint'),
                style: TextStyle(color: theme.textDim, fontSize: 13),
              ),
              const SizedBox(height: 18),
              // Sub-navigation instead of one long vertical stack: each
              // section used to render fully every time regardless of what
              // the user actually came here to do, forcing a long scroll
              // past retrieval settings, files, analytics, pinned facts,
              // history and debug search just to reach any one of them.
              // Splitting into pills that swap the content pane below keeps
              // each screenful focused and lets the pill itself carry a
              // live count (files/facts/history) as an at-a-glance signal.
              SectionTabBar(
                selected: _activeSection,
                onSelected: (i) => setState(() => _activeSection = i),
                items: [
                  SectionTabItem(
                    Icons.tune,
                    L10n.t('memory_tab_settings'),
                    count: memoryAsync.valueOrNull?.length,
                  ),
                  SectionTabItem(
                    Icons.fact_check_outlined,
                    L10n.t('memory_tab_facts'),
                    count: _knownFacts.length,
                  ),
                  SectionTabItem(
                    Icons.history,
                    L10n.t('memory_tab_history'),
                    count: _convTotal,
                  ),
                  SectionTabItem(
                    Icons.insights_outlined,
                    L10n.t('memory_tab_analytics'),
                  ),
                  SectionTabItem(
                    Icons.bug_report_outlined,
                    L10n.t('memory_tab_debug'),
                  ),
                ],
              ),
            ],
          ),
        ),
        const SizedBox(height: 14),
        Divider(height: 1, color: theme.borderSoft),
        Expanded(
          child: _buildActiveSection(context, settingsAsync, memoryAsync),
        ),
      ],
    );
  }

  Widget _buildActiveSection(
    BuildContext context,
    AsyncValue<MemorySettings> settingsAsync,
    AsyncValue<List<MemoryFileInfo>> memoryAsync,
  ) {
    switch (_activeSection) {
      case 0:
        return _sectionScroll(
          _buildSettingsSection(context, settingsAsync, memoryAsync),
        );
      case 1:
        return _sectionScroll(_buildFactsSection(context));
      case 2:
        return _sectionScroll(_buildHistorySection(context));
      case 3:
        return _sectionScroll(_buildAnalyticsSection(context));
      default:
        return _sectionScroll(_buildDebugSection(context));
    }
  }

  Widget _sectionScroll(Widget child) {
    return ListView(
      padding: const EdgeInsets.fromLTRB(32, 20, 32, 32),
      children: [child],
    );
  }

  Widget _buildSettingsSection(
    BuildContext context,
    AsyncValue<MemorySettings> settingsAsync,
    AsyncValue<List<MemoryFileInfo>> memoryAsync,
  ) {
    final theme = MemoTheme.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(
          L10n.t('memory_retrieval_settings'),
          style: TextStyle(
            fontSize: 14,
            fontWeight: FontWeight.w600,
            color: theme.textMain,
          ),
        ),
        const SizedBox(height: 12),
        settingsAsync.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (e, _) => Text('${L10n.t('error')}: ${FriendlyError.describeGeneric(e)}'),
          data: (settings) {
            if (!_settingsInitialized) {
              _topKController.text = settings.topK.toString();
              _minSimilarityController.text = settings.minSimilarity
                  .toStringAsFixed(2);
              _settingsInitialized = true;
            }

            return Container(
              padding: const EdgeInsets.all(16),
              decoration: BoxDecoration(
                color: theme.bgPanel,
                borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                border: Border.all(color: theme.borderSoft),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  MemorySettingField(
                    label: L10n.t('memory_top_k'),
                    controller: _topKController,
                    hint: '3',
                    inputFormatters: [
                      FilteringTextInputFormatter.digitsOnly,
                    ],
                  ),
                  const SizedBox(height: 12),
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
                  const SizedBox(height: 14),
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
                                      .read(memorySettingsProvider.notifier)
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
                                    setState(() => _savingSettings = false);
                                  }
                                }
                              },
                        child: _savingSettings
                            ? const SizedBox(
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
        const SizedBox(height: 28),
        Row(
          children: [
            Expanded(
              child: Text(
                L10n.t('memory_files'),
                overflow: TextOverflow.ellipsis,
                style: Theme.of(context).textTheme.titleMedium?.copyWith(
                  fontWeight: FontWeight.bold,
                  color: theme.textMain,
                ),
              ),
            ),
            const SizedBox(width: 8),
            OutlinedButton.icon(
              icon: const Icon(Icons.delete_sweep, size: 18),
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
                    await ref.read(memoryFilesProvider.notifier).clearAll();
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
        const SizedBox(height: 12),
        memoryAsync.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (e, _) => Center(child: Text('${L10n.t('error')}: ${FriendlyError.describeGeneric(e)}')),
          data: (files) {
            if (files.isEmpty) {
              return Padding(
                padding: const EdgeInsets.only(top: 40),
                child: Center(
                  child: Text(
                    L10n.t('no_memory_files'),
                    style: TextStyle(color: theme.textDim),
                  ),
                ),
              );
            }

            // Collapsed by default, and the list itself is a bounded-
            // height ListView.builder rather than an eager Column — a
            // memory store with hundreds of entries used to build every
            // ListTile/Divider up front (rebuilt on every setState in this
            // tab, e.g. _loadStats/_runDebugSearch), which is what caused
            // the RAM/lag complaint. Collapsed, zero row widgets exist at
            // all; expanded, only the rows actually visible in the fixed-
            // height viewport are built (ListView.builder's laziness only
            // works with a bounded height).
            return Theme(
              data: Theme.of(
                context,
              ).copyWith(dividerColor: Colors.transparent),
              child: ExpansionTile(
                tilePadding: EdgeInsets.zero,
                childrenPadding: EdgeInsets.zero,
                title: Text(
                  L10n.t('memory_files_show', {'n': '${files.length}'}),
                  style: TextStyle(fontSize: 13, color: theme.textMain),
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
                                style: const TextStyle(
                                  fontWeight: FontWeight.w500,
                                ),
                              ),
                              subtitle: Text(
                                '${file.sizeKb} KB • ${file.modified}',
                                style: TextStyle(
                                  color: theme.textDim,
                                  fontSize: 12,
                                ),
                              ),
                              trailing: IconButton(
                                icon: const Icon(Icons.delete_outline),
                                color: MemoTheme.red,
                                onPressed: () async {
                                  try {
                                    await ref
                                        .read(memoryFilesProvider.notifier)
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
                            const Divider(height: 1),
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
      ],
    );
  }

  Widget _buildAnalyticsSection(BuildContext context) {
    final theme = MemoTheme.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Row(
          children: [
            Text(
              L10n.t('memory_stats_title'),
              style: Theme.of(context).textTheme.titleMedium?.copyWith(
                fontWeight: FontWeight.bold,
                color: theme.textMain,
              ),
            ),
            const SizedBox(width: 8),
            if (_statsLoading)
              const SizedBox(
                width: 14,
                height: 14,
                child: CircularProgressIndicator(strokeWidth: 2),
              )
            else
              IconButton(
                icon: const Icon(Icons.refresh, size: 18),
                color: theme.textDim,
                onPressed: _loadStats,
                tooltip: L10n.t('refresh'),
                padding: EdgeInsets.zero,
                constraints: const BoxConstraints(),
              ),
          ],
        ),
        const SizedBox(height: 12),
        if (_memoryStats != null)
          Container(
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(
              color: theme.bgPanel,
              borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
              border: Border.all(color: theme.borderSoft),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Wrap(
                  spacing: 8,
                  runSpacing: 8,
                  children: [
                    _StatChip(
                      label: L10n.t('memory_stats_total'),
                      value: '${_memoryStats!.count}',
                    ),
                    _StatChip(
                      label: L10n.t('memory_stats_pinned'),
                      value: '${_memoryStats!.explicitCount}',
                      accent: true,
                    ),
                    _StatChip(
                      label: L10n.t('memory_stats_pinned_tokens'),
                      value: _formatCompactCount(_memoryStats!.pinnedTokens),
                      accent: true,
                    ),
                    _StatChip(
                      label: L10n.t('memory_stats_this_week'),
                      value: '+${_memoryStats!.addedThisWeek}',
                    ),
                    if (_memoryStats!.pendingDeletion > 0)
                      _StatChip(
                        label: L10n.t('memory_stats_expiring'),
                        value: '${_memoryStats!.pendingDeletion}',
                        warn: true,
                      ),
                  ],
                ),
                if (_memoryStats!.topRetrieved.isNotEmpty) ...[
                  const SizedBox(height: 16),
                  Text(
                    L10n.t('memory_stats_most_accessed'),
                    style: TextStyle(
                      fontSize: 12,
                      fontWeight: FontWeight.w600,
                      color: theme.textDim,
                    ),
                  ),
                  const SizedBox(height: 8),
                  ..._memoryStats!.topRetrieved.asMap().entries.map((e) {
                    final r = e.value;
                    return Padding(
                      padding: const EdgeInsets.only(bottom: 6),
                      child: Row(
                        children: [
                          SizedBox(
                            width: 20,
                            child: Text(
                              '#${e.key + 1}',
                              textAlign: TextAlign.center,
                              style: TextStyle(
                                fontSize: 11,
                                color: theme.textDim,
                              ),
                            ),
                          ),
                          const SizedBox(width: 8),
                          Expanded(
                            child: Text(
                              r.content.length > 80
                                  ? '${r.content.substring(0, 80)}…'
                                  : r.content,
                              style: TextStyle(
                                fontSize: 12,
                                color: theme.textMain,
                              ),
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                            ),
                          ),
                          const SizedBox(width: 8),
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
          )
        else if (_statsLoading)
          const Padding(
            padding: EdgeInsets.only(top: 40),
            child: Center(child: CircularProgressIndicator()),
          ),
      ],
    );
  }

  Widget _buildFactsSection(BuildContext context) {
    final theme = MemoTheme.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Row(
          children: [
            Expanded(
              child: Text(
                L10n.t('memory_known_facts_title'),
                style: Theme.of(context).textTheme.titleMedium?.copyWith(
                  fontWeight: FontWeight.bold,
                  color: theme.textMain,
                ),
              ),
            ),
            TextButton.icon(
              onPressed: _factsBusy ? null : _showAddFactDialog,
              icon: const Icon(Icons.add, size: 16),
              label: Text(L10n.t('memory_known_facts_add_btn')),
            ),
            const SizedBox(width: 4),
            TextButton(
              onPressed: _knownFactsLoading ? null : _loadKnownFacts,
              child: _knownFactsLoading
                  ? const SizedBox(
                      width: 14,
                      height: 14,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : Text(L10n.t('memory_known_facts_refresh_btn')),
            ),
          ],
        ),
        const SizedBox(height: 8),
        Text(
          L10n.t('memory_known_facts_hint'),
          style: TextStyle(color: theme.textDim, fontSize: 13),
        ),
        const SizedBox(height: 12),
        if (_knownFactsError != null)
          Text(
            _knownFactsError!,
            style: TextStyle(color: MemoTheme.red, fontSize: 12),
          )
        else if (!_knownFactsLoading && _knownFacts.isEmpty)
          Text(
            L10n.t('memory_known_facts_empty'),
            style: TextStyle(color: theme.textDim, fontSize: 13),
          )
        else if (_knownFacts.isNotEmpty) ...[
          _SelectionBar(
            selectedCount: _selectedFactIds.length,
            busy: _factsBusy,
            onSelectAll: () => setState(
              () => _selectedFactIds.addAll(_knownFacts.map((f) => f.id)),
            ),
            onDeselectAll: () => setState(() => _selectedFactIds.clear()),
            onDeleteSelected: () =>
                _deleteIds(_selectedFactIds.toList(), isFacts: true),
          ),
          const SizedBox(height: 8),
          // Same bounded-height ListView.builder reasoning as the
          // debug-search results list — a well-populated pinned set
          // shouldn't eagerly build every decorated row up front.
          SizedBox(
            height: 420,
            child: ListView.builder(
              itemCount: _knownFacts.length,
              itemBuilder: (context, i) {
                final f = _knownFacts[i];
                return _SelectableMemoryTile(
                  selected: _selectedFactIds.contains(f.id),
                  onSelectedChanged: (v) => setState(() {
                    if (v == true) {
                      _selectedFactIds.add(f.id);
                    } else {
                      _selectedFactIds.remove(f.id);
                    }
                  }),
                  content: f.content,
                  timestamp: f.timestamp,
                  busy: _factsBusy,
                  onEdit: () => _showEditFactDialog(f),
                  onDelete: () => _deleteIds([f.id], isFacts: true),
                );
              },
            ),
          ),
        ],
      ],
    );
  }

  Widget _buildHistorySection(BuildContext context) {
    final theme = MemoTheme.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Row(
          children: [
            Expanded(
              child: Text(
                L10n.t('memory_conversation_title'),
                style: Theme.of(context).textTheme.titleMedium?.copyWith(
                  fontWeight: FontWeight.bold,
                  color: theme.textMain,
                ),
              ),
            ),
            TextButton(
              onPressed: _convLoading
                  ? null
                  : () => _loadConversationMemories(),
              child: _convLoading
                  ? const SizedBox(
                      width: 14,
                      height: 14,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : Text(L10n.t('memory_known_facts_refresh_btn')),
            ),
          ],
        ),
        const SizedBox(height: 8),
        Text(
          L10n.t('memory_conversation_hint'),
          style: TextStyle(color: theme.textDim, fontSize: 13),
        ),
        const SizedBox(height: 12),
        if (_convError != null)
          Text(
            _convError!,
            style: TextStyle(color: MemoTheme.red, fontSize: 12),
          )
        else if (!_convLoading && _convMemories.isEmpty)
          Text(
            L10n.t('memory_conversation_empty'),
            style: TextStyle(color: theme.textDim, fontSize: 13),
          )
        else if (_convMemories.isNotEmpty) ...[
          _SelectionBar(
            selectedCount: _selectedConvIds.length,
            busy: _convBusy,
            onSelectAll: () => setState(
              () => _selectedConvIds.addAll(_convMemories.map((f) => f.id)),
            ),
            onDeselectAll: () => setState(() => _selectedConvIds.clear()),
            onDeleteSelected: () =>
                _deleteIds(_selectedConvIds.toList(), isFacts: false),
          ),
          const SizedBox(height: 8),
          SizedBox(
            height: 420,
            child: ListView.builder(
              itemCount: _convMemories.length,
              itemBuilder: (context, i) {
                final m = _convMemories[i];
                return _SelectableMemoryTile(
                  selected: _selectedConvIds.contains(m.id),
                  onSelectedChanged: (v) => setState(() {
                    if (v == true) {
                      _selectedConvIds.add(m.id);
                    } else {
                      _selectedConvIds.remove(m.id);
                    }
                  }),
                  content: m.content,
                  timestamp: m.timestamp,
                  busy: _convBusy,
                  onDelete: () => _deleteIds([m.id], isFacts: false),
                );
              },
            ),
          ),
          const SizedBox(height: 8),
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                L10n.t('memory_conversation_showing', {
                  'shown': '${_convMemories.length}',
                  'total': '$_convTotal',
                }),
                style: TextStyle(fontSize: 12, color: theme.textDim),
              ),
              if (_convMemories.length < _convTotal)
                TextButton(
                  onPressed: _convLoading
                      ? null
                      : () => _loadConversationMemories(reset: false),
                  child: Text(L10n.t('memory_conversation_load_more')),
                ),
            ],
          ),
        ],
      ],
    );
  }

  Widget _buildDebugSection(BuildContext context) {
    final theme = MemoTheme.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(
          L10n.t('memory_debug_search'),
          style: Theme.of(context).textTheme.titleMedium?.copyWith(
            fontWeight: FontWeight.bold,
            color: theme.textMain,
          ),
        ),
        const SizedBox(height: 8),
        Text(
          L10n.t('memory_debug_hint'),
          style: TextStyle(color: theme.textDim, fontSize: 13),
        ),
        const SizedBox(height: 12),
        Row(
          children: [
            Expanded(
              child: SizedBox(
                height: 36,
                child: TextField(
                  controller: _debugQueryController,
                  style: const TextStyle(fontSize: 13),
                  onSubmitted: (_) => _runDebugSearch(),
                  decoration: InputDecoration(
                    hintText: L10n.t('memory_debug_placeholder'),
                    contentPadding: const EdgeInsets.symmetric(
                      horizontal: 12,
                    ),
                    filled: true,
                    fillColor: theme.bgApp,
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(
                        MemoTheme.radiusSm,
                      ),
                      borderSide: BorderSide(color: theme.borderSoft),
                    ),
                    enabledBorder: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(
                        MemoTheme.radiusSm,
                      ),
                      borderSide: BorderSide(color: theme.borderSoft),
                    ),
                    focusedBorder: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(
                        MemoTheme.radiusSm,
                      ),
                      borderSide: const BorderSide(color: MemoTheme.accent),
                    ),
                  ),
                ),
              ),
            ),
            const SizedBox(width: 8),
            SizedBox(
              height: 36,
              child: ElevatedButton(
                onPressed: _debugSearching ? null : _runDebugSearch,
                child: _debugSearching
                    ? const SizedBox(
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
            padding: const EdgeInsets.only(top: 8),
            child: Text(
              _debugError!,
              style: TextStyle(color: MemoTheme.red, fontSize: 12),
            ),
          ),
        if (_debugSearched && _debugResults.isEmpty && _debugError == null)
          Padding(
            padding: const EdgeInsets.only(top: 12),
            child: Text(
              L10n.t('memory_debug_no_results'),
              style: TextStyle(color: theme.textDim, fontSize: 13),
            ),
          ),
        if (_debugResults.isNotEmpty)
          Padding(
            padding: const EdgeInsets.only(top: 12),
            // Bounded-height ListView.builder, not an eager Column — each
            // result renders a decorated Container (border + rounded
            // corners, pricier than a plain ListTile), and with a well-
            // populated memory store this list is what caused the RAM/lag
            // complaint alongside the Memory Files list above. Same fix:
            // only the rows actually scrolled into view get built.
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
                    margin: const EdgeInsets.only(bottom: 8),
                    padding: const EdgeInsets.all(12),
                    decoration: BoxDecoration(
                      color: theme.bgApp,
                      borderRadius: BorderRadius.circular(
                        MemoTheme.radiusSm,
                      ),
                      border: Border.all(color: theme.borderSoft),
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
                                color: theme.textDim,
                                fontSize: 12,
                              ),
                            ),
                            const SizedBox(width: 8),
                            Container(
                              padding: const EdgeInsets.symmetric(
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
                                style: const TextStyle(
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
                                color: theme.textDim,
                                fontSize: 11,
                              ),
                            ),
                          ],
                        ),
                        const SizedBox(height: 6),
                        Text(
                          r.content.length > 200
                              ? '${r.content.substring(0, 200)}…'
                              : r.content,
                          style: TextStyle(
                            fontSize: 12,
                            color: theme.textMain,
                          ),
                        ),
                        const SizedBox(height: 4),
                        Text(
                          r.timestamp,
                          style: TextStyle(
                            fontSize: 11,
                            color: theme.textDim,
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
    );
  }
}

/// Selection toolbar shared by the pinned-facts and conversation-history
/// lists — select-all/deselect-all (scoped to whatever is currently
/// loaded, not the whole store) plus a delete-selected action that's
/// disabled until at least one row is checked, so there's no way to fire
/// a bulk delete with an empty/accidental selection.
class _SelectionBar extends StatelessWidget {
  final int selectedCount;
  final bool busy;
  final VoidCallback onSelectAll;
  final VoidCallback onDeselectAll;
  final VoidCallback onDeleteSelected;

  const _SelectionBar({
    required this.selectedCount,
    required this.busy,
    required this.onSelectAll,
    required this.onDeselectAll,
    required this.onDeleteSelected,
  });

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    // Wrap, not Row+Spacer — four independently-sized, localized labels
    // (two text buttons, an optional count, a delete button whose label
    // grows with the count) packed into one Row never fit at narrow
    // dialog widths; Wrap folds the overflow onto a second line instead
    // of throwing a RenderFlex overflow (see AGENTS.md's Flutter gotcha
    // on localized action rows).
    return Wrap(
      spacing: 8,
      runSpacing: 4,
      crossAxisAlignment: WrapCrossAlignment.center,
      children: [
        TextButton(
          onPressed: busy ? null : onSelectAll,
          child: Text(L10n.t('memory_select_all_btn')),
        ),
        TextButton(
          onPressed: busy ? null : onDeselectAll,
          child: Text(L10n.t('memory_deselect_all_btn')),
        ),
        if (selectedCount > 0)
          Text(
            L10n.t('memory_selection_count', {'n': '$selectedCount'}),
            style: TextStyle(fontSize: 12, color: theme.textDim),
          ),
        OutlinedButton.icon(
          icon: busy
              ? SizedBox(
                  width: 14,
                  height: 14,
                  child: CircularProgressIndicator(strokeWidth: 2),
                )
              : Icon(Icons.delete_sweep, size: 16),
          label: Text(
            L10n.t('memory_delete_selected_btn', {'n': '$selectedCount'}),
          ),
          style: OutlinedButton.styleFrom(
            foregroundColor: MemoTheme.red,
            side: BorderSide(color: MemoTheme.red),
          ),
          onPressed: (busy || selectedCount == 0) ? null : onDeleteSelected,
        ),
      ],
    );
  }
}

/// One row in the pinned-facts or conversation-history list: a checkbox
/// (a checked row gets a visibly highlighted border/background — a bare
/// checkbox with no other feedback is too easy to lose track of in a
/// scrolled list), the content + timestamp, an optional edit action
/// (pinned facts only — editing raw conversation history doesn't make
/// sense), and a delete action. `busy` disables all actions while a
/// bulk/single delete for this list is already in flight.
class _SelectableMemoryTile extends StatelessWidget {
  final bool selected;
  final ValueChanged<bool?> onSelectedChanged;
  final String content;
  final String timestamp;
  final bool busy;
  final VoidCallback? onEdit;
  final VoidCallback onDelete;

  const _SelectableMemoryTile({
    required this.selected,
    required this.onSelectedChanged,
    required this.content,
    required this.timestamp,
    required this.busy,
    this.onEdit,
    required this.onDelete,
  });

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    return Container(
      margin: EdgeInsets.only(bottom: 8),
      padding: EdgeInsets.only(right: 4),
      decoration: BoxDecoration(
        color: selected
            ? MemoTheme.accent.withValues(alpha: 0.08)
            : theme.bgApp,
        borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
        border: Border.all(
          color: selected
              ? MemoTheme.accent.withValues(alpha: 0.6)
              : theme.borderSoft,
        ),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Checkbox(
            value: selected,
            onChanged: busy ? null : onSelectedChanged,
            activeColor: MemoTheme.accent,
          ),
          Expanded(
            child: Padding(
              padding: EdgeInsets.symmetric(vertical: 10),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    content,
                    style: TextStyle(fontSize: 13, color: theme.textMain),
                  ),
                  SizedBox(height: 4),
                  Text(
                    timestamp,
                    style: TextStyle(fontSize: 11, color: theme.textDim),
                  ),
                ],
              ),
            ),
          ),
          if (onEdit != null)
            IconButton(
              icon: Icon(Icons.edit_outlined, size: 18),
              color: theme.textDim,
              tooltip: L10n.t('memory_known_facts_edit_tooltip'),
              onPressed: busy ? null : onEdit,
            ),
          IconButton(
            icon: Icon(Icons.delete_outline, size: 18),
            color: MemoTheme.red,
            tooltip: L10n.t('memory_delete_tooltip'),
            onPressed: busy ? null : onDelete,
          ),
        ],
      ),
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
