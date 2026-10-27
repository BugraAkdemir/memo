import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/friendly_error.dart';
import '../../../core/l10n.dart';
import '../../../core/theme.dart';
import '../../../models/telemetry.dart';
import '../../../providers/telemetry_provider.dart';
import '../../telemetry_details.dart';

/// Settings → Privacy: the person's answer about the optional anonymous daily
/// summary (docs/TELEMETRY.md), what it contains, the exact text that would be
/// sent next, a new random id, and deleting what the server holds.
///
/// The answer is a deliberate choice between two equal buttons; nothing here is
/// preselected, and a change is saved only when a button is pressed.
class PrivacyTab extends ConsumerStatefulWidget {
  const PrivacyTab({super.key});

  @override
  ConsumerState<PrivacyTab> createState() => _PrivacyTabState();
}

class _PrivacyTabState extends ConsumerState<PrivacyTab> {
  bool _busy = false;

  Future<void> _run(Future<void> Function() action, String doneKey) async {
    if (_busy) return;
    setState(() => _busy = true);
    final messenger = ScaffoldMessenger.of(context);
    try {
      await action();
      messenger.showSnackBar(SnackBar(content: Text(L10n.t(doneKey))));
    } catch (e) {
      messenger.showSnackBar(
        SnackBar(content: Text(L10n.t('telemetry_failed', {'error': FriendlyError.describeGeneric(e)}))),
      );
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _confirmDelete() async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(L10n.t('telemetry_delete_confirm_title')),
        content: Text(L10n.t('telemetry_delete_confirm_body')),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: Text(L10n.t('cancel'))),
          TextButton(onPressed: () => Navigator.pop(ctx, true), child: Text(L10n.t('delete'))),
        ],
      ),
    );
    if (ok == true && mounted) {
      await _run(() => ref.read(telemetryActionsProvider).deleteData(), 'telemetry_delete_done');
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    final status = ref.watch(telemetryStatusProvider).valueOrNull;

    return ListView(
      padding: const EdgeInsets.all(32),
      children: [
        Text(
          L10n.t('telemetry_title'),
          style: Theme.of(context).textTheme.headlineSmall?.copyWith(fontWeight: FontWeight.bold, color: theme.textMain),
        ),
        const SizedBox(height: 12),
        Text(L10n.t('telemetry_intro'), style: TextStyle(height: 1.6, color: theme.textMuted)),
        const SizedBox(height: 10),
        Text(L10n.t('telemetry_install_note'), style: TextStyle(height: 1.6, fontSize: 13, color: theme.textDim)),
        const SizedBox(height: 20),
        if (status == null)
          const Padding(
            padding: EdgeInsets.symmetric(vertical: 8),
            child: SizedBox(width: 18, height: 18, child: CircularProgressIndicator(strokeWidth: 2)),
          )
        else
          ..._answer(context, status),
        const SizedBox(height: 28),
        const TelemetryDetails(),
        if (status != null && status.deviceId.isNotEmpty) ...[
          const SizedBox(height: 28),
          _deviceBlock(context, status),
        ],
      ],
    );
  }

  List<Widget> _answer(BuildContext context, TelemetryStatus st) {
    final theme = MemoTheme.of(context);
    final label = st.consent == 'yes'
        ? 'telemetry_status_yes'
        : st.consent == 'no'
            ? 'telemetry_status_no'
            : 'telemetry_status_not_asked';
    return [
      Text(L10n.t(label), key: const Key('telemetryStatusText'), style: TextStyle(fontWeight: FontWeight.w600, color: theme.textMain)),
      if (st.envOff) ...[
        const SizedBox(height: 6),
        Text(L10n.t('telemetry_env_off'), style: TextStyle(color: theme.textDim, fontSize: 13, height: 1.5)),
      ],
      if (!st.answered && st.build == 'source') ...[
        const SizedBox(height: 6),
        Text(L10n.t('telemetry_source_build'), style: TextStyle(color: theme.textDim, fontSize: 13, height: 1.5)),
      ],
      const SizedBox(height: 12),
      Wrap(
        spacing: 12,
        runSpacing: 8,
        children: [
          // Two buttons of the same weight: neither is the default and neither is
          // marked as the recommended one. The current answer is shown above, not
          // by highlighting a button.
          OutlinedButton(
            key: const Key('telemetryDecline'),
            onPressed: _busy ? null : () => _run(() => ref.read(telemetryActionsProvider).answer(false), 'telemetry_saved_no'),
            child: Text(L10n.t('telemetry_btn_decline')),
          ),
          OutlinedButton(
            key: const Key('telemetryShare'),
            onPressed: _busy ? null : () => _run(() => ref.read(telemetryActionsProvider).answer(true), 'telemetry_saved_yes'),
            child: Text(L10n.t('telemetry_btn_share')),
          ),
        ],
      ),
    ];
  }

  Widget _deviceBlock(BuildContext context, TelemetryStatus st) {
    final theme = MemoTheme.of(context);
    final dim = TextStyle(color: theme.textDim, fontSize: 13, height: 1.5);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('${L10n.t('telemetry_device_id')}: ${st.deviceId}…', style: TextStyle(fontWeight: FontWeight.w600, color: theme.textMain)),
        const SizedBox(height: 2),
        Text(L10n.t('telemetry_device_id_note'), style: dim),
        if (st.consentDay.isNotEmpty) Text(L10n.t('telemetry_answered_on', {'day': st.consentDay}), style: dim),
        if (st.lastSentDay.isNotEmpty) Text(L10n.t('telemetry_last_sent', {'day': st.lastSentDay}), style: dim),
        const SizedBox(height: 14),
        Wrap(
          spacing: 12,
          runSpacing: 8,
          children: [
            OutlinedButton(
              key: const Key('telemetryResetId'),
              onPressed: _busy ? null : () => _run(() => ref.read(telemetryActionsProvider).resetId(), 'telemetry_reset_done'),
              child: Text(L10n.t('telemetry_reset_id')),
            ),
            OutlinedButton(
              key: const Key('telemetryDelete'),
              onPressed: _busy ? null : _confirmDelete,
              child: Text(L10n.t('telemetry_delete')),
            ),
          ],
        ),
        const SizedBox(height: 6),
        Text(L10n.t('telemetry_reset_id_desc'), style: dim),
        Text(L10n.t('telemetry_delete_desc'), style: dim),
      ],
    );
  }
}
