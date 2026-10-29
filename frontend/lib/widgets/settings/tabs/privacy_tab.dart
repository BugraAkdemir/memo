import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/friendly_error.dart';
import '../../../core/l10n.dart';
import '../../../core/theme.dart';
import '../../../providers/telemetry_provider.dart';

class PrivacyTab extends ConsumerStatefulWidget {
  const PrivacyTab({super.key});

  @override
  ConsumerState<PrivacyTab> createState() => _PrivacyTabState();
}

class _PrivacyTabState extends ConsumerState<PrivacyTab> {
  bool _busy = false;
  String? _error;

  Future<void> _setSharing(bool share) async {
    if (_busy) return;
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await ref.read(telemetryActionsProvider).answer(share);
      await ref.read(telemetryStatusProvider.future);
    } catch (e) {
      if (mounted) {
        setState(() {
          _error = L10n.t('telemetry_failed', {
            'error': FriendlyError.describeGeneric(e),
          });
        });
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    final status = ref.watch(telemetryStatusProvider).valueOrNull;

    return ListView(
      padding: const EdgeInsets.all(32),
      children: [
        Text(
          L10n.t('tab_privacy'),
          style: Theme.of(context).textTheme.headlineSmall?.copyWith(
            fontWeight: FontWeight.bold,
            color: theme.textMain,
          ),
        ),
        const SizedBox(height: 24),
        Row(
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    L10n.t('privacy_share_usage_title'),
                    style: TextStyle(
                      fontWeight: FontWeight.w600,
                      color: theme.textMain,
                    ),
                  ),
                  const SizedBox(height: 6),
                  Text(
                    L10n.t('privacy_share_usage_desc'),
                    style: TextStyle(height: 1.5, color: theme.textMuted),
                  ),
                ],
              ),
            ),
            const SizedBox(width: 16),
            Semantics(
              label: L10n.t('privacy_share_usage_title'),
              child: Switch.adaptive(
                key: const Key('telemetrySharingToggle'),
                value: status?.sharing ?? false,
                onChanged: _busy || status == null || status.envOff
                    ? null
                    : _setSharing,
              ),
            ),
          ],
        ),
        if (_error != null) ...[
          const SizedBox(height: 12),
          Text(_error!, style: const TextStyle(color: MemoTheme.red)),
        ],
      ],
    );
  }
}
