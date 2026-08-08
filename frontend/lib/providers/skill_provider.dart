import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../providers/chat_provider.dart' show apiClientProvider, errorMessageProvider;
import '../core/friendly_error.dart';

/// A skill definition from the backend.
class SkillDefinition {
  final String name;
  final String description;
  final String dangerLevel;
  final bool isActive;

  const SkillDefinition({
    required this.name,
    required this.description,
    this.dangerLevel = 'safe',
    this.isActive = false,
  });

  factory SkillDefinition.fromJson(Map<String, dynamic> json, {bool isActive = false}) {
    final manifest = json['Manifest'] as Map<String, dynamic>? ?? {};
    return SkillDefinition(
      name: manifest['name'] as String? ?? 'unknown',
      description: manifest['description'] as String? ?? '',
      dangerLevel: manifest['danger_level'] as String? ?? 'safe',
      isActive: isActive,
    );
  }
}

/// Provider that lists all installed skills.
final skillListProvider = AsyncNotifierProvider<SkillListNotifier, List<SkillDefinition>>(
  SkillListNotifier.new,
);

class SkillListNotifier extends AsyncNotifier<List<SkillDefinition>> {
  @override
  Future<List<SkillDefinition>> build() async {
    return _fetchSkills();
  }

  Future<List<SkillDefinition>> _fetchSkills() async {
    final api = ref.read(apiClientProvider);
    final activeNames = await api.getActiveSkills();
    final activeSet = activeNames.toSet();
    final skills = await api.listSkills();
    return skills.map((s) => SkillDefinition.fromJson(s, isActive: activeSet.contains(s['Manifest']?['name']))).toList();
  }

  /// Install a skill from a local path.
  Future<String?> installSkill(String path) async {
    try {
      final api = ref.read(apiClientProvider);
      final result = await api.installSkill(path);
      final manifest = result['Manifest'] as Map<String, dynamic>?;
      final name = manifest?['name'] as String? ?? 'unknown';
      ref.invalidateSelf();
      return name;
    } catch (e) {
      ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: Beceri yüklenemedi (${FriendlyError.describeGeneric(e)})';
      return null;
    }
  }

  /// Remove a skill by name.
  Future<bool> removeSkill(String name) async {
    try {
      final api = ref.read(apiClientProvider);
      await api.removeSkill(name);
      ref.invalidateSelf();
      return true;
    } catch (e) {
      ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: Beceri kaldırılamadı (${FriendlyError.describeGeneric(e)})';
      return false;
    }
  }

  /// Toggle a skill on/off.
  Future<bool> toggleSkill(String name, bool active) async {
    try {
      final api = ref.read(apiClientProvider);
      final current = await api.getActiveSkills();
      final updated = Set<String>.from(current);
      if (active) {
        updated.add(name);
      } else {
        updated.remove(name);
      }
      await api.setActiveSkills(updated.toList());
      ref.invalidateSelf();
      return true;
    } catch (e) {
      ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: Beceri durumu değiştirilemedi (${FriendlyError.describeGeneric(e)})';
      return false;
    }
  }
}
