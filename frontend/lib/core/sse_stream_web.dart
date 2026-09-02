import 'dart:async';
import 'dart:js_interop';

import 'package:web/web.dart' as web;

/// Web SSE transport: a true streaming reader built on the Fetch API's
/// `Response.body` `ReadableStream`, used in place of Dio's non-streaming web
/// adapter for the handful of SSE endpoints (see `sse_stream.dart` for why).
///
/// Yields raw byte chunks exactly as they arrive off the wire; the caller runs
/// the same `utf8.decoder` + `LineSplitter` transform it already applies to
/// Dio's native byte stream, so parsing downstream is identical on both
/// platforms.
///
/// [cancelSignal] mirrors Dio's `CancelToken`: when it completes the fetch is
/// aborted. The returned stream also aborts the fetch if the subscription is
/// cancelled.
Stream<List<int>> platformByteStream({
  required String url,
  required String method,
  Map<String, String>? headers,
  String? body,
  Future<void>? cancelSignal,
}) {
  final abort = web.AbortController();
  var cancelled = false;

  void doAbort() {
    if (cancelled) return;
    cancelled = true;
    try {
      abort.abort();
    } catch (_) {/* already aborted / torn down */}
  }

  cancelSignal?.whenComplete(doAbort);

  late final StreamController<List<int>> controller;

  Future<void> pump() async {
    try {
      final headerObj = web.Headers();
      (headers ?? const <String, String>{}).forEach((k, v) {
        headerObj.set(k, v);
      });

      final init = web.RequestInit(
        method: method,
        headers: headerObj,
        signal: abort.signal,
        body: body?.toJS,
      );

      final resp = await web.window.fetch(url.toJS, init).toDart;

      if (!resp.ok) {
        var detail = '';
        try {
          detail = (await resp.text().toDart).toDart;
        } catch (_) {/* body not readable */}
        throw Exception(
          'HTTP ${resp.status}${detail.isNotEmpty ? ': $detail' : ''}',
        );
      }

      final rs = resp.body;
      if (rs == null) {
        await controller.close();
        return;
      }

      final reader = web.ReadableStreamDefaultReader(rs);
      try {
        while (!cancelled) {
          final result = await reader.read().toDart;
          if (result.done) break;
          final value = result.value;
          if (value == null) continue;
          controller.add((value as JSUint8Array).toDart);
        }
      } finally {
        try {
          reader.releaseLock();
        } catch (_) {/* stream already errored/closed */}
      }
      await controller.close();
    } catch (e, st) {
      if (!controller.isClosed) {
        // A caller-initiated abort is a normal end-of-stream, not an error —
        // Dio's path returns silently on DioExceptionType.cancel, match that.
        if (cancelled) {
          await controller.close();
        } else {
          controller.addError(e, st);
          await controller.close();
        }
      }
    }
  }

  controller = StreamController<List<int>>(
    onListen: pump,
    onCancel: doAbort,
  );
  return controller.stream;
}
