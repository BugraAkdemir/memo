import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/theme.dart';
import '../../../core/l10n.dart';
import '../../../providers/chat_provider.dart';
import '../../../providers/learning_provider.dart';

class LearningTab extends ConsumerWidget {
  const LearningTab({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final settingsAsync = ref.watch(learningSettingsProvider);
    final patternsAsync = ref.watch(learningPatternsProvider);
    final theme = MemoTheme.of(context);

    // The whole tab scrolls — this keeps the layout overflow-proof no matter
    // how much content (settings cards + a long patterns list) is present.
    return ListView(
      padding: const EdgeInsets.all(20),
      children: [
        Row(
          children: [
            Text(
              L10n.t('learning_title'),
              style: TextStyle(
                fontSize: 18,
                fontWeight: FontWeight.w600,
                color: theme.textMain,
              ),
            ),
          ],
        ),
        const SizedBox(height: 8),
        Text(
          L10n.t('learning_desc'),
          style: TextStyle(color: theme.textDim, fontSize: 13),
        ),
        const SizedBox(height: 16),

        // Settings card
        settingsAsync.when(
          loading: () => const CircularProgressIndicator(),
          error: (e, _) => Text(L10n.t('learning_error', {'e': '$e'}), style: TextStyle(color: MemoTheme.red)),
          data: (settings) => SettingsCard(settings: settings, ref: ref),
        ),
        const SizedBox(height: 12),

        // Single model mode + calendar reminder.
        const ModelRoutingCard(),
        const SizedBox(height: 16),

        // Patterns header
        Row(
          children: [
            Text(
              L10n.t('learning_patterns_title'),
              style: TextStyle(fontSize: 15, fontWeight: FontWeight.w600, color: theme.textMain),
            ),
            const Spacer(),
            if (patternsAsync.valueOrNull?.isNotEmpty ?? false)
              TextButton.icon(
                onPressed: () => _clearAll(context, ref),
                icon: Icon(Icons.delete_sweep_outlined, size: 16, color: MemoTheme.red),
                label: Text(L10n.t('learning_clear_all_btn'), style: TextStyle(fontSize: 12, color: MemoTheme.red)),
                style: TextButton.styleFrom(visualDensity: VisualDensity.compact),
              ),
          ],
        ),
        const SizedBox(height: 8),

        // Patterns list (rendered inline; the outer ListView handles scrolling).
        patternsAsync.when(
          loading: () => const Center(child: Padding(
            padding: EdgeInsets.all(24),
            child: CircularProgressIndicator(),
          )),
          error: (e, _) => Center(
            child: Text(L10n.t('learning_patterns_load_error', {'e': '$e'}), style: TextStyle(color: theme.textDim)),
          ),
          data: (patterns) {
            if (patterns.isEmpty) {
              return Padding(
                padding: const EdgeInsets.symmetric(vertical: 24),
                child: Center(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(Icons.auto_awesome_outlined, size: 48, color: theme.textDim),
                      const SizedBox(height: 12),
                      Text(
                        L10n.t('learning_no_patterns'),
                        style: TextStyle(color: theme.textDim, fontSize: 14),
                      ),
                      const SizedBox(height: 4),
                      Text(
                        L10n.t('learning_no_patterns_desc'),
                        style: TextStyle(color: theme.textDim, fontSize: 12),
                        textAlign: TextAlign.center,
                      ),
                    ],
                  ),
                ),
              );
            }
            return Column(
              children: [
                for (int i = 0; i < patterns.length; i++) ...[
                  if (i > 0) Divider(height: 1, color: theme.borderSoft),
                  PatternCard(pattern: patterns[i]),
                ],
              ],
            );
          },
        ),
      ],
    );
  }

  Future<void> _clearAll(BuildContext context, WidgetRef ref) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(L10n.t('learning_clear_title')),
        content: Text(L10n.t('learning_clear_confirm')),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: Text(L10n.t('cancel'))),
          TextButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: Text(L10n.t('learning_clear_all_confirm'), style: TextStyle(color: Colors.red)),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    await ref.read(apiClientProvider).clearLearningData();
    ref.invalidate(learningPatternsProvider);
  }
}

class SettingsCard extends StatelessWidget {
  final Map<String, dynamic> settings;
  final WidgetRef ref;
  const SettingsCard({super.key, required this.settings, required this.ref});

