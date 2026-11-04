import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:url_launcher/url_launcher.dart';

import '../core/clipboard_copy.dart';
import '../core/friendly_error.dart';
import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/mail.dart';
import '../providers/chat_provider.dart';

/// Mail account connection dialogs (OAuth device flow for Gmail/Outlook and the
/// manual IMAP/SMTP form). Shared by the Mail screen and the setup wizard.
class MailOAuthDialog extends ConsumerStatefulWidget {
  final String provider;

  const MailOAuthDialog({super.key, required this.provider});

  @override
  ConsumerState<MailOAuthDialog> createState() => _MailOAuthDialogState();
}

class _MailOAuthDialogState extends ConsumerState<MailOAuthDialog> {
  final TextEditingController _manualController = TextEditingController();
  Timer? _pollTimer;
  MailOAuthSession? _session;
  String? _error;
  bool _starting = true;
  bool _submittingManual = false;

  @override
  void initState() {
    super.initState();
    _startFlow();
  }

  @override
  void dispose() {
    _pollTimer?.cancel();
    _manualController.dispose();
    super.dispose();
  }

  Future<void> _startFlow() async {
    try {
      final sess = await ref
          .read(apiClientProvider)
          .startMailOAuth(widget.provider);
      if (!mounted) return;
      setState(() {
        _session = sess;
        _starting = false;
      });
      if (sess.authUrl.isNotEmpty) {
        final uri = Uri.tryParse(sess.authUrl);
        if (uri != null) {
          unawaited(launchUrl(uri, mode: LaunchMode.externalApplication));
        }
      }
      _startPolling(sess.state);
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _starting = false;
        _error = FriendlyError.describeGeneric(e);
      });
    }
  }

  void _startPolling(String stateToken) {
    _pollTimer?.cancel();
    _pollTimer = Timer.periodic(const Duration(seconds: 2), (timer) async {
      try {
        final st = await ref
            .read(apiClientProvider)
            .getMailOAuthStatus(stateToken);
        if (!mounted) return;
        if (st.status == 'completed') {
          timer.cancel();
          Navigator.of(context).pop(true);
        } else if (st.status == 'error' && st.error.isNotEmpty) {
          timer.cancel();
          setState(() => _error = st.error);
        }
      } catch (_) {}
    });
  }

  Future<void> _submitManualCallback() async {
    final sess = _session;
    final raw = _manualController.text.trim();
    if (sess == null || raw.isEmpty || _submittingManual) return;
    setState(() {
      _submittingManual = true;
      _error = null;
    });
    try {
      await ref
          .read(apiClientProvider)
          .completeManualMailOAuth(sess.state, raw);
      if (!mounted) return;
      _pollTimer?.cancel();
      Navigator.of(context).pop(true);
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _submittingManual = false;
        _error = FriendlyError.describeGeneric(e);
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final sess = _session;

    return AlertDialog(
      title: Text(L10n.t('mail_oauth_dialog_title')),
      content: SizedBox(
        width: 480,
        child: _starting
            ? const Padding(
                padding: EdgeInsets.all(24),
                child: Center(child: CircularProgressIndicator(strokeWidth: 2)),
              )
            : Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  if (_error != null) ...[
                    Text(
                      L10n.t('mail_error_prefix', {'err': _error!}),
                      style: const TextStyle(
                        color: MemoTheme.red,
                        fontSize: 12,
                      ),
                    ),
                    const SizedBox(height: 12),
                  ],
                  if (sess != null) ...[
                    Row(
                      children: [
                        const SizedBox(
                          width: 16,
                          height: 16,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        ),
                        const SizedBox(width: 10),
                        Expanded(
                          child: Text(
                            L10n.t('mail_oauth_waiting'),
                            style: TextStyle(fontSize: 13, color: c.textMain),
                          ),
                        ),
                      ],
                    ),
                    const SizedBox(height: 14),
                    if (sess.userCode.isNotEmpty) ...[
                      Text(
                        L10n.t('mail_oauth_device_code_label'),
                        style: TextStyle(fontSize: 12, color: c.textDim),
                      ),
                      const SizedBox(height: 4),
                      SelectableText(
                        sess.userCode,
                        style: TextStyle(
                          fontSize: 20,
                          fontWeight: FontWeight.w700,
                          color: MemoTheme.accent,
                        ),
                      ),
                      const SizedBox(height: 12),
                    ],
                    Wrap(
                      spacing: 8,
                      runSpacing: 8,
                      children: [
                        ElevatedButton.icon(
                          onPressed: () {
                            final uri = Uri.tryParse(sess.authUrl);
                            if (uri != null) {
                              launchUrl(
                                uri,
                                mode: LaunchMode.externalApplication,
                              );
                            }
                          },
                          icon: const Icon(Icons.open_in_new, size: 16),
                          label: Text(L10n.t('mail_oauth_open_browser')),
                        ),
                        OutlinedButton.icon(
                          onPressed: () async {
                            final ok = await copyToClipboard(sess.authUrl);
                            if (!context.mounted) return;
                            ScaffoldMessenger.of(context).showSnackBar(
                              SnackBar(
                                content: Text(
                                  L10n.t(
                                    ok
                                        ? 'mail_oauth_url_copied'
                                        : 'copy_failed',
                                  ),
                                ),
                              ),
                            );
                          },
                          icon: const Icon(Icons.copy, size: 16),
                          label: Text(L10n.t('mail_oauth_copy_url')),
                        ),
                      ],
                    ),
                    const SizedBox(height: 16),
                    Text(
                      L10n.t('mail_oauth_manual_hint'),
                      style: TextStyle(fontSize: 12, color: c.textDim),
                    ),
                    const SizedBox(height: 8),
                    Row(
                      children: [
                        Expanded(
                          child: TextField(
                            controller: _manualController,
                            style: TextStyle(fontSize: 12, color: c.textMain),
                            decoration: const InputDecoration(isDense: true),
                          ),
                        ),
                        const SizedBox(width: 8),
                        OutlinedButton(
                          onPressed: _submittingManual
                              ? null
                              : _submitManualCallback,
                          child: Text(L10n.t('mail_oauth_manual_submit')),
                        ),
                      ],
                    ),
                  ],
                ],
              ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(false),
          child: Text(L10n.t('cancel')),
        ),
      ],
    );
  }
}

