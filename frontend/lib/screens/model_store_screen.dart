import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:file_picker/file_picker.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../providers/chat_provider.dart';
import '../models/local_model.dart';
import '../providers/models_provider.dart';
import '../widgets/gpu_badge.dart';
import '../widgets/model_config_dialog.dart';

class ModelStoreScreen extends ConsumerStatefulWidget {
  ModelStoreScreen({super.key});

  @override
  ConsumerState<ModelStoreScreen> createState() => _ModelStoreScreenState();
}

class _ModelStoreScreenState extends ConsumerState<ModelStoreScreen> {
  final _searchController = TextEditingController();
  Timer? _debounce;

  @override
  void dispose() {
    _debounce?.cancel();
    _searchController.dispose();
    super.dispose();
  }

  void _onSearchChanged(String val) {
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
    return Container(
      color: MemoTheme.of(context).bgApp,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          // ─── Header ──────────────────────────────────────
          Container(
            padding: EdgeInsets.symmetric(horizontal: 32, vertical: 24),
            decoration: BoxDecoration(
              border: Border(
                bottom: BorderSide(color: MemoTheme.of(context).borderSoft),
              ),
            ),
            child: Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      L10n.t('model_store'),
                      style: Theme.of(context).textTheme.headlineSmall
                          ?.copyWith(
                            fontWeight: FontWeight.bold,
                            color: MemoTheme.of(context).textMain,
                          ),
                    ),
                    SizedBox(height: 4),
                    Text(
                      L10n.t('model_store_subtitle'),
                      style: TextStyle(color: MemoTheme.of(context).textDim),
                    ),
                  ],
                ),
                GPUBadge(),
              ],
            ),
          ),

          // ─── Content ─────────────────────────────────────
          Expanded(
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                // ─── Left Panel: Search & Download ─────────
                Expanded(
                  flex: 3,
                  child: Container(
                    decoration: BoxDecoration(
                      border: Border(
                        right: BorderSide(
                          color: MemoTheme.of(context).borderSoft,
                        ),
                      ),
                    ),
                    child: Column(
                      children: [
                        // Search Bar
                        Padding(
                          padding: EdgeInsets.all(24),
                          child: TextField(
                            controller: _searchController,
                            decoration: InputDecoration(
                              hintText: L10n.t('search_models'),
                              prefixIcon: Icon(
                                Icons.search,
                                color: MemoTheme.of(context).textDim,
                              ),
                              filled: true,
                              fillColor: MemoTheme.of(context).bgPanel,
                              border: OutlineInputBorder(
                                borderRadius: BorderRadius.circular(
                                  MemoTheme.radiusMd,
                                ),
                                borderSide: BorderSide(
                                  color: MemoTheme.of(context).borderSoft,
                                ),
                              ),
                              enabledBorder: OutlineInputBorder(
                                borderRadius: BorderRadius.circular(
                                  MemoTheme.radiusMd,
                                ),
                                borderSide: BorderSide(
                                  color: MemoTheme.of(context).borderSoft,
                                ),
                              ),
                            ),
                            onChanged: _onSearchChanged,
                            onSubmitted: (val) {
                              ref
                                      .read(modelSearchQueryProvider.notifier)
                                      .state =
                                  val;
                            },
                          ),
                        ),

                        // Search Results / Downloading
                        Expanded(child: _SearchResultsPanel()),
                      ],
                    ),
                  ),
                ),

                // ─── Right Panel: Local Models & Active ────
                Expanded(
                  flex: 2,
                  child: Container(
                    color: MemoTheme.of(context).bgPanel.withValues(alpha: 0.3),
                    child: Column(
                      children: [
                        // Active Model
                        Padding(
                          padding: EdgeInsets.all(24),
                          child: _ActiveModelCard(),
                        ),

                        // Download Progress
                        Padding(
                          padding: EdgeInsets.symmetric(horizontal: 24),
                          child: _DownloadProgressCard(),
                        ),

                        SizedBox(height: 24),
                        Divider(height: 1),

                        // Local Models
                        Expanded(child: _LocalModelsList()),
                      ],
                    ),
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

class _SearchResultsPanel extends ConsumerWidget {
  _SearchResultsPanel();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final query = ref.watch(modelSearchQueryProvider);
    final searchResultsAsync = ref.watch(modelSearchResultsProvider);

    if (query.isEmpty) {
      return Center(
        child: Text(
          L10n.t('model_search_placeholder'),
          textAlign: TextAlign.center,
          style: TextStyle(color: MemoTheme.of(context).textDim),
        ),
      );
    }

    return searchResultsAsync.when(
      loading: () => Center(child: CircularProgressIndicator()),
      error: (e, _) => Center(child: Text('${L10n.t('error')}: $e')),
      data: (results) {
        if (results.isEmpty) {
          return Center(
            child: Text(
              L10n.t('no_search_results'),
              style: TextStyle(color: MemoTheme.of(context).textDim),
            ),
          );
        }
        return ListView.separated(
          padding: EdgeInsets.symmetric(horizontal: 24, vertical: 8),
          itemCount: results.length,
          separatorBuilder: (_, __) => SizedBox(height: 12),
          itemBuilder: (context, index) {
            final res = results[index];
            return _SearchResultCard(result: res);
          },
        );
      },
    );
  }
}

class _SearchResultCard extends StatefulWidget {
  final HFModelResult result;

  _SearchResultCard({required this.result});

  @override
  State<_SearchResultCard> createState() => _SearchResultCardState();
}

class _SearchResultCardState extends State<_SearchResultCard> {
  bool _isHovered = false;

  static const _avatarColors = [
    Color(0xFFE53935),
    Color(0xFF1E88E5),
    Color(0xFF43A047),
    Color(0xFFFB8C00),
    Color(0xFF8E24AA),
    Color(0xFF00ACC1),
    Color(0xFFD81B60),
    Color(0xFF3949AB),
    Color(0xFF6D4C41),
  ];

  Color get _avatarColor {
    final hash = widget.result.id.hashCode;
    return _avatarColors[hash.abs() % _avatarColors.length];
  }

  String _formatCount(int count) {
    if (count >= 1000000) {
      return '${(count / 1000000).toStringAsFixed(1)}M';
    } else if (count >= 1000) {
      return '${(count / 1000).toStringAsFixed(1)}K';
    }
    return count.toString();
  }

  @override
  Widget build(BuildContext context) {
    return MouseRegion(
      onEnter: (_) => setState(() => _isHovered = true),
      onExit: (_) => setState(() => _isHovered = false),
      child: AnimatedContainer(
        duration: Duration(milliseconds: 200),
        padding: EdgeInsets.all(20),
        decoration: BoxDecoration(
          color: _isHovered
              ? MemoTheme.of(context).bgHover.withValues(alpha: 0.3)
              : MemoTheme.of(context).bgPanel,
          borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
          border: Border.all(
            color: _isHovered
                ? MemoTheme.accent.withValues(alpha: 0.5)
                : MemoTheme.of(context).borderSoft,
            width: _isHovered ? 1.5 : 1.0,
          ),
          boxShadow: _isHovered ? MemoTheme.shadowMd : [],
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                // Avatar (first letter of repo)
                Container(
                  width: 40,
                  height: 40,
                  margin: EdgeInsets.only(right: 12),
                  decoration: BoxDecoration(
                    color: _avatarColor,
                    borderRadius: BorderRadius.circular(10),
                  ),
                  child: Center(
                    child: Text(
                      widget.result.id.isNotEmpty
                          ? widget.result.id[0].toUpperCase()
                          : '?',
                      style: TextStyle(
                        color: Colors.white,
                        fontWeight: FontWeight.bold,
                        fontSize: 18,
                      ),
                    ),
                  ),
                ),
                Expanded(
                  child: Text(
                    widget.result.id,
                    style: TextStyle(
                      fontWeight: FontWeight.bold,
                      fontSize: 16,
                      color: MemoTheme.of(context).textMain,
                    ),
                  ),
                ),
                // Popular badge
                if (widget.result.downloads > 500000)
                  Container(
                    margin: EdgeInsets.only(right: 8),
                    padding: EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                    decoration: BoxDecoration(
                      color: MemoTheme.accent.withValues(alpha: 0.15),
                      borderRadius: BorderRadius.circular(20),
                    ),
                    child: Text(
                      L10n.t('popular_badge'),
                      style: TextStyle(
                        fontSize: 10,
                        fontWeight: FontWeight.w600,
                        color: MemoTheme.accent,
                      ),
                    ),
                  ),
                // GGUF badge
                if (widget.result.tags.any(
                  (t) => t.toLowerCase().contains('gguf'),
                ))
                  Container(
                    margin: EdgeInsets.only(right: 8),
                    padding: EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                    decoration: BoxDecoration(
                      color: MemoTheme.warmBrown.withValues(alpha: 0.15),
                      borderRadius: BorderRadius.circular(20),
                    ),
                    child: Text(
                      'GGUF',
                      style: TextStyle(
                        fontSize: 10,
                        fontWeight: FontWeight.w600,
                        color: MemoTheme.warmBrown,
                      ),
                    ),
                  ),
                Container(
                  padding: EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                  decoration: BoxDecoration(
                    color: MemoTheme.of(context).bgElement,
                    borderRadius: BorderRadius.circular(20),
                  ),
                  child: Row(
                    children: [
                      Icon(
                        Icons.download_rounded,
                        size: 14,
                        color: MemoTheme.of(context).textMuted,
                      ),
                      SizedBox(width: 6),
                      Text(
                        _formatCount(widget.result.downloads),
                        style: TextStyle(
                          fontSize: 12,
                          fontWeight: FontWeight.w600,
                          color: MemoTheme.of(context).textMuted,
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ),
            SizedBox(height: 8),
            Row(
              children: [
                Icon(
                  Icons.person_outline,
                  size: 14,
                  color: MemoTheme.of(context).textDim,
                ),
                SizedBox(width: 4),
                Text(
                  widget.result.author,
                  style: TextStyle(
                    fontSize: 13,
                    color: MemoTheme.of(context).textDim,
                  ),
                ),
                SizedBox(width: 12),
                Icon(
                  Icons.favorite_border_rounded,
                  size: 14,
                  color: MemoTheme.of(context).textDim,
                ),
                SizedBox(width: 4),
                Text(
                  L10n.t('likes_count', {'count': widget.result.likes.toString()}),
                  style: TextStyle(
                    fontSize: 12,
                    color: MemoTheme.of(context).textDim,
                  ),
                ),
              ],
            ),
            SizedBox(height: 16),
            Wrap(
              spacing: 8,
              runSpacing: 8,
              children: widget.result.tags.take(5).map((t) {
                final lower = t.toLowerCase();
                Color chipBg;
                Color chipText;
                if (lower.contains('gguf') || lower.contains('llama')) {
                  chipBg = MemoTheme.warmBrown.withValues(alpha: 0.12);
                  chipText = MemoTheme.warmBrown;
                } else if (lower.contains('transformers') ||
                    lower.contains('safetensors')) {
                  chipBg = MemoTheme.accent.withValues(alpha: 0.12);
                  chipText = MemoTheme.accent;
                } else if (lower.contains('text') ||
                    lower.contains('conversational')) {
                  chipBg = Color(0xFF1E88E5).withValues(alpha: 0.12);
                  chipText = const Color(0xFF1E88E5);
                } else {
                  chipBg = MemoTheme.of(context).bgApp;
                  chipText = MemoTheme.of(context).textMuted;
                }
                return Container(
                  padding: EdgeInsets.symmetric(horizontal: 10, vertical: 5),
                  decoration: BoxDecoration(
                    color: chipBg,
                    borderRadius: BorderRadius.circular(8),
                    border: Border.all(color: chipText.withValues(alpha: 0.2)),
                  ),
                  child: Text(
                    t,
                    style: TextStyle(fontSize: 11, color: chipText),
                  ),
                );
              }).toList(),
            ),
            SizedBox(height: 20),
            Align(
              alignment: Alignment.centerRight,
              child: ElevatedButton.icon(
                onPressed: () {
                  showDialog(
                    context: context,
                    builder: (_) => _ModelFilesDialog(repoId: widget.result.id),
                  );
                },
                icon: Icon(Icons.folder_open_rounded, size: 18),
                label: Text(L10n.t('browse_files')),
                style: ElevatedButton.styleFrom(
                  padding: EdgeInsets.symmetric(horizontal: 20, vertical: 12),
                  backgroundColor: _isHovered
                      ? MemoTheme.accent
                      : MemoTheme.accent.withValues(alpha: 0.9),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _LocalModelsList extends ConsumerWidget {
  _LocalModelsList();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final localModelsAsync = ref.watch(localModelsProvider);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: EdgeInsets.all(24).copyWith(bottom: 8),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                L10n.t('local_models'),
                style: TextStyle(fontWeight: FontWeight.w600, fontSize: 16),
              ),
              TextButton.icon(
                icon: Icon(Icons.file_upload, size: 16),
                label: Text(L10n.t('import_model')),
                style: TextButton.styleFrom(foregroundColor: MemoTheme.accent),
                onPressed: () async {
                  final result = await FilePicker.platform.pickFiles(
                    type: FileType.any, // GGUF isn't a standard mime type
                    allowMultiple: false,
                  );
                  if (result != null && result.files.single.path != null) {
                    final sourcePath = result.files.single.path!;

                    if (context.mounted) {
                      ScaffoldMessenger.of(context).showSnackBar(
                        SnackBar(content: Text(L10n.t('importing_model'))),
                      );
                    }

                    try {
                      await ref.read(apiClientProvider).importModel(sourcePath);
                      ref.invalidate(localModelsProvider);
                      if (context.mounted) {
                        ScaffoldMessenger.of(context).showSnackBar(
                          SnackBar(
                            content: Text(L10n.t('import_success')),
                          ),
                        );
                      }
                    } catch (e) {
                      if (context.mounted) {
                        ScaffoldMessenger.of(context).showSnackBar(
                          SnackBar(content: Text(L10n.t('import_error', {'e': e.toString()}))),
                        );
                      }
                    }
                  }
                },
              ),
            ],
          ),
        ),
        Expanded(
          child: localModelsAsync.when(
            loading: () => Center(child: CircularProgressIndicator()),
            error: (e, _) => Center(child: Text('${L10n.t('error')}: $e')),
            data: (models) {
              if (models.isEmpty) {
                return Center(
                  child: Text(
                    L10n.t('no_models'),
                    style: TextStyle(color: MemoTheme.of(context).textDim),
                  ),
                );
              }

              return ListView.separated(
                padding: EdgeInsets.symmetric(horizontal: 24, vertical: 8),
                itemCount: models.length,
                separatorBuilder: (_, __) => SizedBox(height: 12),
                itemBuilder: (context, index) {
                  final model = models[index];
                  return _LocalModelCard(model: model);
                },
              );
            },
          ),
        ),
      ],
    );
  }
}

class _LocalModelCard extends ConsumerWidget {
  final LocalModel model;

  _LocalModelCard({required this.model});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Container(
      padding: EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: MemoTheme.of(context).bgApp,
        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
        border: Border.all(color: MemoTheme.of(context).borderSoft),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  model.filename,
                  style: TextStyle(fontWeight: FontWeight.w500, fontSize: 14),
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              IconButton(
                icon: Icon(Icons.delete_outline, size: 18),
                color: MemoTheme.red,
                padding: EdgeInsets.zero,
                constraints: BoxConstraints(),
                onPressed: () async {
                  final confirmed = await showDialog<bool>(
                    context: context,
                    builder: (ctx) => AlertDialog(
                      backgroundColor: MemoTheme.of(context).bgPanel,
                      title: Text(L10n.t('delete_model_title')),
                      content: Text(
                        L10n.t('delete_model_confirm_name', {'name': model.filename}),
                      ),
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
                          child: Text(L10n.t('delete')),
                        ),
                      ],
                    ),
                  );
                  if (confirmed == true) {
                    ref
                        .read(localModelsProvider.notifier)
                        .deleteModel(model.path);
                  }
                },
              ),
            ],
          ),
          SizedBox(height: 4),
          Text(
            model.repoId,
            style: TextStyle(
              fontSize: 11,
              color: MemoTheme.of(context).textDim,
            ),
          ),
          SizedBox(height: 8),
          Builder(
            builder: (context) {
              final tags = <String>[];
              final name = model.filename.toLowerCase();
              if (model.isEmbedding ||
                  name.contains('embed') ||
                  name.contains('bge')) {
                tags.add('Embedding');
              }
              if (name.contains('vision') ||
                  name.contains('llava') ||
                  name.contains('pixtral')) {
                tags.add('Vision');
              }
              if (name.contains('think') ||
                  name.contains('reason') ||
                  name.contains('r1') ||
                  name.contains('o1')) {
                tags.add('Think');
              }
              if (name.contains('tool') ||
                  name.contains('function') ||
                  name.contains('fc')) {
                tags.add('Tool');
              }
              if (tags.isEmpty) {
                tags.add('Text');
              }

              return Row(
                children: tags
                    .map(
                      (t) => Container(
                        margin: EdgeInsets.only(right: 6),
                        padding: EdgeInsets.symmetric(
                          horizontal: 6,
                          vertical: 2,
                        ),
                        decoration: BoxDecoration(
                          color: MemoTheme.accentPale.withValues(alpha: 0.5),
                          borderRadius: BorderRadius.circular(4),
                          border: Border.all(
                            color: MemoTheme.accent.withValues(alpha: 0.3),
                          ),
                        ),
                        child: Text(
                          t,
                          style: TextStyle(
                            fontSize: 10,
                            color: MemoTheme.accent,
                            fontWeight: FontWeight.w600,
                          ),
                        ),
                      ),
                    )
                    .toList(),
              );
            },
          ),
          SizedBox(height: 10),
          Row(
            children: [
              Container(
                padding: EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                decoration: BoxDecoration(
                  color: MemoTheme.of(context).bgElement,
                  borderRadius: BorderRadius.circular(4),
                ),
                child: Text(
                  model.sizeFormatted,
                  style: TextStyle(
                    fontSize: 10,
                    color: MemoTheme.of(context).textMuted,
                  ),
                ),
              ),
              Spacer(),
              Consumer(
                builder: (context, ref, _) {
                  final installed =
                      ref.watch(llamaInstalledProvider).valueOrNull ?? false;
                  return ElevatedButton(
                    onPressed: !installed
                        ? null
                        : () {
                            showDialog(
                              context: context,
                              builder: (context) =>
                                  ModelConfigDialog(model: model),
                            );
                          },
                    style: ElevatedButton.styleFrom(
                      backgroundColor: MemoTheme.accent,
                      foregroundColor: MemoTheme.of(context).textInverse,
                      padding: EdgeInsets.symmetric(
                        horizontal: 16,
                        vertical: 0,
                      ),
                      minimumSize: Size(0, 32),
                    ),
                    child: Text(
                      model.isEmbedding
                          ? L10n.t('start_embedding')
                          : L10n.t('start_model'),
                      style: TextStyle(fontSize: 12),
                    ),
                  );
                },
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _ActiveModelCard extends ConsumerWidget {
  _ActiveModelCard();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final statusAsync = ref.watch(modelStatusProvider);
    final embStatusAsync = ref.watch(embeddingStatusProvider);

    return Container(
      padding: EdgeInsets.all(16),
      decoration: BoxDecoration(
        gradient: LinearGradient(
          colors: [
            MemoTheme.accentPale.withValues(alpha: 0.5),
            MemoTheme.of(context).bgApp,
          ],
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
        ),
        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
        border: Border.all(color: MemoTheme.accent.withValues(alpha: 0.3)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(Icons.memory, color: MemoTheme.accent, size: 20),
              SizedBox(width: 8),
              Text(
                L10n.t('running_model'),
                style: TextStyle(fontWeight: FontWeight.bold, fontSize: 14),
              ),
              Spacer(),
              statusAsync.when(
                data: (status) => _StatusIndicator(active: status.running),
                loading: () => _StatusIndicator(active: false),
                error: (_, __) => _StatusIndicator(active: false),
              ),
            ],
          ),
          SizedBox(height: 16),
          statusAsync.when(
            loading: () => Text(L10n.t('getting_status')),
            error: (e, _) => Text(L10n.t('engine_error', {'e': e.toString()})),
            data: (status) {
              if (status.running) {
                return Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      status.modelPath.split('/').last,
                      style: TextStyle(
                        fontWeight: FontWeight.w500,
                        fontSize: 13,
                      ),
                    ),
                    SizedBox(height: 12),
                    Row(
                      mainAxisAlignment: MainAxisAlignment.end,
                      children: [
                        OutlinedButton(
                          onPressed: () async {
                            try {
                              await ref.read(apiClientProvider).stopModel();
                            } catch (e) {
                              if (context.mounted) {
                                ScaffoldMessenger.of(
                                  context,
                                ).showSnackBar(SnackBar(content: Text('$e')));
                              }
                            }
                            ref.invalidate(modelStatusProvider);
                          },
                          style: OutlinedButton.styleFrom(
                            foregroundColor: MemoTheme.red,
                            side: BorderSide(color: MemoTheme.red),
                            padding: EdgeInsets.symmetric(
                              horizontal: 12,
                              vertical: 0,
                            ),
                            minimumSize: Size(0, 32),
                          ),
                          child: Text(
                            L10n.t('stop_model'),
                            style: TextStyle(fontSize: 12),
                          ),
                        ),
                      ],
                    ),
                  ],
                );
              } else {
                return Text(
                  L10n.t('stopped'),
                  style: TextStyle(color: MemoTheme.of(context).textDim),
                );
              }
            },
          ),

          // Embedding Model Status
          embStatusAsync.when(
            data: (embStatus) {
              if (embStatus.running) {
                return Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    SizedBox(height: 12),
                    Divider(color: MemoTheme.accent.withValues(alpha: 0.2)),
                    SizedBox(height: 12),
                    Row(
                      children: [
                        Icon(
                          Icons.hub_outlined,
                          color: MemoTheme.accent,
                          size: 16,
                        ),
                        SizedBox(width: 8),
                        Text(
                          L10n.t('memory_model'),
                          style: TextStyle(
                            fontWeight: FontWeight.bold,
                            fontSize: 13,
                          ),
                        ),
                        Spacer(),
                        _StatusIndicator(active: true),
                      ],
                    ),
                    SizedBox(height: 8),
                    Text(
                      embStatus.modelPath.split('/').last,
                      style: TextStyle(
                        fontWeight: FontWeight.w500,
                        fontSize: 13,
                      ),
                    ),
                    SizedBox(height: 12),
                    Row(
                      mainAxisAlignment: MainAxisAlignment.end,
                      children: [
                        OutlinedButton(
                          onPressed: () async {
                            try {
                              await ref
                                  .read(apiClientProvider)
                                  .stopEmbeddingModel();
                            } catch (e) {
                              if (context.mounted) {
                                ScaffoldMessenger.of(
                                  context,
                                ).showSnackBar(SnackBar(content: Text('$e')));
                              }
                            }
                            ref.invalidate(embeddingStatusProvider);
                          },
                          style: OutlinedButton.styleFrom(
                            foregroundColor: MemoTheme.red,
                            side: BorderSide(color: MemoTheme.red),
                            padding: EdgeInsets.symmetric(
                              horizontal: 12,
                              vertical: 0,
                            ),
                            minimumSize: Size(0, 32),
                          ),
                          child: Text(
                            L10n.t('stop_memory_model'),
                            style: TextStyle(fontSize: 12),
                          ),
                        ),
                      ],
                    ),
                  ],
                );
              }
              return SizedBox.shrink();
            },
            loading: () => SizedBox.shrink(),
            error: (_, __) => SizedBox.shrink(),
          ),
        ],
      ),
    );
  }
}

