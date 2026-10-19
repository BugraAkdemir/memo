/// Answer of POST /api/projects/status — whether a folder can be a Memo project,
/// whether it already is one, and whether the app should offer setting it up.
/// Mirrors Go `project.UIStatus`.
class ProjectStatus {
  final String root;
  final String id;
  final String name;
  final bool setUp;
  final bool declined;

  /// False for a folder that can never be a project (the home folder, Memo's own
  /// folders, a `.memo` Memo did not create); [reason] says why.
  final bool allowed;
  final String reason;

  /// False while project folders are not switched on (Beta).
  final bool enabled;

  /// True when the app should ask "set up Memo for this project?".
  final bool shouldAsk;

  const ProjectStatus({
    this.root = '',
    this.id = '',
    this.name = '',
    this.setUp = false,
    this.declined = false,
    this.allowed = false,
    this.reason = '',
    this.enabled = false,
    this.shouldAsk = false,
  });

  factory ProjectStatus.fromJson(Map<String, dynamic> json) => ProjectStatus(
        root: json['root'] as String? ?? '',
        id: json['id'] as String? ?? '',
        name: json['name'] as String? ?? '',
        setUp: json['set_up'] as bool? ?? false,
        declined: json['declined'] as bool? ?? false,
        allowed: json['allowed'] as bool? ?? false,
        reason: json['reason'] as String? ?? '',
        enabled: json['enabled'] as bool? ?? false,
        shouldAsk: json['should_ask'] as bool? ?? false,
      );
}

/// Answer of POST /api/projects/setup — what was created. Paths are relative to
/// [root]. Mirrors Go `project.ScaffoldResult`.
class ProjectSetupResult {
  final String root;
  final List<String> created;
  final List<String> skipped;

  const ProjectSetupResult({this.root = '', this.created = const [], this.skipped = const []});

  factory ProjectSetupResult.fromJson(Map<String, dynamic> json) => ProjectSetupResult(
        root: json['root'] as String? ?? '',
        created: (json['created'] as List?)?.map((e) => '$e').toList() ?? const [],
        skipped: (json['skipped'] as List?)?.map((e) => '$e').toList() ?? const [],
      );
}
