import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:file_picker/file_picker.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/curated_models.dart';
import '../models/gpu_info.dart';
import '../models/local_model.dart';
import '../providers/chat_provider.dart';
import '../providers/models_provider.dart';
import '../widgets/model_config_dialog.dart';

/// Model screen — guided, recommendation-first. Two tabs: Discover (curated
/// download) and My Models (local + start/stop). Quantization is hidden behind
/// a smart default; hardware fit is surfaced up front.
class ModelStoreScreen extends ConsumerStatefulWidget {
  const ModelStoreScreen({super.key});

  @override
  ConsumerState<ModelStoreScreen> createState() => _ModelStoreScreenState();
}

class _ModelStoreScreenState extends ConsumerState<ModelStoreScreen> {
  int _tab = 0; // 0 = Discover, 1 = My Models

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

// ─── Header: title + segmented tabs + hardware chip ──────────────

class _Header extends ConsumerWidget {
  final int tab;
  final ValueChanged<int> onTab;
  const _Header({required this.tab, required this.onTab});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = MemoTheme.of(context);
    return Container(
      padding: const EdgeInsets.fromLTRB(32, 24, 32, 16),
      decoration: BoxDecoration(
        border: Border(bottom: BorderSide(color: c.borderSoft)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                L10n.t('nav_models'),
                style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                      fontWeight: FontWeight.w600,
                      color: c.textMain,
                    ),
              ),
              const _HardwareChip(),
            ],
          ),
          const SizedBox(height: 16),
          _SegmentedTabs(tab: tab, onTab: onTab),
        ],
      ),
    );
  }
}

class _SegmentedTabs extends StatelessWidget {
  final int tab;
  final ValueChanged<int> onTab;
  const _SegmentedTabs({required this.tab, required this.onTab});

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    return Container(
      padding: const EdgeInsets.all(3),
      decoration: BoxDecoration(
        color: c.bgElement,
        borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          _segment(context, 0, Icons.explore_outlined, L10n.t('tab_discover')),
          _segment(context, 1, Icons.folder_outlined, L10n.t('tab_my_models')),
        ],
      ),
    );
  }

  Widget _segment(BuildContext context, int index, IconData icon, String label) {
    final c = MemoTheme.of(context);
    final active = tab == index;
    return MouseRegion(
      cursor: SystemMouseCursors.click,
      child: GestureDetector(
        onTap: () => onTab(index),
        child: AnimatedContainer(
          duration: const Duration(milliseconds: 200),
          padding: const EdgeInsets.symmetric(horizontal: 18, vertical: 9),
          decoration: BoxDecoration(
            color: active ? MemoTheme.accent : Colors.transparent,
            borderRadius: BorderRadius.circular(MemoTheme.radiusSm - 2),
          ),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(icon, size: 16, color: active ? c.textInverse : c.textMuted),
              const SizedBox(width: 8),
              Text(
                label,
                style: TextStyle(
                  fontSize: 13,
                  fontWeight: FontWeight.w600,
                  color: active ? c.textInverse : c.textMuted,
                ),
              ),
            ],
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
    final gpuAsync = ref.watch(gpuInfoProvider);
    final gpu = gpuAsync.valueOrNull ?? const GPUInfo();

    final parts = <String>[];
    if (gpu.ramTotalMb > 0) parts.add('${gpu.ramFormatted} RAM');
    if (gpu.hasGpu) {
      parts.add(gpu.vramMb > 0 ? '${gpu.name} ${gpu.vramFormatted}' : gpu.name);
    } else {
      parts.add('CPU');
    }

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 7),
      decoration: BoxDecoration(
        color: c.bgElement,
        borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
        border: Border.all(color: c.borderSoft),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(gpu.hasGpu ? Icons.memory : Icons.developer_board,
              size: 15, color: MemoTheme.accent),
          const SizedBox(width: 8),
          Text(
            '${L10n.t('hw_your_device')}: ${parts.join(' · ')}',
            style: TextStyle(
                fontSize: 12, fontWeight: FontWeight.w500, color: c.textMuted),
          ),
        ],
      ),
    );
  }
}