class _StatusIndicator extends StatelessWidget {
  final bool active;
  _StatusIndicator({required this.active});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: active
            ? MemoTheme.green.withValues(alpha: 0.1)
            : MemoTheme.of(context).textDim.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: active
              ? MemoTheme.green.withValues(alpha: 0.3)
              : MemoTheme.of(context).textDim.withValues(alpha: 0.3),
        ),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(
            width: 6,
            height: 6,
            decoration: BoxDecoration(
              shape: BoxShape.circle,
              color: active ? MemoTheme.green : MemoTheme.of(context).textDim,
            ),
          ),
          SizedBox(width: 4),
          Text(
            active ? L10n.t('running') : L10n.t('stopped'),
            style: TextStyle(
              fontSize: 10,
              fontWeight: FontWeight.w600,
              color: active ? MemoTheme.green : MemoTheme.of(context).textDim,
            ),
          ),
        ],
      ),
    );
  }
}

class _DownloadProgressCard extends ConsumerStatefulWidget {
  @override
  ConsumerState<_DownloadProgressCard> createState() =>
      _DownloadProgressCardState();
}

class _DownloadProgressCardState extends ConsumerState<_DownloadProgressCard> {
  String _formatBytes(int bytes) {
    if (bytes >= 1024 * 1024 * 1024) {
      return '${(bytes / (1024 * 1024 * 1024)).toStringAsFixed(1)} GB';
    } else if (bytes >= 1024 * 1024) {
      return '${(bytes / (1024 * 1024)).toStringAsFixed(1)} MB';
    } else if (bytes >= 1024) {
      return '${(bytes / 1024).toStringAsFixed(0)} KB';
    }
    return '$bytes B';
  }

