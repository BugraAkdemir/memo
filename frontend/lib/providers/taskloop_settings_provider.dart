import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'chat_provider.dart' show apiClientProvider;

/// Raw task-loop settings map from GET /api/taskloop/settings. The Task Loop
/// settings tab watches this and PUTs changes back.
final taskLoopSettingsProvider =
    FutureProvider.autoDispose<Map<String, dynamic>>((ref) async {
  final api = ref.watch(apiClientProvider);
  return api.getTaskLoopSettings();
});