  @override
  Widget build(BuildContext context) {
    final enabled = settings['enabled'] as bool? ?? false;
    final level = settings['level'] as String? ?? 'off';
    final theme = MemoTheme.of(context);

    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: theme.bgElement,
        borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
        border: Border.all(color: theme.borderSoft),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Text(L10n.t('learning_proactive_title'), style: TextStyle(fontWeight: FontWeight.w500, color: theme.textMain)),
              const Spacer(),
              Switch(
                value: enabled,
                onChanged: (v) {
                  final notifier = ref.read(learningSettingsProvider.notifier);
                  // Enabling with a still-"off" level would leave the engine
                  // dormant, so default to "normal" when turning it on.
                  if (v) {
                    notifier.update(true, (level == 'off' || level.isEmpty) ? 'normal' : level);
                  } else {
                    notifier.update(false, level);
                  }
                },
                activeThumbColor: MemoTheme.accent,
              ),
            ],
          ),
          if (enabled) ...[
            const SizedBox(height: 8),
            Text(L10n.t('learning_level_label'), style: TextStyle(fontSize: 12, color: theme.textDim)),
            const SizedBox(height: 4),
            Wrap(
              spacing: 6,
              children: ['subtle', 'normal', 'assertive'].map((l) {
                final selected = level == l;
                return ChoiceChip(
                  label: Text(l, style: TextStyle(fontSize: 11, color: selected ? Colors.white : theme.textDim)),
                  selected: selected,
                  selectedColor: MemoTheme.accent,
                  onSelected: selected ? null : (v) => ref.read(learningSettingsProvider.notifier).update(enabled, l),
                  visualDensity: VisualDensity.compact,
                );
              }).toList(),
            ),
          ],
        ],
      ),
    );
  }
}

// Single model mode + calendar reminder lead time. Loads/saves directly via
// the API client; both belong to the learning system's model routing.
class ModelRoutingCard extends ConsumerStatefulWidget {
  const ModelRoutingCard({super.key});

  @override
  ConsumerState<ModelRoutingCard> createState() => ModelRoutingCardState();
}

class ModelRoutingCardState extends ConsumerState<ModelRoutingCard> {
  bool _loading = true;
  String? _error;
  bool _singleModel = false;
  final _modelCtrl = TextEditingController();
  int _reminderLead = 30;
  bool _guessTime = true;
  bool _saving = false;

  static const _leadOptions = [10, 15, 30, 60, 120];

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    _modelCtrl.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    try {
      final api = ref.read(apiClientProvider);
      final learning = await api.getLearningSettings();
      final calendar = await api.getCalendarSettings();
      if (!mounted) return;
      setState(() {
        _singleModel = learning['single_model_enabled'] as bool? ?? false;
        _modelCtrl.text = learning['model_id'] as String? ?? '';
        final lead = calendar['reminder_lead_minutes'] as int? ?? 30;
        _reminderLead = _leadOptions.contains(lead) ? lead : 30;
        _guessTime = !(calendar['disable_time_guess'] as bool? ?? false);
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = '$e';
        _loading = false;
      });
    }
  }

  Future<void> _save() async {
    setState(() => _saving = true);
    try {
      final api = ref.read(apiClientProvider);
      await api.updateLearningSettings(_singleModel, _modelCtrl.text.trim());
      await api.updateCalendarSettings(_reminderLead, disableTimeGuess: !_guessTime);
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(L10n.t('learning_settings_saved'))));
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(L10n.t('learning_error', {'e': '$e'}))));
      }
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    if (_loading) {
      return const Padding(
        padding: EdgeInsets.all(8),
        child: SizedBox(height: 18, width: 18, child: CircularProgressIndicator(strokeWidth: 2)),
      );
    }
    if (_error != null) {
      return Text(L10n.t('learning_error', {'e': _error!}), style: TextStyle(color: MemoTheme.red, fontSize: 12));
    }

    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: theme.bgElement,
        borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
        border: Border.all(color: theme.borderSoft),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Text(L10n.t('learning_single_model_title'),
                  style: TextStyle(fontWeight: FontWeight.w500, color: theme.textMain)),
              const Spacer(),
              Switch(
                value: _singleModel,
                activeThumbColor: MemoTheme.accent,
                onChanged: (v) => setState(() => _singleModel = v),
              ),
            ],
          ),
          Text(
            L10n.t('learning_single_model_desc'),
            style: TextStyle(fontSize: 12, color: theme.textDim),
          ),
          if (_singleModel) ...[
            const SizedBox(height: 8),
            TextField(
              controller: _modelCtrl,
              style: TextStyle(color: theme.textMain, fontSize: 13),
              decoration: InputDecoration(
                isDense: true,
                hintText: L10n.t('learning_model_id_hint'),
                hintStyle: TextStyle(color: theme.textDim, fontSize: 13),
                border: const OutlineInputBorder(),
              ),
            ),
          ],
          const SizedBox(height: 14),
          Row(
            children: [
              Text(L10n.t('learning_calendar_reminder'),
                  style: TextStyle(fontSize: 13, color: theme.textMain)),
              const Spacer(),
              DropdownButton<int>(
                value: _reminderLead,
                underline: const SizedBox.shrink(),
                style: TextStyle(fontSize: 13, color: theme.textMain),
                items: _leadOptions
                    .map((m) => DropdownMenuItem(
                          value: m,
                          child: Text(m < 60
                              ? L10n.t('learning_reminder_min_before', {'m': '$m'})
                              : L10n.t('learning_reminder_hour_before', {'h': '${m ~/ 60}'})),
                        ))
                    .toList(),
                onChanged: (v) => setState(() => _reminderLead = v ?? 30),
              ),
            ],
          ),
          const SizedBox(height: 6),
          SwitchListTile(
            title: Text(L10n.t('learning_guess_time_title'),
                style: TextStyle(fontSize: 13, color: theme.textMain)),
            subtitle: Text(
              L10n.t('learning_guess_time_desc'),
              style: TextStyle(fontSize: 11, color: theme.textDim),
            ),
            value: _guessTime,
            onChanged: (v) => setState(() => _guessTime = v),
            dense: true,
            contentPadding: EdgeInsets.zero,
            activeThumbColor: MemoTheme.accent,
          ),
          const SizedBox(height: 10),
          Align(
            alignment: Alignment.centerRight,
            child: TextButton(
              onPressed: _saving ? null : _save,
              child: _saving
                  ? const SizedBox(height: 16, width: 16, child: CircularProgressIndicator(strokeWidth: 2))
                  : Text(L10n.t('save')),
            ),
          ),
        ],
      ),
    );
  }
}