// ─── Fit badge ───────────────────────────────────────────────────

class _FitBadge extends StatelessWidget {
  final HardwareFit fit;
  const _FitBadge({required this.fit});

  @override
  Widget build(BuildContext context) {
    final (color, icon) = switch (fit.level) {
      FitLevel.good => (MemoTheme.green, Icons.check_circle_outline),
      FitLevel.ok => (MemoTheme.accent, Icons.check_circle_outline),
      FitLevel.warn => (MemoTheme.warningOrange, Icons.warning_amber_rounded),
    };
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(icon, size: 14, color: color),
        const SizedBox(width: 6),
        Flexible(
          child: Text(
            fit.label,
            style: TextStyle(fontSize: 12, color: color, fontWeight: FontWeight.w500),
          ),
        ),
      ],
    );
  }
}

String _formatGb(int bytes) {
  if (bytes >= 1024 * 1024 * 1024) {
    return '${(bytes / (1024 * 1024 * 1024)).toStringAsFixed(1)} GB';
  }
  return '${(bytes / (1024 * 1024)).toStringAsFixed(0)} MB';
}

// ─── Discover tab ────────────────────────────────────────────────

class _DiscoverTab extends ConsumerStatefulWidget {
  const _DiscoverTab();

  @override
  ConsumerState<_DiscoverTab> createState() => _DiscoverTabState();
}

class _DiscoverTabState extends ConsumerState<_DiscoverTab> {
  bool _advancedOpen = false;

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    return ListView(
      padding: const EdgeInsets.all(32),
      children: [
        Text(
          L10n.t('recommended_for_you'),
          style: TextStyle(
              fontSize: 15, fontWeight: FontWeight.w600, color: c.textMain),
        ),
        const SizedBox(height: 16),
        LayoutBuilder(builder: (context, constraints) {
          final cols = constraints.maxWidth > 760 ? 2 : 1;
          const gap = 16.0;
          final cardW = (constraints.maxWidth - gap * (cols - 1)) / cols;
          return Wrap(
            spacing: gap,
            runSpacing: gap,
            children: [
              for (final m in curatedModels)
                SizedBox(width: cardW, child: _RecommendedCard(model: m)),
            ],
          );
        }),
        const SizedBox(height: 28),
        Divider(color: c.borderSoft),
        const SizedBox(height: 12),
        _AdvancedToggle(
          open: _advancedOpen,
          onToggle: () => setState(() => _advancedOpen = !_advancedOpen),
        ),
        if (_advancedOpen) ...[
          const SizedBox(height: 16),
          const _AdvancedSearch(),
        ],
      ],
    );
  }
}

class _AdvancedToggle extends StatelessWidget {
  final bool open;
  final VoidCallback onToggle;
  const _AdvancedToggle({required this.open, required this.onToggle});

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    return MouseRegion(
      cursor: SystemMouseCursors.click,
      child: GestureDetector(
        onTap: onToggle,
        child: Row(
          children: [
            Icon(open ? Icons.expand_less : Icons.expand_more,
                size: 18, color: c.textMuted),
            const SizedBox(width: 8),
            Text(
              open
                  ? L10n.t('advanced_search_close')
                  : L10n.t('advanced_search_open'),
              style: TextStyle(
                  fontSize: 13, color: c.textMuted, fontWeight: FontWeight.w500),
            ),
          ],
        ),
      ),
    );
  }
}

/// True while any model download is active — used to disable other download
/// buttons so two downloads never collide.
final _downloadingNowProvider = Provider.autoDispose<bool>((ref) {
  return ref.watch(downloadProgressProvider).valueOrNull?.active ?? false;
});

class _RecommendedCard extends ConsumerStatefulWidget {
  final CuratedModel model;
  const _RecommendedCard({required this.model});

