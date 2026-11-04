import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/l10n.dart';
import '../../core/theme.dart';
import '../../providers/mail_provider.dart';
import '../mail_connect_dialogs.dart';
import 'setup_widgets.dart';

/// Optional step: connect one or more mail accounts with the same dialogs the
/// Mail screen uses. Connecting makes the next step (the reply model) required.
class SetupMailStep extends ConsumerWidget {
  final ThemeColors color;

  const SetupMailStep({super.key, required this.color});

  Future<void> _connected(WidgetRef ref, bool? ok) async {
    if (ok != true) return;
    await ref.read(mailAccountsProvider.notifier).reload();
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final accounts = ref.watch(mailAccountsProvider).valueOrNull ?? const [];
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        SetupCard(
          color: color,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                L10n.t('setup_mail_intro'),
                style: TextStyle(
                  color: color.textDim,
                  fontSize: 12,
                  height: 1.5,
                ),
              ),
              if (accounts.isNotEmpty) ...[
                const SizedBox(height: 12),
                for (final a in accounts)
                  Padding(
                    padding: const EdgeInsets.only(bottom: 4),
                    child: Row(
                      key: Key('setup_mail_account_${a.id}'),
                      children: [
                        const Icon(
                          Icons.check_circle_outline,
                          size: 18,
                          color: MemoTheme.green,
                        ),
                        const SizedBox(width: 8),
                        Expanded(
                          child: Text(
                            a.email,
                            style: TextStyle(color: color.textMain),
                          ),
                        ),
                      ],
                    ),
                  ),
                const SizedBox(height: 8),
                Text(
                  L10n.t('setup_mail_needs_model'),
                  style: const TextStyle(
                    fontSize: 12,
                    color: MemoTheme.warningOrange,
                    height: 1.3,
                  ),
                ),
              ],
            ],
          ),
        ),
        const SizedBox(height: 12),
        SetupConnectionChoice(
          key: const Key('setup_mail_gmail'),
          color: color,
          selected: false,
          icon: Icons.mail_outline,
          title: L10n.t('mail_connect_gmail'),
          description: L10n.t('mail_connect_gmail_desc'),
          onTap: () async => _connected(
            ref,
            await showDialog<bool>(
              context: context,
              barrierDismissible: false,
              builder: (_) => const MailOAuthDialog(provider: 'google'),
            ),
          ),
        ),
        const SizedBox(height: 8),
        SetupConnectionChoice(
          key: const Key('setup_mail_outlook'),
          color: color,
          selected: false,
          icon: Icons.mail_outline,
          title: L10n.t('mail_connect_outlook'),
          description: L10n.t('mail_connect_outlook_desc'),
          onTap: () async => _connected(
            ref,
            await showDialog<bool>(
              context: context,
              barrierDismissible: false,
              builder: (_) => const MailOAuthDialog(provider: 'microsoft'),
            ),
          ),
        ),
        const SizedBox(height: 8),
        SetupConnectionChoice(
          key: const Key('setup_mail_custom'),
          color: color,
          selected: false,
          icon: Icons.dns_outlined,
          title: L10n.t('mail_connect_custom'),
          description: L10n.t('mail_connect_custom_desc'),
          onTap: () async => _connected(
            ref,
            await showDialog<bool>(
              context: context,
              builder: (_) => const MailCustomConnectDialog(),
            ),
          ),
        ),
      ],
    );
  }
}
