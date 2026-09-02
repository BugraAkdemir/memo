/// Native stub — never invoked. Every call site is guarded by `kIsWeb`, and on
/// native Dio's own `ResponseType.stream` path is used instead. This exists so
/// `sse_stream.dart`'s conditional export has a target that does not import
/// `dart:js_interop` / `package:web`.
Stream<List<int>> platformByteStream({
  required String url,
  required String method,
  Map<String, String>? headers,
  String? body,
  Future<void>? cancelSignal,
}) {
  throw UnsupportedError('platformByteStream is web-only');
}