class MailCustomConnectDialog extends ConsumerStatefulWidget {
  const MailCustomConnectDialog({super.key});

  @override
  ConsumerState<MailCustomConnectDialog> createState() =>
      _MailCustomConnectDialogState();
}

class _MailCustomConnectDialogState
    extends ConsumerState<MailCustomConnectDialog> {
  final TextEditingController _emailCtrl = TextEditingController();
  final TextEditingController _passwordCtrl = TextEditingController();
  final TextEditingController _displayNameCtrl = TextEditingController();
  final TextEditingController _imapHostCtrl = TextEditingController();
  final TextEditingController _imapPortCtrl = TextEditingController(
    text: '993',
  );
  final TextEditingController _smtpHostCtrl = TextEditingController();
  final TextEditingController _smtpPortCtrl = TextEditingController(
    text: '587',
  );

  bool _discovering = false;
  bool _connecting = false;
  String? _error;

  @override
  void dispose() {
    _emailCtrl.dispose();
    _passwordCtrl.dispose();
    _displayNameCtrl.dispose();
    _imapHostCtrl.dispose();
    _imapPortCtrl.dispose();
    _smtpHostCtrl.dispose();
    _smtpPortCtrl.dispose();
    super.dispose();
  }

  Future<void> _autodiscover() async {
    final email = _emailCtrl.text.trim();
    if (email.isEmpty || _discovering) return;
    setState(() {
      _discovering = true;
      _error = null;
    });
    try {
      final cfg = await ref.read(apiClientProvider).autodiscoverMail(email);
      if (!mounted) return;
      setState(() {
        _discovering = false;
        if (cfg.imapHost.isNotEmpty) _imapHostCtrl.text = cfg.imapHost;
        if (cfg.imapPort > 0) _imapPortCtrl.text = '${cfg.imapPort}';
        if (cfg.smtpHost.isNotEmpty) _smtpHostCtrl.text = cfg.smtpHost;
        if (cfg.smtpPort > 0) _smtpPortCtrl.text = '${cfg.smtpPort}';
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _discovering = false;
        _error = FriendlyError.describeGeneric(e);
      });
    }
  }

  Future<void> _connect() async {
    final email = _emailCtrl.text.trim();
    final password = _passwordCtrl.text;
    if (email.isEmpty || password.isEmpty || _connecting) return;
    setState(() {
      _connecting = true;
      _error = null;
    });
    try {
      await ref
          .read(apiClientProvider)
          .connectMailPassword(
            email: email,
            password: password,
            displayName: _displayNameCtrl.text.trim(),
            imapHost: _imapHostCtrl.text.trim(),
            imapPort: int.tryParse(_imapPortCtrl.text.trim()) ?? 993,
            smtpHost: _smtpHostCtrl.text.trim(),
            smtpPort: int.tryParse(_smtpPortCtrl.text.trim()) ?? 587,
          );
      if (!mounted) return;
      Navigator.of(context).pop(true);
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _connecting = false;
        _error = FriendlyError.describeGeneric(e);
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: Text(L10n.t('mail_custom_dialog_title')),
      content: SizedBox(
        width: 460,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              if (_error != null) ...[
                Text(
                  L10n.t('mail_error_prefix', {'err': _error!}),
                  style: const TextStyle(color: MemoTheme.red, fontSize: 12),
                ),
                const SizedBox(height: 12),
              ],
              Row(
                children: [
                  Expanded(
                    child: TextField(
                      controller: _emailCtrl,
                      decoration: InputDecoration(
                        labelText: L10n.t('mail_custom_email_label'),
                        isDense: true,
                      ),
                    ),
                  ),
                  const SizedBox(width: 8),
                  OutlinedButton(
                    onPressed: _discovering ? null : _autodiscover,
                    child: Text(L10n.t('mail_custom_autodiscover_btn')),
                  ),
                ],
              ),
              const SizedBox(height: 10),
              TextField(
                controller: _passwordCtrl,
                obscureText: true,
                decoration: InputDecoration(
                  labelText: L10n.t('mail_custom_password_label'),
                  isDense: true,
                ),
              ),
              const SizedBox(height: 10),
              TextField(
                controller: _displayNameCtrl,
                decoration: InputDecoration(
                  labelText: L10n.t('mail_custom_display_name_label'),
                  isDense: true,
                ),
              ),
              const SizedBox(height: 12),
              Row(
                children: [
                  Expanded(
                    flex: 3,
                    child: TextField(
                      controller: _imapHostCtrl,
                      decoration: InputDecoration(
                        labelText: L10n.t('mail_custom_imap_host'),
                        isDense: true,
                      ),
                    ),
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: TextField(
                      controller: _imapPortCtrl,
                      decoration: InputDecoration(
                        labelText: L10n.t('mail_custom_imap_port'),
                        isDense: true,
                      ),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 10),
              Row(
                children: [
                  Expanded(
                    flex: 3,
                    child: TextField(
                      controller: _smtpHostCtrl,
                      decoration: InputDecoration(
                        labelText: L10n.t('mail_custom_smtp_host'),
                        isDense: true,
                      ),
                    ),
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: TextField(
                      controller: _smtpPortCtrl,
                      decoration: InputDecoration(
                        labelText: L10n.t('mail_custom_smtp_port'),
                        isDense: true,
                      ),
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(false),
          child: Text(L10n.t('cancel')),
        ),
        ElevatedButton(
          onPressed: _connecting ? null : _connect,
          child: Text(L10n.t('mail_custom_connect_btn')),
        ),
      ],
    );
  }
}
