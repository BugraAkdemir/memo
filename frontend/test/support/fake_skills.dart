import 'package:memo_flutter/providers/skill_provider.dart';

/// A dozen installed skills — enough that the "/" menu has to scroll. Names mix
/// the two shapes real ones have: spaced titles and dashed ids.
List<SkillDefinition> fakeSkills() => const [
      SkillDefinition(name: 'Pair Programming', description: 'Driver and navigator modes for pair work'),
      SkillDefinition(name: 'codebase-memory', description: 'Use the codebase knowledge graph'),
      SkillDefinition(name: 'frontend-design', description: 'Build distinctive frontend interfaces'),
      SkillDefinition(name: 'Verification & Quality Assurance', description: 'Verify code quality and coverage'),
      SkillDefinition(name: 'github-code-review', description: 'Review pull requests with a swarm'),
      SkillDefinition(name: 'sparc-methodology', description: 'Specification, pseudocode, architecture'),
      SkillDefinition(name: 'stream-chain', description: 'Chain model calls as a pipeline'),
      SkillDefinition(name: 'swarm-advanced', description: 'Advanced swarm orchestration'),
      SkillDefinition(name: 'testsprite-verify', description: 'Verify the app with generated tests'),
      SkillDefinition(name: 'ui-ux-pro-max', description: 'UI and UX design intelligence'),
      SkillDefinition(name: 'V3 Security Overhaul', description: 'Security architecture overhaul'),
      SkillDefinition(name: 'reverse-engineer-anything', description: 'Reverse engineering workflow'),
    ];

/// Serves [fakeSkills] instead of asking a backend.
class FakeSkillList extends SkillListNotifier {
  @override
  Future<List<SkillDefinition>> build() async => fakeSkills();
}
