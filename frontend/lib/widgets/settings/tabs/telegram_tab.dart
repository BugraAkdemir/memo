import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:url_launcher/url_launcher.dart';

import '../../../core/l10n.dart';
import '../../../core/theme.dart';
import '../../../models/telegram.dart';
import '../../../providers/telegram_provider.dart';

/// Settings → Telegram. A bot token from @BotFather stands in for
/// WhatsApp's QR pairing step — there's no multi-device session to log
/// into, just a token to validate and start long-polling with. Once
/// connected, the bot replies only to whichever Telegram chat messages it
/// first (see internal/app/telegram.go's shouldReplyToTelegram) — unlike a
/// personal WhatsApp account, a bot token is reachable by anyone who finds
/// it, so that owner lock is shown here as its own explicit state rather
/// than assumed.
class TelegramTab extends ConsumerStatefulWidget {
  const TelegramTab({super.key});

  @override
  ConsumerState<TelegramTab> createState() => _TelegramTabState();
}

class _TelegramTabState extends ConsumerState<TelegramTab> {
  final _tokenController = TextEditingController();
  bool _tokenVisible = false;
  bool _connecting = false;

  /// Held so dispose() can stop the poll without touching `ref` —
  /// ConsumerState.ref throws once the widget is unmounted.
  TelegramStatusNotifier? _tg;

  @override
  void initState() {
    super.initState();
    Future.microtask(() {
      if (!mounted) return;
      _tg = ref.read(telegramStatusProvider.notifier);
      _tg!.startPolling();
    });
  }

