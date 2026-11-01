import 'package:flutter/material.dart';
import '../core/l10n.dart';

/// Decisions are reviewable here; delivery is handled by the approval bridge.
class AutonomyPanel extends StatelessWidget {
  final Map<String, dynamic> settings;
  final List<Map<String, dynamic>> actions;
  final bool busy;
  final ValueChanged<Map<String, dynamic>> onSettings;
  final ValueChanged<String> onCancel;
  const AutonomyPanel({
    super.key,
    required this.settings,
    required this.actions,
    required this.busy,
    required this.onSettings,
    required this.onCancel,
  });

  @override
  Widget build(BuildContext context) => ListView(
    padding: const EdgeInsets.all(16),
    children: [
      Text(L10n.t('auto_stage_notice')),
      Text(L10n.t('auto_voice_help')),
      for (final key in ['enabled', 'mail', 'whatsapp', 'voice'])
        SwitchListTile(
          title: Text(L10n.t('auto_$key')),
          value: settings[key] == true,
          onChanged: busy ? null : (v) => onSettings({...settings, key: v}),
        ),
      DropdownButtonFormField<String>(
        isExpanded: true,
        key: ValueKey(settings['mode']),
        initialValue:
            ['draft_only', 'full_auto', 'manual'].contains(settings['mode'])
            ? settings['mode'] as String
            : 'draft_only',
        decoration: InputDecoration(labelText: L10n.t('auto_mode')),
        items: [
          for (final key in ['draft_only', 'full_auto', 'manual'])
            DropdownMenuItem(value: key, child: Text(L10n.t('auto_$key'))),
        ],
        onChanged: busy
            ? null
            : (v) {
                if (v != null) onSettings({...settings, 'mode': v});
              },
      ),
      Text(L10n.t('auto_policy_help')),
      DropdownButtonFormField<int>(
        isExpanded: true,
        key: ValueKey(settings['grace_seconds']),
        initialValue: [30, 60, 120, 300].contains(settings['grace_seconds'])
            ? settings['grace_seconds'] as int
            : 60,
        decoration: InputDecoration(labelText: L10n.t('auto_grace')),
        items: [
          for (final n in [30, 60, 120, 300])
            DropdownMenuItem(
              value: n,
              child: Text(L10n.t('auto_seconds', {'n': '$n'})),
            ),
        ],
        onChanged: busy
            ? null
            : (v) {
                if (v != null) onSettings({...settings, 'grace_seconds': v});
              },
      ),
      const SizedBox(height: 16),
      Text(
        L10n.t('auto_actions'),
        style: Theme.of(context).textTheme.titleMedium,
      ),
      if (actions.isEmpty) Text(L10n.t('auto_empty')),
      for (final a in actions)
        Card(
          child: Padding(
            padding: const EdgeInsets.all(12),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('${a['recipient'] ?? ''} · ${a['channel'] ?? ''}'),
                Text(a['subject'] is String ? a['subject'] as String : ''),
                Text(L10n.t('auto_state_${a['state']}')),
                Text(_reason(a['reason'])),
                if (a['decision_score'] is num)
                  Text(
                    L10n.t('mail_reply_score', {
                      'score': (a['decision_score'] as num).toStringAsFixed(2),
                    }),
                  ),
                if (a['score_status'] == 'unavailable')
                  Text(L10n.t('mail_reply_score_unavailable')),
                if (a['mode'] == 'grace_period_auto')
                  Text(L10n.t('auto_candidate')),
                if (a['conflict'] == true) Text(L10n.t('auto_conflict')),
                if (a['alternatives'] is List)
                  for (final t in a['alternatives'] as List) Text(t.toString()),
                if (a['draft_saved'] == true) Text(L10n.t('auto_saved')),
                SelectableText(a['body'] is String ? a['body'] as String : ''),
                if (a['state'] == 'pending')
                  OutlinedButton(
                    onPressed: busy ? null : () => onCancel(a['id'] as String),
                    child: Text(L10n.t('cancel')),
                  ),
              ],
            ),
          ),
        ),
    ],
  );
  String _reason(dynamic code) => L10n.t(switch (code) {
    'drafted' => 'auto_reason_drafted',
    'stale_message' => 'auto_reason_stale',
    'contact_manual' => 'auto_reason_manual',
    'contact_never' => 'auto_reason_never',
    'automated_sender' => 'auto_reason_automated',
    'no_action' => 'auto_reason_none',
    'prompt_injection' || 'encoded_instruction' => 'auto_reason_injection',
    'secret_api_key' ||
    'secret_private_key' ||
    'secret_env' ||
    'secret_otp' => 'auto_reason_dlp',
    'draft_upload_failed' => 'auto_reason_upload',
    _ => 'auto_reason_review',
  });
}
