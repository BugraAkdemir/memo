import 'dart:typed_data';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/core/api_client.dart';

class _EventsAdapter implements HttpClientAdapter {
  RequestOptions? request;
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? data,
    Future<void>? cancel,
  ) async {
    request = options;
    return ResponseBody.fromString(
      ': ready\n\nevent: autonomy:action_pending\ndata: {"id":"one","state":"pending"}\n\n: ping\n\n',
      200,
      headers: {
        Headers.contentTypeHeader: ['text/event-stream'],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  test(
    'twin SSE uses a per-request idle timeout and skips heartbeats',
    () async {
      final api = MemoApiClient(baseUrl: 'http://memo.test');
      final adapter = _EventsAdapter();
      api.dio.httpClientAdapter = adapter;
      final signals = await api.twinChangeStream().toList();
      expect(signals, hasLength(1));
      expect(adapter.request!.path, '/api/twin/events');
      expect(adapter.request!.receiveTimeout, const Duration(seconds: 300));
      expect(api.dio.options.receiveTimeout, const Duration(seconds: 120));
    },
  );
}
