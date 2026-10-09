import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/clipboard_copy.dart';
import '../../../core/friendly_error.dart';
import '../../../core/l10n.dart';
import '../../../core/theme.dart';
import '../../../models/provider_models.dart';
import '../../../models/subscriptions.dart';
import '../../quota_badge.dart';
import '../../../providers/chat_provider.dart';
import '../../../providers/provider_provider.dart';
import '../../../providers/settings_provider.dart';

/// Settings → Subscriptions. Sign in with an Antigravity, Claude or Codex
/// account and use its models in Memo chat, the terminal and the local /v1
/// gateway. The work is done by CLIProxyAPI, which ships INSIDE Memo (it is
/// never downloaded at run time); this screen only drives its sign-in and shows
/// what is available. Backed by /api/subscriptions and internal/app/subs.go.
///
/// Models are not picked here: they appear in the chat's model selector and
/// `/model`, which is where the choice is made.
class SubscriptionsTab extends ConsumerStatefulWidget {
  const SubscriptionsTab({super.key});

  @override
  ConsumerState<SubscriptionsTab> createState() => _SubscriptionsTabState();
}

class _SubscriptionsTabState extends ConsumerState<SubscriptionsTab> {
  Timer? _poll;
  Timer? _modelsPoll; // waits for the sidecar's model list after a sign-in
  Timer? _quotaTimer; // keeps the remaining-allowance figures current
  String _loginFor = ''; // provider whose sign-in this tab is waiting on
  String _authUrl = '';
  bool _starting = false;

  @override
  void initState() {
    super.initState();
    // The allowance left changes as it is used: re-read every 50 seconds while
    // this tab is open (the backend waits briefly for fresh figures on this call).
    _quotaTimer = Timer.periodic(const Duration(seconds: 50), (_) async {
      final st = ref.read(subscriptionsProvider).valueOrNull;
      if (st == null || !st.bundled || st.accounts.isEmpty) return;
      try {
        await ref.read(subscriptionsProvider.notifier).reload();
      } catch (e) {
        debugPrint('subscriptions_tab: quota refresh failed, will retry: $e');
      }
    });
  }

  @override
  void dispose() {
    _poll?.cancel();
    _modelsPoll?.cancel();
    _quotaTimer?.cancel();
    super.dispose();
  }

  /// The sidecar serves no model list until ~30 s after it (re)starts, and a
  /// sign-in restarts it — so right after signing in the account is there but
  /// the list is still empty. Nothing else would ask again, which left "no
  /// models yet" on screen (and the model menus without the Subscriptions
  /// section) until the app was reopened. Re-read every few seconds until the
  /// models arrive, then refresh the selectors that cached the empty answer.
  void _watchForModels(SubscriptionsState st) {
    final waiting = st.bundled && st.accounts.isNotEmpty && st.models.isEmpty;
    if (!waiting || _modelsPoll != null) return;
    var ticks = 0;
    _modelsPoll = Timer.periodic(const Duration(seconds: 3), (t) async {
      ticks++;
      try {
        await ref.read(subscriptionsProvider.notifier).reload();
      } catch (e) {
        debugPrint('subscriptions_tab: waiting for models, will retry: $e');
      }
      final now = ref.read(subscriptionsProvider).valueOrNull;
      final arrived = now != null && now.models.isNotEmpty;
      if (arrived || now == null || now.accounts.isEmpty || ticks >= 60) {
        t.cancel();
        _modelsPoll = null;
        if (arrived) _refreshModelSelectors();
      }
    });
  }

