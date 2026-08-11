import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../providers/chat_provider.dart';
import 'auth_gate_provider.dart';
import 'gate_guard.dart';

// BUG-ONB4 follow-up: both providers used to be built as
// `Stream.periodic(...).asyncExpand((_) async* { if (blocked) return; ...
// }).distinct()`. While the gate is blocked, every tick's inner generator
// returns an *empty* stream (no `yield` at all) — and an asyncExpand+distinct
// chain built on top of Stream.periodic does not reliably cancel the
// underlying periodic Timer when its subscription is cancelled in that
// all-empty state (confirmed directly: a minimal, network-free repro of just
// that shape leaves a pending Timer after disposal; the identical shape with
// at least one `yield` per tick disposes cleanly). Any all-empty
// asyncExpand+periodic combination is suspect for the same reason — this
// isn't specific to the mood polling use case. Rewritten as an explicit
// `while (alive)` loop using `cancellablePause` (gate_guard.dart), the same
// proven-safe pattern modelStatusProvider/embeddingStatusProvider/
// downloadProgressProvider already use.

// 10 saniyede bir mood skorunu yeniler.
final moodScoreProvider = StreamProvider.autoDispose<double>((ref) async* {
  var alive = true;
  ref.onDispose(() => alive = false);
  final api = ref.read(apiClientProvider);
  double? last;
  while (alive) {
    // BUG-ONB4: login/setup gate up -> every tick would 401. Skip the
    // request; the pause re-checks until the gate opens.
    if (!authGateBlocked(ref.read(authGateProvider).valueOrNull)) {
      try {
        final score = await api.getMoodScore();
        if (score != last) {
          last = score;
          yield score;
        }
      } catch (_) {
        // Transient — next tick retries. No provider-visible error state,
        // matching the previous asyncMap-based behavior's tolerant consumers.
      }
    }
    await cancellablePause(ref, const Duration(seconds: 10));
  }
});

// 10 saniyede bir mood enabled durumunu yeniler.
final moodEnabledProvider = StreamProvider.autoDispose<bool>((ref) async* {
  var alive = true;
  ref.onDispose(() => alive = false);
  final api = ref.read(apiClientProvider);
  bool? last;
  while (alive) {
    if (!authGateBlocked(ref.read(authGateProvider).valueOrNull)) {
      try {
        final enabled = await api.getMoodEnabled();
        if (enabled != last) {
          last = enabled;
          yield enabled;
        }
      } catch (_) {
        // Transient — next tick retries.
      }
    }
    await cancellablePause(ref, const Duration(seconds: 10));
  }
});
