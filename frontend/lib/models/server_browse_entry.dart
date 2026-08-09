/// One immediate child of a server-browsed directory — mirrors Go
/// `app.ServerBrowseEntry`.
class ServerBrowseEntry {
  final String name;
  final bool isDir;
  final int size;

  const ServerBrowseEntry({
    required this.name,
    required this.isDir,
    this.size = 0,
  });

  factory ServerBrowseEntry.fromJson(Map<String, dynamic> json) =>
      ServerBrowseEntry(
        name: json['name'] as String? ?? '',
        isDir: json['is_dir'] as bool? ?? false,
        size: json['size'] as int? ?? 0,
      );
}

/// A listed directory on the backend's own filesystem — mirrors Go
/// `app.ServerBrowseResult`. See MemoApiClient.browseServer's doc comment
/// for why this exists (server-side file browser, Faz 5.1 follow-up).
class ServerBrowseResult {
  final String path;
  final String parent;
  final List<ServerBrowseEntry> entries;

  const ServerBrowseResult({
    required this.path,
    required this.parent,
    required this.entries,
  });

  factory ServerBrowseResult.fromJson(Map<String, dynamic> json) =>
      ServerBrowseResult(
        path: json['path'] as String? ?? '',
        parent: json['parent'] as String? ?? '',
        entries: (json['entries'] as List<dynamic>?)
                ?.map((e) => ServerBrowseEntry.fromJson(e as Map<String, dynamic>))
                .toList() ??
            const [],
      );
}
