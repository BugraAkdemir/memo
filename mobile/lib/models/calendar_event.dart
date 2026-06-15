class CalendarEvent {
  final String id;
  final String title;
  final DateTime startTime;
  final DateTime? endTime;
  final String description;
  final String source;
  final String contactName;
  final DateTime createdAt;
  final bool reminderSent;

  const CalendarEvent({
    required this.id,
    required this.title,
    required this.startTime,
    this.endTime,
    this.description = '',
    this.source = 'manual',
    this.contactName = '',
    required this.createdAt,
    this.reminderSent = false,
  });

  factory CalendarEvent.fromJson(Map<String, dynamic> json) => CalendarEvent(
        id: json['id'] as String? ?? '',
        title: json['title'] as String? ?? '',
        startTime: DateTime.parse(json['start_time'] as String),
        endTime: json['end_time'] != null
            ? DateTime.parse(json['end_time'] as String)
            : null,
        description: json['description'] as String? ?? '',
        source: json['source'] as String? ?? 'manual',
        contactName: json['contact_name'] as String? ?? '',
        createdAt: DateTime.parse(json['created_at'] as String),
        reminderSent: json['reminder_sent'] as bool? ?? false,
      );

  Map<String, dynamic> toJson() => {
        'id': id,
        'title': title,
        'start_time': startTime.toIso8601String(),
        if (endTime != null) 'end_time': endTime!.toIso8601String(),
        'description': description,
        'source': source,
        'contact_name': contactName,
        'created_at': createdAt.toIso8601String(),
        'reminder_sent': reminderSent,
      };
}

class CalendarSettings {
  final int reminderLeadMinutes;

  const CalendarSettings({this.reminderLeadMinutes = 30});

  factory CalendarSettings.fromJson(Map<String, dynamic> json) =>
      CalendarSettings(
        reminderLeadMinutes: json['reminder_lead_minutes'] as int? ?? 30,
      );

  Map<String, dynamic> toJson() => {
        'reminder_lead_minutes': reminderLeadMinutes,
      };
}

class LearningSettings {
  final bool singleModelEnabled;
  final String modelId;

  const LearningSettings({
    this.singleModelEnabled = false,
    this.modelId = '',
  });

  factory LearningSettings.fromJson(Map<String, dynamic> json) =>
      LearningSettings(
        singleModelEnabled: json['single_model_enabled'] as bool? ?? false,
        modelId: json['model_id'] as String? ?? '',
      );

  Map<String, dynamic> toJson() => {
        'single_model_enabled': singleModelEnabled,
        'model_id': modelId,
      };
}
