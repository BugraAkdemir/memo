import 'dart:async';
import 'dart:io';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../providers/auth_gate_provider.dart';
import '../providers/chat_provider.dart';
import '../providers/settings_provider.dart';
import 'clear_saved_sign_in_button.dart';

/// True when [error] means "couldn't reach the backend at all" (dead host,
/// refused connection, timed out) rather than a real response the backend
/// sent back (a malformed body, an unexpected status, ...). Only the former
/// is what [BackendUnreachableView] is for — a genuine backend-side failure
/// still deserves its own, different message.
bool isBackendUnreachableError(Object error) {
  if (error is! DioException) return false;
  switch (error.type) {
    case DioExceptionType.connectionError:
    case DioExceptionType.connectionTimeout:
    case DioExceptionType.receiveTimeout:
    case DioExceptionType.sendTimeout:
      return true;
    default:
      return false;
  }
}

/// App-wide overlay: whenever [connectionStatusProvider] has confirmed the
/// backend is unreachable, this covers every screen (chat, calendar, model
/// store, ...) with [BackendUnreachableView] instead of letting each one
/// independently hit and display its own raw connection-error dump. Placed
/// in app_shell.dart's Stack alongside LlamaInstallerOverlay.
class BackendUnreachableOverlay extends ConsumerWidget {
  const BackendUnreachableOverlay({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final connected = ref.watch(connectionStatusProvider).valueOrNull;
    // null covers both "still loading the first check" and "stream errored"
    // (isAlive() itself never throws, but valueOrNull is null either way) —
    // only a confirmed `false` should block the whole app.
    if (connected != false) return const SizedBox.shrink();

    // 401 is "need credentials", not "no backend" — the auth gate owns that
    // state; without this the two overlays would fight over the screen.
    // A null gate ("still rebuilding after login's invalidate()") is also
    // "don't know yet", never a reason to cover the app — the gate decides
    // whether this is a login problem or a real backend problem, not the
    // connectivity poll (BUG-ONB3: the RPi flashed this overlay for the
    // whole stale-false window right after a successful login).
    final auth = ref.watch(authGateProvider).valueOrNull;
    if (auth == null || auth.state != AuthGateState.ok) return const SizedBox.shrink();

    return Container(
      color: MemoTheme.of(context).bgApp.withValues(alpha: 0.95),
      child: const BackendUnreachableView(),
    );
  }
}

/// Replaces a raw exception dump (e.g. "DioException [connection error]:
/// The connection errored: Connection refused ... SocketException ...")
/// with a plain-language explanation of what's actually wrong and three
/// concrete ways out: retry now, point Memo at a different server, or
/// restart Memo outright.
///
/// This is what a user pointed at a since-shut-down Remote Access URL
/// (Settings -> Remote Access -> Backend Server URL) hits on every screen
/// that needs the backend — previously that raw exception text, unfiltered,
/// was the *entire* error experience.
class BackendUnreachableView extends ConsumerWidget {
  const BackendUnreachableView({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final baseUrl = ref.watch(apiClientProvider).baseUrl;
    final theme = MemoTheme.of(context);

    return Center(
      child: Container(
        width: 420,
        padding: const EdgeInsets.all(32),
        decoration: BoxDecoration(
          color: theme.bgPanel,
          borderRadius: BorderRadius.circular(MemoTheme.radiusLg),
          border: Border.all(color: theme.borderSoft),
          boxShadow: [
            BoxShadow(
              color: Colors.black.withValues(alpha: 0.2),
              blurRadius: 20,
              offset: const Offset(0, 10),
            ),
          ],
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: 64,
              height: 64,
              decoration: const BoxDecoration(
                color: MemoTheme.accentPale,
                shape: BoxShape.circle,
              ),
              child: Icon(Icons.cloud_off_rounded, size: 30, color: MemoTheme.accent),
            ),
            const SizedBox(height: 20),
            Text(
              L10n.t('backend_unreachable_title'),
              style: TextStyle(
                fontSize: 18,
                fontWeight: FontWeight.bold,
                color: theme.textMain,
              ),
            ),
            const SizedBox(height: 10),
            Text(
              L10n.t('backend_unreachable_desc', {'url': baseUrl}),
              textAlign: TextAlign.center,
              style: TextStyle(color: theme.textDim, height: 1.5, fontSize: 13),
            ),
            const SizedBox(height: 28),
            SizedBox(
              width: double.infinity,
              height: 44,
              child: ElevatedButton(
                onPressed: () {
                  ref.invalidate(connectionStatusProvider);
                  ref.invalidate(messagesProvider);
                },
                style: ElevatedButton.styleFrom(
                  backgroundColor: MemoTheme.accent,
                  foregroundColor: theme.textInverse,
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                  ),
                ),
                child: Text(L10n.t('retry'), style: const TextStyle(fontWeight: FontWeight.w600)),
              ),
            ),
            const SizedBox(height: 10),
            SizedBox(
              width: double.infinity,
              height: 40,
              child: OutlinedButton(
                // Deliberately its own small dialog, not a hop into the full
                // Settings dialog — a user stuck here can't do anything else
                // in the app anyway, and shouldn't have to go find Remote
                // Access among 20 settings tabs just to fix the one field
                // that's actually blocking them right now.
                onPressed: () => showDialog(
                  context: context,
                  builder: (context) => const ChangeServerDialog(),
                ),
                style: OutlinedButton.styleFrom(
                  foregroundColor: theme.textMain,
                  side: BorderSide(color: theme.borderSoft),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                  ),
                ),
                child: Text(
                  L10n.t('backend_unreachable_change_server'),
                  style: const TextStyle(fontWeight: FontWeight.w500),
                ),
              ),
            ),
            const SizedBox(height: 10),
            TextButton(
              onPressed: () => _confirmRestart(context),
              child: Text(
                L10n.t('backend_unreachable_restart'),
                style: TextStyle(color: theme.textDim, fontWeight: FontWeight.w500),
              ),
            ),
            // "Cannot reach the backend" is also what a client stuck on a
            // credential the server no longer honours can look like, so the
            // escape hatch belongs here too, not only on the auth gate.
            const ClearSavedSignInButton(),
          ],
        ),
      ),
    );
  }

  void _confirmRestart(BuildContext context) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(L10n.t('backend_unreachable_restart_confirm_title')),
        content: Text(L10n.t('backend_unreachable_restart_confirm_body')),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(),
            child: Text(L10n.t('cancel')),
          ),
          TextButton(
            onPressed: () => _restartNow(),
            child: Text(L10n.t('backend_unreachable_restart')),
          ),
        ],
      ),
    );
  }
}

