// Platform byte stream for SSE endpoints.
//
// Flutter web's Dio adapter (`dio_web_adapter`) has **no streaming code path**:
// `BrowserHttpClientAdapter.fetch()` uses `XMLHttpRequest` with
// `responseType = 'arraybuffer'` and completes the response only on
// `xhr.onLoad` — i.e. the entire body is buffered and delivered as a single
// chunk once the request finishes. For a finite SSE turn that means no
// token-by-token streaming and, worse, mid-stream `agent_event` chunks
// (`permission_request`!) arriving only after the backend's 60s permission
// wait has already auto-denied. For an *infinite* stream (`/api/tasks/events`)
// `onLoad` never fires, so the request just aborts (`net::ERR_ABORTED`) and
// the live Self-Driving task card never populates.
//
// The browser's own Fetch API *does* stream (`Response.body` is a
// `ReadableStream`), so on web we bypass Dio for the streaming endpoints and
// read the `ReadableStream` directly (see `sse_stream_web.dart`). On native,
// Dio already streams fine and this file resolves to a stub that is never
// called (call sites guard on `kIsWeb`).

export 'sse_stream_stub.dart'
    if (dart.library.js_interop) 'sse_stream_web.dart';
