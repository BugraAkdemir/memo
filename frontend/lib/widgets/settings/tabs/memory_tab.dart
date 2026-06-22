import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/theme.dart';
import '../../../core/l10n.dart';
import 'package:flutter/services.dart';
import '../../../providers/settings_provider.dart';

class MemoryTab extends ConsumerStatefulWidget {
  MemoryTab();

  @override
  ConsumerState<MemoryTab> createState() => MemoryTabState();
}

class MemoryTabState extends ConsumerState<MemoryTab> {
  final _topKController = TextEditingController();
  final _minSimilarityController = TextEditingController();
  bool _settingsInitialized = false;
  bool _savingSettings = false;

  @override
  void dispose() {
    _topKController.dispose();
    _minSimilarityController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final memoryAsync = ref.watch(memoryFilesProvider);
    final settingsAsync = ref.watch(memorySettingsProvider);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Expanded(
          child: ListView(
            padding: EdgeInsets.all(32),
            children: [
              Text(
                L10n.t('memory'),
                style: Theme.of(context).textTheme.titleLarge?.copyWith(
                  fontWeight: FontWeight.bold,
                  color: MemoTheme.of(context).textMain,
                ),
              ),
              SizedBox(height: 8),
              Text(
                L10n.t('memory_advanced_hint'),
                style: TextStyle(
                  color: MemoTheme.of(context).textDim,
                  fontSize: 13,
                ),
              ),
              SizedBox(height: 20),
              settingsAsync.when(
                loading: () => Center(child: CircularProgressIndicator()),
                error: (e, _) => Text('${L10n.t('error')}: $e'),
                data: (settings) {
                  if (!_settingsInitialized) {
                    _topKController.text = settings.topK.toString();
                    _minSimilarityController.text = settings.minSimilarity
                        .toStringAsFixed(2);
                    _settingsInitialized = true;
                  }

                  return Container(
                    padding: EdgeInsets.all(16),
                    decoration: BoxDecoration(
                      color: MemoTheme.of(context).bgPanel,
                      borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                      border: Border.all(
                        color: MemoTheme.of(context).borderSoft,
                      ),
                    ),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        Text(
                          L10n.t('memory_retrieval_settings'),
                          style: TextStyle(
                            fontSize: 14,
                            fontWeight: FontWeight.w600,
                            color: MemoTheme.of(context).textMain,
                          ),
                        ),
                        SizedBox(height: 14),
                        MemorySettingField(
                          label: L10n.t('memory_top_k'),
                          controller: _topKController,
                          hint: '3',
                          inputFormatters: [
                            FilteringTextInputFormatter.digitsOnly,
                          ],
                        ),
                        SizedBox(height: 12),
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
                        SizedBox(height: 14),
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
                                            .read(
                                              memorySettingsProvider.notifier,
                                            )
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
                                                '${L10n.t('error')}: $e',
                                              ),
                                            ),
                                          );
                                        }
                                      } finally {
                                        if (mounted) {
                                          setState(
                                            () => _savingSettings = false,
                                          );
                                        }
                                      }
                                    },
                              child: _savingSettings
                                  ? SizedBox(
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
              SizedBox(height: 28),
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Text(
                    L10n.t('memory_files'),
                    style: Theme.of(context).textTheme.titleMedium?.copyWith(
                      fontWeight: FontWeight.bold,
                      color: MemoTheme.of(context).textMain,
                    ),
                  ),
                  OutlinedButton.icon(
                    icon: Icon(Icons.delete_sweep, size: 18),
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
                          content: Text(
                            L10n.t('clear_memory_confirm_ext'),
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
                              SnackBar(content: Text('${L10n.t('error')}: $e')),
                            );
                          }
                        }
                      }
                    },
                  ),
                ],
              ),
              SizedBox(height: 12),
              memoryAsync.when(
                loading: () => Center(child: CircularProgressIndicator()),
                error: (e, _) => Center(child: Text('${L10n.t('error')}: $e')),
                data: (files) {
                  if (files.isEmpty) {
                    return Padding(
                      padding: EdgeInsets.only(top: 40),
                      child: Center(
                        child: Text(
                          L10n.t('no_memory_files'),
                          style: TextStyle(
                            color: MemoTheme.of(context).textDim,
                          ),
                        ),
                      ),
                    );
                  }

                  return Column(
                    children: files.map((file) {
                      return Column(
                        children: [
                          ListTile(
                            contentPadding: EdgeInsets.zero,
                            title: Text(
                              file.name,
                              style: TextStyle(fontWeight: FontWeight.w500),
                            ),
                            subtitle: Text(
                              '${file.sizeKb} KB • ${file.modified}',
                              style: TextStyle(
                                color: MemoTheme.of(context).textDim,
                                fontSize: 12,
                              ),
                            ),
                            trailing: IconButton(
                              icon: Icon(Icons.delete_outline),
                              color: MemoTheme.red,
                              onPressed: () async {
                                try {
                                  await ref
                                      .read(memoryFilesProvider.notifier)
                                      .deleteFile(file.path);
                                } catch (e) {
                                  if (context.mounted) {
                                    ScaffoldMessenger.of(context).showSnackBar(
                                      SnackBar(content: Text('${L10n.t('error')}: $e')),
                                    );
                                  }
                                }
                              },
                            ),
                          ),
                          Divider(),
                        ],
                      );
                    }).toList(),
                  );
                },
              ),
            ],
          ),
        ),
      ],
    );
  }
}

class MemorySettingField extends StatelessWidget {
  final String label;
  final TextEditingController controller;
  final String hint;
  final List<TextInputFormatter> inputFormatters;

  MemorySettingField({
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
