import 'package:flutter/material.dart';

import '../../core/l10n.dart';
import '../../core/theme.dart';
import 'setup_widgets.dart';

/// What the "local model" choice shows: instead of one recommended download it
/// points to the full Model Store (opened as a window) and reports how many chat
/// models are ready, so the user can see the required step is satisfied.
class SetupLocalModelCard extends StatelessWidget {
  final ThemeColors color;
  final bool checking;
  final int chatModelCount;
  final VoidCallback onOpenStore;

  const SetupLocalModelCard({
    super.key,
    required this.color,
    required this.checking,
    required this.chatModelCount,
    required this.onOpenStore,
  });

  @override
  Widget build(BuildContext context) {
    final ready = chatModelCount > 0;
    return SetupCard(
      color: color,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            L10n.t('setup_store_intro'),
            style: TextStyle(color: color.textDim, fontSize: 12, height: 1.5),
          ),
          const SizedBox(height: 12),
          if (checking)
            Row(
              children: [
                const SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(strokeWidth: 2),
                ),
                const SizedBox(width: 10),
                Text(
                  L10n.t('setup_model_checking'),
                  style: TextStyle(color: color.textDim, fontSize: 12),
                ),
              ],
            )
          else
            Row(
              key: Key(ready ? 'setup_local_ready' : 'setup_local_empty'),
              children: [
                Icon(
                  ready ? Icons.check_circle_outline : Icons.info_outline,
                  size: 18,
                  color: ready ? MemoTheme.green : MemoTheme.warningOrange,
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    ready
                        ? L10n.t('setup_local_have', {
                            'count': '$chatModelCount',
                          })
                        : L10n.t('setup_local_none'),
                    style: TextStyle(
                      color: ready ? MemoTheme.green : color.textDim,
                      fontSize: 13,
                    ),
                  ),
                ),
              ],
            ),
          const SizedBox(height: 14),
          FilledButton.icon(
            key: const Key('setup_open_store'),
            onPressed: onOpenStore,
            icon: const Icon(Icons.storefront_outlined, size: 18),
            label: Text(L10n.t('setup_store_open')),
          ),
        ],
      ),
    );
  }
}
