import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/theme.dart';
import '../../../core/l10n.dart';
import '../../../core/friendly_error.dart';
import '../../../providers/chat_provider.dart';
import '../../../providers/settings_provider.dart';
import '../../../models/dream.dart';

class DreamTab extends ConsumerStatefulWidget {
  const DreamTab({super.key});

  @override
  ConsumerState<DreamTab> createState() => _DreamTabState();
}

class _DreamTabState extends ConsumerState<DreamTab> {
  final _delayController = TextEditingController();
  final _intervalController = TextEditingController();
  bool _settingsInitialized = false;
  bool? _enabled;
  bool _saving = false;
  String? _saveError;

  bool _running = false;
  DreamRunResult? _runResult;
  String? _runError;

  @override
  void dispose() {
    _delayController.dispose();
    _intervalController.dispose();
    super.dispose();
  }

  Future<void> _save(DreamSettings current) async {
    final enabled = _enabled ?? current.enabled;
    final delay =
        int.tryParse(_delayController.text) ?? current.initialDelayMinutes;
    final interval =
        int.tryParse(_intervalController.text) ?? current.intervalHours;

    setState(() {
      _saving = true;
      _saveError = null;
    });
    try {
      await ref
          .read(dreamSettingsProvider.notifier)
          .save(
            enabled: enabled,
            initialDelayMinutes: delay,
            intervalHours: interval,
          );
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(L10n.t('saved'))));
      }
    } catch (e) {
      if (mounted) {
        setState(
          () => _saveError = FriendlyError.describeGeneric(e),
        );
      }
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  Future<void> _runNow() async {
    setState(() {
      _running = true;
      _runError = null;
      _runResult = null;
    });
    try {
      final result = await ref.read(apiClientProvider).runDreamNow();
      if (mounted) setState(() => _runResult = result);
    } catch (e) {
      if (mounted) setState(() => _runError = FriendlyError.describeGeneric(e));
    } finally {
      if (mounted) setState(() => _running = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    final settingsAsync = ref.watch(dreamSettingsProvider);

    return ListView(
      padding: const EdgeInsets.all(32),
      children: [
        Text(
          L10n.t('tab_dream'),
          style: Theme.of(context).textTheme.titleLarge?.copyWith(
            fontWeight: FontWeight.bold,
            color: theme.textMain,
          ),
        ),
        const SizedBox(height: 8),
        Text(
          L10n.t('dream_subtitle'),
          style: TextStyle(color: theme.textDim, fontSize: 13),
        ),
        const SizedBox(height: 20),
        settingsAsync.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (e, _) => Text(
            '${L10n.t('error')}: ${FriendlyError.describeGeneric(e)}',
          ),
          data: (settings) {
            if (!_settingsInitialized) {
              _enabled = settings.enabled;
              _delayController.text = settings.initialDelayMinutes.toString();
              _intervalController.text = settings.intervalHours.toString();
              _settingsInitialized = true;
            }

            return Container(
              padding: const EdgeInsets.all(16),
              decoration: BoxDecoration(
                color: theme.bgPanel,
                borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                border: Border.all(color: theme.borderSoft),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Row(
                    children: [
                      Expanded(
                        child: Text(
                          L10n.t('dream_enabled_label'),
                          style: TextStyle(
                            fontSize: 14,
                            fontWeight: FontWeight.w500,
                            color: theme.textMain,
                          ),
                        ),
                      ),
                      Switch(
                        value: _enabled ?? settings.enabled,
                        onChanged: (v) => setState(() => _enabled = v),
                        activeThumbColor: MemoTheme.accent,
                      ),
                    ],
                  ),
                  const SizedBox(height: 14),
                  _DreamNumberField(
                    label: L10n.t('dream_initial_delay_label'),
                    controller: _delayController,
                    hint: '5',
                  ),
                  const SizedBox(height: 12),
                  _DreamNumberField(
                    label: L10n.t('dream_interval_label'),
                    controller: _intervalController,
                    hint: '24',
                  ),
                  if (_saveError != null) ...[
                    const SizedBox(height: 10),
                    Text(
                      _saveError!,
                      style: TextStyle(color: MemoTheme.red, fontSize: 12),
                    ),
                  ],
                  const SizedBox(height: 14),
                  Row(
                    mainAxisAlignment: MainAxisAlignment.end,
                    children: [
                      ElevatedButton(
                        onPressed: _saving ? null : () => _save(settings),
                        child: _saving
                            ? const SizedBox(
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
        const SizedBox(height: 28),
        Text(
          L10n.t('dream_run_now_title'),
          style: Theme.of(context).textTheme.titleMedium?.copyWith(
            fontWeight: FontWeight.bold,
            color: theme.textMain,
          ),
        ),
        const SizedBox(height: 8),
        Text(
          L10n.t('dream_run_now_hint'),
          style: TextStyle(color: theme.textDim, fontSize: 13),
        ),
        const SizedBox(height: 12),
        Row(
          children: [
            ElevatedButton(
              onPressed: _running ? null : _runNow,
              child: _running
                  ? const SizedBox(
                      width: 14,
                      height: 14,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : Text(L10n.t('dream_run_now_btn')),
            ),
          ],
        ),
        if (_runError != null)
          Padding(
            padding: const EdgeInsets.only(top: 12),
            child: Text(
              _runError!,
              style: TextStyle(color: MemoTheme.red, fontSize: 12),
            ),
          ),
        if (_runResult != null)
          Padding(
            padding: const EdgeInsets.only(top: 12),
            child: Text(
              _runResult!.ran
                  ? L10n.t('dream_run_result_compressed', {
                      'before': '${_runResult!.before}',
                      'after': '${_runResult!.after}',
                    })
                  : L10n.t('dream_run_result_not_enough'),
              style: TextStyle(
                fontSize: 13,
                color: _runResult!.ran ? MemoTheme.accent : theme.textDim,
              ),
            ),
          ),
      ],
    );
  }
}

class _DreamNumberField extends StatelessWidget {
  final String label;
  final TextEditingController controller;
  final String hint;

  const _DreamNumberField({
    required this.label,
    required this.controller,
    required this.hint,
  });

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    return Row(
      children: [
        Expanded(
          child: Text(
            label,
            style: const TextStyle(fontWeight: FontWeight.w500, fontSize: 13),
          ),
        ),
        SizedBox(
          width: 100,
          height: 36,
          child: TextField(
            controller: controller,
            keyboardType: TextInputType.number,
            inputFormatters: [FilteringTextInputFormatter.digitsOnly],
            style: const TextStyle(fontSize: 13),
            textAlign: TextAlign.center,
            decoration: InputDecoration(
              hintText: hint,
              contentPadding: const EdgeInsets.symmetric(horizontal: 8),
              filled: true,
              fillColor: theme.bgApp,
              border: OutlineInputBorder(
                borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                borderSide: BorderSide(color: theme.borderSoft),
              ),
              enabledBorder: OutlineInputBorder(
                borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                borderSide: BorderSide(color: theme.borderSoft),
              ),
              focusedBorder: OutlineInputBorder(
                borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                borderSide: BorderSide(color: MemoTheme.accent),
              ),
            ),
          ),
        ),
      ],
    );
  }
}
