import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:url_launcher/url_launcher.dart';

import '../../../core/friendly_error.dart';
import '../../../core/l10n.dart';
import '../../../core/theme.dart';
import '../../../providers/chat_provider.dart';
import '../../../providers/provider_provider.dart';
import '../../../providers/settings_provider.dart';

/// Settings → Gemini Subscription. Sign in with a personal Google account
/// (browser OAuth, gemini-cli's public client — nothing to register) and use
/// Gemini on the account's Google AI Pro/Ultra quota, both in Memo chat and
/// on the local /v1 gateway. Backed by /api/dev-gateway/google-account and
/// internal/app/gemauth.go. Disconnect revokes and removes the token.
class GeminiSubscriptionTab extends ConsumerStatefulWidget {
  const GeminiSubscriptionTab({super.key});

  @override
  ConsumerState<GeminiSubscriptionTab> createState() => _GeminiSubscriptionTabState();
}

class _GeminiSubscriptionTabState extends ConsumerState<GeminiSubscriptionTab> {
  Timer? _pollTimer;
  bool _busy = false;
  String _authUrl = '';

  @override
  void dispose() {
    _pollTimer?.cancel();
    super.dispose();
  }

  Future<void> _connect() async {
    setState(() {
      _busy = true;
      _authUrl = '';
    });
    try {
      final url = await ref.read(apiClientProvider).startGoogleAuth();
      if (!mounted) return;
      setState(() => _authUrl = url);
      if (url.isNotEmpty) {
        // Best-effort: on a headless / minimal desktop launchUrl can fail;
        // the copyable link below is the fallback, always shown.
        try {
          await launchUrl(Uri.parse(url), mode: LaunchMode.externalApplication);
        } catch (e) {
          debugPrint('gemini_subscription_tab: launchUrl failed, falling back to the copyable link: $e');
        }
      }

      var attempts = 0;
      _pollTimer?.cancel();
      _pollTimer = Timer.periodic(const Duration(seconds: 3), (t) async {
        attempts++;
        try {
          await ref.read(googleAccountProvider.notifier).reload();
          final st = ref.read(googleAccountProvider).valueOrNull;
          if (st?.connected == true) {
            t.cancel();
            ref.invalidate(gatewayModelsProvider);
            ref.invalidate(providerListProvider);
            if (mounted) {
              setState(() {
                _busy = false;
                _authUrl = '';
              });
            }
          }
        } catch (_) {
          // transient — keep polling
        }
        if (attempts > 40) {
          t.cancel();
          if (mounted) {
            setState(() => _busy = false);
            ScaffoldMessenger.of(context).showSnackBar(
              SnackBar(content: Text(L10n.t('google_account_timeout'))),
            );
          }
        }
      });
    } catch (e) {
      if (mounted) {
        setState(() => _busy = false);
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(L10n.t('google_account_error', {'e': FriendlyError.describeGeneric(e)}))),
        );
      }
    }
  }

  Future<void> _disconnect() async {
    _pollTimer?.cancel();
    setState(() {
      _busy = false;
      _authUrl = '';
    });
    try {
      await ref.read(googleAccountProvider.notifier).disconnect();
      ref.invalidate(gatewayModelsProvider);
      ref.invalidate(providerListProvider);
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(L10n.t('google_account_error', {'e': FriendlyError.describeGeneric(e)}))),
        );
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    final stateAsync = ref.watch(googleAccountProvider);

    return ListView(
      padding: const EdgeInsets.all(32),
      children: [
        Text(
          L10n.t('tab_gemini_subscription'),
          style: TextStyle(fontSize: 18, fontWeight: FontWeight.w700, color: theme.textMain),
        ),
        const SizedBox(height: 8),
        Text(
          L10n.t('google_account_connect_desc'),
          style: TextStyle(fontSize: 13, height: 1.45, color: theme.textDim),
        ),
        const SizedBox(height: 24),
        stateAsync.when(
          loading: () => const Center(child: Padding(
            padding: EdgeInsets.all(24),
            child: SizedBox(width: 20, height: 20, child: CircularProgressIndicator(strokeWidth: 2)),
          )),
          error: (e, _) => Text(
            '${L10n.t('error')}: ${FriendlyError.describeGeneric(e)}',
            style: const TextStyle(color: MemoTheme.red, fontSize: 12),
          ),
          data: (st) => Container(
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(
              color: theme.bgPanel,
              borderRadius: BorderRadius.circular(10),
              border: Border.all(color: theme.borderSoft),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                if (st.connected) ...[
                  Row(
                    children: [
                      const Icon(Icons.check_circle_rounded, size: 18, color: MemoTheme.green),
                      const SizedBox(width: 8),
                      Expanded(
                        child: Text(
                          st.email.isEmpty
                              ? L10n.t('google_account_connected')
                              : L10n.t('google_account_connected_as', {'email': st.email}),
                          style: TextStyle(fontSize: 13, color: theme.textMain),
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 6),
                  Text(
                    L10n.t('google_account_usage_hint'),
                    style: TextStyle(fontSize: 11, color: theme.textDim, height: 1.4),
                  ),
                  const SizedBox(height: 16),
                  Text(
                    L10n.t('google_account_model_label'),
                    style: TextStyle(fontSize: 12, fontWeight: FontWeight.w600, color: theme.textMain),
                  ),
                  const SizedBox(height: 6),
                  _ModelDropdown(current: st.model),
                  const SizedBox(height: 16),
                  Align(
                    alignment: Alignment.centerLeft,
                    child: OutlinedButton.icon(
                      onPressed: _disconnect,
                      icon: const Icon(Icons.logout_rounded, size: 16),
                      label: Text(L10n.t('google_account_disconnect_cta')),
                    ),
                  ),
                ] else ...[
                  Align(
                    alignment: Alignment.centerLeft,
                    child: FilledButton.icon(
                      onPressed: _busy ? null : _connect,
                      icon: _busy
                          ? const SizedBox(width: 14, height: 14, child: CircularProgressIndicator(strokeWidth: 2))
                          : const Icon(Icons.link_rounded, size: 16),
                      label: Text(_busy ? L10n.t('google_account_connecting') : L10n.t('google_account_connect_cta')),
                    ),
                  ),
                  if (_authUrl.isNotEmpty) ...[
                    const SizedBox(height: 12),
                    Text(
                      L10n.t('google_account_manual_link_hint'),
                      style: TextStyle(fontSize: 11, color: theme.textDim),
                    ),
                    const SizedBox(height: 4),
                    _CopyableLink(url: _authUrl),
                  ],
                ],
              ],
            ),
          ),
        ),
      ],
    );
  }
}

