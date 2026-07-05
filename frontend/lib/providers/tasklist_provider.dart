import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../models/task_list.dart';
import 'chat_provider.dart';

class TaskListsNotifier extends AsyncNotifier<List<TaskListInfo>> {
  Timer? _pollTimer;

  @override
  Future<List<TaskListInfo>> build() async {
    ref.onDispose(stopPolling);
    final api = ref.read(apiClientProvider);
    return api.listTaskLists();
  }

  Future<void> refresh() async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() async {
      final api = ref.read(apiClientProvider);
      return api.listTaskLists();
    });
  }

  /// Re-fetches without flipping to a loading state first, so a periodic
  /// poll doesn't flash the whole list to a spinner every tick.
  Future<void> _silentRefresh() async {
    final api = ref.read(apiClientProvider);
    state = await AsyncValue.guard(() => api.listTaskLists());
  }

  /// The engine runs a task list's items in the background with no push
  /// channel to the frontend, so the Tasks screen would otherwise look
  /// frozen after Start until the user manually hits refresh. Call this
  /// while the screen showing task lists is visible; call [stopPolling]
  /// when it isn't.
  void startPolling() {
    _pollTimer?.cancel();
    _pollTimer = Timer.periodic(const Duration(seconds: 3), (_) => _silentRefresh());
  }

  void stopPolling() {
    _pollTimer?.cancel();
    _pollTimer = null;
  }

  /// Runs [action] and, on failure, surfaces the error via the app-wide
  /// error toast instead of leaving callers with an unhandled Future
  /// rejection (create/start/stop/delete are all fire-and-forget from the
  /// UI, so this is the only place that reliably sees the error).
  Future<void> _guarded(Future<void> Function() action) async {
    try {
      await action();
      await refresh();
    } catch (e) {
      ref.read(errorMessageProvider.notifier).state = '${L10n.t('error')}: $e';
    }
  }

  Future<void> createTaskList(String chatId, String title, List<String> items) {
    final api = ref.read(apiClientProvider);
    return _guarded(() => api.createTaskList(chatId, title, items));
  }

  Future<void> deleteTaskList(String id) {
    final api = ref.read(apiClientProvider);
    return _guarded(() => api.deleteTaskList(id));
  }

  Future<void> startTaskList(String id) {
    final api = ref.read(apiClientProvider);
    return _guarded(() => api.startTaskList(id));
  }

  Future<void> stopTaskList(String id) {
    final api = ref.read(apiClientProvider);
    return _guarded(() => api.stopTaskList(id));
  }
}

final taskListsProvider =
    AsyncNotifierProvider<TaskListsNotifier, List<TaskListInfo>>(
        TaskListsNotifier.new);
