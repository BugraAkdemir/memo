import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/friendly_error.dart';
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
  // Free-text fallback, only ever used when widget.model.maxContext is 0
  // (unknown — the backend's GGUF parser didn't recognize this model's
  // architecture) and the slider therefore has nothing real to bound itself
  // against.
  final _ctxSizeController = TextEditingController(text: '4096');
  late int _ctxSize;
  final _gpuLayersController = TextEditingController(text: '33');
  final _portController = TextEditingController(text: '8081');

  bool _starting = false;
  String? _selectedEmbeddingPath;
  String _statusMessage = '';

  @override
  void initState() {
    super.initState();
    // Default to 4096 (the previous hardcoded default) or the model's own
    // max, whichever is smaller — never start above what's known-safe.
    final maxContext = widget.model.maxContext;
    _ctxSize = maxContext > 0 ? (4096 < maxContext ? 4096 : maxContext) : 4096;
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
    // widget.model.maxContext == 0 falls back to the free-text field
    // (_ConfigField below); otherwise the slider owns _ctxSize directly and
    // has already structurally clamped it to widget.model.maxContext.
    final ctx = widget.model.maxContext > 0
        ? _ctxSize
        : (int.tryParse(_ctxSizeController.text) ?? 4096);
    final gpuLayers = int.tryParse(_gpuLayersController.text) ?? 33;
    final port = int.tryParse(_portController.text) ?? 8081;
    final isEmbeddingModel = widget.model.isEmbedding;

    setState(() {
      _starting = true;
      _statusMessage =
          '⏳ ${isEmbeddingModel ? L10n.t('start_embedding') : L10n.t('start_model')}...';
    });

    try {
      if (isEmbeddingModel) {
        await ref
            .read(apiClientProvider)
            .startEmbeddingModel(path: widget.model.path, gpuLayers: gpuLayers);
        ref.invalidate(embeddingStatusProvider);
      } else {
        // 1. Start the main chat model
        await ref
            .read(apiClientProvider)
            .startModel(
              path: widget.model.path,
              ctxSize: ctx,
              port: port,
              gpuLayers: gpuLayers,
            );

        // 2. Start embedding model if selected and main model is not an embedding model
        if (_selectedEmbeddingPath != null) {
          if (mounted) {
            setState(() {
              _statusMessage =
                  '⏳ ${L10n.t('embedding_model')} ${L10n.t('starting')}...';
            });
          }

          try {
            await ref
                .read(apiClientProvider)
                .startEmbeddingModel(
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
                  content: Text(
                    '⚠️ ${L10n.t('embedding_model')}: ${FriendlyError.describe(e)}',
                  ),
                  backgroundColor: MemoTheme.warningOrange,
                ),
              );
            }
          }
        }

        // Refresh status after chat model start
        ref.invalidate(modelStatusProvider);
      }

      if (mounted) {
        final messenger = ScaffoldMessenger.of(context);
        Navigator.of(context).pop();
        messenger.showSnackBar(
          SnackBar(
            content: Text(
              '✅ ${widget.model.filename} ${L10n.t('starting')}...',
            ),
            backgroundColor: MemoTheme.green,
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('${L10n.t('error')}: ${FriendlyError.describe(e)}')),
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
      backgroundColor: MemoTheme.of(context).bgApp,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
      ),
      child: Container(
        width: 440,
        padding: EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              L10n.t('model_config'),
              style: Theme.of(context).textTheme.titleLarge?.copyWith(
                fontWeight: FontWeight.bold,
                color: MemoTheme.of(context).textMain,
              ),
            ),
            SizedBox(height: 8),
            Text(
              widget.model.filename,
              style: TextStyle(color: MemoTheme.accent, fontSize: 13),
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
            ),
            SizedBox(height: 24),

            if (!isEmbeddingModel) ...[
              // Context Size — bounded by the model's own real max when
              // known (see LocalModel.maxContext's doc comment); falls back
              // to a free-text field only when that couldn't be determined.
              if (widget.model.maxContext > 0)
                _ContextSizeSlider(
                  label: L10n.t('ctx_size'),
                  value: _ctxSize,
                  max: widget.model.maxContext,
                  onChanged: (v) => setState(() => _ctxSize = v),
                )
              else ...[
                _ConfigField(
                  label: L10n.t('ctx_size'),
                  controller: _ctxSizeController,
                  hint: '4096',
                ),
                SizedBox(height: 4),
                Text(
                  L10n.t('ctx_size_max_unknown'),
                  style: TextStyle(fontSize: 11, color: MemoTheme.warningOrange),
                ),
              ],
              SizedBox(height: 16),
            ],

            // GPU Layers
            _ConfigField(
              label: L10n.t('gpu_layers'),
              controller: _gpuLayersController,
              hint: '33',
            ),
            SizedBox(height: 16),

            if (!isEmbeddingModel) ...[
              // Port
              _ConfigField(
                label: L10n.t('port'),
                controller: _portController,
                hint: '8081',
              ),
            ],

            // Embedding Model Selector — only show when starting a chat model
            if (!isEmbeddingModel) ...[
              SizedBox(height: 20),
              Divider(color: MemoTheme.of(context).borderSoft),
              SizedBox(height: 12),
              Row(
                children: [
                  Icon(Icons.memory, size: 16, color: MemoTheme.accent),
                  SizedBox(width: 8),
                  Text(
                    L10n.t('embedding_model'),
                    style: TextStyle(
                      fontWeight: FontWeight.w600,
                      fontSize: 13,
                      color: MemoTheme.of(context).textMain,
                    ),
                  ),
                ],
              ),
              SizedBox(height: 4),
              Text(
                L10n.t('embedding_hint'),
                style: TextStyle(
                  fontSize: 11,
                  color: MemoTheme.of(context).textDim,
                ),
              ),
              SizedBox(height: 8),
              Container(
                height: 40,
                decoration: BoxDecoration(
                  color: MemoTheme.of(context).bgPanel,
                  borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                  border: Border.all(color: MemoTheme.of(context).borderSoft),
                ),
                padding: EdgeInsets.symmetric(horizontal: 12),
                child: embeddingModels.isEmpty
                    ? Row(
                        children: [
                          Icon(
                            Icons.warning_amber_rounded,
                            size: 16,
                            color: MemoTheme.warningOrange,
                          ),
                          SizedBox(width: 8),
                          Text(
                            L10n.t('no_embedding_model'),
                            style: TextStyle(
                              fontSize: 12,
                              color: MemoTheme.warningOrange,
                            ),
                          ),
                        ],
                      )
                    : DropdownButtonHideUnderline(
                        child: DropdownButton<String>(
                          isExpanded: true,
                          dropdownColor: MemoTheme.of(context).bgPanel.withValues(alpha: 1.0),
                          elevation: 8,
                          value: _selectedEmbeddingPath,
                          style: TextStyle(
                            fontSize: 12,
                            color: MemoTheme.of(context).textMain,
                          ),
                          icon: Icon(
                            Icons.arrow_drop_down,
                            color: MemoTheme.of(context).textDim,
                          ),
                          items: [
                            DropdownMenuItem<String>(
                              value: null,
                              child: Text(
                                L10n.t('no_embedding_selection'),
                                style: TextStyle(
                                  color: MemoTheme.of(context).textDim,
                                  fontSize: 12,
                                ),
                              ),
                            ),
                            ...embeddingModels.map(
                              (m) => DropdownMenuItem(
                                value: m.path,
                                child: Text(
                                  '${m.filename}  (${m.sizeFormatted})',
                                  style: TextStyle(fontSize: 12),
                                  overflow: TextOverflow.ellipsis,
                                ),
                              ),
                            ),
                          ],
                          onChanged: (v) {
                            setState(() => _selectedEmbeddingPath = v);
                          },
                        ),
                      ),
              ),
            ],

            SizedBox(height: 24),

            // Status message while starting
            if (_statusMessage.isNotEmpty)
              Padding(
                padding: EdgeInsets.only(bottom: 12),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        SizedBox(
                          width: 14,
                          height: 14,
                          child: CircularProgressIndicator(
                            strokeWidth: 2,
                            color: MemoTheme.accent,
                          ),
                        ),
                        SizedBox(width: 8),
                        Expanded(
                          child: Text(
                            _statusMessage,
                            style: TextStyle(fontSize: 12, color: MemoTheme.accent),
                          ),
                        ),
                      ],
                    ),
                    // Loading a multi-GB GGUF (first cold read off disk,
                    // especially) can genuinely take close to a minute — the
                    // backend itself budgets up to 180s (chat model) / ~6min
                    // across retries (embedding) before giving up. Without
                    // this, a first-time user watching the spinner sit still
                    // that long has no way to tell "still working" apart from
                    // "frozen."
                    Padding(
                      padding: EdgeInsets.only(left: 22, top: 4),
                      child: Text(
                        L10n.t('model_start_slow_hint'),
                        style: TextStyle(fontSize: 11, color: MemoTheme.of(context).textDim),
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
                  onPressed: _starting
                      ? null
                      : () => Navigator.of(context).pop(),
                  child: Text(
                    L10n.t('cancel'),
                    style: TextStyle(color: MemoTheme.of(context).textDim),
                  ),
                ),
                SizedBox(width: 12),
                ElevatedButton(
                  onPressed: _starting ? null : _startModel,
                  style: ElevatedButton.styleFrom(
                    backgroundColor: MemoTheme.accent,
                    foregroundColor: MemoTheme.of(context).textInverse,
                    padding: EdgeInsets.symmetric(horizontal: 24, vertical: 12),
                  ),
                  child: _starting
                      ? SizedBox(
                          width: 16,
                          height: 16,
                          child: CircularProgressIndicator(
                            strokeWidth: 2,
                            color: MemoTheme.of(context).textInverse,
                          ),
                        )
                      : Text(
                          isEmbeddingModel
                              ? L10n.t('start_embedding')
                              : L10n.t('start_model'),
                        ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

/// Bounded context-size control (BUG fix: the previous free-text field
/// accepted any value the user typed, including far more than a given model
/// actually supports — e.g. 10,000,000 against a model trained for 131,072
/// — which crashed llama-server outright at startup instead of failing
/// cleanly). A plain [Slider]'s own `max` makes an out-of-range value
/// structurally unreachable, not just discouraged by a hint string.
class _ContextSizeSlider extends StatelessWidget {
  final String label;
  final int value;
  final int max;
  final ValueChanged<int> onChanged;

  static const _minContext = 512;

  const _ContextSizeSlider({
    required this.label,
    required this.value,
    required this.max,
    required this.onChanged,
  });

  @override
  Widget build(BuildContext context) {
    // A model whose real max is already below the usual floor (rare, but
    // possible for a tiny/legacy conversion) still needs a valid [min, max]
    // range for Slider — collapse to a fixed point rather than crashing.
    final sliderMin = _minContext < max ? _minContext : max;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Text(label, style: TextStyle(fontWeight: FontWeight.w500, fontSize: 13)),
            Spacer(),
            Text(
              L10n.t('ctx_size_of_max', {'cur': '$value', 'max': '$max'}),
              style: TextStyle(fontSize: 12, color: MemoTheme.of(context).textDim),
            ),
          ],
        ),
        Slider(
          value: value.toDouble().clamp(sliderMin.toDouble(), max.toDouble()),
          min: sliderMin.toDouble(),
          max: max.toDouble(),
          activeColor: MemoTheme.accent,
          label: '$value',
          onChanged: sliderMin == max
              ? null
              : (v) => onChanged(v.round()),
        ),
      ],
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
            style: TextStyle(fontWeight: FontWeight.w500, fontSize: 13),
          ),
        ),
        Expanded(
          child: SizedBox(
            height: 36,
            child: TextField(
              controller: controller,
              keyboardType: TextInputType.number,
              inputFormatters: [FilteringTextInputFormatter.digitsOnly],
              style: TextStyle(fontSize: 13),
              decoration: InputDecoration(
                hintText: hint,
                contentPadding: EdgeInsets.symmetric(horizontal: 12),
                filled: true,
                fillColor: MemoTheme.of(context).bgPanel,
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
