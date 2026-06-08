import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/api_client.dart';
import 'chat_provider.dart';

/// Holds the result of a version check.
class VersionInfo {
  final String current;
  final String? latest;
  final String? error;
  final DateTime checkedAt;

  VersionInfo({
    required this.current,
    this.latest,
    this.error,
    DateTime? checkedAt,
  }) : checkedAt = checkedAt ?? DateTime.now();

  bool get hasUpdate => latest != null && latest!.isNotEmpty;

  factory VersionInfo.fromMap(Map<String, dynamic> map) {
    return VersionInfo(
      current: map['current'] as String? ?? 'unknown',
      latest: map['latest'] as String?,
      error: map['error'] as String?,
    );
  }
}

/// Periodically checks for new app versions.
class VersionCheckNotifier extends AutoDisposeAsyncNotifier<VersionInfo> {
  Timer? _timer;

  @override
  Future<VersionInfo> build() async {
    ref.onDispose(() {
      _timer?.cancel();
    });
    _timer = Timer.periodic(const Duration(hours: 6), (_) {
      _check();
    });
    return await _check();
  }

  Future<VersionInfo> _check() async {
    try {
      final api = ref.read(apiClientProvider);
      final result = await api.checkVersion();
      return VersionInfo.fromMap(result);
    } catch (e) {
      return VersionInfo(current: 'V3.0.0', error: e.toString());
    }
  }

  /// Force a manual version check.
  Future<void> refreshVersion() async {
    state = const AsyncValue.loading();
    state = AsyncValue.data(await _check());
  }
}

final versionCheckProvider =
    AutoDisposeAsyncNotifierProvider<VersionCheckNotifier, VersionInfo>(
      VersionCheckNotifier.new,
    );
