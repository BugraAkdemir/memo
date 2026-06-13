import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:qr_flutter/qr_flutter.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/whatsapp.dart';
import '../providers/whatsapp_provider.dart';
import '../providers/chat_provider.dart' show apiClientProvider;

const _waGreen = Color(0xFF25D366);
const _waGreenDim = Color(0xFF1A9B4A);

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
    _msgTimer = Timer.periodic(const Duration(seconds: 5), (_) {
      if (mounted && _selectedJid == jid) {
        ref.invalidate(whatsAppMessagesProvider(jid));
      }
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
    final tr = L10n.locale == MemoLocale.tr;
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
                    color: _waGreen.withValues(alpha: 0.12),
                    borderRadius: BorderRadius.circular(20),
                  ),
                  child: const Icon(Icons.message_rounded, size: 38, color: _waGreen),
                ),
                const SizedBox(height: 24),
                Text(
                  'WhatsApp',
                  style: TextStyle(
                    fontSize: 24,
                    fontWeight: FontWeight.w700,
                    color: c.textMain,
                  ),
                ),
                const SizedBox(height: 8),
                Text(
                  tr
                      ? 'WhatsApp mesajlarını Memo üzerinden oku, yaz ve yapay zeka ile yönet.'
                      : 'Read, send, and manage your WhatsApp messages with AI assistance.',
                  textAlign: TextAlign.center,
                  style: TextStyle(fontSize: 14, color: c.textDim),
                ),
                const SizedBox(height: 32),
                SizedBox(
                  width: double.infinity,
                  child: ElevatedButton.icon(
                    onPressed: () => ref.read(whatsAppStatusProvider.notifier).connect(),
                    icon: const Icon(Icons.link_rounded, size: 18),
                    label: Text(tr ? 'WhatsApp\'a Bağlan' : 'Connect WhatsApp'),
                    style: ElevatedButton.styleFrom(
                      backgroundColor: _waGreen,
                      foregroundColor: Colors.white,
                      padding: const EdgeInsets.symmetric(vertical: 14),
                      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
                    ),
                  ),
                ),
                const SizedBox(height: 16),
                Text(
                  tr
                      ? 'WhatsApp Web protokolü — telefon çevrimiçi olmalıdır.'
                      : 'Uses WhatsApp Web — your phone must stay online.',
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
    final tr = L10n.locale == MemoLocale.tr;
    return Container(
      color: c.bgApp,
      child: Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const CircularProgressIndicator(color: _waGreen, strokeWidth: 2),
            const SizedBox(height: 20),
            Text(
              tr ? 'QR kodu hazırlanıyor...' : 'Preparing QR code...',
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
    final tr = L10n.locale == MemoLocale.tr;
    return Container(
      color: c.bgApp,
      child: Center(
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(40),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(
                tr ? 'WhatsApp\'ı Bağla' : 'Link WhatsApp',
                style: TextStyle(
                  fontSize: 20,
                  fontWeight: FontWeight.w700,
                  color: c.textMain,
                ),
              ),
              const SizedBox(height: 12),
              Text(
                tr
                    ? 'WhatsApp\'ı aç  →  Bağlı cihazlar  →  Cihaz ekle  →  QR\'ı okut'
                    : 'Open WhatsApp  →  Linked Devices  →  Link a Device  →  Scan QR',
                style: TextStyle(fontSize: 13, color: c.textDim),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 28),
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
              const SizedBox(height: 20),
              Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  const SizedBox(
                    width: 14,
                    height: 14,
                    child: CircularProgressIndicator(
                      strokeWidth: 2,
                      color: _waGreen,
                    ),
                  ),
                  const SizedBox(width: 10),
                  Text(
                    tr ? 'QR taranıyor bekleniyor...' : 'Waiting for QR scan...',
                    style: TextStyle(fontSize: 13, color: c.textDim),
                  ),
                ],
              ),
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
    final tr = L10n.locale == MemoLocale.tr;
    return Container(
      height: 52,
      padding: const EdgeInsets.symmetric(horizontal: 14),
      decoration: BoxDecoration(
        color: c.bgPanel,
        border: Border(bottom: BorderSide(color: c.borderSoft)),
      ),
      child: Row(
        children: [
          const Icon(Icons.message_rounded, size: 18, color: _waGreen),
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
                      ? _waGreen
                      : c.textDim,
            ),
          ),
          // Reconnect button if disconnected
          if (!status.connected && !status.reconnecting)
            Tooltip(
              message: tr ? 'Yeniden bağlan' : 'Reconnect',
              child: IconButton(
                icon: Icon(Icons.refresh, size: 16, color: c.textDim),
                onPressed: () => ref.read(whatsAppStatusProvider.notifier).connect(),
                padding: EdgeInsets.zero,
                constraints: const BoxConstraints(minWidth: 28, minHeight: 28),
              ),
            ),
          // Logout button
          Tooltip(
            message: tr ? 'Çıkış yap' : 'Logout',
            child: IconButton(
              icon: Icon(Icons.logout, size: 16, color: c.textDim),
              onPressed: () => _confirmLogout(c, tr),
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
          child: CircularProgressIndicator(strokeWidth: 2, color: _waGreen)),
      error: (e, _) => _buildError(c, e.toString()),
      data: (chats) {
        if (chats.isEmpty) {
          return Center(
            child: Padding(
              padding: const EdgeInsets.all(24),
              child: Text(
                L10n.locale == MemoLocale.tr
                    ? 'Henüz mesaj yok.\nBirileri sana yazdığında burada görünür.'
                    : 'No messages yet.\nChats will appear here when you receive messages.',
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
    final tr = L10n.locale == MemoLocale.tr;
    return Container(
      color: c.bgApp,
      child: Center(
        child: Text(
          tr ? 'Bir sohbet seç' : 'Select a conversation',
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
              // Avatar
              Container(
                width: 32,
                height: 32,
                decoration: BoxDecoration(
                  color: _waGreen.withValues(alpha: 0.18),
                  shape: BoxShape.circle,
                ),
                alignment: Alignment.center,
                child: Text(
                  _selectedName.isNotEmpty ? _selectedName[0].toUpperCase() : '?',
                  style: const TextStyle(
                    color: _waGreenDim,
                    fontWeight: FontWeight.w700,
                    fontSize: 14,
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
                child: CircularProgressIndicator(strokeWidth: 2, color: _waGreen)),
            error: (e, _) => _buildError(c, e.toString()),
            data: (msgs) {
              if (msgs.isEmpty) {
                return Center(
                  child: Text(
                    L10n.locale == MemoLocale.tr ? 'Mesaj yok' : 'No messages',
                    style: TextStyle(color: c.textDim),
                  ),
                );
              }
              return ListView.builder(
                controller: _scrollCtrl,
                padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
                reverse: true,
                itemCount: msgs.length,
                itemBuilder: (_, i) => _MessageBubble(
                  msg: msgs[msgs.length - 1 - i],
                ),
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
                        hintText: L10n.locale == MemoLocale.tr
                            ? 'Mesaj yaz...'
                            : 'Message...',
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
                    color: _waGreen,
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
                  L10n.locale == MemoLocale.tr
                      ? 'Bağlantı kesildi — yeniden bağlanıyor...'
                      : 'Disconnected — reconnecting...',
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
    if (text.isEmpty || _selectedJid == null || _sending) return;
    setState(() => _sending = true);
    _sendCtrl.clear();
    try {
      await ref.read(apiClientProvider).sendWhatsApp(_selectedJid!, text);
      ref.invalidate(whatsAppMessagesProvider(_selectedJid!));
      ref.invalidate(whatsAppChatsProvider);
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Gönderilemedi: $e'),
            backgroundColor: Colors.red,
            behavior: SnackBarBehavior.floating,
          ),
        );
      }
    } finally {
      if (mounted) setState(() => _sending = false);
    }
  }

  void _confirmLogout(ThemeColors c, bool tr) {
    showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        backgroundColor: c.bgPanel,
        title: Text(
          tr ? 'WhatsApp\'tan Çıkış Yap?' : 'Logout from WhatsApp?',
          style: TextStyle(color: c.textMain, fontSize: 16),
        ),
        content: Text(
          tr
              ? 'Oturum silinecek. Tekrar bağlanmak için QR okutman gerekecek.'
              : 'Your session will be removed. You\'ll need to scan a QR code to reconnect.',
          style: TextStyle(color: c.textDim, fontSize: 13),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: Text(tr ? 'İptal' : 'Cancel',
                style: TextStyle(color: c.textDim)),
          ),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              ref.read(whatsAppStatusProvider.notifier).logout();
            },
            child: const Text('Logout',
                style: TextStyle(color: Colors.red)),
          ),
        ],
      ),
    );
  }

  Widget _buildLoading(ThemeColors c) => Container(
        color: c.bgApp,
        child: const Center(
            child: CircularProgressIndicator(strokeWidth: 2, color: _waGreen)),
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
                  L10n.locale == MemoLocale.tr
                      ? 'Bağlantı hatası'
                      : 'Connection error',
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
                  label: Text(L10n.locale == MemoLocale.tr ? 'Tekrar dene' : 'Retry'),
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
              ? _waGreen.withValues(alpha: 0.1)
              : Colors.transparent,
          child: Container(
            decoration: BoxDecoration(
              border: Border(
                left: BorderSide(
                  color: isSelected ? _waGreen : Colors.transparent,
                  width: 2,
                ),
              ),
            ),
            padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
            child: Row(
              children: [
                Container(
                  width: 38,
                  height: 38,
                  decoration: BoxDecoration(
                    color: _waGreen.withValues(alpha: 0.15),
                    shape: BoxShape.circle,
                  ),
                  alignment: Alignment.center,
                  child: Text(
                    chat.displayName.isNotEmpty
                        ? chat.displayName[0].toUpperCase()
                        : '?',
                    style: const TextStyle(
                      color: _waGreenDim,
                      fontWeight: FontWeight.w700,
                      fontSize: 16,
                    ),
                  ),
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
                                color: isSelected ? _waGreen : c.textMain,
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

// ─── Message bubble ───────────────────────────────────────────

class _MessageBubble extends StatelessWidget {
  final WhatsAppMessage msg;
  const _MessageBubble({required this.msg});

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final isMe = msg.fromMe;

    return Padding(
      padding: const EdgeInsets.only(bottom: 6),
      child: Align(
        alignment: isMe ? Alignment.centerRight : Alignment.centerLeft,
        child: ConstrainedBox(
          constraints: BoxConstraints(
            maxWidth: MediaQuery.of(context).size.width * 0.55,
          ),
          child: Container(
            padding: const EdgeInsets.fromLTRB(12, 8, 12, 6),
            decoration: BoxDecoration(
              color: isMe
                  ? _waGreen.withValues(alpha: 0.18)
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
                        color: _waGreenDim,
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
