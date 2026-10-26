import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/friendly_error.dart';
import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/telemetry.dart';
import '../providers/settings_provider.dart';
import '../providers/telemetry_provider.dart';
import 'telemetry_details.dart';

/// Whether the first-run question is due now: the backend says this install has
/// not been asked (only an official build says so), and the person is past the
/// setup wizard, so the question does not land on top of onboarding.
bool shouldAskTelemetry(TelemetryStatus? status, {required bool setupComplete}) {
  return status != null && status.needsAsk && setupComplete;
}

/// Asks the first-run question once per launch while it is unanswered. Renders
/// nothing itself; mount it once in the app shell next to the update prompt.
///
/// A forced choice: two buttons of the same weight, nothing preselected, no
/// "later". Nothing is counted or sent until an answer is given, so closing the
/// app without answering costs nothing, and the question comes back next launch.
class TelemetryPrompt extends ConsumerStatefulWidget {
  const TelemetryPrompt({super.key});

  @override
  ConsumerState<TelemetryPrompt> createState() => _TelemetryPromptState();
}

class _TelemetryPromptState extends ConsumerState<TelemetryPrompt> {
  bool _shown = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _consider());
  }

  void _consider() {
    if (!mounted || _shown) return;
    final status = ref.read(telemetryStatusProvider).valueOrNull;
    if (!shouldAskTelemetry(status, setupComplete: ref.read(setupCompleteProvider))) return;
    // Another window (an update, a permission request) is on top: wait for it
    // instead of stacking a second question over it.
    if (ModalRoute.of(context)?.isCurrent == false) {
      Future<void>.delayed(const Duration(seconds: 2), _consider);
      return;
    }
    _shown = true;
    showDialog<void>(
      context: context,
      barrierDismissible: false,
      builder: (_) => const TelemetryAskDialog(),
    );
  }

  @override
  Widget build(BuildContext context) {
    ref.listen<AsyncValue<TelemetryStatus?>>(telemetryStatusProvider, (_, _) => _consider());
    ref.listen<bool>(setupCompleteProvider, (_, _) => _consider());
    return const SizedBox.shrink();
  }
}

class TelemetryAskDialog extends ConsumerStatefulWidget {
  const TelemetryAskDialog({super.key});

  @override
  ConsumerState<TelemetryAskDialog> createState() => _TelemetryAskDialogState();
}

class _TelemetryAskDialogState extends ConsumerState<TelemetryAskDialog> {
  bool _busy = false;
  String _error = '';

  Future<void> _answer(bool share) async {
    if (_busy) return;
    setState(() {
      _busy = true;
      _error = '';
    });
    try {
      await ref.read(telemetryActionsProvider).answer(share);
      if (mounted) Navigator.of(context).pop();
    } catch (e) {
      // Not saved: the dialog stays, because an unrecorded answer is no answer.
      if (mounted) {
        setState(() {
          _busy = false;
          _error = L10n.t('telemetry_failed', {'error': FriendlyError.describeGeneric(e)});
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    return PopScope(
      canPop: false,
      child: AlertDialog(
        title: Text(L10n.t('telemetry_ask_title')),
        content: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 520),
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(L10n.t('telemetry_ask_body'), style: TextStyle(height: 1.5, color: theme.textMuted)),
                const SizedBox(height: 8),
                Theme(
                  data: Theme.of(context).copyWith(dividerColor: Colors.transparent),
                  child: ExpansionTile(
                    key: const Key('telemetryAskDetails'),
                    tilePadding: EdgeInsets.zero,
                    childrenPadding: EdgeInsets.zero,
                    title: Text(L10n.t('telemetry_ask_details'), style: const TextStyle(fontSize: 14)),
                    children: const [TelemetryDetails()],
                  ),
                ),
                Text(L10n.t('telemetry_ask_change_later'), style: TextStyle(fontSize: 12, color: theme.textDim)),
                if (_error.isNotEmpty) ...[
                  const SizedBox(height: 8),
                  Text(_error, key: const Key('telemetryAskError'), style: const TextStyle(color: Colors.redAccent, fontSize: 12)),
                ],
              ],
            ),
          ),
        ),
        actions: [
          // Same widget, same weight, same size: neither answer is nudged.
          OutlinedButton(
            key: const Key('telemetryAskDecline'),
            onPressed: _busy ? null : () => _answer(false),
            child: Text(L10n.t('telemetry_btn_decline')),
          ),
          OutlinedButton(
            key: const Key('telemetryAskShare'),
            onPressed: _busy ? null : () => _answer(true),
            child: Text(L10n.t('telemetry_btn_share')),
          ),
        ],
      ),
    );
  }
}
