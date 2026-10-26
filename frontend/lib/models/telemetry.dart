/// What the backend says about the optional anonymous daily summary
/// (docs/TELEMETRY.md). Parsed defensively: this feeds an ambient first-run
/// question and a settings tab, and unreadable input must never switch anything
/// on or pop the question up.
class TelemetryStatus {
  /// Sharing is actually in effect (answered yes, not switched off by the
  /// environment).
  final bool enabled;

  /// An official build whose person has not answered yet: the first-run choice
  /// is due. Nothing is counted or sent until it is answered.
  final bool needsAsk;

  /// '' (not asked), 'yes' or 'no'.
  final String consent;

  /// MEMO_TELEMETRY=off is set on the backend: it overrides any answer.
  final bool envOff;

  /// 'official' or 'source'.
  final String build;
  final String endpoint;

  /// The first characters of the random id this install uses.
  final String deviceId;
  final String consentDay;
  final String lastSentDay;

  const TelemetryStatus({
    this.enabled = false,
    this.needsAsk = false,
    this.consent = '',
    this.envOff = false,
    this.build = '',
    this.endpoint = '',
    this.deviceId = '',
    this.consentDay = '',
    this.lastSentDay = '',
  });

  bool get answered => consent == 'yes' || consent == 'no';

  /// A summary may leave this machine.
  bool get sharing => enabled && !envOff && consent == 'yes';

  factory TelemetryStatus.fromJson(Object? raw) {
    if (raw is! Map) return const TelemetryStatus();
    String s(String k) => raw[k] is String ? raw[k] as String : '';
    return TelemetryStatus(
      enabled: raw['enabled'] == true,
      needsAsk: raw['needs_ask'] == true,
      consent: s('consent'),
      envOff: raw['env_off'] == true,
      build: s('build'),
      endpoint: s('endpoint'),
      deviceId: s('device_id'),
      consentDay: s('consent_day'),
      lastSentDay: s('last_sent_day'),
    );
  }
}

/// The report that would be sent next, as the exact text (not a re-rendering).
class TelemetryPreview {
  final String day;
  final bool empty;
  final String json;

  const TelemetryPreview({this.day = '', this.empty = true, this.json = ''});

  factory TelemetryPreview.fromJson(Object? raw) {
    if (raw is! Map) return const TelemetryPreview();
    return TelemetryPreview(
      day: raw['day'] is String ? raw['day'] as String : '',
      empty: raw['empty'] == true || raw['json'] is! String || (raw['json'] as String).isEmpty,
      json: raw['json'] is String ? raw['json'] as String : '',
    );
  }
}
