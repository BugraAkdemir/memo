class TelegramStatus {
  final bool configured;
  final bool connected;
  final bool reconnecting;
  final String lastError;
  final String botUsername;
  final bool ownerLinked;
  final String ownerName;

  const TelegramStatus({
    required this.configured,
    required this.connected,
    required this.reconnecting,
    required this.lastError,
    required this.botUsername,
    required this.ownerLinked,
    required this.ownerName,
  });

  factory TelegramStatus.fromJson(Map<String, dynamic> json) => TelegramStatus(
        configured: json['configured'] as bool? ?? false,
        connected: json['connected'] as bool? ?? false,
        reconnecting: json['reconnecting'] as bool? ?? false,
        lastError: json['last_error'] as String? ?? '',
        botUsername: json['bot_username'] as String? ?? '',
        ownerLinked: json['owner_linked'] as bool? ?? false,
        ownerName: json['owner_name'] as String? ?? '',
      );
}
