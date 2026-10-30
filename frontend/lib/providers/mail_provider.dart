import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/mail.dart';
import 'auth_gate_provider.dart';
import 'chat_provider.dart';
import 'gate_guard.dart';

/// Currently selected email account ID in the Mail screen.
final activeMailAccountIdProvider = StateProvider<String?>((ref) => null);

/// Connected email accounts from `/api/mail/accounts`.
final mailAccountsProvider =
    AsyncNotifierProvider<MailAccountsNotifier, List<MailAccount>>(
  MailAccountsNotifier.new,
);

class MailAccountsNotifier extends AsyncNotifier<List<MailAccount>> {
  @override
  Future<List<MailAccount>> build() async {
    if (authGateBlocked(ref.read(authGateProvider).valueOrNull)) {
      return const [];
    }
    final api = ref.read(apiClientProvider);
    final accounts = await api.getMailAccounts();
    final activeId = ref.read(activeMailAccountIdProvider);
    if (accounts.isNotEmpty &&
        (activeId == null || !accounts.any((a) => a.id == activeId))) {
      Future.microtask(() {
        ref.read(activeMailAccountIdProvider.notifier).state = accounts.first.id;
      });
    } else if (accounts.isEmpty && activeId != null) {
      Future.microtask(() {
        ref.read(activeMailAccountIdProvider.notifier).state = null;
      });
    }
    return accounts;
  }

  Future<void> reload() async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() => build());
  }

  Future<void> removeAccount(String id) async {
    await ref.read(apiClientProvider).removeMailAccount(id);
    await reload();
  }
}

/// Gatekeeper & Watcher statistics from `/api/mail/stats`.
final mailStatsProvider =
    AsyncNotifierProvider<MailStatsNotifier, MailWatcherStats>(
  MailStatsNotifier.new,
);

class MailStatsNotifier extends AsyncNotifier<MailWatcherStats> {
  @override
  Future<MailWatcherStats> build() async {
    if (authGateBlocked(ref.read(authGateProvider).valueOrNull)) {
      return const MailWatcherStats();
    }
    return ref.read(apiClientProvider).getMailStats();
  }

  Future<MailWatcherStats> syncNow() async {
    final stats = await ref.read(apiClientProvider).syncMailNow();
    state = AsyncValue.data(stats);
    return stats;
  }

  Future<void> reload() async {
    state = await AsyncValue.guard(
      () => ref.read(apiClientProvider).getMailStats(),
    );
  }
}
