/// One slash command offered by a CLI-backed chat provider (Claude Code /
/// Codex) — mirrors Go `agentcli.Command`.
///
/// These are the CLI's own commands, read from its own command directories
/// on the backend, not Memo's built-in prompt templates: in a CLI chat the
/// "/" dropdown shows these instead, since Memo's templates are just canned
/// prompt text and a coding agent's commands are real, configured workflows.
class CLICommand {
  /// Command name without its leading slash ("review", "git:commit").
  final String name;

  /// One-line summary, empty when the command file declared none.
  final String description;

  /// Where it came from: "project", "user", "skill", or "builtin".
  final String source;

  const CLICommand({
    required this.name,
    this.description = '',
    this.source = '',
  });

  factory CLICommand.fromJson(Map json) => CLICommand(
        name: json['name'] as String? ?? '',
        description: json['description'] as String? ?? '',
        source: json['source'] as String? ?? '',
      );

  /// What the user types and what gets inserted into the composer.
  String get slash => '/$name';
}