  @override
  ConsumerState<_RecommendedCard> createState() => _RecommendedCardState();
}

class _RecommendedCardState extends ConsumerState<_RecommendedCard> {
  bool _preparing = false;
  bool _hover = false;

  Future<void> _download() async {
    setState(() => _preparing = true);
    final api = ref.read(apiClientProvider);
    final messenger = ScaffoldMessenger.of(context);
    try {
      final files = await api.getModelFiles(widget.model.repoId);
      final pick = pickRecommendedFile(files);
      if (pick == null) {
        if (mounted) {
          messenger.showSnackBar(
              SnackBar(content: Text(L10n.t('no_files_found'))));
        }
        return;
      }
      await api.downloadModel(widget.model.repoId, pick.filename);
      if (mounted) {
        messenger.showSnackBar(SnackBar(
          content: Text(L10n.t('download_started', {'file': pick.filename})),
          backgroundColor: MemoTheme.green,
        ));
      }
    } catch (e) {
      if (mounted) {
        messenger.showSnackBar(
            SnackBar(content: Text(L10n.t('import_error', {'e': e.toString()}))));
      }
    } finally {
      if (mounted) setState(() => _preparing = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final gpu = ref.watch(gpuInfoProvider).valueOrNull ?? const GPUInfo();
    final fit = hardwareFit(widget.model.approxBytes, gpu);
    final downloadingElsewhere = ref.watch(_downloadingNowProvider);
    final localAsync = ref.watch(localModelsProvider);
    final alreadyHave = localAsync.valueOrNull?.any((m) =>
            m.repoId.toLowerCase() == widget.model.repoId.toLowerCase()) ??
        false;

    final (kindIcon, kindLabel) = switch (widget.model.kind) {
      ModelKind.chat => (Icons.chat_bubble_outline, L10n.t('kind_chat')),
      ModelKind.memory => (Icons.hub_outlined, L10n.t('kind_memory')),
      ModelKind.vision => (Icons.image_outlined, L10n.t('kind_vision')),
    };

    return MouseRegion(
      onEnter: (_) => setState(() => _hover = true),
      onExit: (_) => setState(() => _hover = false),
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 200),
        padding: const EdgeInsets.all(20),
        decoration: BoxDecoration(
          color: c.bgPanel,
          borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
          border: Border.all(
            color: _hover ? c.borderHover : c.borderSoft,
          ),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Container(
                  width: 38,
                  height: 38,
                  decoration: BoxDecoration(
                    color: MemoTheme.accentMuted,
                    borderRadius: BorderRadius.circular(10),
                  ),
                  child: Icon(kindIcon, size: 19, color: MemoTheme.accent),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        widget.model.name,
                        style: TextStyle(
                            fontWeight: FontWeight.w600,
                            fontSize: 15,
                            color: c.textMain),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                      const SizedBox(height: 2),
                      Text(
                        widget.model.desc,
                        style: TextStyle(fontSize: 13, color: c.textMuted),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ],
                  ),
                ),
              ],
            ),
            const SizedBox(height: 16),
            Row(
              children: [
                _Pill(text: kindLabel),
                const SizedBox(width: 8),
                _Pill(text: '~${_formatGb(widget.model.approxBytes)}'),
              ],
            ),
            const SizedBox(height: 12),
            _FitBadge(fit: fit),
            const SizedBox(height: 16),
            SizedBox(
              width: double.infinity,
              child: alreadyHave
                  ? OutlinedButton.icon(
                      onPressed: null,
                      icon: const Icon(Icons.check, size: 18),
                      label: Text(L10n.t('downloaded_ready')),
                    )
                  : ElevatedButton.icon(
                      onPressed: (_preparing || downloadingElsewhere)
                          ? null
                          : _download,
                      icon: _preparing
                          ? const SizedBox(
                              width: 16,
                              height: 16,
                              child: CircularProgressIndicator(strokeWidth: 2),
                            )
                          : const Icon(Icons.download_rounded, size: 18),
                      label: Text(_preparing
                          ? L10n.t('preparing_download')
                          : L10n.t('download')),
                    ),
            ),
          ],
        ),
      ),
    );
  }
}