class PatternCard extends ConsumerWidget {
  final LearnedPattern pattern;
  const PatternCard({super.key, required this.pattern});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = MemoTheme.of(context);
    final confPct = (pattern.confidence * 100).round();

    Color confColor;
    if (confPct >= 70) {
      confColor = MemoTheme.green;
    } else if (confPct >= 40) {
      confColor = MemoTheme.accent;
    } else {
      confColor = theme.textDim;
    }

    return Container(
      padding: const EdgeInsets.all(12),
      margin: const EdgeInsets.symmetric(vertical: 4),
      decoration: BoxDecoration(
        color: theme.bgElement,
        borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
        border: Border.all(color: theme.borderSoft),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                decoration: BoxDecoration(
                  color: confColor.withValues(alpha: 0.15),
                  borderRadius: BorderRadius.circular(4),
                ),
                child: Text(
                  confPct >= 70 ? '🔥 $confPct%' : confPct >= 40 ? '📈 $confPct%' : '📊 $confPct%',
                  style: TextStyle(fontSize: 11, fontWeight: FontWeight.w600, color: confColor, fontFamily: 'JetBrains Mono'),
                ),
              ),
              const SizedBox(width: 8),
              Text(
                pattern.activityType,
                style: TextStyle(fontWeight: FontWeight.w600, fontSize: 13, color: theme.textMain),
              ),
              const Spacer(),
              IconButton(
                icon: Icon(Icons.delete_outline, size: 16, color: theme.textDim),
                onPressed: () => _forget(context, ref),
                visualDensity: VisualDensity.compact,
                padding: EdgeInsets.zero,
                constraints: const BoxConstraints(),
              ),
            ],
          ),
          const SizedBox(height: 6),
          Row(
            children: [
              _infoChip(Icons.schedule, pattern.timeDisplay, theme),
              const SizedBox(width: 8),
              _infoChip(Icons.timer_outlined, pattern.stdDisplay, theme),
              const SizedBox(width: 8),
              _infoChip(Icons.calendar_view_week, pattern.daysDisplay, theme),
              if (pattern.totalCount > 0) ...[
                const SizedBox(width: 8),
                _infoChip(Icons.repeat, '${pattern.totalCount}x', theme),
              ],
            ],
          ),
        ],
      ),
    );
  }

  Widget _infoChip(IconData icon, String text, ThemeColors theme) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 5, vertical: 2),
      decoration: BoxDecoration(
        color: theme.bgApp,
        borderRadius: BorderRadius.circular(3),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 10, color: theme.textDim),
          const SizedBox(width: 3),
          Text(text, style: TextStyle(fontSize: 10, color: theme.textDim, fontFamily: 'JetBrains Mono')),
        ],
      ),
    );
  }

  Future<void> _forget(BuildContext context, WidgetRef ref) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(L10n.t('learning_forget_pattern_title')),
        content: Text(L10n.t('learning_forget_pattern_confirm', {'type': pattern.activityType})),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: Text(L10n.t('cancel'))),
          TextButton(onPressed: () => Navigator.pop(ctx, true), child: Text(L10n.t('learning_forget_btn'), style: TextStyle(color: Colors.red))),
        ],
      ),
    );
    if (confirmed != true) return;
    final api = ref.read(apiClientProvider);
    await api.forgetPattern(pattern.id);
    ref.invalidate(learningPatternsProvider);
  }
}
