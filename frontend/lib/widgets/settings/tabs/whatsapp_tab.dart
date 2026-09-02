import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:qr_flutter/qr_flutter.dart';

import '../../../core/l10n.dart';
import '../../../core/theme.dart';
import '../../../models/whatsapp.dart';
import '../../../providers/whatsapp_provider.dart';

/// Settings → WhatsApp. Connection management only (pair/reconnect/logout +
/// the self-chat assistant toggle) — replaces the old dedicated WhatsApp
/// nav tab and its full chat/message-browsing UI, which was both redundant
/// (actually sending/reading messages already works from normal chat via
/// the agent's whatsapp_send tool, and directly from WhatsApp itself once
/// the self-chat assistant is on) and the source of a real bug: as a
/// pushed/popped route it was mounted and disposed on every visit, and a
/// fast navigate-in/navigate-out could dispose the screen while its
/// WhatsAppStatusNotifier.startPolling() microtask was still in flight —
/// "Cannot use 'ref' after the widget was disposed" — and, separately,
/// notifying a since-defunct listener could crash out of the polling
/// Timer's callback entirely, permanently killing the recursive poll (the
/// user's reported "stuck on Preparing QR code forever"). A settings tab
/// still gets opened/closed, but far less rapidly than a main nav item —
/// and the same startPolling/stopPolling lifecycle here is now guarded
/// with a mounted check, closing the race regardless.
class WhatsAppTab extends ConsumerStatefulWidget {
  const WhatsAppTab({super.key});

  @override
  ConsumerState<WhatsAppTab> createState() => _WhatsAppTabState();
}

class _WhatsAppTabState extends ConsumerState<WhatsAppTab> {
  /// Held so dispose() can stop the poll without touching `ref` —
  /// ConsumerState.ref throws once the widget is unmounted.
  WhatsAppStatusNotifier? _wa;

  @override
  void initState() {
    super.initState();
    Future.microtask(() {
      if (!mounted) return;
      _wa = ref.read(whatsAppStatusProvider.notifier);
      _wa!.startPolling();
    });
  }

  @override
  void dispose() {
    _wa?.stopPolling();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    final statusAsync = ref.watch(whatsAppStatusProvider);

    return ListView(
      padding: const EdgeInsets.all(32),
      children: [
        Text(
          L10n.t('tab_whatsapp'),
          style: TextStyle(fontSize: 18, fontWeight: FontWeight.w700, color: theme.textMain),
        ),
        const SizedBox(height: 8),
        Text(
          L10n.t('whatsapp_tab_desc'),
          style: TextStyle(fontSize: 13, height: 1.45, color: theme.textDim),
        ),
        const SizedBox(height: 24),
        statusAsync.when(
          loading: () => const Center(
            child: Padding(
              padding: EdgeInsets.all(24),
              child: CircularProgressIndicator(strokeWidth: 2, color: MemoTheme.accent),
            ),
          ),
          error: (e, _) => _buildError(theme, e.toString()),
          data: (status) => _buildStatusBody(theme, status),
        ),
      ],
    );
  }

  Widget _buildStatusBody(ThemeColors theme, WhatsAppStatus status) {
    if (!status.initialized) return _buildWelcome(theme);
    if (!status.loggedIn) {
      return status.qrCodes.isNotEmpty ? _buildQR(theme, status) : _buildConnecting(theme, status);
    }
    return _buildConnected(theme, status);
  }

