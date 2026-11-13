import 'dart:io';

import 'package:path/path.dart' as p;

Map<String, String> readStartupEnvironment() => Platform.environment;

String? resolveStartupLogPath(Map<String, String> env) {
  final explicit = env['MEMO_STARTUP_LOG']?.trim();
  if (explicit != null && explicit.isNotEmpty) {
    return explicit;
  }
  final dataDir = env['MEMO_DATA_DIR']?.trim();
  if (dataDir != null && dataDir.isNotEmpty) {
    return p.join(dataDir, 'startup-timing.log');
  }
  if (Platform.isWindows) {
    final programData = (env['ProgramData'] ?? env['ALLUSERSPROFILE'])?.trim();
    if (programData != null && programData.isNotEmpty) {
      final installedData = p.join(programData, 'Memo', 'data');
      if (Directory(installedData).existsSync()) {
        return p.join(installedData, 'startup-timing.log');
      }
    }
  }
  return null;
}

Future<void> appendStartupLogLine(String path, String line) async {
  final file = File(path);
  await file.parent.create(recursive: true);
  await file.writeAsString(line, mode: FileMode.append, flush: false);
}
