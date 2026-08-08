import 'dart:async';
import 'dart:io';

import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:qr_flutter/qr_flutter.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/whatsapp.dart';
import '../providers/whatsapp_provider.dart';
import '../providers/chat_provider.dart' show apiClientProvider;
import 'app_shell.dart';
import '../core/friendly_error.dart';

// This screen uses Memo's own palette so it sits naturally beside the other
// pages: the bronze accent for interactive/brand elements, and the muted theme
// green only for the live-connection dot (a near-universal "connected" cue).

class WhatsAppScreen extends ConsumerStatefulWidget {
  const WhatsAppScreen({super.key});

  @override
  ConsumerState<WhatsAppScreen> createState() => _WhatsAppScreenState();
}

class _WhatsAppScreenState extends ConsumerState<WhatsAppScreen> {
  String? _selectedJid;
  String _selectedName = '';
  final _sendCtrl = TextEditingController();
  final _scrollCtrl = ScrollController();
  Timer? _msgTimer;
  bool _sending = false;
  Duration _pollInterval = const Duration(seconds: 5);

  /// Messages shown instantly after the user hits send, before the backend
  /// round-trip + refetch completes. Cleared once the refetch (which already
  /// includes them — the backend saves synchronously) returns. Keyed nowhere:
  /// only the currently-selected chat can have pending sends.
  final List<WhatsAppMessage> _optimistic = [];

  @override
  void initState() {
    super.initState();
    Future.microtask(() => ref.read(whatsAppStatusProvider.notifier).startPolling());
  }

  @override
  void dispose() {
    _sendCtrl.dispose();
    _scrollCtrl.dispose();
    _msgTimer?.cancel();
    ref.read(whatsAppStatusProvider.notifier).stopPolling();
    super.dispose();
  }

  void _startMsgRefresh(String jid) {
    _msgTimer?.cancel();
    _pollInterval = const Duration(seconds: 5);
    _schedulePoll(jid);
  }

  void _schedulePoll(String jid) {
    _msgTimer = Timer(_pollInterval, () {
      if (!mounted || _selectedJid != jid) {
        _msgTimer?.cancel();
        return;
      }
      final state = ref.read(whatsAppMessagesProvider(jid));
      if (state.hasError) {
        // Exponential backoff on errors, capped at 30s (Duration has no clamp).
        const minInterval = Duration(seconds: 5);
        const maxInterval = Duration(seconds: 30);
        var next = _pollInterval * 2;
        if (next < minInterval) next = minInterval;
        if (next > maxInterval) next = maxInterval;
        _pollInterval = next;
      } else {
        _pollInterval = const Duration(seconds: 5);
      }
      ref.invalidate(whatsAppMessagesProvider(jid));
      _schedulePoll(jid);
    });
  }

  void _selectChat(String jid, String name) {
    setState(() {
      _selectedJid = jid;
      _selectedName = name;
    });
    _startMsgRefresh(jid);
  }

  void _deselectChat() {
    _msgTimer?.cancel();
    setState(() {
      _selectedJid = null;
      _selectedName = '';
    });
  }

  @override
  Widget build(BuildContext context) {
    ref.listen<int>(activeTabProvider, (prev, next) {
      if (next != 3) {
        _msgTimer?.cancel();
        _msgTimer = null;
      } else if (_selectedJid != null) {
        _startMsgRefresh(_selectedJid!);
      }
    });
    final c = MemoTheme.of(context);
    final statusAsync = ref.watch(whatsAppStatusProvider);

    return statusAsync.when(
      loading: () => _buildLoading(c),
      error: (e, _) => _buildError(c, e.toString()),
      data: (status) => _buildBody(c, status),
    );
  }

  // ─── State routing ────────────────────────────────────────────

  Widget _buildBody(ThemeColors c, WhatsAppStatus status) {
    // Not yet initialized → welcome screen
    if (!status.initialized) return _buildWelcome(c);
    // Initialized but not logged in → waiting for QR or QR ready
    if (!status.loggedIn) {
      return status.qrCodes.isNotEmpty ? _buildQR(c, status) : _buildConnecting(c, status);
    }
    // Logged in → main two-panel UI
    return _buildMain(c, status);
  }

  // ─── Welcome / connect ────────────────────────────────────────

