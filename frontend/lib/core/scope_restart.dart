import 'package:flutter/foundation.dart';

/// Bumped to make main.dart rebuild the whole provider tree from scratch.
///
/// Many long-lived providers capture their MemoApiClient once with
/// `ref.read` (see RestartRequiredDialog), so invalidating
/// `apiClientProvider` alone never moves them to a newly saved backend
/// address. Desktop and web answer that with a full app restart; a phone
/// cannot — `exit(0)` there just closes the app on the user — so the mobile
/// connect flow swaps the ProviderScope instead: every provider is rebuilt
/// against the address that was just saved.
///
/// Only the mobile paths call [softRestartScope]. Desktop and web keep their
/// restart dialog untouched.
final ValueNotifier<int> scopeEpoch = ValueNotifier<int>(0);

void softRestartScope() => scopeEpoch.value++;
