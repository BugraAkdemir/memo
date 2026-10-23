/// Temporary, chat-scoped child-agent transcript. Malformed entries are ignored.
class SubagentJob {
  final String id, chatId, title, prompt, status, output, error, branch;
  final String resultDelivery, deliveryError;
  final List<Map<String, dynamic>> history, events;
  const SubagentJob({
    required this.id,
    required this.chatId,
    this.title = '',
    this.prompt = '',
    this.status = 'queued',
    this.output = '',
    this.error = '',
    this.branch = '',
    this.resultDelivery = '',
    this.deliveryError = '',
    this.history = const [],
    this.events = const [],
  });

  static List<Map<String, dynamic>> records(dynamic value) => value is List
      ? value
            .whereType<Map>()
            .map((e) => e.map((k, v) => MapEntry(k.toString(), v)))
            .toList()
      : const [];
  factory SubagentJob.fromJson(Map<String, dynamic> json) {
    String s(String key, [String fallback = '']) =>
        json[key] is String ? json[key] as String : fallback;
    return SubagentJob(
      id: s('id'),
      chatId: s('chat_id'),
      title: s('title'),
      prompt: s('prompt'),
      status: s('status', 'queued'),
      output: s('output'),
      error: s('error'),
      branch: s('branch'),
      resultDelivery: s('result_delivery'),
      deliveryError: s('delivery_error'),
      history: records(json['history'] ?? json['messages']),
      events: records(json['events']),
    );
  }
  bool get active => status == 'running' || status == 'queued';
  bool get closed => status == 'closed' || status == 'cancelled';
}
