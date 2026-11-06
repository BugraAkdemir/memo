import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/l10n.dart';
import '../../core/theme.dart';
import '../../providers/learning_provider.dart';
import '../../providers/settings_provider.dart';
import 'setup_widgets.dart';

/// Memory and learning in one place: remembering the user (needs the embedding
/// model), the nightly "dream" consolidation, proactive suggestions and minimal
/// mode. Each switch writes the same setting as in Settings.
class SetupMemoryStep extends ConsumerWidget {
  final ThemeColors color;
  final bool hasEmbeddingModel;

  const SetupMemoryStep({
    super.key,
    required this.color,
    required this.hasEmbeddingModel,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final memory = ref.watch(memoryEnabledProvider);
    final dream = ref.watch(dreamSettingsProvider);
    final learning = ref.watch(learningSettingsProvider);
    final minimal = ref.watch(minimalModeProvider);
    final memoryOn = memory.valueOrNull ?? true;
    final proactiveLevel =
        learning.valueOrNull?['level'] as String? ?? 'subtle';
    return Column(
      children: [
        SetupPreferenceToggle(
          icon: Icons.memory_outlined,
          title: L10n.t('setup_memory_enabled_title'),
          desc: L10n.t(
            hasEmbeddingModel || !memoryOn
                ? 'setup_memory_enabled_desc'
                : 'setup_memory_enabled_needs_embedding',
          ),
          value: memoryOn,
          color: color,
          onChanged: memory.isLoading
              ? null
              : (_) => ref.read(memoryEnabledProvider.notifier).toggle(),
        ),
        const SizedBox(height: 10),
        SetupPreferenceToggle(
          icon: Icons.bedtime_outlined,
          title: L10n.t('setup_dream_title'),
          desc: L10n.t('setup_dream_desc'),
          value: dream.valueOrNull?.enabled ?? false,
          color: color,
          onChanged: dream.valueOrNull == null
              ? null
              : (v) => ref
                    .read(dreamSettingsProvider.notifier)
                    .save(
                      enabled: v,
                      initialDelayMinutes: dream.value!.initialDelayMinutes,
                      intervalHours: dream.value!.intervalHours,
                    ),
        ),
        const SizedBox(height: 10),
        SetupPreferenceToggle(
          icon: Icons.psychology_outlined,
          title: L10n.t('learning_proactive_title'),
          desc: L10n.t('setup_pref_proactive_desc'),
          value: learning.valueOrNull?['enabled'] as bool? ?? false,
          color: color,
          onChanged: learning.isLoading
              ? null
              : (v) => ref
                    .read(learningSettingsProvider.notifier)
                    .update(v, proactiveLevel),
        ),
        const SizedBox(height: 10),
        SetupPreferenceToggle(
          icon: Icons.speed_outlined,
          title: L10n.t('minimal_mode_section'),
          desc: L10n.t('setup_pref_minimal_desc'),
          value: minimal.valueOrNull ?? false,
          color: color,
          onChanged: minimal.isLoading
              ? null
              : (_) => ref.read(minimalModeProvider.notifier).toggle(),
        ),
      ],
    );
  }
}
