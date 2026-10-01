import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:url_launcher/url_launcher.dart';

import '../../../core/friendly_error.dart';
import '../../../core/l10n.dart';
import '../../../core/theme.dart';
import '../../../providers/provider_provider.dart';
import '../../../providers/settings_provider.dart';

/// Settings → Claude Subscription. Use the Claude Pro/Max plan the user already
/// pays for instead of a pay-per-token API key, both in Memo chat and on the
/// local /v1 gateway. Backed by /api/dev-gateway/claude-account and
/// internal/app/claudeauth.go.
///
/// The flow has two shapes and which one you get is decided by the machine,
/// not by you. If Claude Code (or a CLAUDE_CODE_OAUTH_TOKEN) is already signed
/// in here, Connect completes with no browser at all — that is the common case
/// for anyone who already lives in Claude Code. Otherwise Connect opens
/// Anthropic's hosted page, which *displays* a code rather than redirecting
/// back to us, and you paste it below. There is no polling here and no timeout
/// spinner: nothing is happening on the backend while you read the page.
class ClaudeSubscriptionTab extends ConsumerStatefulWidget {
  const ClaudeSubscriptionTab({super.key});

  @override
  ConsumerState<ClaudeSubscriptionTab> createState() => _ClaudeSubscriptionTabState();
}

class _ClaudeSubscriptionTabState extends ConsumerState<ClaudeSubscriptionTab> {
  bool _busy = false;
  String _authUrl = '';
  final TextEditingController _codeController = TextEditingController();

  @override
  void dispose() {
    _codeController.dispose();
    super.dispose();
  }

  Future<void> _connect() async {
    setState(() {
      _busy = true;
      _authUrl = '';
    });
    try {
      final attempt = await ref.read(claudeAccountProvider.notifier).connect();
      if (!mounted) return;

      if (attempt.connected) {
        // The local-login path. Nothing to paste, nothing to open.
        ref.invalidate(gatewayModelsProvider);
        ref.invalidate(providerListProvider);
        setState(() => _busy = false);
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(L10n.t('claude_account_adopted'))),
        );
        return;
      }

