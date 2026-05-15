import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/local_model.dart';
import '../providers/models_provider.dart';
import '../providers/chat_provider.dart';

/// Dialog to configure model launch settings (ctx size, gpu layers, port) and start it.
class ModelConfigDialog extends ConsumerStatefulWidget {
  final LocalModel model;

  const ModelConfigDialog({super.key, required this.model});

  @override
  ConsumerState<ModelConfigDialog> createState() => _ModelConfigDialogState();
}

class _ModelConfigDialogState extends ConsumerState<ModelConfigDialog> {
  final _ctxSizeController = TextEditingController(text: '4096');
  final _gpuLayersController = TextEditingController(text: '33');
  final _portController = TextEditingController(text: '8080');

  bool _starting = false;

  @override
  void dispose() {
    _ctxSizeController.dispose();
    _gpuLayersController.dispose();
    _portController.dispose();
    super.dispose();
  }

  Future<void> _startModel() async {
    final ctx = int.tryParse(_ctxSizeController.text) ?? 4096;
    final gpuLayers = int.tryParse(_gpuLayersController.text) ?? 33;
    final port = int.tryParse(_portController.text) ?? 8080;

    setState(() => _starting = true);

    try {
      await ref.read(apiClientProvider).startModel(
            path: widget.model.path,
            ctxSize: ctx,
            port: port,
            gpuLayers: gpuLayers,
          );

      // Auto-start embedding model if the started model is not an embedding model
      if (!widget.model.isEmbedding) {
        final localModelsOpt = ref.read(localModelsProvider).valueOrNull;
        if (localModelsOpt != null) {
          // Find the first embedding model available
          final em = localModelsOpt.where((m) => m.isEmbedding).firstOrNull;
          if (em != null) {
            try {
              // Start it silently in the background with minimal layers
              await ref.read(apiClientProvider).startEmbeddingModel(
                    path: em.path,
                    gpuLayers: 0,
                  );
              ref.invalidate(embeddingStatusProvider);
            } catch (e) {
              debugPrint('Embedding start error: $e');
            }
          }
        }
      }
      
      // Refresh status after start
      ref.invalidate(modelStatusProvider);
      
      if (mounted) {
        Navigator.of(context).pop();
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('${widget.model.filename} başlatılıyor...')),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('${L10n.t('error')}: $e')),
        );
      }
    } finally {
      if (mounted) setState(() => _starting = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Dialog(
      backgroundColor: MemoTheme.bgApp,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
      ),
      child: Container(
        width: 400,
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              L10n.t('model_config'),
              style: Theme.of(context).textTheme.titleLarge?.copyWith(
                    fontWeight: FontWeight.bold,
                    color: MemoTheme.textMain,
                  ),
            ),
            const SizedBox(height: 8),
            Text(
              widget.model.filename,
              style: TextStyle(color: MemoTheme.accent, fontSize: 13),
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
            ),
            const SizedBox(height: 24),

            // Context Size
            _ConfigField(
              label: L10n.t('ctx_size'),
              controller: _ctxSizeController,
              hint: '4096',
            ),
            const SizedBox(height: 16),

            // GPU Layers
            _ConfigField(
              label: L10n.t('gpu_layers'),
              controller: _gpuLayersController,
              hint: '33',
            ),
            const SizedBox(height: 16),

            // Port
            _ConfigField(
              label: L10n.t('port'),
              controller: _portController,
              hint: '8080',
            ),
            const SizedBox(height: 32),

            // Actions
            Row(
              mainAxisAlignment: MainAxisAlignment.end,
              children: [
                TextButton(
                  onPressed: _starting ? null : () => Navigator.of(context).pop(),
                  child: Text(L10n.t('cancel'), style: TextStyle(color: MemoTheme.textDim)),
                ),
                const SizedBox(width: 12),
                ElevatedButton(
                  onPressed: _starting ? null : _startModel,
                  style: ElevatedButton.styleFrom(
                    backgroundColor: MemoTheme.accent,
                    foregroundColor: MemoTheme.textInverse,
                    padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 12),
                  ),
                  child: _starting
                      ? const SizedBox(
                          width: 16,
                          height: 16,
                          child: CircularProgressIndicator(strokeWidth: 2, color: MemoTheme.textInverse),
                        )
                      : Text(L10n.t('start_model')),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _ConfigField extends StatelessWidget {
  final String label;
  final TextEditingController controller;
  final String hint;

  const _ConfigField({
    required this.label,
    required this.controller,
    required this.hint,
  });

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        SizedBox(
          width: 120,
          child: Text(
            label,
            style: const TextStyle(fontWeight: FontWeight.w500, fontSize: 13),
          ),
        ),
        Expanded(
          child: SizedBox(
            height: 36,
            child: TextField(
              controller: controller,
              keyboardType: TextInputType.number,
              inputFormatters: [FilteringTextInputFormatter.digitsOnly],
              style: const TextStyle(fontSize: 13),
              decoration: InputDecoration(
                hintText: hint,
                contentPadding: const EdgeInsets.symmetric(horizontal: 12),
                filled: true,
                fillColor: MemoTheme.bgPanel,
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                  borderSide: BorderSide(color: MemoTheme.borderSoft),
                ),
                enabledBorder: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                  borderSide: BorderSide(color: MemoTheme.borderSoft),
                ),
                focusedBorder: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                  borderSide: const BorderSide(color: MemoTheme.accent),
                ),
              ),
            ),
          ),
        ),
      ],
    );
  }
}
