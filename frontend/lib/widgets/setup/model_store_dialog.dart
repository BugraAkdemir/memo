import 'package:flutter/material.dart';

import '../../core/l10n.dart';
import '../../screens/model_store_screen.dart';

/// Opens the full Model Store in a window on top of the wizard. Used wherever the
/// wizard used to offer a single recommended download: the user sees the same
/// store as in the app, with sizes and hardware fit, and closes it when done.
Future<void> showSetupModelStore(BuildContext context) {
  return showDialog<void>(
    context: context,
    builder: (ctx) => Dialog(
      key: const Key('setup_model_store_dialog'),
      insetPadding: const EdgeInsets.all(16),
      clipBehavior: Clip.antiAlias,
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 1100, maxHeight: 760),
        child: Column(
          children: [
            const Expanded(child: ModelStoreScreen()),
            const Divider(height: 1),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
              child: Row(
                children: [
                  Expanded(
                    child: Text(
                      L10n.t('setup_store_hint'),
                      style: Theme.of(ctx).textTheme.bodySmall,
                    ),
                  ),
                  const SizedBox(width: 12),
                  FilledButton(
                    key: const Key('setup_store_done'),
                    onPressed: () => Navigator.of(ctx).pop(),
                    child: Text(L10n.t('setup_store_done')),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    ),
  );
}
