import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/models_provider.dart';

/// Simulates a dead backend: every request fails the way a real
/// SocketException("Connection refused") does when nothing is listening on
/// the configured host:port (e.g. a saved Remote Access URL pointing at a
/// server that's since been shut down).
class _ConnectionRefusedAdapter implements HttpClientAdapter {
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    throw DioException(
      requestOptions: options,
      type: DioExceptionType.connectionError,
      message: 'Connection refused (OS Error: Connection refused, errno = 111)',
    );
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  // Regression test: llamaInstalledProvider used to catch every error
  // (including plain connectivity failures) and return `false`, making an
  // unreachable backend indistinguishable from "llama.cpp genuinely isn't
  // installed" — which popped LlamaInstallerOverlay's full-screen modal on
  // top of the whole app, with no way back to Settings to fix a bad Remote
  // Access URL. It must let a connection failure surface as an AsyncError
  // instead, which LlamaInstallerOverlay's existing `error` branch already
  // renders as nothing.
  test(
    'llamaInstalledProvider surfaces a connection failure as an error, not a false "not installed"',
    () async {
      final client = MemoApiClient(baseUrl: 'http://192.168.1.106:8090');
      client.dio.httpClientAdapter = _ConnectionRefusedAdapter();

      final container = ProviderContainer(
        overrides: [apiClientProvider.overrideWithValue(client)],
      );
      addTearDown(container.dispose);

      await expectLater(
        container.read(llamaInstalledProvider.future),
        throwsA(isA<DioException>()),
      );
    },
  );
}
