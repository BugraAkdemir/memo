import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/theme.dart';
import '../../../core/l10n.dart';
import '../../../providers/settings_provider.dart';
import '../../error_retry.dart';

/// Settings editor for Code Mode's three sub-mode system prompts
/// (plan/auto/build) — see internal/app/agent_chat_context.go's
/// codeSubModeDirective for the built-in defaults each section's Reset
/// button restores. One tab, three stacked sections (not three tabs): the
/// three prompts are conceptually one feature, and this keeps
/// settings_dialog.dart's tab list from growing by three entries instead
/// of one.
class CodeSubModePromptsTab extends ConsumerWidget {
  const CodeSubModePromptsTab({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final planPrompt = ref.watch(codePlanPromptProvider);
    final autoPrompt = ref.watch(codeAutoPromptProvider);
    final buildPrompt = ref.watch(codeBuildPromptProvider);

    return ListView(
      padding: const EdgeInsets.all(32),
      children: [
        Text(
          L10n.t('code_submode_prompts_title'),
          style: Theme.of(context).textTheme.titleLarge?.copyWith(
            fontWeight: FontWeight.bold,
            color: MemoTheme.of(context).textMain,
          ),
        ),
        const SizedBox(height: 12),
        Text(
          L10n.t('code_submode_prompts_desc'),
          style: TextStyle(color: MemoTheme.of(context).textDim, fontSize: 13),
        ),
        const SizedBox(height: 24),
        _CodeSubModePromptSection(
          label: L10n.t('code_submode_plan_prompt_label'),
          desc: L10n.t('code_submode_plan_prompt_desc'),
          asyncPrompt: planPrompt,
          onRetry: () => ref.invalidate(codePlanPromptProvider),
          onSave: (v) => ref.read(codePlanPromptProvider.notifier).save(v),
          onReset: () => ref.read(codePlanPromptProvider.notifier).reset(),
        ),
        const SizedBox(height: 32),
        _CodeSubModePromptSection(
          label: L10n.t('code_submode_auto_prompt_label'),
          desc: L10n.t('code_submode_auto_prompt_desc'),
          asyncPrompt: autoPrompt,
          onRetry: () => ref.invalidate(codeAutoPromptProvider),
          onSave: (v) => ref.read(codeAutoPromptProvider.notifier).save(v),
          onReset: () => ref.read(codeAutoPromptProvider.notifier).reset(),
        ),
        const SizedBox(height: 32),
        _CodeSubModePromptSection(
          label: L10n.t('code_submode_build_prompt_label'),
          desc: L10n.t('code_submode_build_prompt_desc'),
          asyncPrompt: buildPrompt,
          onRetry: () => ref.invalidate(codeBuildPromptProvider),
          onSave: (v) => ref.read(codeBuildPromptProvider.notifier).save(v),
          onReset: () => ref.read(codeBuildPromptProvider.notifier).reset(),
        ),
      ],
    );
  }
}

/// One sub-mode's editor — structurally the same TextField + Save/Reset row
/// as SystemPromptTab, minus the persona quick-pick (Code Mode's prompts
/// have no preset gallery). Takes its data and actions as plain
/// values/callbacks (not a provider reference) so it stays a single,
/// non-generic widget reused three times by [CodeSubModePromptsTab] —
/// avoids the three sub-mode notifiers needing a shared base class just to
/// satisfy a generic bound.
class _CodeSubModePromptSection extends StatefulWidget {
  final String label;
  final String desc;
  final AsyncValue<String> asyncPrompt;
  final VoidCallback onRetry;
  final ValueChanged<String> onSave;
  final VoidCallback onReset;

  const _CodeSubModePromptSection({
    required this.label,
    required this.desc,
    required this.asyncPrompt,
    required this.onRetry,
    required this.onSave,
    required this.onReset,
  });

  @override
  State<_CodeSubModePromptSection> createState() => _CodeSubModePromptSectionState();
}

class _CodeSubModePromptSectionState extends State<_CodeSubModePromptSection> {
  final _controller = TextEditingController();
  // Same "sync from async data, not from _controller.text" guard as
  // SystemPromptTabState — nothing else writes into this controller here,
  // but keeping the identical idiom avoids a subtly different bug class
  // creeping in later if something ever does.
  String? _lastLoadedPrompt;

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(
          widget.label,
          style: TextStyle(
            fontSize: 13,
            fontWeight: FontWeight.w600,
            color: MemoTheme.of(context).textMain,
          ),
        ),
        const SizedBox(height: 4),
        Text(
          widget.desc,
          style: TextStyle(color: MemoTheme.of(context).textDim, fontSize: 12),
        ),
        const SizedBox(height: 12),
        widget.asyncPrompt.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (e, _) => ErrorRetryLine(error: e, onRetry: widget.onRetry),
          data: (prompt) {
            if (_lastLoadedPrompt != prompt) {
              _lastLoadedPrompt = prompt;
              _controller.text = prompt;
            }
            return Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                TextField(
                  controller: _controller,
                  maxLines: 8,
                  style: const TextStyle(fontSize: 13, fontFamily: 'JetBrains Mono'),
                  decoration: InputDecoration(
                    alignLabelWithHint: true,
                    hintText: prompt.isEmpty ? L10n.t('code_submode_prompt_default_hint') : null,
                  ),
                ),
                const SizedBox(height: 12),
                Row(
                  mainAxisAlignment: MainAxisAlignment.end,
                  children: [
                    TextButton(
                      onPressed: widget.onReset,
                      child: Text(L10n.t('reset_prompt')),
                    ),
                    const SizedBox(width: 12),
                    ElevatedButton(
                      onPressed: () {
                        widget.onSave(_controller.text);
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