/// Model picker for the connected account — populated live from Google
/// (googleSubModelsProvider), never a hard-coded list. Falls back to a plain
/// display of the current model while the list is loading or unavailable.
class _ModelDropdown extends ConsumerWidget {
  final String current;
  const _ModelDropdown({required this.current});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = MemoTheme.of(context);
    final modelsAsync = ref.watch(googleSubModelsProvider);

    return modelsAsync.when(
      loading: () => Row(
        children: [
          const SizedBox(width: 14, height: 14, child: CircularProgressIndicator(strokeWidth: 2)),
          const SizedBox(width: 8),
          Text(current, style: TextStyle(fontSize: 12, color: theme.textDim, fontFamily: 'JetBrainsMono')),
        ],
      ),
      error: (e, _) => Text(current, style: TextStyle(fontSize: 12, color: theme.textDim, fontFamily: 'JetBrainsMono')),
      data: (models) {
        // Keep the current value selectable even if it's not in the fetched
        // list (offline fallback, or a model the account just lost access to).
        final items = <String>{...models, if (current.isNotEmpty) current}.toList()..sort();
        return DropdownButtonFormField<String>(
          key: ValueKey(current),
          initialValue: items.contains(current) ? current : (items.isNotEmpty ? items.first : null),
          isDense: true,
          style: TextStyle(fontFamily: 'JetBrainsMono', fontSize: 13, color: theme.textMain),
          decoration: const InputDecoration(
            isDense: true,
            contentPadding: EdgeInsets.symmetric(horizontal: 12, vertical: 10),
          ),
          items: [for (final m in items) DropdownMenuItem(value: m, child: Text(m))],
          onChanged: (m) {
            if (m == null || m == current) return;
            ref.read(googleAccountProvider.notifier).setModel(m);
          },
        );
      },
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
            onPressed: () {
              Clipboard.setData(ClipboardData(text: url));
              ScaffoldMessenger.of(context).showSnackBar(
                SnackBar(content: Text(L10n.t('copied')), duration: const Duration(seconds: 1)),
              );
            },
          ),
        ],
      ),
    );
  }
}