  Widget _buildCard(ThemeColors theme, {required List<Widget> children}) {
    return Container(
      padding: const EdgeInsets.all(20),
      decoration: BoxDecoration(
        color: theme.bgPanel,
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: theme.borderSoft),
      ),
      child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: children),
    );
  }

  Widget _buildWelcome(ThemeColors theme) {
    return _buildCard(theme, children: [
      Row(
        children: [
          Container(
            width: 44,
            height: 44,
            decoration: BoxDecoration(
              color: MemoTheme.accent.withValues(alpha: 0.12),
              borderRadius: BorderRadius.circular(12),
            ),
            child: const Icon(Icons.message_rounded, size: 22, color: MemoTheme.accent),
          ),
          const SizedBox(width: 14),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(L10n.t('whatsapp_empty_title'),
                    style: TextStyle(fontSize: 15, fontWeight: FontWeight.w700, color: theme.textMain)),
                const SizedBox(height: 4),
                Text(L10n.t('whatsapp_empty_desc'), style: TextStyle(fontSize: 12, color: theme.textDim)),
              ],
            ),
          ),
        ],
      ),
      const SizedBox(height: 16),
      SizedBox(
        width: double.infinity,
        child: ElevatedButton.icon(
          onPressed: () => ref.read(whatsAppStatusProvider.notifier).connect(),
          icon: const Icon(Icons.link_rounded, size: 18),
          label: Text(L10n.t('launchpad_connect_wa')),
          style: ElevatedButton.styleFrom(
            backgroundColor: MemoTheme.accent,
            foregroundColor: Colors.white,
            padding: const EdgeInsets.symmetric(vertical: 12),
          ),
        ),
      ),
      const SizedBox(height: 10),
      Text(
        L10n.t('uses_whatsapp_web_your_phone_must_stay_online'),
        style: TextStyle(fontSize: 11, color: theme.textMuted),
      ),
    ]);
  }

  Widget _buildConnecting(ThemeColors theme, WhatsAppStatus status) {
    return _buildCard(theme, children: [
      const Center(child: CircularProgressIndicator(strokeWidth: 2, color: MemoTheme.accent)),
      const SizedBox(height: 14),
      Center(
        child: Text(L10n.t('preparing_qr_code'), style: TextStyle(color: theme.textDim, fontSize: 13)),
      ),
      if (status.lastError.isNotEmpty) ...[
        const SizedBox(height: 10),
        Center(
          child: Text(status.lastError, style: TextStyle(color: MemoTheme.warningOrange, fontSize: 12)),
        ),
      ],
    ]);
  }

  Widget _buildQR(ThemeColors theme, WhatsAppStatus status) {
    return _buildCard(theme, children: [
      Text(L10n.t('link_whatsapp'),
          style: TextStyle(fontSize: 16, fontWeight: FontWeight.w700, color: theme.textMain)),
      const SizedBox(height: 6),
      Text(
        L10n.t('open_whatsapp_linked_devices_link_a_device_scan_qr'),
        style: TextStyle(fontSize: 12, color: theme.textDim),
      ),
      const SizedBox(height: 18),
      Center(
        child: Container(
          padding: const EdgeInsets.all(14),
          decoration: BoxDecoration(color: Colors.white, borderRadius: BorderRadius.circular(12)),
          child: QrImageView(
            data: status.qrCodes.first,
            version: QrVersions.auto,
            size: 180,
            backgroundColor: Colors.white,
          ),
        ),
      ),
      const SizedBox(height: 14),
      Row(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          const SizedBox(
            width: 14,
            height: 14,
            child: CircularProgressIndicator(strokeWidth: 2, color: MemoTheme.accent),
          ),
          const SizedBox(width: 10),
          Text(L10n.t('waiting_for_qr_scan'), style: TextStyle(fontSize: 12, color: theme.textDim)),
        ],
      ),
    ]);
  }

  Widget _buildConnected(ThemeColors theme, WhatsAppStatus status) {
    final selfChatEnabled = ref.watch(whatsAppSelfChatAssistantProvider).valueOrNull ?? false;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _buildCard(theme, children: [
          Row(
            children: [
              Container(
                width: 8,
                height: 8,
                decoration: BoxDecoration(
                  shape: BoxShape.circle,
                  color: status.reconnecting
                      ? MemoTheme.warningOrange
                      : status.connected
                          ? MemoTheme.green
                          : theme.textDim,
                ),
              ),
              const SizedBox(width: 10),
              Expanded(
                child: Text(
                  status.connected ? L10n.t('connections_status_connected') : L10n.t('disconnected_reconnecting'),
                  style: TextStyle(fontSize: 14, fontWeight: FontWeight.w600, color: theme.textMain),
                ),
              ),
              if (!status.connected && !status.reconnecting)
                TextButton.icon(
                  onPressed: () => ref.read(whatsAppStatusProvider.notifier).connect(),
                  icon: const Icon(Icons.refresh, size: 16),
                  label: Text(L10n.t('reconnect')),
                ),
              TextButton.icon(
                onPressed: _confirmLogout,
                icon: const Icon(Icons.logout, size: 16, color: MemoTheme.red),
                label: Text(L10n.t('logout'), style: const TextStyle(color: MemoTheme.red)),
              ),
            ],
          ),
        ]),
        const SizedBox(height: 12),
        _buildCard(theme, children: [
          Row(
            children: [
              Icon(Icons.smart_toy_outlined, size: 18, color: selfChatEnabled ? MemoTheme.accent : theme.textDim),
              const SizedBox(width: 10),
              Expanded(
                child: Text(
                  L10n.t('whatsapp_self_chat_assistant_title'),
                  style: TextStyle(fontSize: 14, fontWeight: FontWeight.w600, color: theme.textMain),
                ),
              ),
              Switch(
                value: selfChatEnabled,
                onChanged: (_) => ref.read(whatsAppSelfChatAssistantProvider.notifier).toggle(),
                activeThumbColor: MemoTheme.accent,
                inactiveThumbColor: theme.textDim,
                inactiveTrackColor: theme.bgHover,
                trackOutlineColor: WidgetStateProperty.all(theme.borderHover),
              ),
            ],
          ),
          const SizedBox(height: 6),
          Text(
            L10n.t('whatsapp_self_chat_assistant_desc'),
            style: TextStyle(fontSize: 12, color: theme.textDim),
          ),
        ]),
      ],
    );
  }

  Widget _buildError(ThemeColors theme, String message) {
    return _buildCard(theme, children: [
      Row(
        children: [
          Icon(Icons.error_outline, size: 20, color: theme.textDim),
          const SizedBox(width: 10),
          Expanded(
            child: Text(L10n.t('connection_error'), style: TextStyle(color: theme.textMain, fontSize: 14)),
          ),
        ],
      ),
      const SizedBox(height: 6),
      Text(message, style: TextStyle(color: theme.textDim, fontSize: 11)),
      const SizedBox(height: 10),
      TextButton.icon(
        onPressed: () => ref.read(whatsAppStatusProvider.notifier).refresh(),
        icon: const Icon(Icons.refresh, size: 16),
        label: Text(L10n.t('retry')),
      ),
    ]);
  }

  void _confirmLogout() {
    final theme = MemoTheme.of(context);
    showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        backgroundColor: theme.bgPanel,
        title: Text(L10n.t('logout_from_whatsapp'), style: TextStyle(color: theme.textMain, fontSize: 16)),
        content: Text(
          L10n.t('your_session_will_be_removed_you_ll_need_to_scan_a'),
          style: TextStyle(color: theme.textDim, fontSize: 13),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: Text(L10n.t('cancel'), style: TextStyle(color: theme.textDim)),
          ),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              ref.read(whatsAppStatusProvider.notifier).logout();
            },
            child: Text(L10n.t('logout'), style: const TextStyle(color: MemoTheme.red)),
          ),
        ],
      ),
    );
  }
}
