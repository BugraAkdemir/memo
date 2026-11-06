import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/friendly_error.dart';
import '../core/l10n.dart';
import '../models/mail.dart';
import '../providers/chat_provider.dart';

String mailSecurityReason(String code) {
  const codes = {
    'clear',
    'prompt_injection',
    'encoded_instruction',
    'secret_api_key',
    'secret_private_key',
    'secret_env',
    'secret_otp',
    'message_too_large',
    'review_required',
    'review_unavailable',
    'group_or_cc',
    'automated_sender',
    'rate_limit',
  };
  return L10n.t(
    codes.contains(code)
        ? 'mail_security_reason_$code'
        : 'mail_security_reason_unknown',
  );
}

String describeMailError(Object error) {
  if (error is DioException) {
    final data = error.response?.data;
    if (data is Map && data['error'] == 'mail_security_blocked') {
      return L10n.t('mail_security_denied', {
        'reason': mailSecurityReason(
          data['code'] is String ? data['code'] : '',
        ),
      });
    }
  }
  return FriendlyError.describeGeneric(error);
}

/// A no-send preview of the same incoming/outgoing guards used by the backend.
class MailSecurityDialog extends ConsumerStatefulWidget {
  const MailSecurityDialog({super.key});

  @override
  ConsumerState<MailSecurityDialog> createState() => _MailSecurityDialogState();
}

class _MailSecurityDialogState extends ConsumerState<MailSecurityDialog> {
  final _text = TextEditingController();
  bool _busy = false;
  MailSecurityReport? _report;
  String? _error;

  @override
  void dispose() {
    _text.dispose();
    super.dispose();
  }

  Future<void> _check() async {
    if (_busy || _text.text.trim().isEmpty) return;
    setState(() {
      _busy = true;
      _report = null;
      _error = null;
    });
    try {
      final report = await ref
          .read(apiClientProvider)
          .checkMailSecurity(_text.text);
      if (!mounted) return;
      setState(() => _report = report);
    } catch (e) {
      if (!mounted) return;
      setState(() => _error = L10n.t('mail_security_check_failed'));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Widget _result(String title, MailSecurityVerdict verdict) {
    return ListTile(
      contentPadding: EdgeInsets.zero,
      leading: Icon(
        verdict.blocked ? Icons.gpp_bad_outlined : Icons.gpp_good_outlined,
        color: verdict.blocked ? Colors.orange : Colors.green,
      ),
      title: Text(title),
      subtitle: Text(
        '${mailSecurityReason(verdict.code)}\n${L10n.t(verdict.check == 'decision' ? 'mail_security_decision' : 'mail_security_rules')}',
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: Text(L10n.t('mail_security_title')),
      content: SizedBox(
        width: 520,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(L10n.t('mail_security_explainer')),
              const SizedBox(height: 12),
              TextField(
                controller: _text,
                enabled: !_busy,
                minLines: 4,
                maxLines: 8,
                maxLength: 12000,
                onChanged: (_) => setState(() {
                  _report = null;
                  _error = null;
                }),
                decoration: InputDecoration(
                  labelText: L10n.t('mail_security_text'),
                ),
              ),
              if (_busy) const LinearProgressIndicator(),
              if (_error != null) Text(_error!),
              if (_report != null) ...[
                _result(L10n.t('mail_security_inbound'), _report!.inbound),
                _result(L10n.t('mail_security_outbound'), _report!.outbound),
              ],
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: Text(L10n.t('mail_security_close')),
        ),
        FilledButton(
          onPressed: _busy || _text.text.trim().isEmpty ? null : _check,
          child: Text(L10n.t('mail_security_check')),
        ),
      ],
    );
  }
}
