import 'dart:io';
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

class _SearchResultCard extends StatelessWidget {
  final HFModelResult result;

  const _SearchResultCard({required this.result});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: MemoTheme.bgPanel,
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
                  result.id,
                  style: const TextStyle(fontWeight: FontWeight.w600, fontSize: 15),
                ),
              ),
              Row(
                children: [
                  const Icon(Icons.download, size: 14, color: MemoTheme.textDim),
                  const SizedBox(width: 4),
                  Text(
                    '${result.downloads}',
                    style: TextStyle(fontSize: 12, color: MemoTheme.textDim),
                  ),
                ],
              ),
            ],
          ),
          const SizedBox(height: 8),
          Text(
            'Author: ${result.author}',
            style: TextStyle(fontSize: 12, color: MemoTheme.textMuted),
          ),
          const SizedBox(height: 12),
          Wrap(
            spacing: 6,
            runSpacing: 6,
            children: result.tags.take(4).map((t) {
              return Container(
                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                decoration: BoxDecoration(
                  color: MemoTheme.bgElement,
                  borderRadius: BorderRadius.circular(4),
                ),
                child: Text(
                  t,
                  style: TextStyle(fontSize: 10, color: MemoTheme.textDim),
                ),
              );
            }).toList(),
          ),
          const SizedBox(height: 16),
          Align(
            alignment: Alignment.centerRight,
            child: ElevatedButton.icon(
              onPressed: () {
                showDialog(
                  context: context,
                  builder: (_) => _ModelFilesDialog(repoId: result.id),
                );
              },
              icon: const Icon(Icons.search, size: 16),
              label: const Text('Dosyaları Gör'),
              style: ElevatedButton.styleFrom(
                padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
              ),
            ),
          ),
        ],
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
                    final sourceFile = File(result.files.single.path!);
                    final filename = result.files.single.name;
                    // ../data/models -> proje kökündeki data/models (backend'in baktığı yer)
                    final modelsDir = Directory('../data/models');
                    if (!await modelsDir.exists()) {
                      await modelsDir.create(recursive: true);
                    }
                    final destPath = '${modelsDir.path}/$filename';
                    
                    if (context.mounted) {
                      ScaffoldMessenger.of(context).showSnackBar(
                        SnackBar(content: Text('Model içe aktarılıyor: $filename...')),
                      );
                    }
                    
                    try {
                      await sourceFile.copy(destPath);
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
              ElevatedButton(
                onPressed: () {
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

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final progressAsync = ref.watch(downloadProgressProvider);

    return progressAsync.when(
      data: (progress) {
        if (!progress.active) return const SizedBox.shrink();

        return Container(
          padding: const EdgeInsets.all(16),
          decoration: BoxDecoration(
            color: MemoTheme.bgApp,
            borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
            border: Border.all(color: MemoTheme.accent.withValues(alpha: 0.5)),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  const Icon(Icons.downloading, color: MemoTheme.accent, size: 20),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      progress.filename,
                      style: const TextStyle(fontWeight: FontWeight.w500, fontSize: 13),
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 12),
              ClipRRect(
                borderRadius: BorderRadius.circular(4),
                child: LinearProgressIndicator(
                  value: progress.percent / 100.0,
                  backgroundColor: MemoTheme.bgElement,
                  color: MemoTheme.accent,
                  minHeight: 6,
                ),
              ),
              const SizedBox(height: 8),
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Text(
                    '${progress.percent.toStringAsFixed(1)}%',
                    style: TextStyle(fontSize: 12, color: MemoTheme.textDim, fontWeight: FontWeight.bold),
                  ),
                  Text(
                    progress.speed,
                    style: TextStyle(fontSize: 12, color: MemoTheme.textMuted),
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

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      backgroundColor: MemoTheme.bgApp,
      title: Text('Model Dosyaları', style: TextStyle(color: MemoTheme.textMain)),
      content: SizedBox(
        width: double.maxFinite,
        height: 400,
        child: _files == null && _error == null
            ? const Center(child: CircularProgressIndicator())
            : _error != null
                ? Center(child: Text('Hata: $_error', style: TextStyle(color: MemoTheme.red)))
                : _files!.isEmpty
                    ? Center(child: Text('Bu modelde GGUF dosyası bulunamadı.', style: TextStyle(color: MemoTheme.textDim)))
                    : ListView.separated(
                        itemCount: _files!.length,
                        separatorBuilder: (_, __) => const Divider(),
                        itemBuilder: (context, index) {
                          final file = _files![index];
                          final sizeMB = (file.size / (1024 * 1024)).toStringAsFixed(1);
                          return ListTile(
                            title: Text(file.filename, style: TextStyle(color: MemoTheme.textMain, fontSize: 13)),
                            subtitle: Text('$sizeMB MB', style: TextStyle(color: MemoTheme.textDim, fontSize: 11)),
                            trailing: IconButton(
                              icon: const Icon(Icons.download, color: MemoTheme.accent),
                              onPressed: () {
                                ref.read(apiClientProvider).downloadModel(widget.repoId, file.filename);
                                Navigator.pop(context);
                                ScaffoldMessenger.of(context).showSnackBar(
                                  SnackBar(content: Text('${file.filename} indirmesi başlatıldı...')),
                                );
                              },
                            ),
                          );
                        },
                      ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(context),
          child: const Text('Kapat'),
        ),
      ],
    );
  }
}

