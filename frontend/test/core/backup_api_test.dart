import 'dart:convert';
import 'dart:typed_data';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/core/api_client.dart';

class _Adapter implements HttpClientAdapter {
  final requests = <RequestOptions>[];
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? stream,
    Future<void>? cancel,
  ) async {
    requests.add(options);
    if (options.path == '/api/export') {
      return ResponseBody.fromBytes([1, 2, 3], 200);
    }
    return ResponseBody.fromString(
      '{}',
      200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  test(
    'encrypted backup password stays in request body and bytes roundtrip',
    () async {
      final api = MemoApiClient(baseUrl: 'http://memo.test');
      final adapter = _Adapter();
      api.dio.httpClientAdapter = adapter;
      const password = 'a-private-password';
      expect(await api.exportData(password: password), [1, 2, 3]);
      await api.importData([1, 2, 3], password: password);
      expect(adapter.requests.map((r) => r.method), ['POST', 'POST']);
      for (final r in adapter.requests) {
        expect(r.uri.toString(), isNot(contains(password)));
        expect(r.headers.toString(), isNot(contains(password)));
        expect((r.data as Map)['password'], password);
      }
      expect(
        base64Decode((adapter.requests.last.data as Map)['data'] as String),
        [1, 2, 3],
      );
      api.dio.close(force: true);
    },
  );
}
