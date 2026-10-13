import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/friendly_error.dart';
import '../../../core/l10n.dart';
import '../../../core/theme.dart';
import '../../../models/provider_config.dart';
import '../../../providers/auth_gate_provider.dart';
import '../../../providers/chat_provider.dart';
import '../../../providers/gate_guard.dart';
import '../../../providers/provider_provider.dart';

/// State of the ChatGPT / claude.ai web chats (Beta), from /api/webchat.
///
/// Ambient-safe like every settings provider: while the auth gate is up or the
/// request fails it answers "nothing known" instead of raising, so this tab
/// never toasts an error the person did not ask for.
final webChatProvider = FutureProvider.autoDispose<Map<String, dynamic>>((ref) async {
  // watch, not read: the gate answers a moment after the app starts, and a
  // value cached from before that would stay empty for the whole session.
  if (authGateBlocked(ref.watch(authGateProvider).valueOrNull)) return const {};
  try {
    return await ref.read(apiClientProvider).getWebChat();
  } catch (e) {
    debugPrint('webchat: state error: ${FriendlyError.describeGeneric(e)}');
    return const {};
  }
});

/// Provider types and the model id each one is created with. The page on the
/// site picks its own model; this id only names the entry in the picker.
const _siteProvider = {
  'chatgpt': ('web-chatgpt', 'chatgpt-web'),
  'claude': ('web-claude', 'claude-web'),
};

/// Settings → Web chat (Beta). Use your own ChatGPT / Claude web chat from
/// Memo instead of an API. Text only; Memo drives a real browser window you
/// sign in to yourself. Backed by internal/webchat and /api/webchat.
class WebChatTab extends ConsumerStatefulWidget {
  const WebChatTab({super.key});

  @override
  ConsumerState<WebChatTab> createState() => _WebChatTabState();
}

class _WebChatTabState extends ConsumerState<WebChatTab> {
  Timer? _poll;
  String? _busySite;

  @override
  void initState() {
    super.initState();
    // Sign-in happens in another window: while a connect is waiting for the
    // person, keep the status current. Idle, nothing is polled.
    _poll = Timer.periodic(const Duration(seconds: 2), (_) {
      if (!mounted) return;
      final sites = ref.read(webChatProvider).valueOrNull?['sites'];
      final waiting = sites is List && sites.any((s) => s is Map && s['connecting'] == true);
      if (waiting) ref.invalidate(webChatProvider);
    });
  }

  @override
  void dispose() {
    _poll?.cancel();
    super.dispose();
  }

  Future<void> _act(String action, String site, {bool forget = false}) async {
    if (_busySite != null) return;
    setState(() => _busySite = site);
    try {
      await ref.read(apiClientProvider).webChatAction(action, site, forget: forget);
      ref.invalidate(webChatProvider);
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('${L10n.t('error')}: ${FriendlyError.describeGeneric(e)}')),
        );
      }
    } finally {
      if (mounted) setState(() => _busySite = null);
    }
  }

  Future<void> _enable(String site, String name) async {
    final accepted = await showDialog<bool>(
      context: context,
      builder: (_) => _NoticeDialog(siteName: name),
    );
    if (accepted == true) await _act('consent', site);
  }

  Future<void> _addModel(String site, String name) async {
    final (type, model) = _siteProvider[site]!;
    await ref.read(providerListProvider.notifier).updateProvider(
          ProviderConfig(
            type: type,
            name: '$name (Web)',
            model: model,
            enabled: true,
          ),
        );
  }

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    final state = ref.watch(webChatProvider).valueOrNull ?? const <String, dynamic>{};
    // The backend owns Beta; until it has answered, nothing is offered.
    final loaded = state.containsKey('beta');
    final beta = state['beta'] == true;
    final sites = (state['sites'] is List) ? (state['sites'] as List).whereType<Map>().toList() : const <Map>[];
    final providers = ref.watch(providerListProvider).valueOrNull ?? const <ProviderConfig>[];

    return ListView(
      padding: const EdgeInsets.all(32),
      children: [
        Text(
          L10n.t('tab_web_chat'),
          style: TextStyle(fontSize: 18, fontWeight: FontWeight.w700, color: theme.textMain),
        ),
        const SizedBox(height: 8),
        Text(
          L10n.t('web_chat_desc'),
          style: TextStyle(fontSize: 13, height: 1.45, color: theme.textDim),
        ),
        const SizedBox(height: 16),
        _Callout(text: L10n.t('web_chat_risk_short')),
        const SizedBox(height: 16),
        if (!loaded)
          const Center(child: Padding(padding: EdgeInsets.all(24), child: CircularProgressIndicator()))
        else if (!beta)
          _Callout(text: L10n.t('web_chat_beta_off'))
        else if (sites.isEmpty)
          const Center(child: Padding(padding: EdgeInsets.all(24), child: CircularProgressIndicator()))
        else
          for (final s in sites) ...[
            _SiteCard(
              site: Map<String, dynamic>.from(s),
              busy: _busySite == s['site'],
              hasProvider: providers.any((p) => p.type == (_siteProvider[s['site']]?.$1 ?? '')),
              onEnable: () => _enable('${s['site']}', '${s['name']}'),
              onRevoke: () => _act('revoke', '${s['site']}'),
              onConnect: () => _act('connect', '${s['site']}'),
              onDisconnect: ({bool forget = false}) => _act('disconnect', '${s['site']}', forget: forget),
              onAddModel: () => _addModel('${s['site']}', '${s['name']}'),
            ),
            const SizedBox(height: 12),
          ],
      ],
    );
  }
}

