import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/l10n.dart';
import '../../../core/theme.dart';
import '../../../providers/chat_provider.dart';
import '../../../providers/settings_provider.dart';
import '../../../core/friendly_error.dart';

/// Settings → Beta Features.
///
/// Experimental toggles used to live under Remote Access (because the first
/// beta feature was Tailscale). Swarm and any future beta gates don't belong
/// there — this is the single place that writes `cfg.Beta` via the API and
/// keeps the local [betaFeaturesProvider] prefs in sync for UI that can't
/// wait on a network round-trip (e.g. Swarm nav rail visibility).
class BetaFeaturesTab extends ConsumerStatefulWidget {
  const BetaFeaturesTab({super.key});

  @override
  ConsumerState<BetaFeaturesTab> createState() => _BetaFeaturesTabState();
}

class _BetaFeaturesTabState extends ConsumerState<BetaFeaturesTab> {
  bool _busy = false;

  Future<void> _setBeta(bool enabled) async {
    if (_busy) return;
    setState(() => _busy = true);
    try {
      await ref.read(apiClientProvider).setBeta(enabled);
      // Keep local prefs in lockstep so Swarm nav / other beta UI that fall
      // back to betaFeaturesProvider don't lag or disagree with the backend.
      await ref.read(betaFeaturesProvider.notifier).setEnabled(enabled);
      ref.invalidate(remoteAccessProvider);
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('${L10n.t('error')}: ${FriendlyError.describeGeneric(e)}')),
        );
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    final raAsync = ref.watch(remoteAccessProvider);

    return raAsync.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (err, _) => Center(
        child: Padding(
          padding: const EdgeInsets.all(32),
          child: Text(
            L10n.t('remote_load_failed', {'err': '$err'}),
            style: TextStyle(color: MemoTheme.red),
          ),
        ),
      ),
      data: (data) {
        final beta = data['beta'] as bool? ?? false;
        return ListView(
          padding: const EdgeInsets.all(32),
          children: [
            Text(
              L10n.t('tab_beta_features'),
              style: TextStyle(
                fontSize: 18,
                fontWeight: FontWeight.w700,
                color: theme.textMain,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              L10n.t('beta_features_page_desc'),
              style: TextStyle(
                fontSize: 13,
                height: 1.45,
                color: theme.textDim,
              ),
            ),
            const SizedBox(height: 24),
            SwitchListTile(
              title: Text(
                L10n.t('remote_beta_features'),
                style: TextStyle(fontSize: 14, color: theme.textMain),
              ),
              subtitle: Text(
                L10n.t('remote_beta_features_desc'),
                style: TextStyle(fontSize: 12, color: theme.textDim),
              ),
              value: beta,
              onChanged: _busy ? null : _setBeta,
              dense: true,
              contentPadding: EdgeInsets.zero,
              activeThumbColor: MemoTheme.accent,
            ),
            if (_busy) ...[
              const SizedBox(height: 8),
              const LinearProgressIndicator(minHeight: 2),
            ],
            const SizedBox(height: 28),
            Text(
              L10n.t('beta_features_includes_title'),
              style: TextStyle(
                fontSize: 13,
                fontWeight: FontWeight.w600,
                color: theme.textMain,
              ),
            ),
            const SizedBox(height: 12),
            _BetaFeatureRow(
              icon: Icons.hub_outlined,
              title: L10n.t('beta_item_swarm_title'),
              body: L10n.t('beta_item_swarm_desc'),
              enabled: beta,
            ),
            const SizedBox(height: 24),
            Container(
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                color: MemoTheme.warningOrange.withValues(alpha: 0.1),
                borderRadius: BorderRadius.circular(8),
                border: Border.all(
                  color: MemoTheme.warningOrange.withValues(alpha: 0.35),
                ),
              ),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Icon(Icons.info_outline,
                      size: 18, color: MemoTheme.warningOrange),
                  const SizedBox(width: 10),
                  Expanded(
                    child: Text(
                      L10n.t('beta_features_warning'),
                      style: TextStyle(
                        fontSize: 12,
                        height: 1.4,
                        color: theme.textSecondary,
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ],
        );
      },
    );
  }
}

class _BetaFeatureRow extends StatelessWidget {
  final IconData icon;
  final String title;
  final String body;
  final bool enabled;

  const _BetaFeatureRow({
    required this.icon,
    required this.title,
    required this.body,
    required this.enabled,
  });

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    final accent = enabled ? MemoTheme.accent : theme.textDim;
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: theme.bgPanel,
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: theme.borderSoft),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            width: 36,
            height: 36,
            decoration: BoxDecoration(
              color: accent.withValues(alpha: 0.12),
              borderRadius: BorderRadius.circular(8),
            ),
            child: Icon(icon, size: 18, color: accent),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  title,
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                    color: theme.textMain,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  body,
                  style: TextStyle(
                    fontSize: 12,
                    height: 1.4,
                    color: theme.textDim,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
