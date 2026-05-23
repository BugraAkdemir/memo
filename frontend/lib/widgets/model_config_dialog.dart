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
  final _portController = TextEditingController(text: '8081');

  bool _starting = false;
  String? _selectedEmbeddingPath;
  String _statusMessage = '';

  @override
  void initState() {
    super.initState();
    // Pre-select the first embedding model if available
    WidgetsBinding.instance.addPostFrameCallback((_) {
      final models = ref.read(localModelsProvider).valueOrNull ?? [];
      final embeddingModels = models.where((m) => m.isEmbedding).toList();
      if (embeddingModels.isNotEmpty && mounted) {
        setState(() {
          _selectedEmbeddingPath = embeddingModels.first.path;
        });
      }
    });
  }

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
    final port = int.tryParse(_portController.text) ?? 8081;

    setState(() {
      _starting = true;
      _statusMessage = '⏳ ${L10n.t('start_model')}...';
    });

    try {
      // 1. Start the main chat model
      await ref.read(apiClientProvider).startModel(
            path: widget.model.path,
            ctxSize: ctx,
            port: port,
            gpuLayers: gpuLayers,
          );

      // 2. Start embedding model if selected and main model is not an embedding model
      if (!widget.model.isEmbedding && _selectedEmbeddingPath != null) {
        if (mounted) {
          setState(() {
            _statusMessage = '⏳ ${L10n.t('embedding_model')} ${L10n.t('starting')}...';
          });
        }

        try {
          await ref.read(apiClientProvider).startEmbeddingModel(
                path: _selectedEmbeddingPath!,
                gpuLayers: 0,
              );
          ref.invalidate(embeddingStatusProvider);
        } catch (e) {
          debugPrint('Embedding start error: $e');
          // Don't block — embedding is optional, show warning
          if (mounted) {
            ScaffoldMessenger.of(context).showSnackBar(
              SnackBar(
                content: Text('⚠️ ${L10n.t('embedding_model')}: $e'),
                backgroundColor: Colors.orange.shade800,
              ),
            );
          }
        }
      }

      // Refresh status after start
      ref.invalidate(modelStatusProvider);

      if (mounted) {
        Navigator.of(context).pop();
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('✅ ${widget.model.filename} ${L10n.t('starting')}...'),
            backgroundColor: Colors.green.shade800,
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('${L10n.t('error')}: $e')),
        );
      }
    } finally {
      if (mounted) {
        setState(() {
          _starting = false;
          _statusMessage = '';
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final models = ref.watch(localModelsProvider).valueOrNull ?? [];
    final embeddingModels = models.where((m) => m.isEmbedding).toList();
    final isEmbeddingModel = widget.model.isEmbedding;

    return Dialog(
      backgroundColor: MemoTheme.bgApp,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
      ),
      child: Container(
        width: 440,
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
              hint: '8081',
            ),

            // Embedding Model Selector — only show when starting a chat model
            if (!isEmbeddingModel) ...[
              const SizedBox(height: 20),
              const Divider(color: MemoTheme.borderSoft),
              const SizedBox(height: 12),
              Row(
                children: [
                  Icon(Icons.memory, size: 16, color: MemoTheme.accent),
                  const SizedBox(width: 8),
                  Text(
                    L10n.t('embedding_model'),
                    style: const TextStyle(
                      fontWeight: FontWeight.w600,
                      fontSize: 13,
                      color: MemoTheme.textMain,
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 4),
              Text(
                L10n.t('embedding_hint'),
                style: TextStyle(
                  fontSize: 11,
                  color: MemoTheme.textDim,
                ),
              ),
              const SizedBox(height: 8),
              Container(
                height: 40,
                decoration: BoxDecoration(
                  color: MemoTheme.bgPanel,
                  borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                  border: Border.all(color: MemoTheme.borderSoft),
                ),
                padding: const EdgeInsets.symmetric(horizontal: 12),
                child: embeddingModels.isEmpty
                    ? Row(
                        children: [
                          Icon(Icons.warning_amber_rounded,
                              size: 16, color: Colors.orange.shade400),
                          const SizedBox(width: 8),
                          Text(
                            L10n.t('no_embedding_model'),
                            style: TextStyle(
                              fontSize: 12,
                              color: Colors.orange.shade400,
                            ),
                          ),
                        ],
                      )
                    : DropdownButtonHideUnderline(
                        child: DropdownButton<String>(
                          isExpanded: true,
                          dropdownColor: MemoTheme.bgPanel,
                          value: _selectedEmbeddingPath,
                          style: const TextStyle(
                            fontSize: 12,
                            color: MemoTheme.textMain,
                          ),
                          icon: const Icon(Icons.arrow_drop_down,
                              color: MemoTheme.textDim),
                          items: [
                            DropdownMenuItem<String>(
                              value: null,
                              child: Text(
                                L10n.t('no_embedding_selection'),
                                style: TextStyle(
                                  color: MemoTheme.textDim,
                                  fontSize: 12,
                                ),
                              ),
                            ),
                            ...embeddingModels.map((m) => DropdownMenuItem(
                                  value: m.path,
                                  child: Text(
                                    '${m.filename}  (${m.sizeFormatted})',
                                    style: const TextStyle(fontSize: 12),
                                    overflow: TextOverflow.ellipsis,
                                  ),
                                )),
                          ],
                          onChanged: (v) {
                            setState(() => _selectedEmbeddingPath = v);
                          },
                        ),
                      ),
              ),
            ],

            const SizedBox(height: 24),

            // Status message while starting
            if (_statusMessage.isNotEmpty)
              Padding(
                padding: const EdgeInsets.only(bottom: 12),
                child: Row(
                  children: [
                    const SizedBox(
                      width: 14,
                      height: 14,
                      child: CircularProgressIndicator(
                        strokeWidth: 2,
                        color: MemoTheme.accent,
                      ),
                    ),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text(
                        _statusMessage,
                        style: TextStyle(
                          fontSize: 12,
                          color: MemoTheme.accent,
                        ),
                      ),
                    ),
                  ],
                ),
              ),

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
