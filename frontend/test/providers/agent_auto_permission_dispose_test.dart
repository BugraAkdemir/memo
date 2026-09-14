import 'dart:async';
import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/providers/agent_provider.dart';
import 'package:memo_flutter/providers/chat_provider.dart';

/// A fake [HttpClientAdapter] that answers GET
/// /api/agent/auto-permission only once [gate] completes — lets the test
/// control exactly when the in-flight request resolves, so it can dispose
/// the provider while the GET is still pending.
class _GatedAdapter implements HttpClientAdapter {
  final Future<void> gate;
  _GatedAdapter(this.gate);

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<List<int>>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    await gate;
    if (options.method == 'GET' &&
        options.path == '/api/agent/auto-permission') {
      return ResponseBody.fromString(
        jsonEncode({'enabled': true}),
        200,
        headers: {
          Headers.contentTypeHeader: [Headers.jsonContentType],
        },
      );
    }
    return ResponseBody.fromString('not found', 404);
  }

  @override
  void close({bool force = false}) {}
}

// Regression test for the P1 finding that AgentAutoPermissionNotifier wrote
// `state` after dispose with no `mounted` guard — unlike its sibling
// AgentEnabledNotifier, which was already fixed for exactly this crash.
// app_shell.dart's centralized auth-gate listener invalidates
// agentAutoPermissionProvider the moment the gate transitions, which can
// dispose this notifier while its init GET is still in flight.
//
// _init()'s own try/catch swallows the resulting StateError before it can
// fail the test outright (that's the "silent" half of the bug — it never
// crashes the app, just prints an error) — so the meaningful assertion here
// is on debugPrint output, not a thrown exception: this must produce ZERO
// "after dispose" errors, whereas the pre-fix code reliably prints
// "Tried to use AgentAutoPermissionNotifier after `dispose` was called" in
// this exact scenario (confirmed via git stash).
void main() {
  test('disposing while the init GET is in flight logs no after-dispose error', () async {
    final gate = Completer<void>();
    final client = MemoApiClient(baseUrl: 'http://memo.test');
    client.dio.httpClientAdapter = _GatedAdapter(gate.future);

    final container = ProviderContainer(
      overrides: [apiClientProvider.overrideWithValue(client)],
    );

    final messages = <String>[];
    final originalDebugPrint = debugPrint;
    debugPrint = (String? message, {int? wrapWidth}) {
      if (message != null) messages.add(message);
    };

    try {
      // Establishes the notifier and starts its constructor-triggered
      // _init() GET, which is now blocked on `gate`.
      container.read(agentAutoPermissionProvider);

      // Dispose the container (and therefore the notifier) while that GET
      // is still pending — mirrors app_shell.dart's ref.invalidate landing
      // mid-request on a real auth-gate transition.
      container.dispose();

      // Let the gated GET resolve now that the notifier is already
      // disposed.
      gate.complete();
      await Future.delayed(const Duration(milliseconds: 20));
    } finally {
      debugPrint = originalDebugPrint;
    }

    final afterDisposeErrors =
        messages.where((m) => m.contains('after `dispose` was called'));
    expect(
      afterDisposeErrors,
      isEmpty,
      reason: 'AgentAutoPermissionNotifier wrote state after dispose with no '
          'mounted guard: $afterDisposeErrors',
    );
  });
}