  @override
  Widget build(BuildContext context) {
    final progressAsync = ref.watch(downloadProgressProvider);

    ref.listen(downloadProgressProvider, (previous, next) {
      final wasActive = previous?.value?.active ?? false;
      final isNowActive = next.value?.active ?? false;
      if (wasActive && !isNowActive) {
        ref.invalidate(localModelsProvider);
      }
    });

    return progressAsync.when(
      data: (progress) {
        if (!progress.active) {
          return SizedBox.shrink();
        }

        final percentText = progress.percent.toStringAsFixed(1);
        final downloadedText = _formatBytes(progress.downloaded);
        final totalText = _formatBytes(progress.totalBytes);

        return Container(
          padding: EdgeInsets.all(24),
          decoration: BoxDecoration(
            gradient: LinearGradient(
              colors: [
                MemoTheme.accent.withValues(alpha: 0.1),
                MemoTheme.of(context).bgPanel,
              ],
              begin: Alignment.topLeft,
              end: Alignment.bottomRight,
            ),
            borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
            border: Border.all(
              color: MemoTheme.accent.withValues(alpha: 0.4),
              width: 1.5,
            ),
            boxShadow: MemoTheme.shadowMd,
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // Filename (Prominent)
              Row(
                children: [
                  Container(
                    padding: EdgeInsets.all(10),
                    decoration: BoxDecoration(
                      color: MemoTheme.accent.withValues(alpha: 0.15),
                      borderRadius: BorderRadius.circular(10),
                    ),
                    child: Icon(
                      Icons.cloud_download_rounded,
                      color: MemoTheme.accent,
                      size: 24,
                    ),
                  ),
                  SizedBox(width: 16),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          progress.filename,
                          style: TextStyle(
                            fontWeight: FontWeight.bold,
                            fontSize: 15,
                            color: MemoTheme.of(context).textMain,
                          ),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                        SizedBox(height: 4),
                        Text(
                          L10n.t('downloading_model'),
                          style: TextStyle(
                            fontSize: 12,
                            color: MemoTheme.of(context).textDim,
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ),

              SizedBox(height: 24),

              // Percentage display
              Row(
                mainAxisAlignment: MainAxisAlignment.center,
                crossAxisAlignment: CrossAxisAlignment.baseline,
                textBaseline: TextBaseline.alphabetic,
                children: [
                  Text(
                    percentText,
                    style: TextStyle(
                      fontSize: 48,
                      fontWeight: FontWeight.w900,
                      color: MemoTheme.accent,
                      letterSpacing: -2,
                    ),
                  ),
                  SizedBox(width: 4),
                  Text(
                    '%',
                    style: TextStyle(
                      fontSize: 20,
                      fontWeight: FontWeight.bold,
                      color: MemoTheme.accent,
                    ),
                  ),
                ],
              ),

              SizedBox(height: 20),

              // Progress bar
              Stack(
                children: [
                  Container(
                    height: 12,
                    decoration: BoxDecoration(
                      color: MemoTheme.of(context).bgElement,
                      borderRadius: BorderRadius.circular(6),
                    ),
                  ),
                  TweenAnimationBuilder<double>(
                    tween: Tween(begin: 0, end: progress.percent / 100.0),
                    duration: Duration(milliseconds: 500),
                    curve: Curves.easeOutCubic,
                    builder: (context, value, _) {
                      return FractionallySizedBox(
                        widthFactor: value,
                        child: Container(
                          height: 12,
                          decoration: BoxDecoration(
                            gradient: LinearGradient(
                              colors: [MemoTheme.accentLight, MemoTheme.accent],
                            ),
                            borderRadius: BorderRadius.circular(6),
                            boxShadow: [
                              BoxShadow(
                                color: MemoTheme.accent.withValues(alpha: 0.3),
                                blurRadius: 4,
                                offset: Offset(0, 2),
                              ),
                            ],
                          ),
                        ),
                      );
                    },
                  ),
                ],
              ),

              SizedBox(height: 20),

              // Size + Speed row
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  // Downloaded / Total
                  Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        L10n.t('downloaded_label'),
                        style: TextStyle(
                          fontSize: 9,
                          fontWeight: FontWeight.w800,
                          color: MemoTheme.of(context).textDim,
                          letterSpacing: 0.5,
                        ),
                      ),
                      SizedBox(height: 2),
                      Text(
                        '$downloadedText / $totalText',
                        style: TextStyle(
                          fontSize: 13,
                          fontWeight: FontWeight.bold,
                          color: MemoTheme.of(context).textMuted,
                        ),
                      ),
                    ],
                  ),
                  // Speed badge
                  if (progress.speed.isNotEmpty)
                    Container(
                      padding: EdgeInsets.symmetric(
                        horizontal: 12,
                        vertical: 6,
                      ),
                      decoration: BoxDecoration(
                        color: MemoTheme.accent.withValues(alpha: 0.1),
                        borderRadius: BorderRadius.circular(8),
                        border: Border.all(
                          color: MemoTheme.accent.withValues(alpha: 0.2),
                        ),
                      ),
                      child: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Icon(
                            Icons.bolt_rounded,
                            size: 14,
                            color: MemoTheme.accent,
                          ),
                          SizedBox(width: 6),
                          Text(
                            progress.speed,
                            style: TextStyle(
                              fontSize: 12,
                              fontWeight: FontWeight.w900,
                              color: MemoTheme.accent,
                            ),
                          ),
                        ],
                      ),
                    ),
                ],
              ),
            ],
          ),
        );
      },
      loading: () => SizedBox.shrink(),
      error: (_, __) => SizedBox.shrink(),
    );
  }
}