  Widget _buildWelcome(ThemeColors c) {
    return Container(
      color: c.bgApp,
      child: Center(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 400),
          child: Padding(
            padding: const EdgeInsets.all(40),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Container(
                  width: 72,
                  height: 72,
                  decoration: BoxDecoration(
                    color: MemoTheme.accent.withValues(alpha: 0.12),
                    borderRadius: BorderRadius.circular(20),
                  ),
                  child: const Icon(Icons.message_rounded, size: 38, color: MemoTheme.accent),
                ),
                const SizedBox(height: 24),
                Text(
                  L10n.t('whatsapp_empty_title'),
                  style: TextStyle(
                    fontSize: 24,
                    fontWeight: FontWeight.w700,
                    color: c.textMain,
                  ),
                ),
                const SizedBox(height: 8),
                Text(
                  L10n.t('whatsapp_empty_desc'),
                  textAlign: TextAlign.center,
                  style: TextStyle(fontSize: 14, color: c.textDim),
                ),
                const SizedBox(height: 32),
                SizedBox(
                  width: double.infinity,
                  child: ElevatedButton.icon(
                    onPressed: () => ref.read(whatsAppStatusProvider.notifier).connect(),
                    icon: const Icon(Icons.link_rounded, size: 18),
                    label: Text(
                      L10n.t('launchpad_connect_wa'),
                    ),
                    style: ElevatedButton.styleFrom(
                      backgroundColor: MemoTheme.accent,
                      foregroundColor: Colors.white,
                      padding: const EdgeInsets.symmetric(vertical: 14),
                      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
                    ),
                  ),
                ),
                const SizedBox(height: 16),
                Text(
                  L10n.t('uses_whatsapp_web_your_phone_must_stay_online'),
                  style: TextStyle(fontSize: 11, color: c.textMuted),
                  textAlign: TextAlign.center,
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  // ─── Connecting / QR ─────────────────────────────────────────

  Widget _buildConnecting(ThemeColors c, WhatsAppStatus status) {
    return Container(
      color: c.bgApp,
      child: Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const CircularProgressIndicator(color: MemoTheme.accent, strokeWidth: 2),
            const SizedBox(height: 20),
            Text(
              L10n.t('preparing_qr_code'),
              style: TextStyle(color: c.textDim, fontSize: 14),
            ),
            if (status.lastError.isNotEmpty) ...[
              const SizedBox(height: 12),
              Text(
                status.lastError,
                style: TextStyle(color: MemoTheme.warningOrange, fontSize: 12),
              ),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildQR(ThemeColors c, WhatsAppStatus status) {
    return Container(
      color: c.bgApp,
      child: Center(
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(40),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(
                L10n.t('link_whatsapp'),
                style: TextStyle(
                  fontSize: 20,
                  fontWeight: FontWeight.w700,
                  color: c.textMain,
                ),
              ),
              const SizedBox(height: 12),
              Text(
                L10n.t('open_whatsapp_linked_devices_link_a_device_scan_qr'),
                style: TextStyle(fontSize: 13, color: c.textDim),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 28),
              if (status.qrCodes.isEmpty)
                Padding(
                  padding: const EdgeInsets.all(20),
                  child: CircularProgressIndicator(color: c.textDim),
                )
              else
                Container(
                padding: const EdgeInsets.all(16),
                decoration: BoxDecoration(
                  color: Colors.white,
                  borderRadius: BorderRadius.circular(16),
                  boxShadow: [
                    BoxShadow(
                      color: Colors.black.withValues(alpha: 0.15),
                      blurRadius: 20,
                      offset: const Offset(0, 4),
                    ),
                  ],
                ),
                child: QrImageView(
                  data: status.qrCodes.first,
                  version: QrVersions.auto,
                  size: 220,
                  backgroundColor: Colors.white,
                  eyeStyle: const QrEyeStyle(
                    eyeShape: QrEyeShape.square,
                    color: Colors.black,
                  ),
                  dataModuleStyle: const QrDataModuleStyle(
                    dataModuleShape: QrDataModuleShape.square,
                    color: Colors.black,
                  ),
                ),
              ),
              if (status.qrCodes.isNotEmpty) ...[
                const SizedBox(height: 20),
                Row(
                  mainAxisSize: MainAxisSize.min,
                children: [
                  const SizedBox(
                    width: 14,
                    height: 14,
                    child: CircularProgressIndicator(
                      strokeWidth: 2,
                      color: MemoTheme.accent,
                    ),
                  ),
                  const SizedBox(width: 10),
                  Text(
                    L10n.t('waiting_for_qr_scan'),
                    style: TextStyle(fontSize: 13, color: c.textDim),
                  ),
                ],
              ),
              ],
            ],
          ),
        ),
      ),
    );
  }

  // ─── Main two-panel UI ────────────────────────────────────────

  Widget _buildMain(ThemeColors c, WhatsAppStatus status) {
    return Row(
      children: [
        // Left panel — chat list
        SizedBox(
          width: 280,
          child: _buildChatListPanel(c, status),
        ),
        Container(width: 1, color: c.borderSoft),
        // Right panel — messages or placeholder
        Expanded(
          child: _selectedJid != null
              ? _buildMessagePanel(c, status)
              : _buildNoSelectionPanel(c),
        ),
      ],
    );
  }

  // ─── Chat list panel ──────────────────────────────────────────

  Widget _buildChatListPanel(ThemeColors c, WhatsAppStatus status) {
    return Column(
      children: [
        _buildChatListHeader(c, status),
        Expanded(child: _buildChatList(c)),
      ],
    );
  }

  Widget _buildChatListHeader(ThemeColors c, WhatsAppStatus status) {
    return Container(
      height: 52,
      padding: const EdgeInsets.symmetric(horizontal: 14),
      decoration: BoxDecoration(
        color: c.bgPanel,
        border: Border(bottom: BorderSide(color: c.borderSoft)),
      ),
      child: Row(
        children: [
          const Icon(Icons.message_rounded, size: 18, color: MemoTheme.accent),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              'WhatsApp',
              style: TextStyle(
                fontSize: 14,
                fontWeight: FontWeight.w700,
                color: c.textMain,
              ),
            ),
          ),
          // Connection status dot
          Container(
            width: 8,
            height: 8,
            margin: const EdgeInsets.only(right: 8),
            decoration: BoxDecoration(
              shape: BoxShape.circle,
              color: status.reconnecting
                  ? MemoTheme.warningOrange
                  : status.connected
                      ? MemoTheme.green
                      : c.textDim,
            ),
          ),
          // Reconnect button if disconnected
          if (!status.connected && !status.reconnecting)
            Tooltip(
              message: L10n.t('reconnect'),
              child: IconButton(
                icon: Icon(Icons.refresh, size: 16, color: c.textDim),
                onPressed: () => ref.read(whatsAppStatusProvider.notifier).connect(),
                padding: EdgeInsets.zero,
                constraints: const BoxConstraints(minWidth: 28, minHeight: 28),
              ),
            ),
          // Logout button
          Tooltip(
            message: L10n.t('logout'),
            child: IconButton(
              icon: Icon(Icons.logout, size: 16, color: c.textDim),
              onPressed: () => _confirmLogout(c),
              padding: EdgeInsets.zero,
              constraints: const BoxConstraints(minWidth: 28, minHeight: 28),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildChatList(ThemeColors c) {
    final chatsAsync = ref.watch(whatsAppChatsProvider);
    return chatsAsync.when(
      loading: () => const Center(
          child: CircularProgressIndicator(strokeWidth: 2, color: MemoTheme.accent)),
      error: (e, _) => _buildError(c, e.toString()),
      data: (chats) {
        if (chats.isEmpty) {
          return Center(
            child: Padding(
              padding: const EdgeInsets.all(24),
              child: Text(
                L10n.t('no_messages_yet_chats_will_appear_here_when_you_re'),
                textAlign: TextAlign.center,
                style: TextStyle(fontSize: 13, color: c.textDim),
              ),
            ),
          );
        }
        return ListView.builder(
          itemCount: chats.length,
          itemBuilder: (_, i) => _ChatTile(
            chat: chats[i],
            isSelected: _selectedJid == chats[i].jid,
            onTap: () => _selectChat(chats[i].jid, chats[i].displayName),
          ),
        );
      },
    );
  }

  // ─── Message panel ────────────────────────────────────────────

  Widget _buildNoSelectionPanel(ThemeColors c) {
    return Container(
      color: c.bgApp,
      child: Center(
        child: Text(
          L10n.t('select_a_conversation'),
          style: TextStyle(fontSize: 14, color: c.textMuted),
        ),
      ),
    );
  }

  Widget _buildMessagePanel(ThemeColors c, WhatsAppStatus status) {
    final msgsAsync = ref.watch(whatsAppMessagesProvider(_selectedJid!));

    return Column(
      children: [
        // Header
        Container(
          height: 52,
          padding: const EdgeInsets.symmetric(horizontal: 14),
          decoration: BoxDecoration(
            color: c.bgPanel,
            border: Border(bottom: BorderSide(color: c.borderSoft)),
          ),
          child: Row(
            children: [
              // Avatar — tap to enlarge + download
              _WaAvatar(
                jid: _selectedJid!,
                name: _selectedName,
                size: 32,
                onTap: () => showDialog<void>(
                  context: context,
                  builder: (_) => _AvatarPreviewDialog(
                    jid: _selectedJid!,
                    name: _selectedName,
                  ),
                ),
              ),
              const SizedBox(width: 10),
              Expanded(
                child: Text(
                  _selectedName.isNotEmpty ? _selectedName : _selectedJid!,
                  style: TextStyle(
                    fontSize: 14,
                    fontWeight: FontWeight.w600,
                    color: c.textMain,
                  ),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              // Refresh
              IconButton(
                icon: Icon(Icons.refresh, size: 16, color: c.textDim),
                onPressed: () => ref.invalidate(whatsAppMessagesProvider(_selectedJid!)),
                padding: EdgeInsets.zero,
                constraints: const BoxConstraints(minWidth: 28, minHeight: 28),
              ),
              // Back (deselect)
              IconButton(
                icon: Icon(Icons.close, size: 16, color: c.textDim),
                onPressed: _deselectChat,
                padding: EdgeInsets.zero,
                constraints: const BoxConstraints(minWidth: 28, minHeight: 28),
              ),
            ],
          ),
        ),

        // Message list
        Expanded(
          child: msgsAsync.when(
            loading: () => const Center(
                child: CircularProgressIndicator(strokeWidth: 2, color: MemoTheme.accent)),
            error: (e, _) => _buildError(c, e.toString()),
            data: (msgs) {
              // Backend returns newest-first (ORDER BY timestamp DESC). Pending
              // optimistic sends for this chat are newer still, so they go in
              // front. With reverse:true the list anchors at the bottom and
              // index 0 renders at the bottom — so a newest-first list puts the
              // newest message at the bottom, oldest at the top (normal chat).
              //
              // An optimistic entry is only removed from _optimistic once its
              // own sendWhatsApp() call resolves — but msgs can refresh from
              // the backend *before* that (e.g. a poll/invalidate elsewhere
              // while the send is still in flight, or the user switching away
              // and back to this chat via the IndexedStack-mounted screen).
              // If the backend already has the real message by then, showing
              // both is a visible duplicate bubble. Filter out any pending
              // entry that already has a matching real (fromMe) message so it
              // disappears as soon as the real one is visible, independent of
              // whether the original send call has returned yet.
              final realSentTexts = msgs
                  .where((m) => m.fromMe)
                  .map((m) => m.text)
                  .toSet();
              final pending = _optimistic
                  .where((m) =>
                      m.chatJid == _selectedJid &&
                      !realSentTexts.contains(m.text))
                  .toList();
              final combined = [...pending.reversed, ...msgs];

              if (combined.isEmpty) {
                return Center(
                  child: Text(
                    L10n.t('no_messages'),
                    style: TextStyle(color: c.textDim),
                  ),
                );
              }
              return LayoutBuilder(
                builder: (context, constraints) {
                  final maxBubble = constraints.maxWidth * 0.72;
                  return ListView.builder(
                    controller: _scrollCtrl,
                    padding:
                        const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
                    reverse: true,
                    itemCount: combined.length,
                    itemBuilder: (_, i) => _MessageBubble(
                      msg: combined[i],
                      maxWidth: maxBubble,
                    ),
                  );
                },
              );
            },
          ),
        ),

        // Input row
        if (status.connected && status.loggedIn)
          Container(
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: c.bgPanel,
              border: Border(top: BorderSide(color: c.borderSoft)),
            ),
            child: Row(
              children: [
                Expanded(
                  child: Container(
                    decoration: BoxDecoration(
                      color: c.bgElement,
                      borderRadius: BorderRadius.circular(22),
                      border: Border.all(color: c.borderSoft),
                    ),
                    child: TextField(
                      controller: _sendCtrl,
                      style: TextStyle(fontSize: 14, color: c.textMain),
                      decoration: InputDecoration(
                        hintText: L10n.t('message'),
                        hintStyle: TextStyle(color: c.textMuted, fontSize: 14),
                        border: InputBorder.none,
                        contentPadding: const EdgeInsets.symmetric(
                            horizontal: 16, vertical: 10),
                      ),
                      onSubmitted: (_) => _sendMessage(),
                    ),
                  ),
                ),
                const SizedBox(width: 8),
                AnimatedContainer(
                  duration: const Duration(milliseconds: 120),
                  decoration: const BoxDecoration(
                    color: MemoTheme.accent,
                    shape: BoxShape.circle,
                  ),
                  child: IconButton(
                    icon: _sending
                        ? const SizedBox(
                            width: 18,
                            height: 18,
                            child: CircularProgressIndicator(
                              strokeWidth: 2,
                              color: Colors.white,
                            ),
                          )
                        : const Icon(Icons.send_rounded,
                            color: Colors.white, size: 18),
                    onPressed: _sending ? null : _sendMessage,
                    padding: const EdgeInsets.all(10),
                    constraints: const BoxConstraints(),
                  ),
                ),
              ],
            ),
          ),
        if (!status.connected)
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
            color: MemoTheme.warningOrange.withValues(alpha: 0.1),
            child: Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                Icon(Icons.wifi_off, size: 14, color: MemoTheme.warningOrange),
                const SizedBox(width: 6),
                Text(
                  L10n.t('disconnected_reconnecting'),
                  style: TextStyle(
                      fontSize: 12, color: MemoTheme.warningOrange),
                ),
              ],
            ),
          ),
      ],
    );
  }

  // ─── Helpers ──────────────────────────────────────────────────

  Future<void> _sendMessage() async {
    final text = _sendCtrl.text.trim();
    final jid = _selectedJid;
    if (text.isEmpty || jid == null || _sending) return;

    // Show the message instantly so it never feels like it "sends late".
    final optimistic = WhatsAppMessage(
      id: 'optimistic_${DateTime.now().microsecondsSinceEpoch}',
      chatJid: jid,
      senderJid: '',
      senderName: 'Ben',
      text: text,
      timestamp: DateTime.now(),
      fromMe: true,
    );
    setState(() {
      _sending = true;
      _optimistic.add(optimistic);
    });
    _sendCtrl.clear();

    try {
      await ref.read(apiClientProvider).sendWhatsApp(jid, text);
      // Backend saved the message synchronously, so the refetch will include
      // it — drop the optimistic copy to avoid showing it twice. Remove it
      // from the underlying list unconditionally: if the widget navigated
      // away (or the user switched chats) before this resolved, `mounted`
      // would be false and a guarded `setState` would skip the removal,
      // leaving a ghost entry in `_optimistic` that reappears whenever the
      // user comes back to this chat. Only the UI-facing bits (setState,
      // provider invalidation, showing a SnackBar) need to be gated on
      // `mounted`.
      _optimistic.remove(optimistic);
      if (mounted) {
        setState(() {});
        ref.invalidate(whatsAppMessagesProvider(jid));
        ref.invalidate(whatsAppChatsProvider);
      }
    } catch (e) {
      // Send failed — remove the optimistic bubble (regardless of mounted,
      // see comment above) and restore the text if we're still around to
      // show it.
      _optimistic.remove(optimistic);
      if (mounted) {
        setState(() {
          if (_sendCtrl.text.isEmpty) _sendCtrl.text = text;
        });
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(L10n.t('failed_to_send_e', {'e': FriendlyError.describeGeneric(e)})),
            backgroundColor: MemoTheme.red,
            behavior: SnackBarBehavior.floating,
          ),
        );
      }
    } finally {
      if (mounted) setState(() => _sending = false);
    }
  }

  void _confirmLogout(ThemeColors c) {
    showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        backgroundColor: c.bgPanel,
        title: Text(
          L10n.t('logout_from_whatsapp'),
          style: TextStyle(color: c.textMain, fontSize: 16),
        ),
        content: Text(
          L10n.t('your_session_will_be_removed_you_ll_need_to_scan_a'),
          style: TextStyle(color: c.textDim, fontSize: 13),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: Text(L10n.t('cancel'),
                style: TextStyle(color: c.textDim)),
          ),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              _deselectChat(); // stop the stale per-chat message poll timer
              ref.read(whatsAppStatusProvider.notifier).logout();
            },
            child: Text(L10n.t('logout'),
                style: const TextStyle(color: MemoTheme.red)),
          ),
        ],
      ),
    );
  }

  Widget _buildLoading(ThemeColors c) => Container(
        color: c.bgApp,
        child: const Center(
            child: CircularProgressIndicator(strokeWidth: 2, color: MemoTheme.accent)),
      );

  Widget _buildError(ThemeColors c, String message) => Container(
        color: c.bgApp,
        child: Center(
          child: Padding(
            padding: const EdgeInsets.all(32),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(Icons.error_outline, size: 40, color: c.textDim),
                const SizedBox(height: 12),
                Text(
                  L10n.t('connection_error'),
                  style: TextStyle(color: c.textMain, fontSize: 15),
                ),
                const SizedBox(height: 6),
                Text(
                  message,
                  style: TextStyle(color: c.textDim, fontSize: 11),
                  textAlign: TextAlign.center,
                ),
                const SizedBox(height: 16),
                TextButton.icon(
                  onPressed: () => ref.read(whatsAppStatusProvider.notifier).refresh(),
                  icon: const Icon(Icons.refresh, size: 16),
                  label: Text(L10n.t('retry')),
                ),
              ],
            ),
          ),
        ),
      );
}

