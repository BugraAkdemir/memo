import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/api_client.dart';
import '../models/task_list.dart';

class TaskListsNotifier extends AsyncNotifier<List<TaskListInfo>> {
  @override
  Future<List<TaskListInfo>> build() async {
    final api = ref.read(apiClientProvider);
    return api.listTaskLists();
  }

  Future<void> createTaskList(String chatId, String title, List<String> items) async {
    final api = ref.read(apiClientProvider);
    await api.createTaskList(chatId, title, items);
    ref.invalidateSelf();
  }

  Future<void> deleteTaskList(String id) async {
    final api = ref.read(apiClientProvider);
    await api.deleteTaskList(id);
    ref.invalidateSelf();
  }

  Future<void> startTaskList(String id) async {
    final api = ref.read(apiClientProvider);
    await api.startTaskList(id);
    ref.invalidateSelf();
  }

  Future<void> stopTaskList(String id) async {
    final api = ref.read(apiClientProvider);
    await api.stopTaskList(id);
    ref.invalidateSelf();
  }
}

final taskListsProvider =
    AsyncNotifierProvider<TaskListsNotifier, List<TaskListInfo>>(
        TaskListsNotifier.new);
