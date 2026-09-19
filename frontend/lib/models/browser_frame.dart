/// A live screenshot pushed from the backend whenever the agent calls
/// browser_screenshot — see internal/app/browser_frame.go's BrowserFrame
/// (the Go struct this mirrors) for the "why" of this shape, notably why it
/// carries no URL (derived client-side from browser_navigate's own
/// AgentEvent instead — see browserCurrentUrlProvider in chat_provider.dart).
class BrowserFrame {
  final String screenshotBase64;
  final int ts;

  const BrowserFrame({required this.screenshotBase64, required this.ts});

  factory BrowserFrame.fromJson(Map<String, dynamic> json) => BrowserFrame(
        screenshotBase64: json['screenshot_base64'] as String? ?? '',
        ts: json['ts'] as int? ?? 0,
      );
}

/// Result of a direct (non-agent) manual browser action — navigate/click/
/// scroll all respond with this same shape, see
/// internal/webserver/handlers_browser_session.go's
/// browserSessionActionResponse (the Go struct this mirrors).
class BrowserSessionAction {
  final String? screenshotBase64;
  final String? url;
  final String? error;

  const BrowserSessionAction({this.screenshotBase64, this.url, this.error});

  factory BrowserSessionAction.fromJson(Map<String, dynamic> json) => BrowserSessionAction(
        screenshotBase64: json['screenshot_base64'] as String?,
        url: json['url'] as String?,
        error: json['error'] as String?,
      );
}

/// Result of GET /api/browser/session/status.
class BrowserSessionStatus {
  final bool active;
  final String? url;

  const BrowserSessionStatus({required this.active, this.url});

  factory BrowserSessionStatus.fromJson(Map<String, dynamic> json) => BrowserSessionStatus(
        active: json['active'] as bool? ?? false,
        url: json['url'] as String?,
      );
}