class _Pill extends StatelessWidget {
  final String text;
  const _Pill({required this.text});

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(
        color: c.bgElement,
        borderRadius: BorderRadius.circular(6),
      ),
      child: Text(
        text,
        style: TextStyle(fontSize: 11, fontWeight: FontWeight.w500, color: c.textMuted),
      ),
    );
  }
}

// ─── Advanced search (collapsible) ───────────────────────────────

class _AdvancedSearch extends ConsumerStatefulWidget {
  const _AdvancedSearch();

  @override
  ConsumerState<_AdvancedSearch> createState() => _AdvancedSearchState();
}

class _AdvancedSearchState extends ConsumerState<_AdvancedSearch> {
  final _controller = TextEditingController();
  Timer? _debounce;

  @override
  void dispose() {
    _debounce?.cancel();
    _controller.dispose();
    super.dispose();
  }

  void _onChanged(String val) {
    _debounce?.cancel();
    if (val.length < 2) {
      ref.read(modelSearchQueryProvider.notifier).state = '';
      return;
    }
    _debounce = Timer(const Duration(milliseconds: 400), () {
      ref.read(modelSearchQueryProvider.notifier).state = val;
    });
  }

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final query = ref.watch(modelSearchQueryProvider);
    final resultsAsync = ref.watch(modelSearchResultsProvider);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        TextField(
          controller: _controller,
          onChanged: _onChanged,
          decoration: InputDecoration(
            hintText: L10n.t('search_other_models'),
            prefixIcon: Icon(Icons.search, color: c.textDim),
          ),
        ),
        const SizedBox(height: 16),
        if (query.isEmpty)
          Padding(
            padding: const EdgeInsets.symmetric(vertical: 24),
            child: Text(
              L10n.t('model_search_placeholder'),
              textAlign: TextAlign.center,
              style: TextStyle(color: c.textDim),
            ),
          )
        else
          resultsAsync.when(
            loading: () => const Padding(
              padding: EdgeInsets.all(24),
              child: Center(child: CircularProgressIndicator()),
            ),
            error: (e, _) => Padding(
              padding: const EdgeInsets.all(24),
              child: Text('${L10n.t('error')}: $e',
                  style: const TextStyle(color: MemoTheme.red)),
            ),
            data: (results) {
              if (results.isEmpty) {
                return Padding(
                  padding: const EdgeInsets.all(24),
                  child: Center(
                    child: Text(L10n.t('no_search_results'),
                        style: TextStyle(color: c.textDim)),
                  ),
                );
              }
              return Column(
                children: [
                  for (final r in results)
                    Padding(
                      padding: const EdgeInsets.only(bottom: 10),
                      child: _SearchResultRow(result: r),
                    ),
                ],
              );
            },
          ),
      ],
    );
  }
}

class _SearchResultRow extends StatelessWidget {
  final HFModelResult result;
  const _SearchResultRow({required this.result});

  String _fmt(int n) {
    if (n >= 1000000) return '${(n / 1000000).toStringAsFixed(1)}M';
    if (n >= 1000) return '${(n / 1000).toStringAsFixed(1)}K';
    return '$n';
  }

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: c.bgPanel,
        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
        border: Border.all(color: c.borderSoft),
      ),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  result.id,
                  style: TextStyle(
                      fontWeight: FontWeight.w600, fontSize: 14, color: c.textMain),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
                const SizedBox(height: 4),
                Row(
                  children: [
                    Icon(Icons.download_rounded, size: 13, color: c.textDim),
                    const SizedBox(width: 4),
                    Text(_fmt(result.downloads),
                        style: TextStyle(fontSize: 12, color: c.textDim)),
                    const SizedBox(width: 12),
                    Icon(Icons.favorite_border_rounded, size: 13, color: c.textDim),
                    const SizedBox(width: 4),
                    Text(_fmt(result.likes),
                        style: TextStyle(fontSize: 12, color: c.textDim)),
                  ],
                ),
              ],
            ),
          ),
          const SizedBox(width: 12),
          OutlinedButton(
            onPressed: () => showDialog(
              context: context,
              builder: (_) => _ModelFilesDialog(repoId: result.id),
            ),
            child: Text(L10n.t('other_versions')),
          ),
        ],
      ),
    );
  }
}