class _Callout extends StatelessWidget {
  final String text;
  const _Callout({required this.text});

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: MemoTheme.warningOrange.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: MemoTheme.warningOrange.withValues(alpha: 0.35)),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(Icons.info_outline, size: 18, color: MemoTheme.warningOrange),
          const SizedBox(width: 10),
          Expanded(
            child: Text(text, style: TextStyle(fontSize: 12, height: 1.4, color: theme.textSecondary)),
          ),
        ],
      ),
    );
  }
}

class _SiteCard extends StatelessWidget {
  final Map<String, dynamic> site;
  final bool busy;
  final bool hasProvider;
  final VoidCallback onEnable;
  final VoidCallback onRevoke;
  final VoidCallback onConnect;
  final void Function({bool forget}) onDisconnect;
  final VoidCallback onAddModel;

  const _SiteCard({
    required this.site,
    required this.busy,
    required this.hasProvider,
    required this.onEnable,
    required this.onRevoke,
    required this.onConnect,
    required this.onDisconnect,
    required this.onAddModel,
  });

  String _stateText() {
    if (site['installed'] == false) return L10n.t('web_chat_state_no_browser');
    if (site['connecting'] == true) return L10n.t('web_chat_state_connecting');
    switch ('${site['state']}') {
      case 'ready':
        return L10n.t('web_chat_state_ready');
      case 'login':
        return L10n.t('web_chat_state_login');
      case 'challenge':
        return L10n.t('web_chat_state_challenge');
      case 'idle':
        return L10n.t('web_chat_state_idle');
      default:
        return L10n.t('web_chat_state_unknown');
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    final consented = site['consented'] == true;
    final connecting = site['connecting'] == true;
    final error = '${site['error'] ?? ''}';
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: theme.bgPanel,
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: theme.borderSoft),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Text(
                '${site['name']}',
                style: TextStyle(fontSize: 14, fontWeight: FontWeight.w600, color: theme.textMain),
              ),
              const SizedBox(width: 8),
              if (site['verified'] != true)
                Tooltip(
                  message: L10n.t('web_chat_experimental_tip'),
                  child: Text(
                    L10n.t('web_chat_experimental'),
                    style: TextStyle(fontSize: 11, color: MemoTheme.warningOrange),
                  ),
                ),
            ],
          ),
          const SizedBox(height: 6),
          Text(
            consented ? _stateText() : L10n.t('web_chat_state_not_enabled'),
            style: TextStyle(fontSize: 12, color: theme.textDim),
          ),
          if (error.isNotEmpty) ...[
            const SizedBox(height: 6),
            Text(error, style: TextStyle(fontSize: 12, color: MemoTheme.red)),
          ],
          const SizedBox(height: 12),
          if (busy || connecting) const LinearProgressIndicator(minHeight: 2),
          if (busy || connecting) const SizedBox(height: 12),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              if (!consented)
                FilledButton(onPressed: busy ? null : onEnable, child: Text(L10n.t('web_chat_enable')))
              else ...[
                FilledButton(
                  onPressed: (busy || connecting) ? null : onConnect,
                  child: Text(L10n.t('web_chat_connect')),
                ),
                OutlinedButton(
                  onPressed: hasProvider ? null : onAddModel,
                  child: Text(hasProvider ? L10n.t('web_chat_added') : L10n.t('web_chat_add_model')),
                ),
                if (site['running'] == true || connecting)
                  TextButton(onPressed: () => onDisconnect(), child: Text(L10n.t('web_chat_disconnect'))),
                TextButton(onPressed: () => onDisconnect(forget: true), child: Text(L10n.t('web_chat_forget'))),
                TextButton(onPressed: busy ? null : onRevoke, child: Text(L10n.t('web_chat_revoke'))),
              ],
            ],
          ),
        ],
      ),
    );
  }
}

/// The legal notice. Accepting needs an explicit tick; there is no default.
class _NoticeDialog extends StatefulWidget {
  final String siteName;
  const _NoticeDialog({required this.siteName});

  @override
  State<_NoticeDialog> createState() => _NoticeDialogState();
}

class _NoticeDialogState extends State<_NoticeDialog> {
  bool _ack = false;

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: Text(L10n.t('web_chat_notice_title', {'site': widget.siteName})),
      content: SizedBox(
        width: 480,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(L10n.t('web_chat_notice_body', {'site': widget.siteName})),
              const SizedBox(height: 12),
              CheckboxListTile(
                value: _ack,
                onChanged: (v) => setState(() => _ack = v == true),
                title: Text(L10n.t('web_chat_notice_accept')),
                controlAffinity: ListTileControlAffinity.leading,
                contentPadding: EdgeInsets.zero,
                dense: true,
              ),
            ],
          ),
        ),
      ),
      actions: [
        TextButton(onPressed: () => Navigator.pop(context, false), child: Text(L10n.t('cancel'))),
        FilledButton(
          onPressed: _ack ? () => Navigator.pop(context, true) : null,
          child: Text(L10n.t('web_chat_notice_confirm')),
        ),
      ],
    );
  }
}
