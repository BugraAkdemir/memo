import 'dart:async';

import 'package:flutter/foundation.dart';

import 'startup_timing_stub.dart'
    if (dart.library.io) 'startup_timing_io.dart' as platform;

/// Records startup stage timestamps across launcher, native Windows runner,
/// Dart entrypoint, and the first rendered Flutter frame so installed desktop
/// cold/warm launches can be measured without guessing.
class StartupTiming {
  StartupTiming({
    int Function()? nowEpochMs,
    int Function()? elapsedMs,
    Map<String, String>? environment,
    Future<void> Function(String path, String line)? logWriter,
  })  : _nowEpochMs = nowEpochMs ?? (() => DateTime.now().millisecondsSinceEpoch),
        _elapsedMs = elapsedMs ?? _defaultElapsedMs,
        _envOverride = environment,
        _logWriter = logWriter ?? platform.appendStartupLogLine;

  static final StartupTiming instance = StartupTiming();

  static final Stopwatch _processWatch = Stopwatch()..start();
  static int _defaultElapsedMs() => _processWatch.elapsedMilliseconds;

  static const _externalMarkers = <(String envKey, String stageName)>[
    ('MEMO_LAUNCH_START_MS', 'launcher_start'),
    ('MEMO_BACKEND_READY_MS', 'backend_ready'),
    ('MEMO_NATIVE_ENTRY_MS', 'native_entry'),
    ('MEMO_NATIVE_CONTROLLER_MS', 'native_controller_ready'),
    ('MEMO_NATIVE_PLUGINS_MS', 'native_plugins_registered'),
  ];

  final int Function() _nowEpochMs;
  final int Function() _elapsedMs;
  final Map<String, String>? _envOverride;
  final Future<void> Function(String path, String line) _logWriter;

  Map<String, String> get _env => _envOverride ?? platform.readStartupEnvironment();

  final Map<String, int> _stagesEpochMs = <String, int>{};
  final Map<String, int> _stagesMonotonicMs = <String, int>{};
  int? _dartBaselineEpochMs;
  int? _dartBaselineMonoMs;
  bool _flushed = false;

  /// Marks [stage] once; repeated marks for the same stage are ignored so
  /// rebuilds never overwrite the initial startup measurement.
  void mark(String stage) {
    if (_stagesEpochMs.containsKey(stage)) return;
    final mono = _elapsedMs();
    final epoch = _nowEpochMs();
    _dartBaselineMonoMs ??= mono;
    _dartBaselineEpochMs ??= epoch;
    _stagesMonotonicMs[stage] = mono;
    _stagesEpochMs[stage] = epoch;
  }

  /// Resolves the timeline in chronological order, combining any valid
  /// external epoch-ms markers with monotonic Dart stage offsets.
  List<(String stage, int offsetMs)> timeline() {
    final points = <(String stage, int epochMs)>[];

    for (final (envKey, stageName) in _externalMarkers) {
      final raw = _env[envKey]?.trim();
      if (raw == null || raw.isEmpty) continue;
      final parsed = int.tryParse(raw);
      if (parsed == null || parsed <= 0) continue;
      points.add((stageName, parsed));
    }

    final baseEpoch = _dartBaselineEpochMs;
    final baseMono = _dartBaselineMonoMs;
    if (baseEpoch != null && baseMono != null) {
      for (final entry in _stagesMonotonicMs.entries) {
        final deltaFromDart = entry.value - baseMono;
        points.add((entry.key, baseEpoch + deltaFromDart));
      }
    }

    if (points.isEmpty) return const [];

    var origin = points.first.$2;
    for (final (_, ts) in points) {
      if (ts < origin) origin = ts;
    }

    return [
      for (final (stage, ts) in points) (stage, ts - origin),
    ];
  }

  /// Formats a single-line human- and grep-friendly startup summary.
  String formatReport() {
    final items = timeline();
    if (items.isEmpty) return '[startup] no stages recorded';
    final total = items.map((e) => e.$2).reduce((a, b) => a > b ? a : b);
    final parts = [
      for (final (stage, offset) in items) '$stage=${offset}ms',
      'total=${total}ms',
    ];
    return '[startup] ${parts.join(' ')}';
  }

  /// Emits the startup report via [debugPrint] and, if a startup log target is
  /// configured, appends it asynchronously without blocking UI rendering.
  Future<void> flushToLog() async {
    if (_flushed) return;
    _flushed = true;
    final line = formatReport();
    debugPrint(line);
    final path = platform.resolveStartupLogPath(_env);
    if (path == null || path.isEmpty) return;
    try {
      await _logWriter(path, '${DateTime.now().toUtc().toIso8601String()} $line\n');
    } catch (_) {
      // Telemetry/timing logging must never break app startup.
    }
  }

  /// Fires-and-forgets [flushToLog] once after the first frame completes.
  void reportOnce() {
    if (_flushed) return;
    unawaited(flushToLog());
  }
}