// ─── Files dialog (plain-language quant) ─────────────────────────

class _ModelFilesDialog extends ConsumerStatefulWidget {
  final String repoId;
  const _ModelFilesDialog({required this.repoId});

  @override
  ConsumerState<_ModelFilesDialog> createState() => _ModelFilesDialogState();
}

class _ModelFilesDialogState extends ConsumerState<_ModelFilesDialog> {
  List<GGUFFile>? _files;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final files = await ref.read(apiClientProvider).getModelFiles(widget.repoId);
      files.sort((a, b) =>
          quantInfo(b.filename).priority.compareTo(quantInfo(a.filename).priority));
      if (mounted) setState(() => _files = files);
    } catch (e) {
      if (mounted) setState(() => _error = e.toString());
    }
  }

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final gpu = ref.watch(gpuInfoProvider).valueOrNull ?? const GPUInfo();
    return Dialog(
      child: Container(
        width: 620,
        constraints: const BoxConstraints(maxHeight: 560),
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Row(
              children: [
                Icon(Icons.folder_open_rounded, color: MemoTheme.accent),
                const SizedBox(width: 12),
                Expanded(
                  child: Text(widget.repoId,
                      style: TextStyle(
                          fontSize: 15,
                          fontWeight: FontWeight.w600,
                          color: c.textMain),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis),
                ),
                IconButton(
                    onPressed: () => Navigator.pop(context),
                    icon: const Icon(Icons.close)),
              ],
            ),
            const SizedBox(height: 16),
            Flexible(
              child: _error != null
                  ? Center(
                      child: Text('${L10n.t('error')}: $_error',
                          style: const TextStyle(color: MemoTheme.red)))
                  : _files == null
                      ? const Center(child: CircularProgressIndicator())
                      : _files!.isEmpty
                          ? Center(
                              child: Text(L10n.t('no_files_found'),
                                  style: TextStyle(color: c.textDim)))
                          : ListView.separated(
                              shrinkWrap: true,
                              itemCount: _files!.length,
                              separatorBuilder: (_, __) => const SizedBox(height: 8),
                              itemBuilder: (context, i) =>
                                  _fileRow(context, _files![i], gpu),
                            ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _fileRow(BuildContext context, GGUFFile file, GPUInfo gpu) {
    final c = MemoTheme.of(context);
    final q = quantInfo(file.filename);
    final fit = hardwareFit(file.size, gpu);
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: c.bgPanel,
        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
        border: Border.all(color: c.borderSoft),
      ),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Text(q.label,
                        style: TextStyle(
                            fontSize: 14,
                            fontWeight: FontWeight.w600,
                            color: c.textMain)),
                    const SizedBox(width: 8),
                    _Pill(text: file.sizeFormatted),
                  ],
                ),
                const SizedBox(height: 6),
                _FitBadge(fit: fit),
              ],
            ),
          ),
          const SizedBox(width: 12),
          ElevatedButton(
            onPressed: () {
              ref.read(apiClientProvider).downloadModel(widget.repoId, file.filename);
              Navigator.pop(context);
              ScaffoldMessenger.of(context).showSnackBar(SnackBar(
                content: Text(L10n.t('download_started', {'file': file.filename})),
                backgroundColor: MemoTheme.green,
              ));
            },
            child: Text(L10n.t('download')),
          ),
        ],
      ),
    );
  }
}

