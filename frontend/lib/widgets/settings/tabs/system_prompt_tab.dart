import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/theme.dart';
import '../../../core/l10n.dart';
import '../../../providers/settings_provider.dart';

class SystemPromptTab extends ConsumerStatefulWidget {
  SystemPromptTab();

  @override
  ConsumerState<SystemPromptTab> createState() => SystemPromptTabState();
}

class SystemPromptTabState extends ConsumerState<SystemPromptTab> {
  final _controller = TextEditingController();

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final asyncPrompt = ref.watch(systemPromptProvider);

    return ListView(
      padding: EdgeInsets.all(32),
      children: [
        Text(
          L10n.t('system_prompt'),
          style: Theme.of(context).textTheme.titleLarge?.copyWith(
            fontWeight: FontWeight.bold,
            color: MemoTheme.of(context).textMain,
          ),
        ),
        SizedBox(height: 12),
        Text(
          'Modelin temel davranışını, kimliğini ve sınırlarını belirleyen ana yönerge.',
          style: TextStyle(color: MemoTheme.of(context).textDim, fontSize: 13),
        ),
        SizedBox(height: 24),

        asyncPrompt.when(
          loading: () => Center(child: CircularProgressIndicator()),
          error: (e, _) => Text('${L10n.t('error')}: $e'),
          data: (prompt) {
            if (_controller.text != prompt) {
              _controller.text = prompt;
            }
            return Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                TextField(
                  controller: _controller,
                  maxLines: 12,
                  style: TextStyle(fontSize: 13, fontFamily: 'JetBrains Mono'),
                  decoration: InputDecoration(alignLabelWithHint: true),
                ),
                SizedBox(height: 16),
                Row(
                  mainAxisAlignment: MainAxisAlignment.end,
                  children: [
                    TextButton(
                      onPressed: () {
                        ref.read(systemPromptProvider.notifier).reset();
                      },
                      child: Text(L10n.t('reset_prompt')),
                    ),
                    SizedBox(width: 12),
                    ElevatedButton(
                      onPressed: () {
                        ref
                            .read(systemPromptProvider.notifier)
                            .save(_controller.text);
                        ScaffoldMessenger.of(context).showSnackBar(
                          SnackBar(content: Text('${L10n.t('save')} başarılı')),
                        );
                      },
                      child: Text(L10n.t('save')),
                    ),
                  ],
                ),
              ],
            );
          },
        ),
      ],
    );
  }
}
