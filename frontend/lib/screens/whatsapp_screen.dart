import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:qr_flutter/qr_flutter.dart';
import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/whatsapp.dart';
import '../providers/whatsapp_provider.dart';
import '../providers/chat_provider.dart' show apiClientProvider;

/// WhatsApp integration screen — QR pairing, chat list, messages.
class WhatsAppScreen extends ConsumerStatefulWidget {
  WhatsAppScreen({super.key});

  @override
  ConsumerState<WhatsAppScreen> createState() => _WhatsAppScreenState();
}

class _WhatsAppScreenState extends ConsumerState<WhatsAppScreen> {
  String? _selectedChatJid;
  final _messageController = TextEditingController();
  final _searchController = TextEditingController();
  bool _searchMode = false;

  @override
  void initState() {
    super.initState();
    Future.microtask(() => ref.read(whatsAppStatusProvider.notifier).startPolling());
  }

  @override
  void dispose() {
    _messageController.dispose();
    _searchController.dispose();
    ref.read(whatsAppStatusProvider.notifier).stopPolling();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final statusAsync = ref.watch(whatsAppStatusProvider);

    return Scaffold(
      backgroundColor: MemoTheme.of(context).bgApp,
      appBar: _buildAppBar(),
      body: statusAsync.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (e, _) => _buildError(e),
        data: (status) => _buildBody(status),
      ),
    );
  }

  PreferredSizeWidget _buildAppBar() {
    return AppBar(
      backgroundColor: MemoTheme.of(context).bgPanel,
      title: Text(
        L10n.t('whatsapp'),
        style: TextStyle(color: MemoTheme.of(context).textMain),
      ),
      actions: [
        IconButton(
          icon: Icon(Icons.search, color: MemoTheme.of(context).textDim),
          onPressed: () => setState(() => _searchMode = !_searchMode),
        ),
      ],
    );
  }

  Widget _buildError(Object e) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.error_outline, size: 48, color: MemoTheme.of(context).textDim),
            const SizedBox(height: 16),
            Text(
              'WhatsApp bağlantı hatası',
              style: TextStyle(color: MemoTheme.of(context).textMain, fontSize: 16),
            ),
            const SizedBox(height: 8),
            Text(
              e.toString(),
              style: TextStyle(color: MemoTheme.of(context).textDim, fontSize: 12),
              textAlign: TextAlign.center,
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildBody(WhatsAppStatus status) {
    if (_selectedChatJid != null) {
      return _buildMessageView(status);
    }
    if (!status.initialized) {
      return _buildNotInitialized();
    }
    if (!status.loggedIn && status.qrCodes.isNotEmpty) {
      return _buildQRView(status);
    }
    if (!status.loggedIn) {
      return _buildConnectingView();
    }
    if (!status.connected) {
      return _buildConnectView();
    }
    return _buildChatListView();
  }

  Widget _buildNotInitialized() {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.message, size: 64, color: const Color(0xFF25D366)),
          const SizedBox(height: 16),
          Text(
            'WhatsApp entegrasyonu',
            style: TextStyle(color: MemoTheme.of(context).textMain, fontSize: 18),
          ),
          const SizedBox(height: 8),
          Text(
            'Ayarlar > config.yaml > whatsapp.enabled: true',
            style: TextStyle(color: MemoTheme.of(context).textDim, fontSize: 13),
          ),
        ],
      ),
    );
  }

  Widget _buildQRView(WhatsAppStatus status) {
    return Center(
      child: SingleChildScrollView(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(
              'WhatsApp\'a Bağlan',
              style: TextStyle(color: MemoTheme.of(context).textMain, fontSize: 20),
            ),
            const SizedBox(height: 8),
            Text(
              'WhatsApp\'ı aç > Bağlı cihazlar > Cihaz bağla > QR okut',
              style: TextStyle(color: MemoTheme.of(context).textDim, fontSize: 13),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 24),
            status.qrCodes.isNotEmpty
                ? QrImageView(
                    data: status.qrCodes.first,
                    version: QrVersions.auto,
                    size: 250,
                    backgroundColor: Colors.white,
                    eyeStyle: const QrEyeStyle(
                      eyeShape: QrEyeShape.square,
                      color: Colors.black,
                    ),
                    dataModuleStyle: const QrDataModuleStyle(
                      dataModuleShape: QrDataModuleShape.square,
                      color: Colors.black,
                    ),
                  )
                : CircularProgressIndicator(color: MemoTheme.accent),
            const SizedBox(height: 16),
            Text(
              status.connected ? '✅ Bağlı' : '⏳ QR taranıyor...',
              style: TextStyle(color: MemoTheme.of(context).textMain),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildConnectView() {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.link_off, size: 48, color: MemoTheme.of(context).textDim),
          const SizedBox(height: 16),
          Text(
            'WhatsApp bağlı değil',
            style: TextStyle(color: MemoTheme.of(context).textMain, fontSize: 16),
          ),
          const SizedBox(height: 16),
          ElevatedButton.icon(
            onPressed: () => ref.read(whatsAppStatusProvider.notifier).connect(),
            icon: const Icon(Icons.link),
            label: Text(L10n.t('connect')),
            style: ElevatedButton.styleFrom(
              backgroundColor: const Color(0xFF25D366),
              foregroundColor: Colors.white,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildConnectingView() {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const CircularProgressIndicator(),
          const SizedBox(height: 16),
          Text(
            'WhatsApp\'a bağlanıyor...',
            style: TextStyle(color: MemoTheme.of(context).textDim, fontSize: 14),
          ),
          const SizedBox(height: 8),
          Text(
            'QR kodu bekleniyor',
            style: TextStyle(color: MemoTheme.of(context).textMuted, fontSize: 12),
          ),
        ],
      ),
    );
  }

  Widget _buildChatListView() {
    final chatsAsync = ref.watch(whatsAppChatsProvider);

    return Column(
      children: [
        if (_searchMode)
          Padding(
            padding: const EdgeInsets.all(12),
            child: TextField(
              controller: _searchController,
              decoration: InputDecoration(
                hintText: 'Mesajlarda ara...',
                prefixIcon: const Icon(Icons.search),
                suffixIcon: _searchController.text.isNotEmpty
                    ? IconButton(
                        icon: const Icon(Icons.clear),
                        onPressed: () {
                          _searchController.clear();
                          setState(() {});
                        },
                      )
                    : null,
              ),
              onChanged: (_) => setState(() {}),
            ),
          ),
        Expanded(
          child: chatsAsync.when(
            loading: () => const Center(child: CircularProgressIndicator()),
            error: (e, _) => _buildError(e),
            data: (chats) {
              if (chats.isEmpty) {
                return Center(
                  child: Text(
                    'Henüz mesaj yok',
                    style: TextStyle(color: MemoTheme.of(context).textDim),
                  ),
                );
              }
              return ListView.builder(
                itemCount: chats.length,
                itemBuilder: (_, i) => _buildChatTile(chats[i]),
              );
            },
          ),
        ),
      ],
    );
  }

  Widget _buildChatTile(WhatsAppChatSummary chat) {
    return ListTile(
      leading: CircleAvatar(
        backgroundColor: MemoTheme.accentPale,
        child: Text(
          chat.displayName.isNotEmpty ? chat.displayName[0].toUpperCase() : '?',
          style: const TextStyle(color: MemoTheme.accent, fontWeight: FontWeight.bold),
        ),
      ),
      title: Text(
        chat.displayName,
        style: TextStyle(color: MemoTheme.of(context).textMain, fontWeight: FontWeight.w500),
      ),
      subtitle: Text(
        chat.lastMessage,
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
        style: TextStyle(color: MemoTheme.of(context).textDim, fontSize: 12),
      ),
      trailing: Text(
        _formatTime(chat.lastTime),
        style: TextStyle(color: MemoTheme.of(context).textDim, fontSize: 11),
      ),
      onTap: () => setState(() => _selectedChatJid = chat.jid),
    );
  }

  Widget _buildMessageView(WhatsAppStatus status) {
    final msgsAsync = ref.watch(whatsAppMessagesProvider(_selectedChatJid!));

    return Column(
      children: [
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
          color: MemoTheme.of(context).bgPanel,
          child: Row(
            children: [
              IconButton(
                icon: const Icon(Icons.arrow_back),
                onPressed: () => setState(() => _selectedChatJid = null),
              ),
              Expanded(
                child: Text(
                  _selectedChatJid ?? '',
                  style: TextStyle(color: MemoTheme.of(context).textMain),
                ),
              ),
              if (status.connected && status.loggedIn)
                Icon(Icons.check_circle, size: 16, color: const Color(0xFF25D366)),
            ],
          ),
        ),
        Expanded(
          child: msgsAsync.when(
            loading: () => const Center(child: CircularProgressIndicator()),
            error: (e, _) => _buildError(e),
            data: (messages) {
              if (messages.isEmpty) {
                return Center(
                  child: Text('Mesaj yok', style: TextStyle(color: MemoTheme.of(context).textDim)),
                );
              }
              return ListView.builder(
                reverse: true,
                padding: const EdgeInsets.all(12),
                itemCount: messages.length,
                itemBuilder: (_, i) => _buildMessageBubble(messages[messages.length - 1 - i]),
              );
            },
          ),
        ),
        if (status.connected && status.loggedIn)
          Container(
            padding: const EdgeInsets.all(12),
            color: MemoTheme.of(context).bgPanel,
            child: Row(
              children: [
                Expanded(
                  child: TextField(
                    controller: _messageController,
                    decoration: const InputDecoration(
                      hintText: 'Mesaj yaz...',
                      border: OutlineInputBorder(),
                      contentPadding: EdgeInsets.symmetric(horizontal: 12, vertical: 8),
                    ),
                    onSubmitted: _sendMessage,
                  ),
                ),
                const SizedBox(width: 8),
                IconButton(
                  icon: const Icon(Icons.send),
                  color: const Color(0xFF25D366),
                  onPressed: () => _sendMessage(_messageController.text),
                ),
              ],
            ),
          ),
      ],
    );
  }

  Widget _buildMessageBubble(WhatsAppMessage msg) {
    final isMe = msg.fromMe;
    return Align(
      alignment: isMe ? Alignment.centerRight : Alignment.centerLeft,
      child: Container(
        margin: const EdgeInsets.only(bottom: 6),
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
        constraints: BoxConstraints(maxWidth: MediaQuery.of(context).size.width * 0.7),
        decoration: BoxDecoration(
          color: isMe ? const Color(0xFF25D366).withValues(alpha: 0.15) : MemoTheme.of(context).bgPanel,
          borderRadius: BorderRadius.only(
            topLeft: const Radius.circular(16),
            topRight: const Radius.circular(16),
            bottomLeft: isMe ? const Radius.circular(16) : Radius.zero,
            bottomRight: isMe ? Radius.zero : const Radius.circular(16),
          ),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            if (!isMe && msg.senderName.isNotEmpty)
              Text(
                msg.senderName,
                style: TextStyle(
                  fontSize: 11,
                  color: MemoTheme.accent,
                  fontWeight: FontWeight.w600,
                ),
              ),
            Text(
              msg.text,
              style: TextStyle(color: MemoTheme.of(context).textMain, fontSize: 14),
            ),
            const SizedBox(height: 2),
            Text(
              _formatTime(msg.timestamp),
              style: TextStyle(color: MemoTheme.of(context).textDim, fontSize: 10),
            ),
          ],
        ),
      ),
    );
  }

  void _sendMessage(String text) {
    if (text.trim().isEmpty || _selectedChatJid == null) return;
    _messageController.clear();
    ref.read(apiClientProvider).sendWhatsApp(_selectedChatJid!, text).then((_) {
      ref.invalidate(whatsAppMessagesProvider(_selectedChatJid!));
      ref.invalidate(whatsAppChatsProvider);
    }).catchError((e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Hata: $e')));
      }
    });
  }

  String _formatTime(DateTime dt) {
    final now = DateTime.now();
    final diff = now.difference(dt);
    if (diff.inMinutes < 1) return 'şimdi';
    if (diff.inHours < 1) return '${diff.inMinutes}d';
    if (diff.inDays < 1) return '${dt.hour.toString().padLeft(2, '0')}:${dt.minute.toString().padLeft(2, '0')}';
    return '${dt.day}/${dt.month}';
  }
}
