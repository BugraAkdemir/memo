/// Timing of the in-flight chat turn: when it started and when this client
/// last heard anything from the backend for it — a content/agent/status
/// chunk, or the backend's periodic "heartbeat" chunk while it is silently
/// busy (a non-streaming agent model call, a running tool, a long prefill).
/// Drives StreamProgressLine: before it, a long agent turn showed nothing
/// between tool calls, and a turn that was still working looked exactly like
/// one that had hung.
class StreamTiming {
  const StreamTiming({required this.startedAt, required this.lastActivity});
  final DateTime startedAt;
  final DateTime lastActivity;
}