class _ModelFilesDialog extends ConsumerStatefulWidget {
  final String repoId;
  _ModelFilesDialog({required this.repoId});

  @override
  ConsumerState<_ModelFilesDialog> createState() => _ModelFilesDialogState();
}

class _ModelFilesDialogState extends ConsumerState<_ModelFilesDialog> {
  List<GGUFFile>? _files;
  String? _error;

  @override
  void initState() {
    super.initState();
    _loadFiles();
  }

  Future<void> _loadFiles() async {
    try {
      final files = await ref
          .read(apiClientProvider)
          .getModelFiles(widget.repoId);
      if (mounted) {
        setState(() {
          _files = files;
        });
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _error = e.toString();
        });
      }
    }
  }

  Widget _buildQuantBadge(String filename) {
    final lower = filename.toLowerCase();
    String? quant;
    if (lower.contains('q4_k_m'))
      quant = 'Q4_K_M';
    else if (lower.contains('q4_k_s'))
      quant = 'Q4_K_S';
    else if (lower.contains('q5_k_m'))
      quant = 'Q5_K_M';
    else if (lower.contains('q5_k_s'))
      quant = 'Q5_K_S';
    else if (lower.contains('q8_0'))
      quant = 'Q8_0';
    else if (lower.contains('q4_0'))
      quant = 'Q4_0';
    else if (lower.contains('fp16'))
      quant = 'FP16';

    if (quant == null) return SizedBox.shrink();

    return Container(
      margin: EdgeInsets.only(left: 8),
      padding: EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      decoration: BoxDecoration(
        color: MemoTheme.accent.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(4),
        border: Border.all(color: MemoTheme.accent.withValues(alpha: 0.3)),
      ),
      child: Text(
        quant,
        style: TextStyle(
          fontSize: 10,
          fontWeight: FontWeight.bold,
          color: MemoTheme.accent,
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Dialog(
      backgroundColor: Colors.transparent,
      insetPadding: EdgeInsets.symmetric(horizontal: 40, vertical: 60),
      child: Container(
        width: 600,
        decoration: BoxDecoration(
          color: MemoTheme.of(context).bgApp,
          borderRadius: BorderRadius.circular(MemoTheme.radiusLg),
          boxShadow: MemoTheme.shadowLg,
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            // Header
            Padding(
              padding: EdgeInsets.all(24),
              child: Row(
                children: [
                  Icon(Icons.folder_open_rounded, color: MemoTheme.accent),
                  SizedBox(width: 12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          L10n.t('model_files'),
                          style: TextStyle(
                            fontSize: 18,
                            fontWeight: FontWeight.bold,
                            color: MemoTheme.of(context).textMain,
                          ),
                        ),
                        Text(
                          widget.repoId,
                          style: TextStyle(
                            fontSize: 12,
                            color: MemoTheme.of(context).textDim,
                          ),
                        ),
                      ],
                    ),
                  ),
                  IconButton(
                    onPressed: () => Navigator.pop(context),
                    icon: Icon(Icons.close),
                  ),
                ],
              ),
            ),
            Divider(),

            // Content
            Flexible(
              child: _files == null && _error == null
                  ? Padding(
                      padding: EdgeInsets.all(40),
                      child: Center(child: CircularProgressIndicator()),
                    )
                  : _error != null
                  ? Padding(
                      padding: EdgeInsets.all(40),
                      child: Center(
                        child: Column(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            Icon(
                              Icons.error_outline,
                              color: MemoTheme.red,
                              size: 48,
                            ),
                            SizedBox(height: 16),
                            Text(
                              '${L10n.t('error')}: $_error',
                              textAlign: TextAlign.center,
                              style: TextStyle(color: MemoTheme.red),
                            ),
                          ],
                        ),
                      ),
                    )
                  : _files!.isEmpty
                  ? Padding(
                      padding: EdgeInsets.all(40),
                      child: Center(
                        child: Text(
                          L10n.t('no_gguf'),
                          style: TextStyle(
                            color: MemoTheme.of(context).textDim,
                          ),
                        ),
                      ),
                    )
                  : ListView.separated(
                      shrinkWrap: true,
                      padding: EdgeInsets.symmetric(
                        horizontal: 16,
                        vertical: 8,
                      ),
                      itemCount: _files!.length,
                      separatorBuilder: (_, __) => SizedBox(height: 8),
                      itemBuilder: (context, index) {
                        final file = _files![index];
                        final sizeGB = (file.size / (1024 * 1024 * 1024));
                        final sizeMB = (file.size / (1024 * 1024));
                        final sizeText = sizeGB >= 1
                            ? '${sizeGB.toStringAsFixed(1)} GB'
                            : '${sizeMB.toStringAsFixed(0)} MB';

                        return Container(
                          decoration: BoxDecoration(
                            color: MemoTheme.of(context).bgPanel,
                            borderRadius: BorderRadius.circular(
                              MemoTheme.radiusMd,
                            ),
                            border: Border.all(
                              color: MemoTheme.of(context).borderSoft,
                            ),
                          ),
                          child: ListTile(
                            contentPadding: EdgeInsets.symmetric(
                              horizontal: 16,
                              vertical: 8,
                            ),
                            title: Row(
                              children: [
                                Expanded(
                                  child: Text(
                                    file.filename,
                                    style: TextStyle(
                                      color: MemoTheme.of(context).textMain,
                                      fontSize: 14,
                                      fontWeight: FontWeight.w500,
                                    ),
                                  ),
                                ),
                                _buildQuantBadge(file.filename),
                              ],
                            ),
                            subtitle: Padding(
                              padding: EdgeInsets.only(top: 8),
                              child: Row(
                                children: [
                                  Container(
                                    padding: EdgeInsets.symmetric(
                                      horizontal: 8,
                                      vertical: 2,
                                    ),
                                    decoration: BoxDecoration(
                                      color: MemoTheme.accent.withValues(
                                        alpha: 0.15,
                                      ),
                                      borderRadius: BorderRadius.circular(4),
                                    ),
                                    child: Text(
                                      sizeText,
                                      style: TextStyle(
                                        color: MemoTheme.accent,
                                        fontSize: 11,
                                        fontWeight: FontWeight.bold,
                                      ),
                                    ),
                                  ),
                                ],
                              ),
                            ),
                            trailing: ElevatedButton(
                              onPressed: () {
                                ref
                                    .read(apiClientProvider)
                                    .downloadModel(
                                      widget.repoId,
                                      file.filename,
                                    );
                                Navigator.pop(context);
                                ScaffoldMessenger.of(context).showSnackBar(
                                  SnackBar(
                                    content: Text(
                                      L10n.t('download_started', {'file': file.filename}),
                                    ),
                                    backgroundColor: MemoTheme.green,
                                  ),
                                );
                              },
                              style: ElevatedButton.styleFrom(
                                padding: EdgeInsets.symmetric(
                                  horizontal: 16,
                                  vertical: 0,
                                ),
                                minimumSize: Size(0, 36),
                              ),
                              child: Text(
                                L10n.t('download'),
                                style: TextStyle(fontSize: 13),
                              ),
                            ),
                          ),
                        );
                      },
                    ),
            ),

            // Footer
            Padding(
              padding: EdgeInsets.all(16),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.end,
                children: [
                  TextButton(
                    onPressed: () => Navigator.pop(context),
                    child: Text(L10n.t('close')),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
