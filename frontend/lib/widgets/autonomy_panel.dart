import 'package:flutter/material.dart';
import '../core/l10n.dart';

/// Reply preparation settings and owner-reviewable activity.
class AutonomyPanel extends StatelessWidget {
  final Map<String, dynamic> settings;
  final Map<String, dynamic> replyModel;
  final List<Map<String, dynamic>> actions;
  final bool busy;
  final ValueChanged<Map<String, dynamic>> onSettings;
  final ValueChanged<String> onCancel;
  const AutonomyPanel({
    super.key,
    required this.settings,
    this.replyModel = const {},
    required this.actions,
    required this.busy,
    required this.onSettings,
    required this.onCancel,
  });

  bool get _mailOn => settings['enabled'] == true && settings['mail'] == true;

  void _channel(String channel, bool enabled) =>
      onSettings({...settings, channel: enabled, if (enabled) 'enabled': true});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colors = theme.colorScheme;
    final mode = settings['mode'] == 'full_auto' || settings['mode'] == 'manual'
        ? settings['mode'] as String
        : 'draft_only';
    final model = '${replyModel['model'] ?? ''}';
    return ListView(
      padding: const EdgeInsets.all(20),
      children: [
        Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    L10n.t('reply_assistant_title'),
                    style: theme.textTheme.titleLarge,
                  ),
                  const SizedBox(height: 6),
                  Text(
                    L10n.t('reply_assistant_subtitle'),
                    style: theme.textTheme.bodyMedium,
                  ),
                ],
              ),
            ),
            const SizedBox(width: 12),
            Chip(
              avatar: Icon(
                _mailOn
                    ? Icons.check_circle_outline
                    : Icons.pause_circle_outline,
                size: 18,
                color: _mailOn ? colors.primary : colors.onSurfaceVariant,
              ),
              label: Text(
                L10n.t(_mailOn ? 'reply_status_on' : 'reply_status_off'),
              ),
            ),
          ],
        ),
        const SizedBox(height: 20),
        Material(
          color: colors.surfaceContainerHighest.withValues(alpha: .4),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(14),
            side: BorderSide(color: colors.outlineVariant),
          ),
          clipBehavior: Clip.antiAlias,
          child: SwitchListTile(
            contentPadding: const EdgeInsets.symmetric(
              horizontal: 16,
              vertical: 8,
            ),
            secondary: Icon(
              Icons.mark_email_read_outlined,
              color: colors.primary,
            ),
            title: Text(
              L10n.t('reply_mail_enable'),
              style: theme.textTheme.titleMedium,
            ),
            subtitle: Padding(
              padding: const EdgeInsets.only(top: 4),
              child: Text(
                L10n.t(_mailOn ? 'reply_mail_on_help' : 'reply_mail_off_help'),
              ),
            ),
            value: _mailOn,
            onChanged: busy ? null : (v) => _channel('mail', v),
          ),
        ),
        const SizedBox(height: 12),
        Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Icon(
              model.isEmpty ? Icons.info_outline : Icons.memory_outlined,
              size: 18,
              color: model.isEmpty ? colors.error : colors.onSurfaceVariant,
            ),
            const SizedBox(width: 8),
            Expanded(
              child: Text(
                model.isEmpty
                    ? L10n.t('mail_auto_no_model')
                    : L10n.t('reply_writer_selected', {'model': model}),
              ),
            ),
          ],
        ),
        const SizedBox(height: 24),
        Text(L10n.t('reply_policy_title'), style: theme.textTheme.titleMedium),
        const SizedBox(height: 6),
        Text(L10n.t('reply_policy_subtitle'), style: theme.textTheme.bodySmall),
        const SizedBox(height: 12),
        LayoutBuilder(
          builder: (context, constraints) {
            final width = constraints.maxWidth >= 760
                ? (constraints.maxWidth - 24) / 3
                : constraints.maxWidth;
            return Wrap(
              spacing: 12,
              runSpacing: 10,
              children: [
                for (final option in ['draft_only', 'full_auto', 'manual'])
                  SizedBox(
                    width: width,
                    child: Semantics(
                      selected: mode == option,
                      child: OutlinedButton(
                        style: OutlinedButton.styleFrom(
                          padding: const EdgeInsets.all(16),
                          alignment: Alignment.centerLeft,
                          foregroundColor: mode == option
                              ? colors.primary
                              : colors.onSurface,
                          backgroundColor: mode == option
                              ? colors.primary.withValues(alpha: .08)
                              : null,
                          side: BorderSide(
                            color: mode == option
                                ? colors.primary
                                : colors.outlineVariant,
                            width: mode == option ? 1.5 : 1,
                          ),
                          shape: RoundedRectangleBorder(
                            borderRadius: BorderRadius.circular(12),
                          ),
                        ),
                        onPressed: busy
                            ? null
                            : () => onSettings({...settings, 'mode': option}),
                        child: Row(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Icon(switch (option) {
                              'draft_only' => Icons.drafts_outlined,
                              'full_auto' => Icons.auto_awesome_outlined,
                              _ => Icons.people_outline,
                            }, size: 22),
                            const SizedBox(width: 10),
                            Expanded(
                              child: Column(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  Text(
                                    L10n.t('auto_$option'),
                                    style: theme.textTheme.labelLarge?.copyWith(
                                      color: mode == option
                                          ? colors.primary
                                          : colors.onSurface,
                                    ),
                                  ),
                                  const SizedBox(height: 5),
                                  Text(
                                    L10n.t('reply_policy_${option}_help'),
                                    style: theme.textTheme.bodySmall,
                                  ),
                                ],
                              ),
                            ),
                            if (mode == option)
                              const Padding(
                                padding: EdgeInsets.only(left: 6),
                                child: Icon(Icons.check_circle, size: 18),
                              ),
                          ],
                        ),
                      ),
                    ),
                  ),
              ],
            );
          },
        ),
        const SizedBox(height: 12),
        Text(
          L10n.t('reply_contact_override_help'),
          style: theme.textTheme.bodySmall,
        ),
        const SizedBox(height: 16),
        ExpansionTile(
          tilePadding: EdgeInsets.zero,
          title: Text(L10n.t('reply_additional_settings')),
          subtitle: Text(L10n.t('reply_additional_help')),
          children: [
            SwitchListTile(
              title: Text(L10n.t('auto_whatsapp')),
              value:
                  settings['enabled'] == true && settings['whatsapp'] == true,
              onChanged: busy ? null : (v) => _channel('whatsapp', v),
            ),
            SwitchListTile(
              title: Text(L10n.t('auto_voice')),
              subtitle: Text(L10n.t('reply_voice_help')),
              value: settings['voice'] == true,
              onChanged: busy
                  ? null
                  : (v) => onSettings({...settings, 'voice': v}),
            ),
          ],
        ),
        const Divider(height: 32),
        Row(
          children: [
            Expanded(
              child: Text(
                L10n.t('reply_activity_title'),
                style: theme.textTheme.titleMedium,
              ),
            ),
            Text('${actions.length}', style: theme.textTheme.labelLarge),
          ],
        ),
        const SizedBox(height: 12),
        if (actions.isEmpty)
          Padding(
            padding: const EdgeInsets.symmetric(vertical: 24),
            child: Column(
              children: [
                Icon(
                  Icons.inbox_outlined,
                  size: 36,
                  color: colors.onSurfaceVariant,
                ),
                const SizedBox(height: 12),
                Text(
                  L10n.t('reply_activity_empty'),
                  style: theme.textTheme.titleSmall,
                ),
                const SizedBox(height: 6),
                Text(
                  L10n.t(
                    _mailOn
                        ? 'reply_activity_empty_on'
                        : 'reply_activity_empty_off',
                  ),
                  textAlign: TextAlign.center,
                  style: theme.textTheme.bodySmall,
                ),
              ],
            ),
          ),
        for (final a in actions) _action(context, a),
      ],
    );
  }

  Widget _action(BuildContext context, Map<String, dynamic> a) {
    final theme = Theme.of(context);
    return Container(
      margin: const EdgeInsets.only(bottom: 12),
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        border: Border.all(color: theme.colorScheme.outlineVariant),
        borderRadius: BorderRadius.circular(12),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Wrap(
            spacing: 12,
            runSpacing: 8,
            crossAxisAlignment: WrapCrossAlignment.center,
            children: [
              Text(
                '${a['recipient'] ?? ''}',
                style: theme.textTheme.titleSmall,
              ),
              Text(
                L10n.t(
                  a['state'] == 'preparing'
                      ? 'mail_auto_stage_${a['stage'] ?? 'analyzing'}'
                      : 'auto_state_${a['state']}',
                ),
                style: theme.textTheme.labelMedium,
              ),
            ],
          ),
          if (a['subject'] is String && (a['subject'] as String).isNotEmpty)
            Padding(
              padding: const EdgeInsets.only(top: 6),
              child: Text(a['subject'] as String),
            ),
          const SizedBox(height: 8),
          Text(_reason(a['reason']), style: theme.textTheme.bodySmall),
          if (a['decision_score'] is num)
            Text(
              L10n.t('mail_reply_score', {
                'score': (a['decision_score'] as num).toStringAsFixed(2),
              }),
            ),
          if (a['score_status'] == 'unavailable')
            Text(L10n.t('mail_reply_score_unavailable')),
          if (a['mode'] == 'grace_period_auto') Text(L10n.t('auto_candidate')),
          if (a['conflict'] == true) Text(L10n.t('auto_conflict')),
          if (a['alternatives'] is List)
            for (final t in a['alternatives'] as List) Text(t.toString()),
          if (a['draft_saved'] == true) Text(L10n.t('auto_saved')),
          if (a['body'] is String && (a['body'] as String).isNotEmpty)
            Padding(
              padding: const EdgeInsets.only(top: 12),
              child: SelectableText(a['body'] as String),
            ),
          if (a['state'] == 'pending')
            Padding(
              padding: const EdgeInsets.only(top: 12),
              child: OutlinedButton.icon(
                onPressed: busy ? null : () => onCancel(a['id'] as String),
                icon: const Icon(Icons.close, size: 18),
                label: Text(L10n.t('cancel')),
              ),
            ),
        ],
      ),
    );
  }

  String _reason(dynamic code) => L10n.t(switch (code) {
    'reply_timeout' => 'auto_reason_timeout',
                    'reply_cancelled' => 'auto_reason_cancelled',
                    'reply_invalid_response' => 'auto_reason_invalid_response',
                    'reply_provider_failed' => 'auto_reason_provider_failed',
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
