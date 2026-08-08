import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/theme.dart';
import '../../../core/l10n.dart';
import '../../../models/persona_presets.dart';
import '../../../providers/settings_provider.dart';
import '../../persona_picker.dart';
import '../../../core/friendly_error.dart';

class SystemPromptTab extends ConsumerStatefulWidget {
  const SystemPromptTab({super.key});

  @override
  ConsumerState<SystemPromptTab> createState() => SystemPromptTabState();
}

class SystemPromptTabState extends ConsumerState<SystemPromptTab> {
  final _controller = TextEditingController();
  // Tracks what _controller was last synced FROM (the async data), as
  // opposed to comparing against _controller.text directly — the persona
  // picker below also writes into _controller outside of that data flow, so
  // a plain "does the controller already match the loaded prompt" check
  // would misfire on every rebuild after a persona pick and stomp the
  // just-applied text back to the old saved prompt.
  String? _lastLoadedPrompt;

  // Quick-pick persona state — purely a generator for _controller's text
  // above, never persisted on its own or read back from the saved prompt:
  // includeCustomOption is false below (this tab's own text field already
  // is the "custom" editor), so _personaKey only ever holds a real preset
  // key once the user taps a card, never 'custom'. Starts pointing at the
  // first preset so a name typed before ever tapping a card still has
  // somewhere to apply to; it just doesn't visually select a card until one
  // is actually tapped (build() below only passes a key to PersonaPicker
  // once _personaTouched is true).
  String _personaKey = personaPresets.first.key;
  bool _personaTouched = false;
  final _personaNameController = TextEditingController();

  void _applyPersona(String key) {
    _personaKey = key;
    setState(() => _personaTouched = true);
    _recomposeFromPersona();
  }

  void _onPersonaNameChanged(String name) {
    if (!_personaTouched) return;
    _recomposeFromPersona();
  }

  void _recomposeFromPersona() {
    for (final p in personaPresets) {
      if (p.key == _personaKey) {
        _controller.text = composePersonaPrompt(_personaNameController.text, p.prompt);
        break;
      }
    }
  }

  @override
  void dispose() {
    _controller.dispose();
    _personaNameController.dispose();
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
          L10n.t('system_prompt_desc'),
          style: TextStyle(color: MemoTheme.of(context).textDim, fontSize: 13),
        ),
        SizedBox(height: 24),

        asyncPrompt.when(
          loading: () => Center(child: CircularProgressIndicator()),
          error: (e, _) => Text('${L10n.t('error')}: ${FriendlyError.describeGeneric(e)}'),
          data: (prompt) {
            if (_lastLoadedPrompt != prompt) {
              _lastLoadedPrompt = prompt;
              _controller.text = prompt;
            }
            return Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Text(
                  L10n.t('system_prompt_quick_pick_title'),
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                    color: MemoTheme.of(context).textMain,
                  ),
                ),
                SizedBox(height: 4),
                Text(
                  L10n.t('system_prompt_quick_pick_desc'),
                  style: TextStyle(color: MemoTheme.of(context).textDim, fontSize: 12),
                ),
                SizedBox(height: 12),
                PersonaPicker(
                  // Empty until the user actually taps a card, so nothing
                  // shows as pre-selected on open — see _personaKey's doc
                  // comment above for why.
                  selectedKey: _personaTouched ? _personaKey : '',
                  nameController: _personaNameController,
                  onSelect: _applyPersona,
                  onNameChanged: _onPersonaNameChanged,
                  includeCustomOption: false,
                ),
                SizedBox(height: 24),
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
