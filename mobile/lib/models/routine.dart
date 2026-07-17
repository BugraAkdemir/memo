class Routine {
  final String id;
  final String createdFromText;
  final String timeOfDay;
  final String prompt;
  final bool agentMode;
  final bool deliveryWhatsApp;
  final bool deliveryMobile;
  final bool enabled;

  const Routine({
    required this.id,
    required this.createdFromText,
    required this.timeOfDay,
    required this.prompt,
    required this.agentMode,
    required this.deliveryWhatsApp,
    required this.deliveryMobile,
    required this.enabled,
  });

  factory Routine.fromJson(Map<String, dynamic> json) {
    final schedule = (json['schedule'] as Map?) ?? {};
    return Routine(
      id: json['id'] as String? ?? '',
      createdFromText: json['created_from_text'] as String? ?? '',
      timeOfDay: schedule['time_of_day'] as String? ?? '',
      prompt: json['prompt'] as String? ?? '',
      agentMode: json['agent_mode'] as bool? ?? false,
      deliveryWhatsApp: json['delivery_whatsapp'] as bool? ?? false,
      deliveryMobile: json['delivery_mobile'] as bool? ?? false,
      enabled: json['enabled'] as bool? ?? false,
    );
  }

  Map<String, dynamic> toJson() => {
        'id': id,
        'created_from_text': createdFromText,
        'schedule': {'time_of_day': timeOfDay},
        'prompt': prompt,
        'agent_mode': agentMode,
        'delivery_whatsapp': deliveryWhatsApp,
        'delivery_mobile': deliveryMobile,
        'enabled': enabled,
      };
}

/// A routine's freshly generated content, ready for the phone to pre-schedule
/// as a local notification ahead of its fire time — see GET
/// /api/routines/mobile-ready.
class RoutineMobileReady {
  final String id;
  final String title;
  final String body;
  final DateTime fireAtUtc;

  const RoutineMobileReady({
    required this.id,
    required this.title,
    required this.body,
    required this.fireAtUtc,
  });

  factory RoutineMobileReady.fromJson(Map<String, dynamic> json) =>
      RoutineMobileReady(
        id: json['id'] as String? ?? '',
        title: json['title'] as String? ?? 'Rutin',
        body: json['body'] as String? ?? '',
        fireAtUtc: DateTime.parse(json['fire_at_utc'] as String),
      );
}