  @override
  void dispose() {
    _tg?.stopPolling();
    _tokenController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    final statusAsync = ref.watch(telegramStatusProvider);

    return ListView(
      padding: const EdgeInsets.all(32),
      children: [
        Text(
          L10n.t('tab_telegram'),
          style: TextStyle(fontSize: 18, fontWeight: FontWeight.w700, color: theme.textMain),
        ),
        const SizedBox(height: 8),
        Text(
          L10n.t('telegram_tab_desc'),
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
          data: (status) => status.configured ? _buildConnected(theme, status) : _buildSetup(theme),
        ),
      ],
    );
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

  /// One numbered row of the "how to connect" walkthrough — a small
  /// circular index badge + the step text, so a user who has never made a
  /// Telegram bot before can follow along without leaving this tab.
  Widget _buildStep(ThemeColors theme, int number, String text) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Container(
          width: 20,
          height: 20,
          alignment: Alignment.center,
          decoration: BoxDecoration(
            color: MemoTheme.accent.withValues(alpha: 0.14),
            shape: BoxShape.circle,
          ),
          child: Text(
            '$number',
            style: const TextStyle(fontSize: 11, fontWeight: FontWeight.w700, color: MemoTheme.accent),
          ),
        ),
        const SizedBox(width: 10),
        Expanded(
          child: Text(text, style: TextStyle(fontSize: 12.5, height: 1.4, color: theme.textMain)),
        ),
      ],
    );
  }

  Widget _buildSetup(ThemeColors theme) {
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
            child: const Icon(Icons.send_rounded, size: 20, color: MemoTheme.accent),
          ),
          const SizedBox(width: 14),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(L10n.t('telegram_empty_title'),
                    style: TextStyle(fontSize: 15, fontWeight: FontWeight.w700, color: theme.textMain)),
                const SizedBox(height: 4),
                Text(L10n.t('telegram_empty_desc'), style: TextStyle(fontSize: 12, color: theme.textDim)),
              ],
            ),
          ),
        ],
      ),
      const SizedBox(height: 18),
      _buildStep(theme, 1, L10n.t('telegram_setup_step_1')),
      const SizedBox(height: 10),
      _buildStep(theme, 2, L10n.t('telegram_setup_step_2')),
      const SizedBox(height: 10),
      _buildStep(theme, 3, L10n.t('telegram_setup_step_3')),
      const SizedBox(height: 16),
      TextButton.icon(
        onPressed: () => launchUrl(Uri.parse('https://t.me/BotFather'), mode: LaunchMode.externalApplication),
        icon: const Icon(Icons.open_in_new, size: 15),
        label: Text(L10n.t('telegram_open_botfather')),
        style: TextButton.styleFrom(padding: EdgeInsets.zero, alignment: Alignment.centerLeft),
      ),
      const SizedBox(height: 8),
      TextField(
        controller: _tokenController,
        obscureText: !_tokenVisible,
        style: TextStyle(fontSize: 13, color: theme.textMain),
        decoration: InputDecoration(
          hintText: L10n.t('telegram_token_hint'),
          hintStyle: TextStyle(fontSize: 12, color: theme.textMuted),
          filled: true,
          fillColor: theme.bgHover,
          contentPadding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
          border: OutlineInputBorder(
            borderRadius: BorderRadius.circular(8),
            borderSide: BorderSide(color: theme.borderSoft),
          ),
          suffixIcon: IconButton(
            icon: Icon(_tokenVisible ? Icons.visibility_off : Icons.visibility, size: 18, color: theme.textDim),
            onPressed: () => setState(() => _tokenVisible = !_tokenVisible),
          ),
        ),
      ),
      const SizedBox(height: 12),
      SizedBox(
        width: double.infinity,
        child: ElevatedButton.icon(
          onPressed: _connecting ? null : _handleConnect,
          icon: _connecting
              ? const SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
                )
              : const Icon(Icons.link_rounded, size: 18),
          label: Text(L10n.t('connect')),
          style: ElevatedButton.styleFrom(
            backgroundColor: MemoTheme.accent,
            foregroundColor: Colors.white,
            padding: const EdgeInsets.symmetric(vertical: 12),
          ),
        ),
      ),
    ]);
  }

  Widget _buildConnected(ThemeColors theme, TelegramStatus status) {
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
                  status.botUsername.isNotEmpty ? '@${status.botUsername}' : L10n.t('connections_status_connected'),
                  style: TextStyle(fontSize: 14, fontWeight: FontWeight.w600, color: theme.textMain),
                ),
              ),
              if (!status.connected && !status.reconnecting)
                TextButton.icon(
                  onPressed: () => ref.read(telegramStatusProvider.notifier).reconnect(),
                  icon: const Icon(Icons.refresh, size: 16),
                  label: Text(L10n.t('reconnect')),
                ),
              TextButton.icon(
                onPressed: () => ref.read(telegramStatusProvider.notifier).stop(),
                icon: Icon(Icons.pause_circle_outline, size: 16, color: theme.textDim),
                label: Text(L10n.t('telegram_pause'), style: TextStyle(color: theme.textDim)),
              ),
              TextButton.icon(
                onPressed: _confirmDisconnect,
                icon: const Icon(Icons.logout, size: 16, color: MemoTheme.red),
                label: Text(L10n.t('logout'), style: const TextStyle(color: MemoTheme.red)),
              ),
            ],
          ),
          if (status.lastError.isNotEmpty) ...[
            const SizedBox(height: 10),
            Text(status.lastError, style: TextStyle(color: MemoTheme.warningOrange, fontSize: 12)),
          ],
        ]),
        const SizedBox(height: 12),
        _buildCard(theme, children: [
          Row(
            children: [
              Icon(
                status.ownerLinked ? Icons.person_rounded : Icons.person_search_rounded,
                size: 18,
                color: status.ownerLinked ? MemoTheme.accent : theme.textDim,
              ),
              const SizedBox(width: 10),
              Expanded(
                child: Text(
                  status.ownerLinked
                      ? '${L10n.t('telegram_owner_linked')}: ${status.ownerName}'
                      : L10n.t('telegram_owner_waiting'),
                  style: TextStyle(fontSize: 13, fontWeight: FontWeight.w600, color: theme.textMain),
                ),
              ),
            ],
          ),
          if (!status.ownerLinked) ...[
            const SizedBox(height: 6),
            Text(
              L10n.t('telegram_owner_waiting_desc'),
              style: TextStyle(fontSize: 12, color: theme.textDim),
            ),
          ],
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
        onPressed: () => ref.read(telegramStatusProvider.notifier).refresh(),
        icon: const Icon(Icons.refresh, size: 16),
        label: Text(L10n.t('retry')),
      ),
    ]);
  }

  Future<void> _handleConnect() async {
    final token = _tokenController.text.trim();
    if (token.isEmpty) return;
    setState(() => _connecting = true);
    await ref.read(telegramStatusProvider.notifier).connect(token);
    if (mounted) {
      setState(() => _connecting = false);
      _tokenController.clear();
    }
  }

  void _confirmDisconnect() {
    final theme = MemoTheme.of(context);
    showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        backgroundColor: theme.bgPanel,
        title: Text(L10n.t('telegram_disconnect_title'), style: TextStyle(color: theme.textMain, fontSize: 16)),
        content: Text(
          L10n.t('telegram_disconnect_desc'),
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
              ref.read(telegramStatusProvider.notifier).disconnect();
            },
            child: Text(L10n.t('logout'), style: const TextStyle(color: MemoTheme.red)),
          ),
        ],
      ),
    );
  }
}
