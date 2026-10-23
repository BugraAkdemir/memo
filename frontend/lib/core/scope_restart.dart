import 'package:flutter/foundation.dart';

/// Bumped to make main.dart rebuild the whole provider tree from scratch.
///
/// Long-lived providers capture their API client. Changing a server rebuilds
/// the entire scope on every platform, keeping the app and its backend alive.
final ValueNotifier<int> scopeEpoch = ValueNotifier<int>(0);

void softRestartScope() => scopeEpoch.value++;
