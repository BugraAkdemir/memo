import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../core/l10n.dart';
import '../providers/chat_provider.dart';

Future<void> showMailReplyDraft(
  BuildContext context,
  Map<String, dynamic> action, {
  VoidCallback? onChanged,
}) async {
  await showDialog<bool>(
    context: context,
    barrierDismissible: false,
    builder: (_) => _MailReplyDraftDialog(action: action),
  );
  onChanged?.call();
}

class MailReplyDraftCard extends StatelessWidget {
  final Map<String, dynamic> action;
  final VoidCallback? onReview;
  final VoidCallback? onCancel;
  const MailReplyDraftCard({
    super.key,
    required this.action,
    this.onReview,
    this.onCancel,
  });
  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final state = '${action['state'] ?? ''}';
    final pending =
        state == 'pending' &&
        action['channel'] == 'mail' &&
        (action['body'] is String && (action['body'] as String).isNotEmpty);
    final uncertain = state == 'send_uncertain' || state == 'sending';
    return Container(
      margin: const EdgeInsets.only(bottom: 12),
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: theme.colorScheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Icon(
                state == 'sent'
                    ? Icons.check_circle_outline
                    : pending
                    ? Icons.drafts_outlined
                    : Icons.info_outline,
                color: state == 'sent'
                    ? theme.colorScheme.primary
                    : theme.colorScheme.onSurfaceVariant,
                size: 20,
              ),
              const SizedBox(width: 10),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      '${action['recipient'] ?? ''}',
                      style: theme.textTheme.titleSmall,
                    ),
                    const SizedBox(height: 4),
                    Text(
                      '${action['subject'] ?? ''}',
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ],
                ),
              ),
            ],
          ),
          const SizedBox(height: 10),
          Text(
            L10n.t(
              state == 'preparing'
                  ? 'mail_auto_stage_${action['stage'] ?? 'analyzing'}'
                  : 'auto_state_$state',
            ),
            style: theme.textTheme.labelLarge,
          ),
          if (uncertain)
            Padding(
              padding: const EdgeInsets.only(top: 6),
              child: Text(L10n.t('mail_reply_uncertain_help')),
            ),
          if (!const [
            'pending',
            'sent',
            'sending',
            'send_uncertain',
            'preparing',
          ].contains(state))
            Text(
              L10n.t(switch (action['reason']) {
                'reply_timeout' => 'auto_reason_timeout',
                'reply_cancelled' => 'auto_reason_cancelled',
                'reply_invalid_response' => 'auto_reason_invalid_response',
                'reply_provider_failed' => 'auto_reason_provider_failed',
                'stale_message' => 'auto_reason_stale',
                'contact_manual' => 'auto_reason_manual',
                'contact_never' => 'auto_reason_never',
                'automated_sender' => 'auto_reason_automated',
                'no_action' => 'auto_reason_none',
                'prompt_injection' ||
                'encoded_instruction' => 'auto_reason_injection',
                'secret_api_key' ||
                'secret_private_key' ||
                'secret_env' ||
                'secret_otp' => 'auto_reason_dlp',
                'draft_upload_failed' => 'auto_reason_upload',
                _ => 'auto_reason_review',
              }),
            ),
          if (action['decision_score'] is num)
            Text(
              L10n.t('mail_reply_score', {
                'score': (action['decision_score'] as num).toStringAsFixed(2),
              }),
            ),
          if (action['mode'] == 'grace_period_auto' && pending)
            Text(L10n.t('auto_candidate')),
          if (action['conflict'] == true) Text(L10n.t('auto_conflict')),
          if (action['alternatives'] is List)
            for (final at in action['alternatives'] as List)
              Text(at.toString()),
          if (action['draft_saved'] == true && state != 'sent')
            Text(L10n.t('auto_saved'), style: theme.textTheme.bodySmall),
          if (action['score_status'] == 'unavailable' && pending)
            Text(
              L10n.t('mail_reply_score_unavailable'),
              style: theme.textTheme.bodySmall,
            ),
          if (action['body'] is String && (action['body'] as String).isNotEmpty)
            Padding(
              padding: const EdgeInsets.only(top: 12),
              child: Text(
                action['body'] as String,
                maxLines: 3,
                overflow: TextOverflow.ellipsis,
              ),
            ),
          if (pending)
            Padding(
              padding: const EdgeInsets.only(top: 12),
              child: Wrap(
                spacing: 8,
                runSpacing: 8,
                children: [
                  FilledButton.icon(
                    onPressed: onReview,
                    icon: const Icon(Icons.rate_review_outlined, size: 18),
                    label: Text(L10n.t('mail_reply_review_send')),
                  ),
                  if (onCancel != null)
                    TextButton(
                      onPressed: onCancel,
                      child: Text(L10n.t('cancel')),
                    ),
                ],
              ),
            ),
        ],
      ),
    );
  }
}