// ─── Chat tile ────────────────────────────────────────────────

class _ChatTile extends StatelessWidget {
  final WhatsAppChatSummary chat;
  final bool isSelected;
  final VoidCallback onTap;

  const _ChatTile({
    required this.chat,
    required this.isSelected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    return MouseRegion(
      cursor: SystemMouseCursors.click,
      child: GestureDetector(
        onTap: onTap,
        child: AnimatedContainer(
          duration: const Duration(milliseconds: 120),
          color: isSelected
              ? MemoTheme.accent.withValues(alpha: 0.1)
              : Colors.transparent,
          child: Container(
            decoration: BoxDecoration(
              border: Border(
                left: BorderSide(
                  color: isSelected ? MemoTheme.accent : Colors.transparent,
                  width: 2,
                ),
              ),
            ),
            padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
            child: Row(
              children: [
                _WaAvatar(
                  jid: chat.jid,
                  name: chat.displayName.isNotEmpty
                      ? chat.displayName
                      : _jidToPhone(chat.jid),
                  size: 38,
                ),
                const SizedBox(width: 10),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          Expanded(
                            child: Text(
                              chat.displayName.isNotEmpty
                                  ? chat.displayName
                                  : _jidToPhone(chat.jid),
                              style: TextStyle(
                                fontSize: 13,
                                fontWeight: FontWeight.w600,
                                color: isSelected ? MemoTheme.accent : c.textMain,
                              ),
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                            ),
                          ),
                          Text(
                            _timeLabel(chat.lastTime),
                            style: TextStyle(
                                fontSize: 10, color: c.textMuted),
                          ),
                        ],
                      ),
                      const SizedBox(height: 2),
                      Text(
                        chat.lastMessage,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: TextStyle(fontSize: 11, color: c.textDim),
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  static String _jidToPhone(String jid) {
    final parts = jid.split('@');
    return parts.first;
  }

  static String _timeLabel(DateTime dt) {
    final now = DateTime.now();
    final diff = now.difference(dt);
    if (diff.inMinutes < 1) return 'şimdi';
    if (diff.inHours < 1) return '${diff.inMinutes}d';
    if (diff.inDays < 1) {
      return '${dt.hour.toString().padLeft(2, '0')}:${dt.minute.toString().padLeft(2, '0')}';
    }
    return '${dt.day}/${dt.month}';
  }
}

// ─── Avatar ───────────────────────────────────────────────────

/// Round avatar that shows the contact/group WhatsApp profile photo (fetched
/// and cached by the backend) and falls back to a bronze letter badge — Memo's
/// own avatar style — whenever there is no photo, it is still loading, or the
/// request fails.
class _WaAvatar extends ConsumerWidget {
  final String jid;
  final String name;
  final double size;
  final VoidCallback? onTap;
  const _WaAvatar({
    required this.jid,
    required this.name,
    this.size = 38,
    this.onTap,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final letter = name.isNotEmpty ? name[0].toUpperCase() : '?';
    final fallback = Container(
      width: size,
      height: size,
      decoration: BoxDecoration(
        color: MemoTheme.accentPale,
        shape: BoxShape.circle,
        border: Border.all(color: MemoTheme.accent.withValues(alpha: 0.3)),
      ),
      alignment: Alignment.center,
      child: Text(
        letter,
        style: TextStyle(
          color: MemoTheme.accent,
          fontWeight: FontWeight.w700,
          fontSize: size * 0.42,
        ),
      ),
    );

    if (jid.isEmpty) return fallback;

    final url = ref.read(apiClientProvider).whatsAppAvatarUrl(jid);
    Widget avatar = ClipOval(
      child: Image.network(
        url,
        width: size,
        height: size,
        fit: BoxFit.cover,
        gaplessPlayback: true,
        // Show the letter badge while loading and on any error (no photo → 404).
        loadingBuilder: (context, child, progress) =>
            progress == null ? child : fallback,
        errorBuilder: (_, _, _) => fallback,
      ),
    );
    if (onTap != null) {
      avatar = MouseRegion(
        cursor: SystemMouseCursors.click,
        child: GestureDetector(onTap: onTap, child: avatar),
      );
    }
    return avatar;
  }
}

/// Full-screen-ish dialog that shows the full-resolution profile photo with a
/// download button. Opened by tapping the header avatar. Falls back to the
/// bronze letter badge if the contact has no photo.
class _AvatarPreviewDialog extends ConsumerStatefulWidget {
  final String jid;
  final String name;
  const _AvatarPreviewDialog({required this.jid, required this.name});

  @override
  ConsumerState<_AvatarPreviewDialog> createState() =>
      _AvatarPreviewDialogState();
}

class _AvatarPreviewDialogState extends ConsumerState<_AvatarPreviewDialog> {
  bool _downloading = false;

  Future<void> _download() async {
    setState(() => _downloading = true);
    try {
      final bytes = await ref
          .read(apiClientProvider)
          .fetchWhatsAppAvatarBytes(widget.jid, full: true);
      final safeName = widget.name.isNotEmpty
          ? widget.name.replaceAll(RegExp(r'[^\w\s-]'), '').trim()
          : 'whatsapp';
      final path = await FilePicker.platform.saveFile(
        dialogTitle: L10n.t('save_profile_photo'),
        fileName: '${safeName.isEmpty ? 'whatsapp' : safeName}_profile.jpg',
      );
      if (path == null) return; // user cancelled
      final outPath =
          path.toLowerCase().endsWith('.jpg') ? path : '$path.jpg';
      await File(outPath).writeAsBytes(bytes);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(L10n.t('photo_saved')),
            backgroundColor: MemoTheme.green,
            behavior: SnackBarBehavior.floating,
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(L10n.t('download_failed_e', {'e': FriendlyError.describeGeneric(e)})),
            backgroundColor: MemoTheme.red,
            behavior: SnackBarBehavior.floating,
          ),
        );
      }
    } finally {
      if (mounted) setState(() => _downloading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final url = ref.read(apiClientProvider).whatsAppAvatarUrl(widget.jid, full: true);

    final letterFallback = Container(
      width: 280,
      height: 280,
      color: c.bgElement,
      alignment: Alignment.center,
      child: Text(
        widget.name.isNotEmpty ? widget.name[0].toUpperCase() : '?',
        style: const TextStyle(
          color: MemoTheme.accent,
          fontWeight: FontWeight.w700,
          fontSize: 96,
        ),
      ),
    );

    return Dialog(
      backgroundColor: c.bgPanel,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(MemoTheme.radiusLg),
      ),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 360),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            // Header: name + close
            Padding(
              padding: const EdgeInsets.fromLTRB(20, 16, 12, 8),
              child: Row(
                children: [
                  Expanded(
                    child: Text(
                      widget.name.isNotEmpty ? widget.name : widget.jid,
                      style: TextStyle(
                        fontSize: 15,
                        fontWeight: FontWeight.w600,
                        color: c.textMain,
                      ),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
                  IconButton(
                    icon: Icon(Icons.close, size: 18, color: c.textDim),
                    onPressed: () => Navigator.of(context).pop(),
                  ),
                ],
              ),
            ),
            // Photo
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 20),
              child: ClipRRect(
                borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                child: Image.network(
                  url,
                  width: 320,
                  fit: BoxFit.contain,
                  gaplessPlayback: true,
                  loadingBuilder: (context, child, progress) => progress == null
                      ? child
                      : SizedBox(
                          width: 280,
                          height: 280,
                          child: Center(
                            child: CircularProgressIndicator(
                              strokeWidth: 2,
                              color: MemoTheme.accent,
                              value: progress.expectedTotalBytes != null
                                  ? progress.cumulativeBytesLoaded /
                                      progress.expectedTotalBytes!
                                  : null,
                            ),
                          ),
                        ),
                  errorBuilder: (_, _, _) => letterFallback,
                ),
              ),
            ),
            // Download button
            Padding(
              padding: const EdgeInsets.all(16),
              child: SizedBox(
                width: double.infinity,
                child: ElevatedButton.icon(
                  onPressed: _downloading ? null : _download,
                  icon: _downloading
                      ? const SizedBox(
                          width: 16,
                          height: 16,
                          child: CircularProgressIndicator(
                            strokeWidth: 2,
                            color: Colors.white,
                          ),
                        )
                      : const Icon(Icons.download_rounded, size: 18),
                  label: Text(L10n.t('download')),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

// ─── Message bubble ───────────────────────────────────────────

class _MessageBubble extends StatelessWidget {
  final WhatsAppMessage msg;
  final double maxWidth;
  const _MessageBubble({required this.msg, required this.maxWidth});

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final isMe = msg.fromMe;

    return Padding(
      padding: const EdgeInsets.only(bottom: 6),
      child: Align(
        alignment: isMe ? Alignment.centerRight : Alignment.centerLeft,
        child: ConstrainedBox(
          constraints: BoxConstraints(maxWidth: maxWidth),
          child: Container(
            padding: const EdgeInsets.fromLTRB(12, 8, 12, 6),
            decoration: BoxDecoration(
              color: isMe
                  ? MemoTheme.accent.withValues(alpha: 0.18)
                  : c.bgPanel,
              borderRadius: BorderRadius.only(
                topLeft: const Radius.circular(14),
                topRight: const Radius.circular(14),
                bottomLeft: Radius.circular(isMe ? 14 : 3),
                bottomRight: Radius.circular(isMe ? 3 : 14),
              ),
              border: isMe
                  ? null
                  : Border.all(color: c.borderSoft),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                if (!isMe && msg.senderName.isNotEmpty)
                  Padding(
                    padding: const EdgeInsets.only(bottom: 3),
                    child: Text(
                      msg.senderName,
                      style: const TextStyle(
                        fontSize: 11,
                        color: MemoTheme.accent,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ),
                Text(
                  msg.text,
                  style: TextStyle(fontSize: 13, color: c.textMain),
                ),
                const SizedBox(height: 3),
                Align(
                  alignment: Alignment.centerRight,
                  child: Text(
                    _timeLabel(msg.timestamp),
                    style: TextStyle(fontSize: 9, color: c.textMuted),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  static String _timeLabel(DateTime dt) {
    final now = DateTime.now();
    final diff = now.difference(dt);
    if (diff.inDays < 1) {
      return '${dt.hour.toString().padLeft(2, '0')}:${dt.minute.toString().padLeft(2, '0')}';
    }
    return '${dt.day}/${dt.month} ${dt.hour.toString().padLeft(2, '0')}:${dt.minute.toString().padLeft(2, '0')}';
  }
}
