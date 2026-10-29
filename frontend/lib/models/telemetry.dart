/// What the backend says about the optional anonymous daily summary
/// (docs/TELEMETRY.md). Parsed defensively: this feeds a settings tab and the
/// setup wizard, and unreadable input must never switch anything on.
class TelemetryStatus {
  /// Sharing is actually in effect (the answer is yes - the default of an
  /// official build - and it is not switched off by the environment).
  final bool enabled;

  /// '' (no answer: a source build), 'yes' or 'no'.
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