/// Small standalone dialog for editing the backend URL/token — the same
/// fields and providers (backendUrlProvider/backendTokenProvider) as
/// RemoteAccessTab's own copy, kept in sync since both read/write the same
/// SharedPreferences-backed state. A separate, lighter widget rather than
/// reusing RemoteAccessTab directly: that tab also renders Tailscale/ngrok
/// status, which needs a live backend connection to mean anything — useless
/// clutter (or worse, its own error state) in a dialog whose entire point is
/// "the backend is the thing that's broken right now."
///
/// Shared with the auth gate (auth_gate_overlay.dart's server footer line):
/// a user stuck on the login screen because the backend moved deserves the
/// same direct way to re-point the app without finding the Remote Access
/// tab in Settings.
class ChangeServerDialog extends ConsumerStatefulWidget {
  const ChangeServerDialog({super.key, this.title});

  /// Overrides the dialog title — the auth gate's "connect to a remote
  /// server" entry opens the same dialog under its own wording.
  final String? title;

  @override
  ConsumerState<ChangeServerDialog> createState() => ChangeServerDialogState();
}

class ChangeServerDialogState extends ConsumerState<ChangeServerDialog> {
  late final _urlCtrl = TextEditingController(text: ref.read(backendUrlProvider));
  late final _tokenCtrl = TextEditingController(text: ref.read(backendTokenProvider));

  @override
  void dispose() {
    _urlCtrl.dispose();
    _tokenCtrl.dispose();
    super.dispose();
  }

