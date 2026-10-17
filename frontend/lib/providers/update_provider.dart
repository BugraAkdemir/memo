import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/app_restart.dart';
import '../core/platform_capabilities.dart';
import '../models/app_update.dart';
import 'auth_gate_provider.dart';
import 'chat_provider.dart';
import 'gate_guard.dart';
import 'settings_provider.dart';

/// SharedPreferences key holding the version the person chose "Skip this
/// version" for. A DEVICE preference (it is about this person's choice on this
/// device, not about a backend), so it stays out of serverCoupledPrefsKeys.
const skippedUpdateKey = 'memo_update_skipped_version';

/// What the backend says about updates. Ambient: it is read once at start by
/// the update prompt, so a failure must stay invisible (null = nothing to
/// offer), and before the auth gate opens a 401 would be cached for the whole
/// session — it mounts the safe default instead and refetches when the gate
/// opens.
///
/// Re-checks every six hours; a newer version than the one already offered
/// prompts again.
final updateInfoProvider = FutureProvider<AppUpdateInfo?>((ref) async {
  final timer = Timer(const Duration(hours: 6), ref.invalidateSelf);
  ref.onDispose(timer.cancel);
  // Watching only the blocked/open FLAG (not the gate object, which its poll
  // re-emits) makes this fetch again by itself the moment the gate opens, and
  // not on every poll.
  final blocked = ref.watch(authGateProvider.select((g) => authGateBlocked(g.valueOrNull)));
  if (blocked) return null;
  try {
    final map = await ref.read(apiClientProvider).checkForUpdate();
    return AppUpdateInfo.fromJson(map);
  } catch (e) {
    debugPrint('update_provider: $e');
    return null;
  }
});

/// The version this device chose to skip ('' = none).
class SkippedUpdateNotifier extends StateNotifier<String> {
  SkippedUpdateNotifier(this._ref) : super(_ref.read(prefsProvider).getString(skippedUpdateKey) ?? '');
  final Ref _ref;

  Future<void> skip(String version) async {
    state = version;
    await _ref.read(prefsProvider).setString(skippedUpdateKey, version);
  }
}

final skippedUpdateProvider = StateNotifierProvider<SkippedUpdateNotifier, String>(SkippedUpdateNotifier.new);

/// Whether the app update should interrupt the person now: there is one, and it
/// is either required or not the version they skipped.
bool shouldPromptApp(AppUpdateInfo? info, String skipped) {
  if (info == null || !info.available) return false;
  if (info.required) return true;
  return info.latest != skipped;
}

/// State of the update the person started from the dialog.
class UpdateFlow {
  final UpdateProgress progress;

  /// The request to start it is in flight.
  final bool starting;

  /// Set when starting failed before the backend ever began (rejected, offline).
  final String startError;

  /// After a restart: the backend went away and has come back.
  final bool backendBack;

  const UpdateFlow({
    this.progress = const UpdateProgress(),
    this.starting = false,
    this.startError = '',
    this.backendBack = false,
  });

  bool get running => starting || progress.isBusy;
  String get error => startError.isNotEmpty ? startError : (progress.isError ? progress.error : '');

  UpdateFlow copyWith({UpdateProgress? progress, bool? starting, String? startError, bool? backendBack}) => UpdateFlow(
        progress: progress ?? this.progress,
        starting: starting ?? this.starting,
        startError: startError ?? this.startError,
        backendBack: backendBack ?? this.backendBack,
      );
}

/// Starts an update and follows it until it ends. When the backend restarts on
/// purpose (an application update) this window has nothing left to talk to: a
/// desktop UI quits so the updater can bring Memo back up; a browser or phone
/// waits for the backend to return.
class UpdateFlowNotifier extends StateNotifier<UpdateFlow> {
  UpdateFlowNotifier(this._ref) : super(const UpdateFlow());
  final Ref _ref;
  Timer? _poll;
  bool _sawBackendDown = false;
  int _restartTicks = 0;

  /// Poll interval; tests shorten it.
  @visibleForTesting
  Duration pollEvery = const Duration(milliseconds: 700);

  /// Leave the process on restart. Tests replace it.
  @visibleForTesting
  void Function() quitApp = restartApp;

  /// Platform decision, replaceable in tests.
  @visibleForTesting
  bool Function() isLocalDesktop = () => !kIsWeb && !isMobilePlatform;

  Future<void> start(String kind, {String component = ''}) async {
    if (state.running) return;
    _sawBackendDown = false;
    _restartTicks = 0;
    state = UpdateFlow(
      starting: true,
      progress: UpdateProgress(state: 'verifying', kind: kind, component: component),
    );
    try {
      await _ref.read(apiClientProvider).applyUpdate(kind, component: component);
    } catch (e) {
      state = UpdateFlow(startError: e.toString());
      return;
    }
    state = state.copyWith(starting: false);
    _poll?.cancel();
    _poll = Timer.periodic(pollEvery, (_) => _tick());
  }

  void reset() {
    _poll?.cancel();
    state = const UpdateFlow();
  }

  Future<void> _tick() async {
    final api = _ref.read(apiClientProvider);
    if (state.progress.isRestarting) {
      await _watchRestart(api);
      return;
    }
    try {
      final p = UpdateProgress.fromJson(await api.getUpdateStatus());
      state = state.copyWith(progress: p);
      if (p.isDone || p.isError) {
        _poll?.cancel();
        if (p.isDone) _ref.invalidate(updateInfoProvider);
      } else if (p.isRestarting && isLocalDesktop()) {
        // Give the dialog a beat to show "restarting", then get out of the way.
        _poll?.cancel();
        Timer(const Duration(milliseconds: 800), quitApp);
      }
    } catch (e) {
      // A status poll can fail while the backend is being replaced; only the
      // restart watcher below treats that as meaningful.
      debugPrint('update_provider: status: $e');
    }
  }

  /// After "restarting" on a browser/phone: wait for the backend to go down and
  /// come back, then tell the dialog.
  Future<void> _watchRestart(dynamic api) async {
    _restartTicks++;
    try {
      await api.getVersion();
      if (_sawBackendDown) {
        _poll?.cancel();
        state = state.copyWith(backendBack: true);
        _ref.invalidate(updateInfoProvider);
      }
    } catch (_) {
      _sawBackendDown = true;
    }
    // Five minutes is far longer than a restart takes: stop polling and let the
    // person decide.
    if (_restartTicks * pollEvery.inMilliseconds > 5 * 60 * 1000) _poll?.cancel();
  }

  @override
  void dispose() {
    _poll?.cancel();
    super.dispose();
  }
}

final updateFlowProvider =
    StateNotifierProvider.autoDispose<UpdateFlowNotifier, UpdateFlow>((ref) => UpdateFlowNotifier(ref));
