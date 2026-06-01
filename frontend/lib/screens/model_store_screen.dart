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
  const ModelStoreScreen({super.key});

  @override
  ConsumerState<ModelStoreScreen> createState() => _ModelStoreScreenState();
}

class _ModelStoreScreenState extends ConsumerState<ModelStoreScreen> {
  final _searchController = TextEditingController();

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      color: MemoTheme.bgApp,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          // ─── Header ──────────────────────────────────────
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 32, vertical: 24),
            decoration: BoxDecoration(
              border: Border(bottom: BorderSide(color: MemoTheme.borderSoft)),
            ),
            child: Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      L10n.t('model_store'),
                      style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                            fontWeight: FontWeight.bold,
                            color: MemoTheme.textMain,
                          ),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      'İndirilen modeller ve HuggingFace araması',
                      style: TextStyle(color: MemoTheme.textDim),
                    ),
                  ],
                ),
                const GPUBadge(),
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
                      border:
                          Border(right: BorderSide(color: MemoTheme.borderSoft)),
                    ),
                    child: Column(
                      children: [
                        // Search Bar
                        Padding(
                          padding: const EdgeInsets.all(24),
                          child: TextField(
                            controller: _searchController,
                            decoration: InputDecoration(
                              hintText: L10n.t('search_models'),
                              prefixIcon:
                                  const Icon(Icons.search, color: MemoTheme.textDim),
                              filled: true,
                              fillColor: MemoTheme.bgPanel,
                              border: OutlineInputBorder(
                                borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                                borderSide: BorderSide(color: MemoTheme.borderSoft),
                              ),
                              enabledBorder: OutlineInputBorder(
                                borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                                borderSide: BorderSide(color: MemoTheme.borderSoft),
                              ),
                            ),
                            onSubmitted: (val) {
                              ref
                                  .read(modelSearchQueryProvider.notifier)
                                  .state = val;
                            },
                          ),
                        ),

                        // Search Results / Downloading
                        Expanded(
                          child: const _SearchResultsPanel(),
                        ),
                      ],
                    ),
                  ),
                ),

                // ─── Right Panel: Local Models & Active ────
                Expanded(
                  flex: 2,
                  child: Container(
                    color: MemoTheme.bgPanel.withValues(alpha: 0.3),
                    child: Column(
                      children: [
                        // Active Model
                        const Padding(
                          padding: EdgeInsets.all(24),
                          child: _ActiveModelCard(),
                        ),

                        // Download Progress
                        const Padding(
                          padding: EdgeInsets.symmetric(horizontal: 24),
                          child: _DownloadProgressCard(),
                        ),

                        const SizedBox(height: 24),
                        const Divider(height: 1),

                        // Local Models
                        Expanded(
                          child: const _LocalModelsList(),
                        ),
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
  const _SearchResultsPanel();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final query = ref.watch(modelSearchQueryProvider);
    final searchResultsAsync = ref.watch(modelSearchResultsProvider);

    if (query.isEmpty) {
      return Center(
        child: Text(
          'Arama yapmak için bir model adı yazın\n(Örn: Llama-3, Mistral, Gemma)',
          textAlign: TextAlign.center,
          style: TextStyle(color: MemoTheme.textDim),
        ),
      );
    }

    return searchResultsAsync.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (e, _) => Center(child: Text('${L10n.t('error')}: $e')),
      data: (results) {
        if (results.isEmpty) {
          return Center(child: Text('Sonuç bulunamadı', style: TextStyle(color: MemoTheme.textDim)));
        }
        return ListView.separated(
          padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 8),
          itemCount: results.length,
          separatorBuilder: (_, __) => const SizedBox(height: 12),
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

  const _SearchResultCard({required this.result});

  @override
  State<_SearchResultCard> createState() => _SearchResultCardState();
}

class _SearchResultCardState extends State<_SearchResultCard> {
  bool _isHovered = false;

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
        duration: const Duration(milliseconds: 200),
        padding: const EdgeInsets.all(20),
        decoration: BoxDecoration(
          color: _isHovered ? MemoTheme.bgHover.withValues(alpha: 0.3) : MemoTheme.bgPanel,
          borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
          border: Border.all(
            color: _isHovered ? MemoTheme.accent.withValues(alpha: 0.5) : MemoTheme.borderSoft,
            width: _isHovered ? 1.5 : 1.0,
          ),
          boxShadow: _isHovered ? MemoTheme.shadowMd : [],
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(
                    widget.result.id,
                    style: const TextStyle(
                      fontWeight: FontWeight.bold,
                      fontSize: 16,
                      color: MemoTheme.textMain,
                    ),
                  ),
                ),
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                  decoration: BoxDecoration(
                    color: MemoTheme.bgElement,
                    borderRadius: BorderRadius.circular(20),
                  ),
                  child: Row(
                    children: [
                      const Icon(Icons.download_rounded, size: 14, color: MemoTheme.textMuted),
                      const SizedBox(width: 6),
                      Text(
                        _formatCount(widget.result.downloads),
                        style: const TextStyle(
                          fontSize: 12,
                          fontWeight: FontWeight.w600,
                          color: MemoTheme.textMuted,
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ),
            const SizedBox(height: 8),
            Row(
              children: [
                const Icon(Icons.person_outline, size: 14, color: MemoTheme.textDim),
                const SizedBox(width: 4),
                Text(
                  widget.result.author,
                  style: const TextStyle(fontSize: 13, color: MemoTheme.textDim),
                ),
                const SizedBox(width: 12),
                const Icon(Icons.favorite_border_rounded, size: 14, color: MemoTheme.textDim),
                const SizedBox(width: 4),
                Text(
                  '${widget.result.likes} beğeni',
                  style: const TextStyle(fontSize: 12, color: MemoTheme.textDim),
                ),
              ],
            ),
            const SizedBox(height: 16),
            Wrap(
              spacing: 8,
              runSpacing: 8,
              children: widget.result.tags.take(5).map((t) {
                return Container(
                  padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
                  decoration: BoxDecoration(
                    color: MemoTheme.bgApp,
                    borderRadius: BorderRadius.circular(8),
                    border: Border.all(color: MemoTheme.borderSoft),
                  ),
                  child: Text(
                    t,
                    style: const TextStyle(fontSize: 11, color: MemoTheme.textMuted),
                  ),
                );
              }).toList(),
            ),
            const SizedBox(height: 20),
            Align(
              alignment: Alignment.centerRight,
              child: ElevatedButton.icon(
                onPressed: () {
                  showDialog(
                    context: context,
                    builder: (_) => _ModelFilesDialog(repoId: widget.result.id),
                  );
                },
                icon: const Icon(Icons.folder_open_rounded, size: 18),
                label: const Text('Dosyaları İncele'),
                style: ElevatedButton.styleFrom(
                  padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 12),
                  backgroundColor: _isHovered ? MemoTheme.accent : MemoTheme.accent.withValues(alpha: 0.9),
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
  const _LocalModelsList();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final localModelsAsync = ref.watch(localModelsProvider);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.all(24).copyWith(bottom: 8),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                L10n.t('local_models'),
                style: const TextStyle(fontWeight: FontWeight.w600, fontSize: 16),
              ),
              TextButton.icon(
                icon: const Icon(Icons.file_upload, size: 16),
                label: const Text('İçe Aktar'),
                style: TextButton.styleFrom(
                  foregroundColor: MemoTheme.accent,
                ),
                onPressed: () async {
                  final result = await FilePicker.platform.pickFiles(
                    type: FileType.any, // GGUF isn't a standard mime type
                    allowMultiple: false,
                  );
                  if (result != null && result.files.single.path != null) {
                    final sourcePath = result.files.single.path!;
                    
                    if (context.mounted) {
                      ScaffoldMessenger.of(context).showSnackBar(
                        SnackBar(content: Text('Model içe aktarılıyor...')),
                      );
                    }
                    
                    try {
                      await ref.read(apiClientProvider).importModel(sourcePath);
                      ref.invalidate(localModelsProvider);
                      if (context.mounted) {
                        ScaffoldMessenger.of(context).showSnackBar(
                          const SnackBar(content: Text('Model başarıyla içe aktarıldı.')),
                        );
                      }
                    } catch (e) {
                      if (context.mounted) {
                        ScaffoldMessenger.of(context).showSnackBar(
                          SnackBar(content: Text('İçe aktarma hatası: $e')),
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
            loading: () => const Center(child: CircularProgressIndicator()),
            error: (e, _) => Center(child: Text('${L10n.t('error')}: $e')),
            data: (models) {
              if (models.isEmpty) {
                return Center(
                  child: Text(L10n.t('no_models'),
                      style: TextStyle(color: MemoTheme.textDim)),
                );
              }

              return ListView.separated(
                padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 8),
                itemCount: models.length,
                separatorBuilder: (_, __) => const SizedBox(height: 12),
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

  const _LocalModelCard({required this.model});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: MemoTheme.bgApp,
        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
        border: Border.all(color: MemoTheme.borderSoft),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  model.filename,
                  style: const TextStyle(fontWeight: FontWeight.w500, fontSize: 14),
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              IconButton(
                icon: const Icon(Icons.delete_outline, size: 18),
                color: MemoTheme.red,
                padding: EdgeInsets.zero,
                constraints: const BoxConstraints(),
                onPressed: () {
                  ref.read(localModelsProvider.notifier).deleteModel(model.path);
                },
              ),
            ],
          ),
          const SizedBox(height: 4),
          Text(
            model.repoId,
            style: TextStyle(fontSize: 11, color: MemoTheme.textDim),
          ),
          const SizedBox(height: 8),
          Builder(
            builder: (context) {
              final tags = <String>[];
              final name = model.filename.toLowerCase();
              if (model.isEmbedding || name.contains('embed') || name.contains('bge')) {
                tags.add('Embedding');
              }
              if (name.contains('vision') || name.contains('llava') || name.contains('pixtral')) {
                tags.add('Vision');
              }
              if (name.contains('think') || name.contains('reason') || name.contains('r1') || name.contains('o1')) {
                tags.add('Think');
              }
              if (name.contains('tool') || name.contains('function') || name.contains('fc')) {
                tags.add('Tool');
              }
              if (tags.isEmpty) {
                tags.add('Text');
              }

              return Row(
                children: tags.map((t) => Container(
                  margin: const EdgeInsets.only(right: 6),
                  padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                  decoration: BoxDecoration(
                    color: MemoTheme.accentPale.withValues(alpha: 0.5),
                    borderRadius: BorderRadius.circular(4),
                    border: Border.all(color: MemoTheme.accent.withValues(alpha: 0.3)),
                  ),
                  child: Text(
                    t, 
                    style: const TextStyle(fontSize: 10, color: MemoTheme.accent, fontWeight: FontWeight.w600)
                  ),
                )).toList(),
              );
            }
          ),
          const SizedBox(height: 10),
          Row(
            children: [
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                decoration: BoxDecoration(
                  color: MemoTheme.bgElement,
                  borderRadius: BorderRadius.circular(4),
                ),
                child: Text(
                  model.sizeFormatted,
                  style: TextStyle(fontSize: 10, color: MemoTheme.textMuted),
                ),
              ),
              const Spacer(),
              Consumer(
                builder: (context, ref, _) {
                  final installed = ref.watch(llamaInstalledProvider).valueOrNull ?? false;
                  return ElevatedButton(
                    onPressed: !installed ? null : () {
                      showDialog(
                        context: context,
                        builder: (context) => ModelConfigDialog(model: model),
                      );
                    },
                    style: ElevatedButton.styleFrom(
                      backgroundColor: MemoTheme.accent,
                      foregroundColor: MemoTheme.textInverse,
                      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 0),
                      minimumSize: const Size(0, 32),
                    ),
                    child: Text(L10n.t('start_model'), style: const TextStyle(fontSize: 12)),
                  );
                }
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _ActiveModelCard extends ConsumerWidget {
  const _ActiveModelCard();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final statusAsync = ref.watch(modelStatusProvider);
    final embStatusAsync = ref.watch(embeddingStatusProvider);

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        gradient: LinearGradient(
          colors: [
            MemoTheme.accentPale.withValues(alpha: 0.5),
            MemoTheme.bgApp,
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
              const Icon(Icons.memory, color: MemoTheme.accent, size: 20),
              const SizedBox(width: 8),
              Text(
                'Çalışan Model',
                style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 14),
              ),
              const Spacer(),
              statusAsync.when(
                data: (status) => _StatusIndicator(active: status.running),
                loading: () => const _StatusIndicator(active: false),
                error: (_, __) => const _StatusIndicator(active: false),
              ),
            ],
          ),
          const SizedBox(height: 16),
          statusAsync.when(
            loading: () => const Text('Durum alınıyor...'),
            error: (e, _) => Text('Hata: $e'),
            data: (status) {
              if (status.running) {
                return Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      status.modelPath.split('/').last,
                      style: const TextStyle(fontWeight: FontWeight.w500, fontSize: 13),
                    ),
                    const SizedBox(height: 12),
                    Row(
                      mainAxisAlignment: MainAxisAlignment.end,
                      children: [
                        OutlinedButton(
                          onPressed: () {
                            ref.read(apiClientProvider).stopModel();
                            ref.invalidate(modelStatusProvider);
                          },
                          style: OutlinedButton.styleFrom(
                            foregroundColor: MemoTheme.red,
                            side: const BorderSide(color: MemoTheme.red),
                            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 0),
                            minimumSize: const Size(0, 32),
                          ),
                          child: Text(L10n.t('stop_model'), style: const TextStyle(fontSize: 12)),
                        ),
                      ],
                    ),
                  ],
                );
              } else {
                return Text(
                  L10n.t('stopped'),
                  style: TextStyle(color: MemoTheme.textDim),
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
                    const SizedBox(height: 12),
                    Divider(color: MemoTheme.accent.withValues(alpha: 0.2)),
                    const SizedBox(height: 12),
                    Row(
                      children: [
                        const Icon(Icons.hub_outlined, color: MemoTheme.accent, size: 16),
                        const SizedBox(width: 8),
                        Text(
                          'Hafıza (Embedding) Modeli',
                          style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 13),
                        ),
                        const Spacer(),
                        const _StatusIndicator(active: true),
                      ],
                    ),
                    const SizedBox(height: 8),
                    Text(
                      embStatus.modelPath.split('/').last,
                      style: const TextStyle(fontWeight: FontWeight.w500, fontSize: 13),
                    ),
                    const SizedBox(height: 12),
                    Row(
                      mainAxisAlignment: MainAxisAlignment.end,
                      children: [
                        OutlinedButton(
                          onPressed: () {
                            ref.read(apiClientProvider).stopEmbeddingModel();
                            ref.invalidate(embeddingStatusProvider);
                          },
                          style: OutlinedButton.styleFrom(
                            foregroundColor: MemoTheme.red,
                            side: const BorderSide(color: MemoTheme.red),
                            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 0),
                            minimumSize: const Size(0, 32),
                          ),
                          child: Text('Hafıza Modelini Durdur', style: const TextStyle(fontSize: 12)),
                        ),
                      ],
                    ),
                  ],
                );
              }
              return const SizedBox.shrink();
            },
            loading: () => const SizedBox.shrink(),
            error: (_, __) => const SizedBox.shrink(),
          ),
        ],
      ),
    );
  }
}

class _StatusIndicator extends StatelessWidget {
  final bool active;
  const _StatusIndicator({required this.active});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: active
            ? MemoTheme.green.withValues(alpha: 0.1)
            : MemoTheme.textDim.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: active
              ? MemoTheme.green.withValues(alpha: 0.3)
              : MemoTheme.textDim.withValues(alpha: 0.3),
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
              color: active ? MemoTheme.green : MemoTheme.textDim,
            ),
          ),
          const SizedBox(width: 4),
          Text(
            active ? L10n.t('running') : L10n.t('stopped'),
            style: TextStyle(
              fontSize: 10,
              fontWeight: FontWeight.w600,
              color: active ? MemoTheme.green : MemoTheme.textDim,
            ),
          ),
        ],
      ),
    );
  }
}