  void _toast(String text) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(text)));
  }

  void _refreshModelSelectors() {
    ref.invalidate(providerListProvider);
    ref.invalidate(gatewayModelsProvider);
  }

  Future<void> _signIn(String provider) async {
    setState(() {
      _starting = true;
      _loginFor = provider;
      _authUrl = '';
    });
    try {
      final url = await ref.read(apiClientProvider).startSubscriptionLogin(provider);
      if (!mounted) return;
      setState(() {
        _starting = false;
        _authUrl = url;
      });
      _watchLogin(provider);
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _starting = false;
        _loginFor = '';
      });
      _toast(L10n.t('subs_login_failed', {'e': FriendlyError.describeGeneric(e)}));
    }
  }

  /// Polls until the sidecar's login process ends. Success shows up as a new
  /// account; failure as an error on the login. Gives up after ~6 minutes (the
  /// backend's own limit).
  void _watchLogin(String provider) {
    _poll?.cancel();
    var ticks = 0;
    _poll = Timer.periodic(const Duration(seconds: 2), (t) async {
      ticks++;
      try {
        await ref.read(subscriptionsProvider.notifier).reload();
      } catch (e) {
        debugPrint('subscriptions_tab: poll failed, will retry: $e');
      }
      final st = ref.read(subscriptionsProvider).valueOrNull;
      final done = st != null && st.login.provider == provider && st.login.done;
      if (done || ticks > 180) {
        t.cancel();
        if (!mounted) return;
        final failed = st?.login.error ?? '';
        setState(() {
          _loginFor = '';
          _authUrl = '';
        });
        if (failed.isNotEmpty) {
          _toast(L10n.t('subs_login_failed', {'e': failed}));
        } else {
          _refreshModelSelectors();
        }
      }
    });
  }

  Future<void> _cancel() async {
    _poll?.cancel();
    setState(() {
      _loginFor = '';
      _authUrl = '';
    });
    try {
      await ref.read(subscriptionsProvider.notifier).cancelLogin();
    } catch (e) {
      debugPrint('subscriptions_tab: cancel failed: $e');
    }
  }

  Future<void> _signOut(String provider) async {
    try {
      await ref.read(subscriptionsProvider.notifier).logout(provider);
      _refreshModelSelectors();
    } catch (e) {
      _toast(L10n.t('subs_error', {'e': FriendlyError.describeGeneric(e)}));
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    final stateAsync = ref.watch(subscriptionsProvider);

    return ListView(
      padding: const EdgeInsets.all(32),
      children: [
        Text(
          L10n.t('tab_subscriptions'),
          style: TextStyle(fontSize: 18, fontWeight: FontWeight.w700, color: theme.textMain),
        ),
        const SizedBox(height: 8),
        Text(
          L10n.t('subs_desc'),
          style: TextStyle(fontSize: 13, height: 1.45, color: theme.textDim),
        ),
        const SizedBox(height: 12),
        Container(
          padding: const EdgeInsets.all(12),
          decoration: BoxDecoration(
            color: MemoTheme.warningOrange.withValues(alpha: 0.10),
            borderRadius: BorderRadius.circular(8),
            border: Border.all(color: MemoTheme.warningOrange.withValues(alpha: 0.4)),
          ),
          child: Text(
            L10n.t('subs_risk_note'),
            style: TextStyle(fontSize: 12, height: 1.4, color: theme.textMain),
          ),
        ),
        const SizedBox(height: 24),
        stateAsync.when(
          loading: () => const Center(
            child: Padding(
              padding: EdgeInsets.all(24),
              child: SizedBox(width: 20, height: 20, child: CircularProgressIndicator(strokeWidth: 2)),
            ),
          ),
          error: (e, _) => Text(
            L10n.t('subs_error', {'e': FriendlyError.describeGeneric(e)}),
            style: const TextStyle(color: MemoTheme.red, fontSize: 12),
          ),
          data: (st) {
            WidgetsBinding.instance.addPostFrameCallback((_) {
              if (mounted) _watchForModels(st);
            });
            return _body(context, st);
          },
        ),
      ],
    );
  }

  Widget _body(BuildContext context, SubscriptionsState st) {
    final theme = MemoTheme.of(context);
    if (!st.bundled) {
      return _Panel(
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Icon(Icons.info_outline_rounded, size: 18, color: MemoTheme.warningOrange),
            const SizedBox(width: 10),
            Expanded(
              child: Text(
                st.problem.isEmpty ? L10n.t('subs_not_bundled') : st.problem,
                style: TextStyle(fontSize: 13, height: 1.45, color: theme.textMain),
              ),
            ),
          ],
        ),
      );
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          '${L10n.t('subs_sidecar_version', {'version': st.version.isEmpty ? '?' : st.version})} · '
          '${st.running ? L10n.t('subs_running') : L10n.t('subs_stopped')}',
          style: TextStyle(fontSize: 11, color: theme.textDim),
        ),
        const SizedBox(height: 12),
        for (final provider in st.providers) ...[
          _providerCard(context, st, provider),
          const SizedBox(height: 12),
        ],
        const SizedBox(height: 12),
        _modelsPanel(context, st),
      ],
    );
  }

  Widget _providerCard(BuildContext context, SubscriptionsState st, String provider) {
    final theme = MemoTheme.of(context);
    final accounts = st.accountsFor(provider);
    final waiting = _loginFor == provider;
    final signedIn = accounts.isNotEmpty;

    return _Panel(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(
                signedIn ? Icons.check_circle_rounded : Icons.radio_button_unchecked_rounded,
                size: 18,
                color: signedIn ? MemoTheme.green : theme.textDim,
              ),
              const SizedBox(width: 10),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      vendorLabel(provider),
                      style: TextStyle(fontSize: 14, fontWeight: FontWeight.w600, color: theme.textMain),
                    ),
                    const SizedBox(height: 2),
                    Text(
                      signedIn
                          ? accounts
                              .map((a) => a.email.isEmpty
                                  ? L10n.t('subs_signed_in')
                                  : L10n.t('subs_signed_in_as', {'email': a.email}))
                              .join('\n')
                          : L10n.t('subs_not_signed_in'),
                      style: TextStyle(fontSize: 12, color: theme.textDim),
                    ),
                  ],
                ),
              ),
              const SizedBox(width: 12),
              if (waiting)
                OutlinedButton(onPressed: _cancel, child: Text(L10n.t('cancel')))
              else if (signedIn)
                OutlinedButton.icon(
                  onPressed: () => _signOut(provider),
                  icon: const Icon(Icons.logout_rounded, size: 16),
                  label: Text(L10n.t('subs_sign_out')),
                )
              else
                FilledButton.icon(
                  onPressed: (_starting || _loginFor.isNotEmpty) ? null : () => _signIn(provider),
                  icon: const Icon(Icons.link_rounded, size: 16),
                  label: Text(L10n.t('subs_sign_in')),
                ),
            ],
          ),
          if (waiting) ...[
            const SizedBox(height: 12),
            Row(
              children: [
                const SizedBox(width: 14, height: 14, child: CircularProgressIndicator(strokeWidth: 2)),
                const SizedBox(width: 10),
                Expanded(
                  child: Text(
                    L10n.t('subs_waiting'),
                    style: TextStyle(fontSize: 12, color: theme.textDim),
                  ),
                ),
              ],
            ),
            if (_authUrl.isNotEmpty) ...[
              const SizedBox(height: 10),
              Text(L10n.t('subs_link_hint'), style: TextStyle(fontSize: 11, color: theme.textDim)),
              const SizedBox(height: 4),
              _CopyableLink(url: _authUrl),
            ],
          ],
        ],
      ),
    );
  }

  Widget _modelsPanel(BuildContext context, SubscriptionsState st) {
    final theme = MemoTheme.of(context);
    return _Panel(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            L10n.t('subs_models_title'),
            style: TextStyle(fontSize: 13, fontWeight: FontWeight.w600, color: theme.textMain),
          ),
          const SizedBox(height: 6),
          if (st.models.isEmpty)
            Text(L10n.t('subs_models_none'), style: TextStyle(fontSize: 12, color: theme.textDim))
          else ...[
            Text(
              L10n.t('subs_models_count', {'n': '${st.models.length}'}),
              style: TextStyle(fontSize: 12, color: theme.textDim),
            ),
            const SizedBox(height: 8),
            Wrap(
              spacing: 6,
              runSpacing: 6,
              children: [
                for (final m in st.models)
                  Container(
                    padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                    decoration: BoxDecoration(
                      color: m.id == st.model ? MemoTheme.accent.withValues(alpha: 0.15) : theme.bgHover,
                      borderRadius: BorderRadius.circular(6),
                      border: Border.all(color: m.id == st.model ? MemoTheme.accent : theme.borderSoft),
                    ),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Flexible(
                          child: Text(
                            m.id,
                            style: TextStyle(fontFamily: 'JetBrainsMono', fontSize: 11, color: theme.textMain),
                          ),
                        ),
                        if (m.remainingPercent != null) ...[
                          const SizedBox(width: 8),
                          QuotaBadge(model: m, fontSize: 11),
                        ],
                      ],
                    ),
                  ),
              ],
            ),
            const SizedBox(height: 10),
            Text(L10n.t('subs_models_hint'), style: TextStyle(fontSize: 11, color: theme.textDim, height: 1.4)),
            if (st.models.any((m) => m.remainingPercent != null)) ...[
              const SizedBox(height: 4),
              Text(L10n.t('subs_quota_hint'), style: TextStyle(fontSize: 11, color: theme.textDim, height: 1.4)),
            ],
          ],
        ],
      ),
    );
  }
}

class _Panel extends StatelessWidget {
  final Widget child;
  const _Panel({required this.child});

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: theme.bgPanel,
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: theme.borderSoft),
      ),
      child: child,
    );
  }
}

class _CopyableLink extends StatelessWidget {
  final String url;
  const _CopyableLink({required this.url});

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    return Container(
      padding: const EdgeInsets.fromLTRB(12, 8, 6, 8),
      decoration: BoxDecoration(
        color: theme.bgHover,
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: theme.borderSoft),
      ),
      child: Row(
        children: [
          Expanded(
            child: SelectableText(
              url,
              maxLines: 2,
              style: TextStyle(fontFamily: 'JetBrainsMono', fontSize: 11, color: theme.textDim),
            ),
          ),
          IconButton(
            tooltip: L10n.t('copy'),
            icon: const Icon(Icons.copy_rounded, size: 15),
            onPressed: () async {
              final ok = await copyToClipboard(url);
              if (!context.mounted) return;
              ScaffoldMessenger.of(context).showSnackBar(
                SnackBar(
                  content: Text(L10n.t(ok ? 'copied' : 'copy_failed')),
                  duration: const Duration(seconds: 1),
                ),
              );
            },
          ),
        ],
      ),
    );
  }
}
