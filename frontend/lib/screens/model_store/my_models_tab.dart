import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:path/path.dart' as p;

import '../../core/l10n.dart';
import '../../core/theme.dart';
import '../../models/local_model.dart';
import '../../providers/chat_provider.dart';
import '../../providers/models_provider.dart';
import '../../widgets/model_config_dialog.dart';
import '../../core/friendly_error.dart';

// ─── My Models tab (unchanged design) ────────────────────────────

class MyModelsTab extends ConsumerWidget {
  const MyModelsTab({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = MemoTheme.of(context);
    final localAsync = ref.watch(localModelsProvider);

    return localAsync.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (e, _) => Center(child: Text('${L10n.t('error')}: ${FriendlyError.describeGeneric(e)}')),
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
                    if (model.supportsTools)
                      _Pill(
                        text: L10n.t('tools'),
                        highlight: true,
                      ),
                    if (model.supportsVision && !model.isVision)
                      _Pill(
                        text: L10n.t('vision_3'),
                        highlight: true,
                      ),
                    if (model.supportsCode)
                      _Pill(
                        text: L10n.t('code'),
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

/// Stacks one banner per in-flight (or errored) download — several files
/// can now download at once (e.g. the setup wizard's chat + memory model).
class DownloadBanner extends ConsumerWidget {
  const DownloadBanner({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final downloads = ref.watch(downloadProgressProvider).valueOrNull ?? [];

    ref.listen(downloadProgressProvider, (prev, next) {
      final prevActive = {
        for (final p in prev?.valueOrNull ?? const <DownloadProgress>[])
          if (p.active) '${p.repoId}/${p.filename}': p,
      };
      final nextByKey = {
        for (final p in next.valueOrNull ?? const <DownloadProgress>[])
          '${p.repoId}/${p.filename}': p,
      };
      for (final key in prevActive.keys) {
        final now = nextByKey[key];
        final nowActive = now?.active ?? false;
        final nowError = now?.error;
        if (!nowActive && (nowError == null || nowError.isEmpty)) {
          ref.invalidate(localModelsProvider);
          break;
        }
      }
    });

    final visible = downloads.where((p) => p.active || p.error != null).toList();
    if (visible.isEmpty) return const SizedBox.shrink();

    return Column(
      children: [for (final p in visible) _DownloadBannerRow(progress: p)],
    );
  }
}

class _DownloadBannerRow extends ConsumerWidget {
  final DownloadProgress progress;
  const _DownloadBannerRow({required this.progress});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = MemoTheme.of(context);

    return Container(
      padding: const EdgeInsets.fromLTRB(28, 12, 28, 12),
      decoration: BoxDecoration(
        color: progress.error != null ? Colors.red.shade50 : c.bgPanel,
        border: Border(bottom: BorderSide(
            color: progress.error != null ? Colors.red.shade200 : c.borderSoft)),
      ),
      child: Row(
        children: [
          Icon(Icons.cloud_download_outlined,
              size: 18,
              color: progress.error != null ? Colors.red : MemoTheme.accent),
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
                      progress.error != null
                          ? 'Failed'
                          : '${progress.percent.toStringAsFixed(0)}%${progress.speed.isNotEmpty ? ' · ${progress.speed}' : ''}',
                      style: TextStyle(
                          fontSize: 12,
                          fontWeight: FontWeight.w600,
                          color: progress.error != null
                              ? Colors.red
                              : MemoTheme.accent),
                    ),
                  ],
                ),
                if (progress.error != null)
                  Padding(
                    padding: const EdgeInsets.only(top: 4),
                    child: Text(
                      progress.error!,
                      style: TextStyle(
                          fontSize: 11,
                          color: Colors.red.shade700,
                          height: 1.2),
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                    ),
                  )
                else
                  ...[
                    const SizedBox(height: 6),
                    ClipRRect(
                      borderRadius: BorderRadius.circular(999),
                      child: LinearProgressIndicator(
                        value: progress.totalBytes > 0
                            ? progress.percent / 100.0
                            : null,
                        minHeight: 4,
                        backgroundColor: c.bgElement,
                        valueColor:
                            const AlwaysStoppedAnimation(MemoTheme.accent),
                      ),
                    ),
                  ],
              ],
            ),
          ),
          const SizedBox(width: 8),
          IconButton(
            onPressed: () => ref
                .read(apiClientProvider)
                .cancelDownload(progress.repoId, progress.filename),
            icon: const Icon(Icons.close_rounded, size: 18),
            tooltip: L10n.t('cancel_download_2'),
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
