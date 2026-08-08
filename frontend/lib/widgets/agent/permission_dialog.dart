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
  // No existing timeout convention elsewhere in the agent code for user-facing
  // prompts; 5 minutes is generous enough to notice the dialog but short
  // enough that the agent pipeline doesn't hang indefinitely if the user is away.
  static const Duration _timeout = Duration(minutes: 5);

  late int _secondsLeft = _timeout.inSeconds;
  Timer? _countdownTimer;
  bool _submitting = false;
  String? _error;

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
    // end without the user answering: the explicit Stop button, switching
    // to a different chat (ActiveChatIdNotifier.switchTo calls
    // stopStreaming() before switching), natural completion, or an error —
    // all of them flip it to false. Once that happens the backend side has
    // already given up waiting on this requestId (context cancellation) or
    // moved on entirely, so leaving the dialog up would only let the user
    // send a decision for a request that no longer exists (BUG-L1). Skip
    // the auto-pop while a submit is already in flight so it can't race
    // Navigator.pop with _submit's own.
    ref.listen<bool>(isSendingProvider, (prev, next) {
      if (!next && !_submitting && mounted) {
        Navigator.of(context).pop();
      }
    });

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
