import 'dart:async';
import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/l10n.dart';
import '../../core/theme.dart';
import '../../models/agent.dart';
import '../../providers/chat_provider.dart';
import '../../utils/tool_names.dart';
import '../../core/friendly_error.dart';

class PermissionDialog extends ConsumerStatefulWidget {
  final AgentEvent event;

  const PermissionDialog({super.key, required this.event});

  @override
  ConsumerState<PermissionDialog> createState() => _PermissionDialogState();
}

class _PermissionDialogState extends ConsumerState<PermissionDialog> {
  // Must match internal/agent/executor.go's permTimer (60 * time.Second)
  // exactly - they were out of sync (this used to say 5 minutes) and it
  // produced a real, reproduced-live bug: the backend auto-denies at 60s
  // regardless of what this dialog tells the user, so anyone who answered
  // between 1 and 5 minutes in saw "Could not send permission: Something
  // went wrong" for a request the backend had already discarded, with a
  // countdown still innocently ticking down from 4-something minutes as if
  // nothing had happened.
  static const Duration _timeout = Duration(seconds: 60);

  late int _secondsLeft = _timeout.inSeconds;
  Timer? _countdownTimer;
  Timer? _staleCheckTimer;
  bool _staleScheduled = false;
  bool _submitting = false;
  String? _error;
  final DateTime _openedAt = DateTime.now();

  // Hard floor: never auto-dismiss within the first 2s of being shown, no
  // matter what isSendingProvider reads. The permission_request event and the
  // isSendingProvider=true write travel on different providers and can be
  // observed out of order on the first frame; without this floor a stale
  // "false" read there closed the dialog before the user's eyes could even
  // land on it ("izin penceresi neden hemen gidiyor").
  static const Duration _minVisible = Duration(seconds: 2);

  // A not-sending signal (either the live isSendingProvider true->false
  // transition or an already-false first build) only closes this dialog if it
  // is STILL not-sending ~1.8s later. isSendingProvider genuinely dips to
  // false for a frame at an agentic tool-round boundary — the exact moment
  // create_task_md / start_self_driving_task raise their permission request —
  // and an un-debounced pop flash-closed the live dialog before the user
  // could answer, while the backend still sat in its full 60s wait
  // (BUG-PERM1, reproduced live). A resume to sending cancels the pending
  // check.
  void _scheduleStaleCheck() {
    _staleScheduled = true;
    _staleCheckTimer?.cancel();
    final elapsed = DateTime.now().difference(_openedAt);
    final delay = elapsed >= _minVisible
        ? const Duration(milliseconds: 1800)
        : (_minVisible - elapsed) + const Duration(milliseconds: 1800);
    _staleCheckTimer = Timer(delay, () {
      if (mounted &&
          !_submitting &&
          !ref.read(isSendingProvider) &&
          widget.event.requestId != null) {
        Navigator.of(context).pop();
      }
    });
  }

  @override
  void initState() {
    super.initState();
    _countdownTimer = Timer.periodic(const Duration(seconds: 1), (_) {
      if (!mounted) return;
      if (_secondsLeft <= 1) {
        _countdownTimer?.cancel();
        // Fail-safe default: auto-deny, never auto-approve on timeout.
        _submit('deny_once');
        return;
      }
      setState(() => _secondsLeft--);
    });
  }

  @override
  void dispose() {
    _countdownTimer?.cancel();
    _staleCheckTimer?.cancel();
    super.dispose();
  }

