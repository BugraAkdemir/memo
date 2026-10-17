/// What the backend's /api/update/check says (internal/updater/types.go).
///
/// Parsed defensively: a field of the wrong type becomes its default instead of
/// throwing — this feeds an ambient dialog that must never take the app down.
class AppUpdateInfo {
  final String current;
  final String latest;
  final bool available;
  final bool required;
  final String noteTr;
  final String noteEn;

  /// `auto` (Memo updates itself), `installer` (Windows Setup.exe) or `manual`
  /// (point the person at the download page).
  final String mode;
  final int sizeBytes;
  final String downloadUrl;
  final List<ComponentUpdate> components;

  const AppUpdateInfo({
    this.current = '',
    this.latest = '',
    this.available = false,
    this.required = false,
    this.noteTr = '',
    this.noteEn = '',
    this.mode = 'manual',
    this.sizeBytes = 0,
    this.downloadUrl = 'https://memocpp.com',
    this.components = const [],
  });

  bool get canUpdateInPlace => mode == 'auto' || mode == 'installer';

  /// Components that can be updated right now.
  List<ComponentUpdate> get pendingComponents =>
      components.where((c) => c.available).toList(growable: false);

  /// The note in the language the UI is showing, falling back to the other one.
  String noteFor(bool turkish) {
    final primary = turkish ? noteTr : noteEn;
    if (primary.trim().isNotEmpty) return primary;
    return turkish ? noteEn : noteTr;
  }

  factory AppUpdateInfo.fromJson(Object? raw) {
    if (raw is! Map) return const AppUpdateInfo();
    String s(String k, [String d = '']) => raw[k] is String ? raw[k] as String : d;
    int i(String k) => raw[k] is num ? (raw[k] as num).toInt() : 0;
    final comps = <ComponentUpdate>[];
    final list = raw['components'];
    if (list is List) {
      for (final c in list) {
        if (c is Map) comps.add(ComponentUpdate.fromJson(c));
      }
    }
    return AppUpdateInfo(
      current: s('current'),
      latest: s('latest'),
      available: raw['available'] == true,
      required: raw['required'] == true,
      noteTr: s('note_tr'),
      noteEn: s('note_en'),
      mode: s('mode', 'manual'),
      sizeBytes: i('size_bytes'),
      downloadUrl: s('download_url', 'https://memocpp.com'),
      components: comps,
    );
  }
}

/// One engine component (llama.cpp, whisper.cpp, the Subscriptions connector).
class ComponentUpdate {
  final String name;
  final String folder;
  final String installed;
  final String latest;
  final bool available;
  final int sizeBytes;
  final String reason;

  const ComponentUpdate({
    required this.name,
    this.folder = '',
    this.installed = '',
    this.latest = '',
    this.available = false,
    this.sizeBytes = 0,
    this.reason = '',
  });

  factory ComponentUpdate.fromJson(Map raw) {
    String s(String k) => raw[k] is String ? raw[k] as String : '';
    return ComponentUpdate(
      name: s('name'),
      folder: s('folder'),
      installed: s('installed'),
      latest: s('latest'),
      available: raw['available'] == true,
      sizeBytes: raw['size_bytes'] is num ? (raw['size_bytes'] as num).toInt() : 0,
      reason: s('reason'),
    );
  }
}

/// Progress of a running update (/api/update/status).
class UpdateProgress {
  /// idle | downloading | verifying | installing | restarting | done | error
  final String state;
  final String kind;
  final String component;
  final int done;
  final int total;
  final String error;

  const UpdateProgress({
    this.state = 'idle',
    this.kind = '',
    this.component = '',
    this.done = 0,
    this.total = 0,
    this.error = '',
  });

  double? get fraction => total > 0 ? (done / total).clamp(0.0, 1.0) : null;

  bool get isRestarting => state == 'restarting';
  bool get isDone => state == 'done';
  bool get isError => state == 'error';
  bool get isBusy => const {'downloading', 'verifying', 'installing', 'restarting'}.contains(state);

  factory UpdateProgress.fromJson(Object? raw) {
    if (raw is! Map) return const UpdateProgress();
    String s(String k, [String d = '']) => raw[k] is String ? raw[k] as String : d;
    int i(String k) => raw[k] is num ? (raw[k] as num).toInt() : 0;
    return UpdateProgress(
      state: s('state', 'idle'),
      kind: s('kind'),
      component: s('component'),
      done: i('done'),
      total: i('total'),
      error: s('error'),
    );
  }
}

/// "123.4 MB" — binary units, one decimal above 1 MB.
String formatUpdateBytes(int bytes) {
  if (bytes <= 0) return '';
  const kb = 1024, mb = kb * 1024, gb = mb * 1024;
  if (bytes >= gb) return '${(bytes / gb).toStringAsFixed(1)} GB';
  if (bytes >= mb) return '${(bytes / mb).toStringAsFixed(bytes >= 100 * mb ? 0 : 1)} MB';
  if (bytes >= kb) return '${(bytes / kb).toStringAsFixed(0)} KB';
  return '$bytes B';
}
