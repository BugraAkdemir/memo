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
