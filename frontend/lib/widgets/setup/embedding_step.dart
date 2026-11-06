import 'package:flutter/material.dart';

import '../../core/l10n.dart';
import '../../core/theme.dart';
import 'setup_widgets.dart';

/// The embedding-model step: memory and search only work with a small local
/// embedding model, whichever way the chat model is connected. It is
/// recommended but skippable, and skipping says plainly what stays off.
class SetupEmbeddingStep extends StatelessWidget {
  final ThemeColors color;
  final bool hasModel;
  final bool downloading;
  final String? error;
  final VoidCallback onDownloadRecommended;
  final VoidCallback onOpenStore;

  const SetupEmbeddingStep({
    super.key,
    required this.color,
    required this.hasModel,
    required this.downloading,
    required this.error,
    required this.onDownloadRecommended,
    required this.onOpenStore,
  });

  @override
  Widget build(BuildContext context) {
    return SetupCard(
      color: color,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (hasModel)
            Row(
              key: const Key('setup_embedding_ready'),
              children: [
                const Icon(
                  Icons.check_circle_outline,
                  size: 18,
                  color: MemoTheme.green,
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    L10n.t('setup_embedding_ready'),
                    style: const TextStyle(
                      color: MemoTheme.green,
                      fontSize: 13,
                    ),
                  ),
                ),
              ],
            )
          else ...[
            Text(
              L10n.t('setup_embedding_why'),
              style: TextStyle(color: color.textDim, fontSize: 12, height: 1.5),
            ),
            const SizedBox(height: 10),
            Container(
              key: const Key('setup_embedding_skip_note'),
              padding: const EdgeInsets.all(10),
              decoration: BoxDecoration(
                color: MemoTheme.warningOrange.withValues(alpha: 0.08),
                borderRadius: BorderRadius.circular(8),
              ),
              child: Text(
                L10n.t('setup_embedding_skip_note'),
                style: const TextStyle(
                  fontSize: 12,
                  color: MemoTheme.warningOrange,
                  height: 1.3,
                ),
              ),
            ),
            if (error != null) ...[
              const SizedBox(height: 10),
              Text(error!, style: const TextStyle(color: MemoTheme.red)),
            ],
            const SizedBox(height: 14),
            Wrap(
              spacing: 10,
              runSpacing: 10,
              children: [
                FilledButton.icon(
                  key: const Key('setup_embedding_download'),
                  onPressed: downloading ? null : onDownloadRecommended,
                  icon: downloading
                      ? const SizedBox(
                          width: 16,
                          height: 16,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : const Icon(Icons.download_outlined, size: 18),
                  label: Text(
                    L10n.t(
                      downloading
                          ? 'setup_memory_model_downloading'
                          : 'setup_embedding_download',
                    ),
                  ),
                ),
                OutlinedButton.icon(
                  key: const Key('setup_embedding_store'),
                  onPressed: downloading ? null : onOpenStore,
                  icon: const Icon(Icons.storefront_outlined, size: 18),
                  label: Text(L10n.t('setup_embedding_store')),
                ),
              ],
            ),
          ],
        ],
      ),
    );
  }
}
