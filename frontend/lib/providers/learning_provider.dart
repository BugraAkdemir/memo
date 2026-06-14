import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'chat_provider.dart';

/// A learned pattern displayed in the Settings > Learning Profile UI.
class LearnedPattern {
  final String id;
  final String activityType;
  final int peakHour;
  final int peakMinute;
  final int stdDevSeconds;
  final double confidence;
  final List<bool> daysActive;
  final int totalCount;

  const LearnedPattern({
    required this.id,
    required this.activityType,
    required this.peakHour,
    required this.peakMinute,
    required this.stdDevSeconds,
    required this.confidence,
    required this.daysActive,
    required this.totalCount,
  });

  factory LearnedPattern.fromJson(Map<String, dynamic> json) {
    final timeSec = json['time_peak_seconds'] as int? ?? 0;
    return LearnedPattern(
      id: json['id'] as String? ?? '',
      activityType: json['activity_type'] as String? ?? 'unknown',
      peakHour: timeSec ~/ 3600,
      peakMinute: (timeSec % 3600) ~/ 60,
      stdDevSeconds: json['std_dev_seconds'] as int? ?? 0,
      confidence: (json['confidence'] as num?)?.toDouble() ?? 0.0,
      daysActive: (json['days_active'] as List?)
              ?.map((e) => e as bool)
              .toList() ??
          List.filled(7, false),
      totalCount: json['total_count'] as int? ?? 0,
    );
  }

  String get confidencePct => '${(confidence * 100).round()}%';
  String get timeDisplay =>
      '${peakHour.toString().padLeft(2, '0')}:${peakMinute.toString().padLeft(2, '0')}';
  String get stdDisplay {
    if (stdDevSeconds < 120) return '+' + stdDevSeconds.toString() + 's';
    return '+' + (stdDevSeconds ~/ 60).toString() + 'dk';
  }

  static const dayNames = ['Paz', 'Pzt', 'Sal', 'Car', 'Per', 'Cum', 'Cmt'];
  String get daysDisplay {
    final buf = <String>[];
    final len = daysActive.length < 7 ? daysActive.length : 7;
    for (int i = 0; i < len; i++) {
      if (i < dayNames.length && daysActive[i]) {
        buf.add(dayNames[i]);
      }
    }
    return buf.isEmpty ? '-' : buf.join(' ');
  }
}

/// Proactive learning settings (enabled + level).
final learningSettingsProvider =
    StateNotifierProvider<LearningSettingsNotifier, AsyncValue<Map<String, dynamic>>>((ref) {
  return LearningSettingsNotifier(ref);
});

class LearningSettingsNotifier extends StateNotifier<AsyncValue<Map<String, dynamic>>> {
  final Ref _ref;
  LearningSettingsNotifier(this._ref) : super(const AsyncLoading()) {
    _load();
  }

  Future<void> _load() async {
    try {
      final api = _ref.read(apiClientProvider);
      state = AsyncData(await api.getProactiveSettings());
    } catch (e) {
      state = AsyncError(e, StackTrace.current);
    }
  }

  Future<void> update(bool enabled, String level) async {
    try {
      final api = _ref.read(apiClientProvider);
      await api.setProactiveSettings(enabled, level);
      await _load();
    } catch (e) {
      state = AsyncError(e, StackTrace.current);
    }
  }
}

/// Learned patterns list.
final learningPatternsProvider = FutureProvider<List<LearnedPattern>>((ref) async {
  final api = ref.read(apiClientProvider);
  final data = await api.getProactivePatterns();
  return data.map((j) => LearnedPattern.fromJson(j)).toList();
});