  Future<void> _apply({String? urlOverride, String? tokenOverride}) async {
    await ref.read(backendUrlProvider.notifier).save(urlOverride ?? _urlCtrl.text);
    await ref.read(backendTokenProvider.notifier).save(tokenOverride ?? _tokenCtrl.text);
    ref.invalidate(apiClientProvider);
    if (!mounted) return;
    Navigator.of(context).pop();
    showDialog(
      context: context,
      barrierDismissible: false,
      builder: (context) => const RestartRequiredDialog(),
    );
  }

  /// "Bu bilgisayarın backend'ine dön" — clears both fields back to Memo's
  /// own default (127.0.0.1:8090) in one tap, for the common recovery case
  /// where the user doesn't remember/care what the old remote address was
  /// and just wants their local backend back.
  void _resetToLocal() => _apply(urlOverride: '', tokenOverride: '');

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    return AlertDialog(
      title: Text(widget.title ?? L10n.t('change_server_dialog_title')),
      content: SizedBox(
        width: 380,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            TextField(
              controller: _urlCtrl,
              autofocus: true,
              decoration: InputDecoration(
                labelText: L10n.t('remote_backend_url_field_label'),
                hintText: 'http://127.0.0.1:8090',
                prefixIcon: const Icon(Icons.link, size: 18),
              ),
              style: const TextStyle(fontFamily: 'JetBrainsMono', fontSize: 14),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _tokenCtrl,
              decoration: InputDecoration(
                labelText: L10n.t('remote_backend_token_field_label'),
                hintText: 'memo-...',
                prefixIcon: const Icon(Icons.vpn_key_outlined, size: 18),
              ),
              style: const TextStyle(fontFamily: 'JetBrainsMono', fontSize: 14),
            ),
            const SizedBox(height: 6),
            Text(
              L10n.t('change_server_token_hint'),
              style: TextStyle(color: theme.textDim, fontSize: 11, height: 1.4),
            ),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: _resetToLocal,
          child: Text(L10n.t('reset_to_local_backend')),
        ),
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: Text(L10n.t('cancel')),
        ),
        FilledButton(
          onPressed: () => _apply(),
          child: Text(L10n.t('apply')),
        ),
      ],
    );
  }
}

/// Shown right after the backend URL changes: some long-lived providers
/// (connectionStatusProvider and friends) capture their MemoApiClient via
/// `ref.read` once at their own stream's start rather than `ref.watch`, so
/// they keep talking to the *old* client even after apiClientProvider is
/// invalidated — a full restart is the only way to guarantee every one of
/// them actually picks up the new address, rather than trusting each
/// provider individually got this right.
class RestartRequiredDialog extends StatefulWidget {
  const RestartRequiredDialog({super.key});

  @override
  State<RestartRequiredDialog> createState() => RestartRequiredDialogState();
}

class RestartRequiredDialogState extends State<RestartRequiredDialog> {
  static const _startSeconds = 10;
  int _remaining = _startSeconds;
  Timer? _timer;

  @override
  void initState() {
    super.initState();
    _timer = Timer.periodic(const Duration(seconds: 1), (_) {
      setState(() => _remaining--);
      if (_remaining <= 0) _restartNow();
    });
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: Text(L10n.t('restart_required_title')),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(L10n.t('restart_required_body')),
          const SizedBox(height: 12),
          Text(
            L10n.t('restart_in_seconds', {'s': '$_remaining'}),
            style: TextStyle(color: MemoTheme.of(context).textDim, fontSize: 12),
          ),
        ],
      ),
      actions: [
        FilledButton(
          onPressed: () => _restartNow(),
          child: Text(L10n.t('restart_now_button')),
        ),
      ],
    );
  }
}

// Memo's own process never launches the backend itself — only the OS-level
// entry point (desktop icon, AppImage, run_memo.sh) does that, once, before
// handing off to this frontend binary. So there is no in-process handle to
// a backend child to kill and respawn here; the reliable way to actually
// restart both is to quit and let the user (or, for the countdown path,
// whatever launched Memo in the first place, if it auto-relaunches) reopen
// it through that same entry point — exactly like the sibling "backend died
// mid-session" dialog (_showBackendDeadDialog in app_shell.dart) already
// does.
void _restartNow() => exit(0);
