import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/task_list.dart';
import 'chat_provider.dart';

class TaskListsNotifier extends AsyncNotifier<List<TaskListInfo>> {
  @override
  Future<List<TaskListInfo>> build() async {
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

  Future<void> createTaskList(String chatId, String title, List<String> items) async {
    final api = ref.read(apiClientProvider);
    await api.createTaskList(chatId, title, items);
    await refresh();
  }

  Future<void> deleteTaskList(String id) async {
    final api = ref.read(apiClientProvider);
    await api.deleteTaskList(id);
    await refresh();
  }

  Future<void> startTaskList(String id) async {
    final api = ref.read(apiClientProvider);
    await api.startTaskList(id);
    await refresh();
  }

  Future<void> stopTaskList(String id) async {
    final api = ref.read(apiClientProvider);
    await api.stopTaskList(id);
    await refresh();
  }
}

final taskListsProvider =
    AsyncNotifierProvider<TaskListsNotifier, List<TaskListInfo>>(
        TaskListsNotifier.new);
