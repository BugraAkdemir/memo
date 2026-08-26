/// Mirrors Go `browserengine.InstallProgress` — same field shape/JSON tags
/// as `DownloadProgress` (local_model.dart), minus repo/filename keying
/// since there's only ever one browser engine to install.
class BrowserInstallProgress {
  final bool active;
  final int totalBytes;
  final int downloaded;
  final double percent;
  final String speed;
  final String? error;

  const BrowserInstallProgress({
    this.active = false,
    this.totalBytes = 0,
    this.downloaded = 0,
    this.percent = 0,
    this.speed = '',
    this.error,
  });

  factory BrowserInstallProgress.fromJson(Map<String, dynamic> json) =>
      BrowserInstallProgress(
        active: json['active'] as bool? ?? false,
        totalBytes: json['total_bytes'] as int? ?? 0,
        downloaded: json['downloaded'] as int? ?? 0,
        percent: (json['percent'] as num?)?.toDouble() ?? 0,
        speed: json['speed'] as String? ?? '',
        error: json['error'] as String?,
      );
}
