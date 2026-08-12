import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../core/local_session_state.dart';
import '../core/theme.dart';
import '../providers/auth_gate_provider.dart';
import '../providers/chat_provider.dart';
import '../providers/models_provider.dart';
import '../providers/settings_provider.dart';

/// Manual escape hatch for a client whose saved sign-in belongs to a
/// backend that is no longer there.
///
/// authGateProvider recovers from this on its own now (install-id
/// comparison, plus an unauthorized probe) — this button exists for the
/// cases those two layers cannot see, so that being stuck never again
/// requires opening DevTools and clearing site data by hand, which is
/// what the 2026-08-13 Raspberry Pi report came down to.
///
/// Shown on the two screens where a user can actually be stuck: the auth
/// gate and the backend-unreachable view.
///
/// The copy is deliberate. "Reset app data" would read as *deleting
/// Memo's data* — chats, memory, models — which is exactly what this does
/// NOT do, and no wording that leaves that in doubt is acceptable on a
/// button people press when they are already confused. The confirmation
/// says so outright.
class ClearSavedSignInButton extends ConsumerWidget {
  const ClearSavedSignInButton({super.key, this.dense = false});

  /// Renders at footer scale (12px), matching the gate's server row.
  final bool dense;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = MemoTheme.of(context);
    return TextButton(
      onPressed: () => _confirm(context, ref),
      style: TextButton.styleFrom(
        foregroundColor: theme.textDim,
        padding: dense
            ? const EdgeInsets.symmetric(horizontal: 8)
            : const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
        minimumSize: dense ? Size.zero : null,
        tapTargetSize:
            dense ? MaterialTapTargetSize.shrinkWrap : MaterialTapTargetSize.padded,
      ),
      child: Text(
        L10n.t('clear_sign_in_button'),
        style: TextStyle(
          fontSize: dense ? 12 : 14,
          fontWeight: FontWeight.w500,
          color: theme.textDim,
        ),
      ),
    );
  }

  Future<void> _confirm(BuildContext context, WidgetRef ref) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(L10n.t('clear_sign_in_title')),
        content: Text(L10n.t('clear_sign_in_body')),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: Text(L10n.t('cancel')),
          ),
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: Text(L10n.t('clear_sign_in_confirm')),
          ),
        ],
      ),
    );
    if (confirmed != true) return;

    await clearServerCoupledState(ref.read(prefsProvider));
    ref.read(apiClientProvider).clearSessionToken();
    // Same pair every gate-opening path invalidates: the gate re-decides
    // which screen to show, and gpuInfoProvider is a one-shot FutureProvider
    // that would otherwise keep serving whatever it cached while blocked
    // (see BUG-ONB5).
    ref.invalidate(authGateProvider);
    ref.invalidate(gpuInfoProvider);
  }
}
