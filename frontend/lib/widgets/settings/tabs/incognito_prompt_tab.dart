import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/theme.dart';
import '../../../core/l10n.dart';
import '../../../providers/settings_provider.dart';
import '../../../core/friendly_error.dart';

class IncognitoPromptTab extends ConsumerStatefulWidget {
  const IncognitoPromptTab({super.key});

  @override
  ConsumerState<IncognitoPromptTab> createState() =>
      IncognitoPromptTabState();
}

class IncognitoPromptTabState extends ConsumerState<IncognitoPromptTab> {
  final _controller = TextEditingController();
  bool _initialized = false;

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final asyncPrompt = ref.watch(incognitoPromptProvider);

    return ListView(
      padding: EdgeInsets.all(32),
      children: [
        Text(
          L10n.t('incognito_prompt'),
          style: Theme.of(context).textTheme.titleLarge?.copyWith(
            fontWeight: FontWeight.bold,
            color: MemoTheme.of(context).textMain,
          ),
        ),
        SizedBox(height: 12),
        Text(
          L10n.t('incognito_prompt_desc'),
          style: TextStyle(color: MemoTheme.of(context).textDim, fontSize: 13),
        ),
        SizedBox(height: 24),

        asyncPrompt.when(
          loading: () => Center(child: CircularProgressIndicator()),
          error: (e, _) => Text('${L10n.t('error')}: ${FriendlyError.describeGeneric(e)}'),
          data: (prompt) {
            // Always update controller text; _initialized prevents overwriting user edits on rebuild
            if (!_initialized || _controller.text.isEmpty) {
              _controller.text = prompt;
              _initialized = true;
            }
            return Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                TextField(
                  controller: _controller,
                  maxLines: 12,
                  style: TextStyle(fontSize: 13, fontFamily: 'JetBrains Mono'),
                ),
                SizedBox(height: 16),
                Row(
                  mainAxisAlignment: MainAxisAlignment.end,
                  children: [
                    ElevatedButton(
                      onPressed: () {
                        ref
                            .read(incognitoPromptProvider.notifier)
                            .save(_controller.text);
                        ScaffoldMessenger.of(context).showSnackBar(
                          SnackBar(content: Text(L10n.t('save_successful'))),
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
