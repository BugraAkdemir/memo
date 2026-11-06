import 'package:flutter/material.dart';
import '../../../core/l10n.dart';
import '../../mail_reply_model_picker.dart';
import '../../twin_dialog.dart';

/// The reply writer is selected independently of the scalar decision model.
/// The selector itself is shared with the setup wizard.
class MailAutonomyTab extends StatelessWidget {
  const MailAutonomyTab({super.key});

  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.all(20),
      children: [
        Text(
          L10n.t('mail_reply_title'),
          style: Theme.of(context).textTheme.titleLarge,
        ),
        const SizedBox(height: 12),
        Text(L10n.t('mail_reply_help')),
        const SizedBox(height: 16),
        MailReplyModelPicker(
          extraActions: [
            OutlinedButton.icon(
              onPressed: () => showDialog<void>(
                context: context,
                builder: (_) => const TwinDialog(initialTab: 3),
              ),
              icon: const Icon(Icons.auto_awesome_outlined, size: 18),
              label: Text(L10n.t('mail_open_twin_panel')),
            ),
          ],
        ),
      ],
    );
  }
}
