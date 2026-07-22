class WhatsAppMessage {
  final String id;
  final String chatJid;
  final String senderJid;
  final String senderName;
  final String text;
  final DateTime timestamp;
  final bool fromMe;

  const WhatsAppMessage({
    required this.id,
    required this.chatJid,
    required this.senderJid,
    required this.senderName,
    required this.text,
    required this.timestamp,
    required this.fromMe,
  });

  factory WhatsAppMessage.fromJson(Map<String, dynamic> json) => WhatsAppMessage(
        id: json['id'] as String? ?? '',
        chatJid: json['chat_jid'] as String? ?? '',
        senderJid: json['sender_jid'] as String? ?? '',
        senderName: json['sender_name'] as String? ?? '',
        text: json['text'] as String? ?? '',
        timestamp: json['timestamp'] != null
            ? DateTime.tryParse(json['timestamp'].toString()) ?? DateTime.now()
            : DateTime.now(),
        fromMe: json['from_me'] == true,
      );
}

class WhatsAppChatSummary {
  final String jid;
  final String displayName;
  final String lastMessage;
  final DateTime lastTime;
  // Lifetime count of messages received in this chat — NOT an unread count
  // (BUG-M4). There is no read/unread tracking in the backend at all; this
  // field (and the backend's matching JSON key) used to be misleadingly
  // named `unread` despite that. Currently unused by any widget.
  final int totalReceived;

  const WhatsAppChatSummary({
    required this.jid,
    required this.displayName,
    required this.lastMessage,
    required this.lastTime,
    required this.totalReceived,
  });

  factory WhatsAppChatSummary.fromJson(Map<String, dynamic> json) =>
      WhatsAppChatSummary(
        jid: json['jid'] as String? ?? '',
        displayName: json['display_name'] as String? ?? '',
        lastMessage: json['last_message'] as String? ?? '',
        lastTime: json['last_time'] != null
            ? DateTime.tryParse(json['last_time'].toString()) ?? DateTime.now()
            : DateTime.now(),
        totalReceived: json['total_received'] as int? ?? 0,
      );
}

class WhatsAppStatus {
  final bool initialized;
  final bool connected;
  final bool loggedIn;
  final bool reconnecting;
  final String lastError;
  final List<String> qrCodes;

  const WhatsAppStatus({
    required this.initialized,
    required this.connected,
    required this.loggedIn,
    required this.reconnecting,
    required this.lastError,
    required this.qrCodes,
  });

  factory WhatsAppStatus.fromJson(Map<String, dynamic> json) {
    final rawCodes = json['qr_codes'];
    final codes = rawCodes is List ? rawCodes.cast<String>() : <String>[];
    return WhatsAppStatus(
      initialized: json['initialized'] as bool? ?? false,
      connected: json['connected'] as bool? ?? false,
      loggedIn: json['logged_in'] as bool? ?? false,
      reconnecting: json['reconnecting'] as bool? ?? false,
      lastError: json['last_error'] as String? ?? '',
      qrCodes: codes,
    );
  }
}
