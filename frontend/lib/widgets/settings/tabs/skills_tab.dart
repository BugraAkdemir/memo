import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/theme.dart';
import '../../../core/l10n.dart';
import '../../../providers/skill_provider.dart';
import '../../skill_config_dialog.dart';
import '../../svg_icon.dart';
import '../../../core/friendly_error.dart';

class SkillsTab extends ConsumerWidget {
  const SkillsTab({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final skillsAsync = ref.watch(skillListProvider);
    final theme = MemoTheme.of(context);

    return Padding(
      padding: const EdgeInsets.all(20),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              SvgIcon('puzzle-piece', size: 24, color: theme.textMain),
              const SizedBox(width: 10),
              Text(
                L10n.t('skills_title'),
                style: TextStyle(
                  fontSize: 18,
                  fontWeight: FontWeight.w600,
                  color: theme.textMain,
                ),
              ),
              const Spacer(),
              TextButton.icon(
                onPressed: () => _showSkillManager(context, ref),
                icon: Icon(Icons.add, size: 16),
                label: Text(L10n.t('skill_management_btn')),
              ),
            ],
          ),
          const SizedBox(height: 8),
          Text(
            L10n.t('skills_desc'),
            style: TextStyle(color: theme.textDim, fontSize: 13),
          ),
          const SizedBox(height: 20),
          Expanded(
            child: skillsAsync.when(
              loading: () => const Center(child: CircularProgressIndicator()),
              error: (e, _) => Center(
                child: Text(L10n.t('skills_load_error', {'e': FriendlyError.describeGeneric(e)}), style: TextStyle(color: theme.textDim)),
              ),
              data: (skills) {
                if (skills.isEmpty) {
                  return Center(
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(Icons.extension_off, size: 48, color: theme.textDim),
                        const SizedBox(height: 12),
                        Text(
                          L10n.t('skills_empty'),
                          style: TextStyle(color: theme.textDim, fontSize: 14),
                        ),
                        const SizedBox(height: 4),
                        Text(
                          L10n.t('skills_empty_hint'),
                          style: TextStyle(color: theme.textDim, fontSize: 12),
                          textAlign: TextAlign.center,
                        ),
                      ],
                    ),
                  );
                }
                return ListView.separated(
                  itemCount: skills.length,
                  separatorBuilder: (_, _) => Divider(height: 1, color: theme.borderSoft),
                  itemBuilder: (_, i) {
                    final s = skills[i];
                    return ListTile(
                      leading: SvgIcon(
                        s.isActive ? 'check-circle' : 'puzzle-piece',
                        size: 24,
                        color: s.isActive ? MemoTheme.green : theme.textDim,
                      ),
                      title: Text(
                        s.name,
                        style: TextStyle(
                          fontWeight: FontWeight.w500,
                          color: theme.textMain,
                          fontFamily: 'JetBrains Mono',
                          fontSize: 13,
                        ),
                      ),
                      subtitle: Text(
                        s.description,
                        style: TextStyle(color: theme.textDim, fontSize: 12),
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                      ),
                      trailing: Switch(
                        value: s.isActive,
                        onChanged: (v) => _toggleSkill(ref, s.name, v),
                        activeThumbColor: MemoTheme.accent,
                      ),
                    );
                  },
                );
              },
            ),
          ),
        ],
      ),
    );
  }

  void _showSkillManager(BuildContext context, WidgetRef ref) {
    showDialog(
      context: context,
      builder: (_) => const SkillConfigDialog(),
    ).then((_) => ref.invalidate(skillListProvider));
  }

  Future<void> _toggleSkill(WidgetRef ref, String name, bool active) async {
    final notifier = ref.read(skillListProvider.notifier);
    await notifier.toggleSkill(name, active);
  }
}
