import 'dart:async';

import 'package:dio/dio.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../providers/auth_gate_provider.dart';
import '../providers/chat_provider.dart';
import '../providers/gate_guard.dart';
import 'api_client.dart';
import 'calendar_reminders.dart';
import 'notification_service.dart';
import 'platform_capabilities.dart';

/// Incremented whenever the mobile calendar change stream detects an out-of-band
/// backend change, so an already-mounted [CalendarScreen] refreshes its view.
final calendarRevisionProvider = StateProvider<int>((ref) => 0);

/// Coordinates mobile OS calendar reminder reconciliation, the `/api/calendar/changes`
/// SSE stream, and foreground/background lifecycle transitions in one place.
class CalendarReminderSync {
  CalendarReminderSync({
    required this.readApi,
    required this.isAuthBlocked,
    this.onRemoteChange,
    this.isMounted,
  });

  final MemoApiClient Function() readApi;
  final bool Function() isAuthBlocked;
  final VoidCallback? onRemoteChange;
  final bool Function()? isMounted;

  final CalendarReminderReconciler _reconciler = CalendarReminderReconciler(
    pending: NotificationService.pendingReminders,
    cancel: NotificationService.cancelReminder,
    schedule: (event, lead) => NotificationService.scheduleReminder(
      eventId: event.id,
      title: event.title,
      whenUtc: NotificationService.fireTimeUtc(
        startUtc: event.startUtc,
        leadMinutes: lead,
      ),
      minutesBefore: lead,
    ),
  );

  AppLifecycleListener? _lifecycleListener;
  bool _foreground = true;
  bool _syncing = false;
  bool _syncRequested = false;
  StreamSubscription<void>? _changeSubscription;
  CancelToken? _changeCancel;
  Timer? _changeRetry;
  Timer? _reminderRetry;
  int _changeGeneration = 0;
  bool _disposed = false;

  bool get _alive => !_disposed && (isMounted == null || isMounted!());

  void attachLifecycle() {
    if (!notificationsSupported || _lifecycleListener != null) return;
    _lifecycleListener = AppLifecycleListener(
      onResume: () {
        _foreground = true;
        _reconciler.invalidate();
        startChangeStream();
        syncNow();
        onRemoteChange?.call();
      },
      onPause: pause,
      onHide: pause,
    );
  }

  void pause() {
    _foreground = false;
    stopChangeStream();
    _reminderRetry?.cancel();
  }

  void stopChangeStream() {
    _changeGeneration++;
    _changeRetry?.cancel();
    _changeCancel?.cancel();
    _changeCancel = null;
    _changeSubscription?.cancel();
    _changeSubscription = null;
  }

  void startChangeStream() {
    if (!notificationsSupported ||
        !_alive ||
        !_foreground ||
        isAuthBlocked()) {
      return;
    }
    stopChangeStream();
    final generation = _changeGeneration;
    final cancel = CancelToken();
    _changeCancel = cancel;
    bool retryAllowed = true;
    void retry() {
      if (!_alive || !_foreground || generation != _changeGeneration) return;
      _changeRetry?.cancel();
      _changeRetry = Timer(const Duration(seconds: 5), startChangeStream);
    }

    _changeSubscription = readApi()
        .calendarChangeStream(cancelToken: cancel)
        .listen(
          (_) {
            if (_alive && generation == _changeGeneration) {
              syncNow();
              onRemoteChange?.call();
            }
          },
          onError: (Object error) {
            if (error is DioException &&
                (CancelToken.isCancel(error) ||
                    [401, 403, 404, 501]
                        .contains(error.response?.statusCode))) {
              retryAllowed = false;
              return;
            }
            debugPrint('calendar: change stream: $error');
            retry();
          },
          onDone: () {
            if (retryAllowed) retry();
          },
        );
  }

  Future<void> syncNow() async {
    if (!notificationsSupported ||
        !NotificationService.initialized ||
        !_alive ||
        !_foreground ||
        isAuthBlocked()) {
      return;
    }
    _syncRequested = true;
    if (_syncing) return;
    _syncing = true;
    try {
      do {
        _syncRequested = false;
        final api = readApi();
        // Full upcoming snapshot: changing the viewed month must not cancel
        // alarms for another month, or miss an out-of-band addition.
        final now = DateTime.now().toUtc();
        final raw = await api.getCalendarEvents(
          from: now,
          to: DateTime.utc(9999),
        );
        final settings = await api.getCalendarSettings();
        if (!_alive || !_foreground) return;
        final lead = (settings['reminder_lead_minutes'] as num?)?.toInt() ?? 30;
        final events = raw
            .map(CalendarReminder.fromJson)
            .whereType<CalendarReminder>()
            .toList();
        await _reconciler.sync(events, lead, now);
      } while (_syncRequested && _alive && _foreground);
    } catch (e) {
      debugPrint('calendar: reminder reconciliation failed: $e');
      if (_alive && _foreground) {
        _reminderRetry?.cancel();
        _reminderRetry = Timer(const Duration(seconds: 10), syncNow);
      }
    } finally {
      _syncing = false;
    }
  }

  void handleAuthTransition({required bool wasBlocked, required bool isBlocked}) {
    if (wasBlocked && !isBlocked) {
      startChangeStream();
      syncNow();
    } else if (isBlocked) {
      stopChangeStream();
      _reminderRetry?.cancel();
    }
  }

  void dispose() {
    _disposed = true;
    _lifecycleListener?.dispose();
    _lifecycleListener = null;
    stopChangeStream();
    _reminderRetry?.cancel();
  }
}

/// Always-mounted headless bridge placed inside [AppShell] on mobile so OS
/// calendar reminders reconcile from launch even when [CalendarScreen] is lazily
/// deferred until first visit.
class CalendarReminderBridge extends ConsumerStatefulWidget {
  const CalendarReminderBridge({super.key});

  static int _activeInstances = 0;
  static bool get isActive => _activeInstances > 0;

  static final List<_CalendarReminderBridgeState> _states = [];

  /// Requests an immediate reminder reconciliation from any open screen
  /// (e.g., after creating or deleting a calendar event in [CalendarScreen]).
  static void requestSync() {
    for (final s in List<_CalendarReminderBridgeState>.of(_states)) {
      s.syncNow();
    }
  }

  @override
  ConsumerState<CalendarReminderBridge> createState() =>
      _CalendarReminderBridgeState();
}

class _CalendarReminderBridgeState
    extends ConsumerState<CalendarReminderBridge> {
  late final CalendarReminderSync _sync;

  void syncNow() => _sync.syncNow();

  @override
  void initState() {
    super.initState();
    CalendarReminderBridge._activeInstances++;
    CalendarReminderBridge._states.add(this);
    _sync = CalendarReminderSync(
      readApi: () => ref.read(apiClientProvider),
      isAuthBlocked: () => authGateBlocked(ref.read(authGateProvider).valueOrNull),
      isMounted: () => mounted,
      onRemoteChange: () {
        if (mounted) {
          ref.read(calendarRevisionProvider.notifier).state++;
        }
      },
    )..attachLifecycle();

    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      _sync.startChangeStream();
      _sync.syncNow();
    });
  }

  @override
  void dispose() {
    CalendarReminderBridge._states.remove(this);
    CalendarReminderBridge._activeInstances--;
    _sync.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    ref.listen<AsyncValue<AuthGateInfo>>(authGateProvider, (prev, next) {
      _sync.handleAuthTransition(
        wasBlocked: authGateBlocked(prev?.valueOrNull),
        isBlocked: authGateBlocked(next.valueOrNull),
      );
    });
    return const SizedBox.shrink();
  }
}