  Future<void> _submit(String policy) async {
    _countdownTimer?.cancel();
    if (widget.event.requestId == null) {
      if (mounted) Navigator.of(context).pop();
      return;
    }

    setState(() {
      _submitting = true;
      _error = null;
    });
    try {
      await ref.read(apiClientProvider).handleAgentPermission(widget.event.requestId!, policy);
      if (mounted) Navigator.of(context).pop();
    } catch (e) {
      // Don't pop on failure — the backend never learned the decision, and
      // its tool call is still blocked waiting for one. Popping here would
      // make the dialog vanish as if everything went fine while the agent
      // stays stuck until its own timeout. Leave the dialog open so the
      // user can see the error and retry.
      if (mounted) {
        setState(() {
          _submitting = false;
          _error = L10n.t('permission_send_failed', {'e': FriendlyError.describeGeneric(e)});
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    // isSendingProvider covers every way this dialog's underlying turn can
    // end without the user answering: the explicit Stop button, switching to
    // a different chat (ActiveChatIdNotifier.switchTo calls stopStreaming()
    // before switching), natural completion, or an error — all of them flip
    // it to false. Once that has really happened the backend has given up on
    // this requestId (context cancellation) or moved on, so leaving the
    // dialog up would only let the user answer a request that no longer
    // exists (BUG-L1). But isSendingProvider ALSO dips to false for a single
    // frame at an agentic tool-round boundary — the exact moment
    // create_task_md / start_self_driving_task raise their permission
    // request — so both the live true->false transition and an already-false
    // first build route through _scheduleStaleCheck()'s ~1.8s debounce
    // instead of popping immediately (BUG-PERM1). A resume to sending
    // cancels the pending check; an in-flight submit suppresses it.
    ref.listen<bool>(isSendingProvider, (prev, next) {
      if (!next && !_submitting && mounted) {
        _scheduleStaleCheck();
      } else if (next) {
        _staleCheckTimer?.cancel();
      }
    });
    if (!ref.read(isSendingProvider) && !_submitting && !_staleScheduled) {
      _scheduleStaleCheck();
    }

    final isDangerous = widget.event.dangerLevel == 'dangerous';
    final isMedium = widget.event.dangerLevel == 'medium';
    final toolName = ToolNames.displayName(widget.event.toolName);
    final toolIcon = ToolNames.icon(widget.event.toolName);
    final timerLabel =
        '${_secondsLeft ~/ 60}:${(_secondsLeft % 60).toString().padLeft(2, '0')}';

    String? shortArg;
    try {
      if (widget.event.args is String) {
        final decoded = json.decode(widget.event.args);
        if (decoded is Map && decoded.length == 1) {
          shortArg = decoded.values.first.toString();
        }
      } else if (widget.event.args is Map) {
        final m = widget.event.args as Map;
        if (m.length == 1) {
          shortArg = m.values.first.toString();
        }
      }
    } catch (_) {}

    return AlertDialog(
      title: Row(
        children: [
          Icon(toolIcon, size: 20, color: MemoTheme.accent),
          const SizedBox(width: 8),
          Text(L10n.t('permission_required')),
        ],
      ),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (isDangerous)
            Container(
              padding: const EdgeInsets.all(8),
              margin: const EdgeInsets.only(bottom: 12),
              decoration: BoxDecoration(
                color: MemoTheme.red.withValues(alpha: 0.1),
                border: Border.all(color: MemoTheme.red.withValues(alpha: 0.5)),
                borderRadius: BorderRadius.circular(8),
              ),
              child: Row(
                children: [
                  Icon(Icons.warning_amber_rounded, size: 16, color: MemoTheme.red),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      L10n.t('permission_warning'),
                      style: const TextStyle(
                          color: MemoTheme.red,
                          fontWeight: FontWeight.bold,
                          fontSize: 12),
                    ),
                  ),
                ],
              ),
            ),
          Text(
            L10n.t('permission_wants_tool', {'tool': toolName}),
            style: const TextStyle(fontSize: 14, fontWeight: FontWeight.w500),
          ),
          if (shortArg != null)
            Padding(
              padding: const EdgeInsets.only(top: 6),
              child: Container(
                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                decoration: BoxDecoration(
                  color: MemoTheme.of(context).bgElement,
                  borderRadius: BorderRadius.circular(4),
                ),
                child: Text(
                  shortArg,
                  style: TextStyle(
                    fontFamily: 'monospace',
                    fontSize: 12,
                    color: MemoTheme.of(context).textDim,
                  ),
                ),
              ),
            ),
          Padding(
            padding: const EdgeInsets.only(top: 10),
            child: Text(
              L10n.t('permission_auto_deny_timer', {'time': timerLabel}),
              style: TextStyle(
                fontSize: 11,
                color: MemoTheme.of(context).textDim,
                fontStyle: FontStyle.italic,
              ),
            ),
          ),
          if (_error != null)
            Padding(
              padding: const EdgeInsets.only(top: 10),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Icon(Icons.error_outline, size: 16, color: MemoTheme.red),
                  const SizedBox(width: 6),
                  Expanded(
                    child: Text(
                      _error!,
                      style: TextStyle(fontSize: 12, color: MemoTheme.red),
                    ),
                  ),
                ],
              ),
            ),
        ],
      ),
      actions: [
        TextButton(
          onPressed: _submitting ? null : () => _submit('deny_once'),
          child: Text(L10n.t('deny'), style: const TextStyle(color: Colors.grey)),
        ),
        if (isMedium || !isDangerous) ...[
          TextButton(
            onPressed: _submitting ? null : () => _submit('allow_session'),
            child: Text(
              L10n.t('allow_session'),
              style: TextStyle(color: MemoTheme.accent.withValues(alpha: 0.7), fontSize: 12),
            ),
          ),
        ],
        TextButton(
          onPressed: _submitting ? null : () => _submit('allow_once'),
          style: TextButton.styleFrom(backgroundColor: MemoTheme.accent),
          child: _submitting
              ? const SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
                )
              : Text(
                  L10n.t('allow_short'),
                  style: const TextStyle(color: Colors.white, fontWeight: FontWeight.w600),
                ),
        ),
      ],
    );
  }
}
