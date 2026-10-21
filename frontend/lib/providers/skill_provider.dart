import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../providers/auth_gate_provider.dart';
import '../providers/chat_provider.dart' show apiClientProvider, errorMessageProvider;
import '../providers/gate_guard.dart';
import '../core/friendly_error.dart';

/// The /name form of a skill name: "Pair Programming" -> "pair-programming".
/// The same rule as the backend's skillSlug (internal/app/skill_invoke.go), so
/// what the "/" menu inserts is what the backend recognises as a call.
String skillSlug(String name) {
  return name
      .replaceAll('İ', 'i') // Unicode lower-casing would leave a combining dot
      .toLowerCase()
      .replaceAll(RegExp(r'[^\p{L}\p{Nd}]+', unicode: true), '-')
      .replaceAll(RegExp(r'^-+|-+$'), '');
}

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
          '${L10n.t('error')}: ${L10n.t('skill_install_failed')} (${FriendlyError.describeGeneric(e)})';
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
          '${L10n.t('error')}: ${L10n.t('skill_delete_failed')} (${FriendlyError.describeGeneric(e)})';
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
