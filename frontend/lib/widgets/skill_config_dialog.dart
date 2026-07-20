import 'dart:io' show Platform;
import '../core/l10n.dart';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/theme.dart';
import '../providers/skill_provider.dart';

/// Dialog for managing skills: list, activate/deactivate, install, remove.
class SkillConfigDialog extends ConsumerStatefulWidget {
  const SkillConfigDialog({super.key});

  @override
  ConsumerState<SkillConfigDialog> createState() => _SkillConfigDialogState();
}

class _SkillConfigDialogState extends ConsumerState<SkillConfigDialog> {
  @override
  Widget build(BuildContext context) {
    final skillsAsync = ref.watch(skillListProvider);
    final theme = MemoTheme.of(context);

    return Dialog(
      insetPadding: const EdgeInsets.symmetric(horizontal: 40, vertical: 60),
      child: Container(
        width: 560,
        constraints: const BoxConstraints(maxHeight: 600),
        decoration: BoxDecoration(
          color: theme.bgPanel,
          borderRadius: BorderRadius.circular(MemoTheme.radiusLg),
        ),
        child: Column(
          children: [
            // Header
            Container(
              padding: const EdgeInsets.all(16),
              decoration: BoxDecoration(
                border: Border(bottom: BorderSide(color: theme.borderSoft)),
              ),
              child: Row(
                children: [
                  Text('🧩', style: TextStyle(fontSize: 20)),
                  const SizedBox(width: 10),
                  Text(
                    'Skill Yönetimi',
                    style: TextStyle(
                      fontSize: 16,
                      fontWeight: FontWeight.w600,
                      color: theme.textMain,
                    ),
                  ),
                  const Spacer(),
                  TextButton(
                    onPressed: () => Navigator.of(context).pop(),
                    child: Text(L10n.t('close'), style: TextStyle(color: theme.textDim)),
                  ),
                ],
              ),
            ),
            // Content
            Expanded(
              child: skillsAsync.when(
                loading: () => const Center(child: CircularProgressIndicator()),
                error: (e, _) => Center(
                  child: Padding(
                    padding: const EdgeInsets.all(24),
                    child: Text(
                      "Skill'ler yuklenemedi: $e",
                      style: TextStyle(color: theme.textDim),
                    ),
                  ),
                ),
                data: (skills) {
                  if (skills.isEmpty) {
                    return Center(
                      child: Padding(
                        padding: const EdgeInsets.all(32),
                        child: Column(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            Text(
                              'Henüz skill yüklenmemiş.',
                              style: TextStyle(color: theme.textDim, fontSize: 14),
                            ),
                            const SizedBox(height: 8),
                            Text(
                              'data/skills/ klasörüne SKILL.md dosyası ekleyin\nveya aşağıdan yükleyin.',
                              style: TextStyle(color: theme.textDim, fontSize: 12),
                              textAlign: TextAlign.center,
                            ),
                          ],
                        ),
                      ),
                    );
                  }
                  return ListView.separated(
                    padding: const EdgeInsets.symmetric(vertical: 8),
                    itemCount: skills.length,
                    separatorBuilder: (_, _) => Divider(height: 1, color: theme.borderSoft),
                    itemBuilder: (context, index) {
                      final skill = skills[index];
                      final isActive = skill.isActive;
                      return ListTile(
                        leading: Text(
                          isActive ? '✅' : '🧩',
                          style: const TextStyle(fontSize: 20),
                        ),
                        title: Text(
                          skill.name,
                          style: TextStyle(
                            fontWeight: FontWeight.w500,
                            color: theme.textMain,
                            fontFamily: 'JetBrains Mono',
                            fontSize: 13,
                          ),
                        ),
                        subtitle: Text(
                          skill.description,
                          style: TextStyle(color: theme.textDim, fontSize: 12),
                          maxLines: 2,
                          overflow: TextOverflow.ellipsis,
                        ),
                        trailing: Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            // Toggle active/inactive
                            Switch(
                              value: isActive,
                              onChanged: (v) => _toggleSkill(skill.name, v),
                              activeThumbColor: MemoTheme.accent,
                            ),
                            // Remove button
                            IconButton(
                              icon: Icon(Icons.delete_outline, size: 18, color: theme.textDim),
                              onPressed: () => _removeSkill(skill.name),
                            ),
                          ],
                        ),
                      );
                    },
                  );
                },
              ),
            ),
            // Bottom actions
            Container(
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                border: Border(top: BorderSide(color: theme.borderSoft)),
              ),
              child: Row(
                children: [
                  Expanded(
                    child: OutlinedButton.icon(
                      onPressed: _installSkill,
                      icon: const Icon(Icons.add, size: 16),
                      label: Text(L10n.t('skill_load')),
                      style: OutlinedButton.styleFrom(
                        foregroundColor: MemoTheme.accent,
                        side: BorderSide(color: MemoTheme.accent.withValues(alpha: 0.4)),
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _toggleSkill(String name, bool active) async {
    final notifier = ref.read(skillListProvider.notifier);
    final ok = await notifier.toggleSkill(name, active);
    if (ok && mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(active ? '✅ $name aktifleştirildi' : '⏸️ $name devre dışı'),
          duration: const Duration(seconds: 2),
        ),
      );
    }
  }

  Future<void> _removeSkill(String name) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(L10n.t('skill_delete_title')),
        content: Text(L10n.t('skill_delete_confirm', {'name': name})),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: Text(L10n.t('cancel')),
          ),
          TextButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: Text(L10n.t('delete'), style: const TextStyle(color: Colors.red)),
          ),
        ],
      ),
    );
    if (confirmed != true) return;

    final notifier = ref.read(skillListProvider.notifier);
    final ok = await notifier.removeSkill(name);
    if (mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(ok ? '🗑️ $name silindi' : '❌ Silme başarısız'),
          duration: const Duration(seconds: 2),
        ),
      );
    }
  }

  Future<void> _installSkill() async {
    final pathController = TextEditingController();
    final path = await showDialog<String>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(L10n.t('skill_load')),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(L10n.t('skill_path_prompt')),
            const SizedBox(height: 12),
            TextField(
              controller: pathController,
              decoration: InputDecoration(
                hintText: Platform.isWindows
                    ? 'C:\\Users\\kullanici\\skills\\benim-skill'
                    : '/home/kullanici/skiller/benim-skill',
                border: OutlineInputBorder(),
                isDense: true,
              ),
              style: const TextStyle(fontFamily: 'JetBrains Mono', fontSize: 13),
            ),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: Text(L10n.t('cancel')),
          ),
          TextButton(
            onPressed: () => Navigator.pop(ctx, pathController.text),
            child: Text(L10n.t('load')),
          ),
        ],
      ),
    );
    if (path == null || path.isEmpty) return;

    final notifier = ref.read(skillListProvider.notifier);
    final name = await notifier.installSkill(path);
    if (mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(name != null ? '✅ $name yüklendi' : '❌ Yükleme başarısız'),
          duration: const Duration(seconds: 2),
        ),
      );
    }
  }
}