class _DownloadProgressCard extends ConsumerWidget {
  const _DownloadProgressCard();

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
  Widget build(BuildContext context, WidgetRef ref) {
    // Listen for download completion to refresh local models
    ref.listen(downloadProgressProvider, (previous, next) {
      final wasActive = previous?.value?.active ?? false;
      final isNowActive = next.value?.active ?? false;
      if (wasActive && !isNowActive) {
        ref.invalidate(localModelsProvider);
      }
    });

    final progressAsync = ref.watch(downloadProgressProvider);

    return progressAsync.when(
      data: (progress) {
        if (!progress.active) {
          return const SizedBox.shrink();
        }

        final percentText = progress.percent.toStringAsFixed(1);
        final downloadedText = _formatBytes(progress.downloaded);
        final totalText = _formatBytes(progress.totalBytes);

        return Container(
          padding: const EdgeInsets.all(24),
          decoration: BoxDecoration(
            gradient: LinearGradient(
              colors: [
                MemoTheme.accent.withValues(alpha: 0.1),
                MemoTheme.bgPanel,
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
                    padding: const EdgeInsets.all(10),
                    decoration: BoxDecoration(
                      color: MemoTheme.accent.withValues(alpha: 0.15),
                      borderRadius: BorderRadius.circular(10),
                    ),
                    child: const Icon(
                      Icons.cloud_download_rounded,
                      color: MemoTheme.accent,
                      size: 24,
                    ),
                  ),
                  const SizedBox(width: 16),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          progress.filename,
                          style: const TextStyle(
                            fontWeight: FontWeight.bold,
                            fontSize: 15,
                            color: MemoTheme.textMain,
                          ),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                        const SizedBox(height: 4),
                        Text(
                          'Model indiriliyor...',
                          style: TextStyle(
                            fontSize: 12,
                            color: MemoTheme.textDim,
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ),

              const SizedBox(height: 24),

              // Percentage display
              Row(
                mainAxisAlignment: MainAxisAlignment.center,
                crossAxisAlignment: CrossAxisAlignment.baseline,
                textBaseline: TextBaseline.alphabetic,
                children: [
                  Text(
                    percentText,
                    style: const TextStyle(
                      fontSize: 48,
                      fontWeight: FontWeight.w900,
                      color: MemoTheme.accent,
                      letterSpacing: -2,
                    ),
                  ),
                  const SizedBox(width: 4),
                  const Text(
                    '%',
                    style: TextStyle(
                      fontSize: 20,
                      fontWeight: FontWeight.bold,
                      color: MemoTheme.accent,
                    ),
                  ),
                ],
              ),

              const SizedBox(height: 20),

              // Progress bar
              Stack(
                children: [
                  Container(
                    height: 12,
                    decoration: BoxDecoration(
                      color: MemoTheme.bgElement,
                      borderRadius: BorderRadius.circular(6),
                    ),
                  ),
                  TweenAnimationBuilder<double>(
                    tween: Tween(begin: 0, end: progress.percent / 100.0),
                    duration: const Duration(milliseconds: 500),
                    curve: Curves.easeOutCubic,
                    builder: (context, value, _) {
                      return FractionallySizedBox(
                        widthFactor: value,
                        child: Container(
                          height: 12,
                          decoration: BoxDecoration(
                            gradient: const LinearGradient(
                              colors: [MemoTheme.accentLight, MemoTheme.accent],
                            ),
                            borderRadius: BorderRadius.circular(6),
                            boxShadow: [
                              BoxShadow(
                                color: MemoTheme.accent.withValues(alpha: 0.3),
                                blurRadius: 4,
                                offset: const Offset(0, 2),
                              ),
                            ],
                          ),
                        ),
                      );
                    },
                  ),
                ],
              ),

              const SizedBox(height: 20),

              // Size + Speed row
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  // Downloaded / Total
                  Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      const Text(
                        'İNDİRİLEN',
                        style: TextStyle(fontSize: 9, fontWeight: FontWeight.w800, color: MemoTheme.textDim, letterSpacing: 0.5),
                      ),
                      const SizedBox(height: 2),
                      Text(
                        '$downloadedText / $totalText',
                        style: const TextStyle(
                          fontSize: 13,
                          fontWeight: FontWeight.bold,
                          color: MemoTheme.textMuted,
                        ),
                      ),
                    ],
                  ),
                  // Speed badge
                  if (progress.speed.isNotEmpty)
                    Container(
                      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
                      decoration: BoxDecoration(
                        color: MemoTheme.accent.withValues(alpha: 0.1),
                        borderRadius: BorderRadius.circular(8),
                        border: Border.all(color: MemoTheme.accent.withValues(alpha: 0.2)),
                      ),
                      child: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          const Icon(Icons.bolt_rounded, size: 14, color: MemoTheme.accent),
                          const SizedBox(width: 6),
                          Text(
                            progress.speed,
                            style: const TextStyle(
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
      loading: () => const SizedBox.shrink(),
      error: (_, __) => const SizedBox.shrink(),
    );
  }
}



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
    _loadFiles();
  }

  Future<void> _loadFiles() async {
    try {
      final files = await ref.read(apiClientProvider).getModelFiles(widget.repoId);
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
    if (lower.contains('q4_k_m')) quant = 'Q4_K_M';
    else if (lower.contains('q4_k_s')) quant = 'Q4_K_S';
    else if (lower.contains('q5_k_m')) quant = 'Q5_K_M';
    else if (lower.contains('q5_k_s')) quant = 'Q5_K_S';
    else if (lower.contains('q8_0')) quant = 'Q8_0';
    else if (lower.contains('q4_0')) quant = 'Q4_0';
    else if (lower.contains('fp16')) quant = 'FP16';

    if (quant == null) return const SizedBox.shrink();

    return Container(
      margin: const EdgeInsets.only(left: 8),
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      decoration: BoxDecoration(
        color: MemoTheme.accent.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(4),
        border: Border.all(color: MemoTheme.accent.withValues(alpha: 0.3)),
      ),
      child: Text(
        quant,
        style: const TextStyle(
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
      insetPadding: const EdgeInsets.symmetric(horizontal: 40, vertical: 60),
      child: Container(
        width: 600,
        decoration: BoxDecoration(
          color: MemoTheme.bgApp,
          borderRadius: BorderRadius.circular(MemoTheme.radiusLg),
          boxShadow: MemoTheme.shadowLg,
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            // Header
            Padding(
              padding: const EdgeInsets.all(24),
              child: Row(
                children: [
                  const Icon(Icons.folder_open_rounded, color: MemoTheme.accent),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        const Text(
                          'Model Dosyaları',
                          style: TextStyle(
                            fontSize: 18,
                            fontWeight: FontWeight.bold,
                            color: MemoTheme.textMain,
                          ),
                        ),
                        Text(
                          widget.repoId,
                          style: const TextStyle(fontSize: 12, color: MemoTheme.textDim),
                        ),
                      ],
                    ),
                  ),
                  IconButton(
                    onPressed: () => Navigator.pop(context),
                    icon: const Icon(Icons.close),
                  ),
                ],
              ),
            ),
            const Divider(),

            // Content
            Flexible(
              child: _files == null && _error == null
                  ? const Padding(
                      padding: EdgeInsets.all(40),
                      child: Center(child: CircularProgressIndicator()),
                    )
                  : _error != null
                      ? Padding(
                          padding: const EdgeInsets.all(40),
                          child: Center(
                            child: Column(
                              mainAxisSize: MainAxisSize.min,
                              children: [
                                const Icon(Icons.error_outline, color: MemoTheme.red, size: 48),
                                const SizedBox(height: 16),
                                Text('Hata: $_error',
                                    textAlign: TextAlign.center,
                                    style: const TextStyle(color: MemoTheme.red)),
                              ],
                            ),
                          ),
                        )
                      : _files!.isEmpty
                          ? const Padding(
                              padding: EdgeInsets.all(40),
                              child: Center(
                                child: Text(
                                  'Bu modelde GGUF dosyası bulunamadı.',
                                  style: TextStyle(color: MemoTheme.textDim),
                                ),
                              ),
                            )
                          : ListView.separated(
                              shrinkWrap: true,
                              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
                              itemCount: _files!.length,
                              separatorBuilder: (_, __) => const SizedBox(height: 8),
                              itemBuilder: (context, index) {
                                final file = _files![index];
                                final sizeGB = (file.size / (1024 * 1024 * 1024));
                                final sizeMB = (file.size / (1024 * 1024));
                                final sizeText = sizeGB >= 1 
                                    ? '${sizeGB.toStringAsFixed(1)} GB' 
                                    : '${sizeMB.toStringAsFixed(0)} MB';
                                
                                return Container(
                                  decoration: BoxDecoration(
                                    color: MemoTheme.bgPanel,
                                    borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                                    border: Border.all(color: MemoTheme.borderSoft),
                                  ),
                                  child: ListTile(
                                    contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
                                    title: Row(
                                      children: [
                                        Expanded(
                                          child: Text(
                                            file.filename,
                                            style: const TextStyle(
                                              color: MemoTheme.textMain,
                                              fontSize: 14,
                                              fontWeight: FontWeight.w500,
                                            ),
                                          ),
                                        ),
                                        _buildQuantBadge(file.filename),
                                      ],
                                    ),
                                    subtitle: Padding(
                                      padding: const EdgeInsets.only(top: 8),
                                      child: Row(
                                        children: [
                                          Container(
                                            padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
                                            decoration: BoxDecoration(
                                              color: MemoTheme.accent.withValues(alpha: 0.15),
                                              borderRadius: BorderRadius.circular(4),
                                            ),
                                            child: Text(
                                              sizeText,
                                              style: const TextStyle(
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
                                        ref.read(apiClientProvider).downloadModel(widget.repoId, file.filename);
                                        Navigator.pop(context);
                                        ScaffoldMessenger.of(context).showSnackBar(
                                          SnackBar(
                                            content: Text('${file.filename} indirmesi başlatıldı...'),
                                            backgroundColor: MemoTheme.green,
                                          ),
                                        );
                                      },
                                      style: ElevatedButton.styleFrom(
                                        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 0),
                                        minimumSize: const Size(0, 36),
                                      ),
                                      child: const Text('İndir', style: TextStyle(fontSize: 13)),
                                    ),
                                  ),
                                );
                              },
                            ),
            ),

            // Footer
            Padding(
              padding: const EdgeInsets.all(16),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.end,
                children: [
                  TextButton(
                    onPressed: () => Navigator.pop(context),
                    child: const Text('Kapat'),
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

