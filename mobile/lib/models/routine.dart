import '../core/l10n.dart';

String _routineFallbackTitle() => L10n.t('routine_fallback');

/// Round-trips every field the backend's Routine struct has (BUG-C2): this
/// model used to only carry a subset (no weekdays/context_source/
/// auto_approve_tools/whatsapp_target_jid), so saving any edit — even just
/// flipping the enable switch — silently wiped those fields via toJson()'s
/// incomplete PUT body. The backend now also merges onto the existing
/// stored routine rather than blindly replacing it, but this model should
/// still carry everything it received rather than relying solely on that
/// server-side safety net.
class Routine {
  final String id;
  final String createdFromText;
  final String timeOfDay;
  final List<int> weekdays;
  final String prompt;
  final bool agentMode;
  final bool autoApproveTools;
  final String contextSourceType;
  final String contextWhatsAppJid;
  final bool deliveryWhatsApp;
  final bool deliveryMobile;
  final String whatsAppTargetJid;
  final bool enabled;

  const Routine({
    required this.id,
    required this.createdFromText,
    required this.timeOfDay,
    required this.weekdays,
    required this.prompt,
    required this.agentMode,
    required this.autoApproveTools,
    required this.contextSourceType,
    required this.contextWhatsAppJid,
    required this.deliveryWhatsApp,
    required this.deliveryMobile,
    required this.whatsAppTargetJid,
    required this.enabled,
  });

  factory Routine.fromJson(Map<String, dynamic> json) {
    final schedule = (json['schedule'] as Map?) ?? {};
    final contextSource = (json['context_source'] as Map?) ?? {};
    return Routine(
      id: json['id'] as String? ?? '',
      createdFromText: json['created_from_text'] as String? ?? '',
      timeOfDay: schedule['time_of_day'] as String? ?? '',
      weekdays: ((schedule['weekdays'] as List?) ?? []).map((e) => e as int).toList(),
      prompt: json['prompt'] as String? ?? '',
      agentMode: json['agent_mode'] as bool? ?? false,
      autoApproveTools: json['auto_approve_tools'] as bool? ?? false,
      contextSourceType: contextSource['type'] as String? ?? 'none',
      contextWhatsAppJid: contextSource['whatsapp_jid'] as String? ?? '',
      deliveryWhatsApp: json['delivery_whatsapp'] as bool? ?? false,
      deliveryMobile: json['delivery_mobile'] as bool? ?? false,
      whatsAppTargetJid: json['whatsapp_target_jid'] as String? ?? '',
      enabled: json['enabled'] as bool? ?? false,
    );
  }

  Map<String, dynamic> toJson() => {
        'id': id,
        'created_from_text': createdFromText,
        'schedule': {'time_of_day': timeOfDay, 'weekdays': weekdays},
        'prompt': prompt,
        'agent_mode': agentMode,
        'auto_approve_tools': autoApproveTools,
        'context_source': {'type': contextSourceType, 'whatsapp_jid': contextWhatsAppJid},
        'delivery_whatsapp': deliveryWhatsApp,
        'delivery_mobile': deliveryMobile,
        'whatsapp_target_jid': whatsAppTargetJid,
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
        title: json['title'] as String? ?? _routineFallbackTitle(),
        body: json['body'] as String? ?? '',
        fireAtUtc: DateTime.parse(json['fire_at_utc'] as String),
      );
}
