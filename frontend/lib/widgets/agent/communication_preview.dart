import 'dart:convert';
import 'package:flutter/material.dart';
import '../../core/l10n.dart';
import '../../models/agent.dart';

/// Show the actual destination and text before approving a communication tool.
class CommunicationPreview extends StatelessWidget {
  const CommunicationPreview({super.key, required this.event});
  final AgentEvent event;

  @override
  Widget build(BuildContext context) {
    if (!const {
      'mail_send',
      'mail_save_draft',
      'whatsapp_send',
      'telegram_send',
    }.contains(event.toolName)) {
      return const SizedBox.shrink();
    }
    Map<dynamic, dynamic> args;
    try {
      final decoded = event.args is String
          ? jsonDecode(event.args as String)
          : event.args;
      if (decoded is! Map) return const SizedBox.shrink();
      args = decoded;
    } catch (_) {
      return const SizedBox.shrink();
    }
    final fields = <String, String>{};
    String value(String key) {
      final v = args[key];
      return v is List ? v.join(', ') : v?.toString() ?? '';
    }

    if (value('account_id').isNotEmpty) {
      fields[L10n.t('communication_account')] = value('account_id');
    }
    if (value('to').isNotEmpty) {
      fields[L10n.t('mail_to')] = value('to');
    }
    if (value('cc').isNotEmpty) {
      fields[L10n.t('mail_cc')] = value('cc');
    }
    if (value('jid').isNotEmpty) {
      fields[L10n.t('mail_to')] = value('jid');
    }
    if (event.toolName == 'telegram_send') {
      fields[L10n.t('mail_to')] = L10n.t('communication_telegram_owner');
    }
    if (value('subject').isNotEmpty) {
      fields[L10n.t('mail_subject')] = value('subject');
    }
    fields[L10n.t('communication_message')] = value('body').isNotEmpty
        ? value('body')
        : value('text');
    return Padding(
      padding: const EdgeInsets.only(top: 10),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxHeight: 220, maxWidth: 480),
        child: SingleChildScrollView(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              for (final field in fields.entries)
                Padding(
                  padding: const EdgeInsets.only(bottom: 8),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        field.key,
                        style: Theme.of(context).textTheme.labelSmall,
                      ),
                      SelectableText(field.value),
                    ],
                  ),
                ),
            ],
          ),
        ),
      ),
    );
  }
}
