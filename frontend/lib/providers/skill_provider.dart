import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../providers/auth_gate_provider.dart';
import '../providers/chat_provider.dart' show apiClientProvider, errorMessageProvider;
import '../providers/gate_guard.dart';
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
    // BUG-ONB6 (see chat_provider.dart's ChatListNotifier for the full
    // story): a one-shot AsyncNotifier whose single build() attempt landing
    // while the auth gate is still up 401s and gets permanently cached as
    // an error. Mount empty instead; app_shell.dart's gate-transition
    // listener re-invalidates this once the gate actually opens.
    if (authGateBlocked(ref.read(authGateProvider).valueOrNull)) return const [];
    return _fetchSkills();
  }

  Future<List<SkillDefinition>> _fetchSkills() async {
    final api = ref.read(apiClientProvider);
    final skills = await api.listSkills();
    // Active/inactive is per-chat now (see chatActiveSkillsProvider) — this
    // plain installed-skills list carries no activation state of its own.
    return skills.map((s) => SkillDefinition.fromJson(s)).toList();
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

  /// Toggle a skill on/off for one chat. Activation is per-chat — this
  /// never affects any other chat's active-skill list.
  Future<bool> toggleSkill(String chatId, String name, bool active) async {
    try {
      final api = ref.read(apiClientProvider);
      final current = await api.getActiveSkills(chatId);
      final updated = Set<String>.from(current);
      if (active) {
        updated.add(name);
      } else {
        updated.remove(name);
      }
      await api.setActiveSkills(chatId, updated.toList());
      ref.invalidate(chatActiveSkillsProvider(chatId));
      return true;
    } catch (e) {
      ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: Beceri durumu değiştirilemedi (${FriendlyError.describeGeneric(e)})';
      return false;
    }
  }
}

/// The set of skill names active for one chat. Mirrors chatCodeModeProvider
/// (agent_provider.dart) — same per-chat FutureProvider.family shape and
/// the same BUG-SCAN8/BUG-ONB6 auth-gate guard: a widget that's always
/// mounted (this one lives behind a dialog the user opens, but the pattern
/// is kept consistent regardless) must not cache a pre-auth-gate 401 as a
/// permanent error.
final chatActiveSkillsProvider =
    FutureProvider.family<Set<String>, String>((ref, chatId) async {
  if (chatId.isEmpty) return const {};
  if (authGateBlocked(ref.read(authGateProvider).valueOrNull)) return const {};
  final names = await ref.read(apiClientProvider).getActiveSkills(chatId);
  return names.toSet();
});
