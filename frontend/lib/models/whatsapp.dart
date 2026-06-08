class WhatsAppMessage {
  final String id;
  final String chatJid;
  final String senderJid;
  final String senderName;
  final String text;
  final DateTime timestamp;
  final bool fromMe;

  WhatsAppMessage({
    required this.id,
    required this.chatJid,
    required this.senderJid,
    required this.senderName,
    required this.text,
    required this.timestamp,
    required this.fromMe,
  });

  factory WhatsAppMessage.fromJson(Map<String, dynamic> json) {
    return WhatsAppMessage(
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
}

class WhatsAppChatSummary {
  final String jid;
  final String lastMessage;
  final DateTime lastTime;
  final int unread;

  WhatsAppChatSummary({
    required this.jid,
    required this.lastMessage,
    required this.lastTime,
    required this.unread,
  });

  factory WhatsAppChatSummary.fromJson(Map<String, dynamic> json) {
    return WhatsAppChatSummary(
      jid: json['jid'] as String? ?? '',
      lastMessage: json['last_message'] as String? ?? '',
      lastTime: json['last_time'] != null
          ? DateTime.tryParse(json['last_time'].toString()) ?? DateTime.now()
          : DateTime.now(),
      unread: json['unread'] as int? ?? 0,
    );
  }

  String get displayName {
    final parts = jid.split('@');
    if (parts.isEmpty) return jid;
    final name = parts[0];
    if (name.contains('-')) return 'Grup';
    return name;
  }
}

class WhatsAppStatus {
  final bool initialized;
  final bool connected;
  final bool loggedIn;
  final List<String> qrCodes;

  WhatsAppStatus({
    required this.initialized,
    required this.connected,
    required this.loggedIn,
    required this.qrCodes,
  });

  factory WhatsAppStatus.fromJson(Map<String, dynamic> json) {
    final rawCodes = json['qr_codes'];
    final codes = rawCodes is List ? rawCodes.cast<String>() : <String>[];
    return WhatsAppStatus(
      initialized: json['initialized'] as bool? ?? false,
      connected: json['connected'] as bool? ?? false,
      loggedIn: json['logged_in'] as bool? ?? false,
      qrCodes: codes,
    );
  }
}