class _MailReplyDraftDialog extends ConsumerStatefulWidget {
  final Map<String, dynamic> action;
  const _MailReplyDraftDialog({required this.action});
  @override
  ConsumerState<_MailReplyDraftDialog> createState() =>
      _MailReplyDraftDialogState();
}

class _MailReplyDraftDialogState extends ConsumerState<_MailReplyDraftDialog> {
  late final TextEditingController _subject;
  late final TextEditingController _body;
  bool _sending = false;
  bool _uncertain = false;
  String? _error;
  @override
  void initState() {
    super.initState();
    final subject = '${widget.action['subject'] ?? ''}'.trim();
    _subject = TextEditingController(
      text: subject.toLowerCase().startsWith('re:') ? subject : 'Re: $subject',
    );
    _body = TextEditingController(text: '${widget.action['body'] ?? ''}');
  }

  @override
  void dispose() {
    _subject.dispose();
    _body.dispose();
    super.dispose();
  }

  Future<void> _send() async {
    if (_sending || _uncertain) return;
    if (_body.text.trim().isEmpty) {
      setState(() => _error = L10n.t('mail_reply_body_required'));
      return;
    }
    setState(() {
      _sending = true;
      _error = null;
    });
    try {
      final result = await ref.read(apiClientProvider).callTwin('send_action', {
        'id': widget.action['id'],
        'subject': _subject.text.trim(),
        'body': _body.text,
      });
      if (!mounted) return;
      if (result is! Map || result['state'] != 'sent') {
        setState(() {
          _uncertain = true;
          _error = L10n.t('mail_reply_send_uncertain');
        });
        return;
      }
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(L10n.t('mail_sent_success'))));
      Navigator.of(context).pop(true);
    } catch (e) {
      if (!mounted) return;
      String code = 'mail_reply_send_uncertain';
      if (e is DioException && e.response?.data is Map) {
        final serverCode = (e.response!.data as Map)['error'];
        if (const {
          'mail_reply_not_pending',
          'mail_reply_sent_history',
          'mail_reply_contact_denied',
          'mail_reply_blocked',
          'twin_invalid',
          'twin_not_found',
          'twin_denied',
        }.contains(serverCode)) {
          code = serverCode as String;
        }
      }
      setState(() {
        _uncertain = !const {
          'mail_reply_blocked',
          'twin_invalid',
        }.contains(code);
        _error = L10n.t(code);
      });
    } finally {
      if (mounted) setState(() => _sending = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return PopScope(
      canPop: !_sending,
      child: AlertDialog(
        title: Text(L10n.t('mail_reply_review_title')),
        content: SizedBox(
          width: 640,
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Text(
                  L10n.t('mail_reply_from_account', {
                    'account': '${widget.action['account'] ?? ''}'.replaceFirst(
                      RegExp(r'^[^:]+:'),
                      '',
                    ),
                  }),
                ),
                const SizedBox(height: 8),
                SelectableText(
                  L10n.t('mail_reply_recipient', {
                    'recipient': '${widget.action['recipient'] ?? ''}',
                  }),
                ),
                const SizedBox(height: 16),
                TextField(
                  controller: _subject,
                  enabled: !_sending && !_uncertain,
                  decoration: InputDecoration(
                    labelText: L10n.t('mail_subject'),
                  ),
                ),
                const SizedBox(height: 16),
                TextField(
                  controller: _body,
                  enabled: !_sending && !_uncertain,
                  minLines: 6,
                  maxLines: 12,
                  decoration: InputDecoration(
                    labelText: L10n.t('mail_reply_body_label'),
                    alignLabelWithHint: true,
                  ),
                ),
                const SizedBox(height: 12),
                Text(
                  L10n.t('mail_reply_remote_copy'),
                  style: Theme.of(context).textTheme.bodySmall,
                ),
                if (_error != null)
                  Padding(
                    padding: const EdgeInsets.only(top: 12),
                    child: Text(
                      _error!,
                      style: TextStyle(
                        color: Theme.of(context).colorScheme.error,
                      ),
                    ),
                  ),
                if (_sending)
                  const Padding(
                    padding: EdgeInsets.only(top: 12),
                    child: LinearProgressIndicator(),
                  ),
              ],
            ),
          ),
        ),
        actions: [
          TextButton(
            onPressed: _sending ? null : () => Navigator.of(context).pop(false),
            child: Text(L10n.t('close')),
          ),
          FilledButton.icon(
            onPressed: _sending || _uncertain ? null : _send,
            icon: const Icon(Icons.send_outlined, size: 18),
            label: Text(L10n.t(_sending ? 'mail_reply_sending' : 'mail_send')),
          ),
        ],
      ),
    );
  }
}