      setState(() => _authUrl = attempt.authUrl);
      if (attempt.authUrl.isNotEmpty) {
        // Best-effort: on a headless / minimal desktop launchUrl can fail; the
        // copyable link below is the fallback and is always shown.
        try {
          await launchUrl(Uri.parse(attempt.authUrl), mode: LaunchMode.externalApplication);
        } catch (e) {
          debugPrint('claude_subscription_tab: launchUrl failed, falling back to the copyable link: $e');
        }
      }
      if (mounted) setState(() => _busy = false);
    } catch (e) {
      if (mounted) {
        setState(() => _busy = false);
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(L10n.t('claude_account_error', {'e': FriendlyError.describeGeneric(e)}))),
        );
      }
    }
  }

  Future<void> _submitCode() async {
    final code = _codeController.text.trim();
    if (code.isEmpty) return;
    setState(() => _busy = true);
    try {
      await ref.read(claudeAccountProvider.notifier).complete(code);
      ref.invalidate(gatewayModelsProvider);
      ref.invalidate(providerListProvider);
      if (!mounted) return;
      _codeController.clear();
      setState(() {
        _busy = false;
        _authUrl = '';
      });
    } catch (e) {
      if (mounted) {
        setState(() => _busy = false);
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(L10n.t('claude_account_error', {'e': FriendlyError.describeGeneric(e)}))),
        );
      }
    }
  }

  Future<void> _disconnect() async {
    setState(() {
      _busy = true;
      _authUrl = '';
      _codeController.clear();
    });
    try {
      await ref.read(claudeAccountProvider.notifier).disconnect();
      ref.invalidate(gatewayModelsProvider);
      ref.invalidate(providerListProvider);
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(L10n.t('claude_account_error', {'e': FriendlyError.describeGeneric(e)}))),
        );
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    final stateAsync = ref.watch(claudeAccountProvider);

    return ListView(
      padding: const EdgeInsets.all(32),
      children: [
        Text(
          L10n.t('tab_claude_subscription'),
          style: TextStyle(fontSize: 18, fontWeight: FontWeight.w700, color: theme.textMain),
        ),
        const SizedBox(height: 8),
        Text(
          L10n.t('claude_account_connect_desc'),
          style: TextStyle(fontSize: 13, height: 1.45, color: theme.textDim),
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
                          L10n.t('claude_account_connected'),
                          style: TextStyle(fontSize: 13, color: theme.textMain),
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                    ],
                  ),
                  if (st.adoptedLocally) ...[
                    const SizedBox(height: 6),
                    Text(
                      L10n.t('claude_account_adopted'),
                      style: TextStyle(fontSize: 11, color: theme.textDim, height: 1.4),
                    ),
                  ],
                  const SizedBox(height: 6),
                  Text(
                    L10n.t('claude_account_usage_hint'),
                    style: TextStyle(fontSize: 11, color: theme.textDim, height: 1.4),
                  ),
                  const SizedBox(height: 16),
                  Text(
                    L10n.t('claude_account_model_label'),
                    style: TextStyle(fontSize: 12, fontWeight: FontWeight.w600, color: theme.textMain),
                  ),
                  const SizedBox(height: 6),
                  _ModelDropdown(current: st.model),
                  const SizedBox(height: 16),
                  Align(
                    alignment: Alignment.centerLeft,
                    child: OutlinedButton.icon(
                      onPressed: _busy ? null : _disconnect,
                      icon: const Icon(Icons.logout_rounded, size: 16),
                      label: Text(L10n.t('claude_account_disconnect_cta')),
                    ),
                  ),
                ] else ...[
                  // Wrap, not Row: Turkish runs longer than English and a Row
                  // of localized actions overflows in one language and not the
                  // other (see the auth-gate footer in AGENTS.md).
                  Wrap(
                    spacing: 12,
                    runSpacing: 8,
                    crossAxisAlignment: WrapCrossAlignment.center,
                    children: [
                      FilledButton.icon(
                        onPressed: _busy ? null : _connect,
                        icon: _busy
                            ? const SizedBox(width: 14, height: 14, child: CircularProgressIndicator(strokeWidth: 2))
                            : const Icon(Icons.link_rounded, size: 16),
                        label: Text(
                          _busy ? L10n.t('claude_account_connecting') : L10n.t('claude_account_connect_cta'),
                        ),
                      ),
                      if (_authUrl.isNotEmpty)
                        OutlinedButton.icon(
                          onPressed: _busy ? null : _submitCode,
                          icon: const Icon(Icons.key_rounded, size: 16),
                          label: Text(L10n.t('claude_account_paste_cta')),
                        ),
                    ],
                  ),
                  if (_authUrl.isNotEmpty) ...[
                    const SizedBox(height: 16),
                    Text(
                      L10n.t('claude_account_browser_needed'),
                      style: TextStyle(fontSize: 12, color: theme.textMain, height: 1.4),
                    ),
                    const SizedBox(height: 10),
                    Text(
                      L10n.t('claude_account_manual_link_hint'),
                      style: TextStyle(fontSize: 11, color: theme.textDim),
                    ),
                    const SizedBox(height: 4),
                    _CopyableLink(url: _authUrl),
                    const SizedBox(height: 16),
                    Text(
                      L10n.t('claude_account_paste_label'),
                      style: TextStyle(fontSize: 12, fontWeight: FontWeight.w600, color: theme.textMain),
                    ),
                    const SizedBox(height: 6),
                    TextField(
                      key: const Key('claude_account_code_field'),
                      controller: _codeController,
                      minLines: 1,
                      maxLines: 3,
                      style: TextStyle(fontFamily: 'JetBrainsMono', fontSize: 12, color: theme.textMain),
                      decoration: InputDecoration(
                        hintText: L10n.t('claude_account_paste_hint'),
                        hintStyle: TextStyle(fontSize: 11, color: theme.textDim),
                        isDense: true,
                        contentPadding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
                        border: OutlineInputBorder(borderRadius: BorderRadius.circular(8)),
                      ),
                      onSubmitted: (_) => _submitCode(),
                    ),
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

/// Model picker for the connected account — populated live from the account's
/// own GET /v1/models (claudeSubModelsProvider), so it lists exactly what the
/// subscription is entitled to rather than a list this app decided on. Falls
/// back to a plain display of the current model while loading or unavailable.
class _ModelDropdown extends ConsumerWidget {
  final String current;
  const _ModelDropdown({required this.current});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = MemoTheme.of(context);
    final modelsAsync = ref.watch(claudeSubModelsProvider);

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
            ref.read(claudeAccountProvider.notifier).setModel(m);
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