// ─── My Models tab ───────────────────────────────────────────────

class _MyModelsTab extends ConsumerWidget {
  const _MyModelsTab();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = MemoTheme.of(context);
    final localAsync = ref.watch(localModelsProvider);

    return localAsync.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (e, _) => Center(child: Text('${L10n.t('error')}: $e')),
      data: (models) {
        return Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(32, 20, 32, 8),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Text(L10n.t('local_models'),
                      style: TextStyle(
                          fontSize: 15,
                          fontWeight: FontWeight.w600,
                          color: c.textMain)),
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
                      padding: const EdgeInsets.fromLTRB(32, 8, 32, 32),
                      itemCount: models.length,
                      separatorBuilder: (_, __) => const SizedBox(height: 12),
                      itemBuilder: (context, i) => _LocalModelCard(model: models[i]),
                    ),
            ),
          ],
        );
      },
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
      messenger.showSnackBar(SnackBar(content: Text(L10n.t('import_success'))));
    } catch (e) {
      messenger.showSnackBar(
          SnackBar(content: Text(L10n.t('import_error', {'e': e.toString()}))));
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
                decoration: BoxDecoration(
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
                  fontSize: 16, fontWeight: FontWeight.w600, color: c.textMain)),
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
            status?.modelPath.split('/').last == model.filename) ||
        (embStatus?.running == true &&
            embStatus?.modelPath.split('/').last == model.filename);
    final installed = ref.watch(llamaInstalledProvider).valueOrNull ?? false;

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: c.bgPanel,
        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
        border: Border.all(
            color: running ? MemoTheme.accent.withValues(alpha: 0.5) : c.borderSoft),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(model.filename,
                    style: TextStyle(
                        fontWeight: FontWeight.w600,
                        fontSize: 14,
                        color: c.textMain),
                    overflow: TextOverflow.ellipsis),
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
          const SizedBox(height: 4),
          Text(model.repoId,
              style: TextStyle(fontSize: 11, color: c.textDim),
              maxLines: 1,
              overflow: TextOverflow.ellipsis),
          const SizedBox(height: 12),
          Row(
            children: [
              _Pill(
                  text: model.isEmbedding
                      ? L10n.t('kind_memory')
                      : (model.isVision ? L10n.t('kind_vision') : L10n.t('kind_chat'))),
              const SizedBox(width: 8),
              _Pill(text: model.sizeFormatted),
              const Spacer(),
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
                                builder: (_) => ModelConfigDialog(model: model),
                              ),
                      style: ElevatedButton.styleFrom(
                        minimumSize: const Size(0, 34),
                        padding: const EdgeInsets.symmetric(horizontal: 16),
                      ),
                      child: Text(
                          model.isEmbedding
                              ? L10n.t('start_embedding')
                              : L10n.t('start_model'),
                          style: const TextStyle(fontSize: 12)),
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
        content: Text(L10n.t('delete_model_confirm_name', {'name': model.filename})),
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

// ─── Download banner (inline progress) ───────────────────────────

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
      padding: const EdgeInsets.fromLTRB(32, 14, 32, 14),
      decoration: BoxDecoration(
        color: c.bgPanel,
        border: Border(bottom: BorderSide(color: c.borderSoft)),
      ),
      child: Row(
        children: [
          Icon(Icons.cloud_download_outlined, size: 20, color: MemoTheme.accent),
          const SizedBox(width: 14),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Expanded(
                      child: Text(progress.filename,
                          style: TextStyle(
                              fontSize: 13,
                              fontWeight: FontWeight.w600,
                              color: c.textMain),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis),
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
                const SizedBox(height: 8),
                ClipRRect(
                  borderRadius: BorderRadius.circular(999),
                  child: LinearProgressIndicator(
                    value: progress.percent / 100.0,
                    minHeight: 6,
                    backgroundColor: c.bgElement,
                    valueColor: const AlwaysStoppedAnimation(MemoTheme.accent),
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
