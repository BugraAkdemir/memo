import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/providers/auth_gate_provider.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/models_provider.dart';
import 'package:memo_flutter/providers/settings_provider.dart';

/// Answers /api/gpu with real hardware data, 401 on everything else — mimics
/// a token-gated backend that is genuinely up and would happily answer once
/// a credential is presented.
class _GpuAdapter implements HttpClientAdapter {
  int gpuCalls = 0;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    if (options.path == '/api/gpu') {
      gpuCalls++;
      return ResponseBody.fromString(
        '{"type":"cpu","name":"CPU","vram_mb":0,"recommended_layers":0,"description":"","ram_total_mb":16384}',
        200,
        headers: {
          Headers.contentTypeHeader: [Headers.jsonContentType],
        },
      );
    }
    return ResponseBody.fromString(
      '{"error":"unauthorized"}',
      401,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  // BUG-ONB5: gpuInfoProvider is a plain (non-polling) FutureProvider, so it
  // only ever runs its body once and caches the result. Before this fix it
  // used ref.read(apiClientProvider) with no gate check at all — the one
  // provider BUG-ONB4's fix missed, since that fix targeted the
  // StreamProvider polling loops (modelStatusProvider/embeddingStatusProvider
  // etc.) and gpuInfoProvider has a structurally different shape. If the
  // single attempt landed while the login gate was still up, /api/gpu 401'd,
  // the catch swallowed it into GPUInfo() (ramTotalMb: 0), and that "success"
  // was cached for the rest of the session — every screen reading RAM (setup
  // wizard's model recommendation, Model Store's hardware-fit badges) stayed
  // stuck recommending as if RAM were unknown, self-correcting only on a full
  // app reload (a fresh provider graph, by which point the gate had opened).
  group('gpuInfoProvider against a token-gated backend', () {
    test('while the gate is blocked, no request is attempted and RAM reads as unknown',
        () async {
      SharedPreferences.setMockInitialValues({});
      final prefs = await SharedPreferences.getInstance();
      final adapter = _GpuAdapter();
      final client = MemoApiClient(baseUrl: 'http://memo.test');
      client.dio.httpClientAdapter = adapter;

      final container = ProviderContainer(overrides: [
        apiClientProvider.overrideWithValue(client),
        prefsProvider.overrideWithValue(prefs),
        authGateProvider.overrideWith(
          (ref) => Stream.value(
            const AuthGateInfo(AuthGateState.loginNeeded, authMode: 'password'),
          ),
        ),
      ]);
      addTearDown(container.dispose);

      final gpu = await container.read(gpuInfoProvider.future);
      expect(gpu.ramTotalMb, 0,
          reason: 'gate is blocked — must not report a false "0 RAM" from a '
              'real 401, just the same unknown default');
      expect(adapter.gpuCalls, 0,
          reason: 'no point 401-ing a request the user cannot authorize yet');
    });

    test(
        'invalidating after the gate opens re-runs the provider for real, '
        'instead of staying stuck on its first (blocked) attempt', () async {
      SharedPreferences.setMockInitialValues({});
      final prefs = await SharedPreferences.getInstance();
      final adapter = _GpuAdapter();
      final client = MemoApiClient(baseUrl: 'http://memo.test');
      client.dio.httpClientAdapter = adapter;

      var gateState = const AuthGateInfo(AuthGateState.loginNeeded, authMode: 'password');
      final container = ProviderContainer(overrides: [
        apiClientProvider.overrideWithValue(client),
        prefsProvider.overrideWithValue(prefs),
        authGateProvider.overrideWith((ref) => Stream.value(gateState)),
      ]);
      addTearDown(container.dispose);

      // First read: gate blocked (e.g. app boot, before login) — must not
      // touch the backend at all.
      final blocked = await container.read(gpuInfoProvider.future);
      expect(blocked.ramTotalMb, 0);
      expect(adapter.gpuCalls, 0);

      // Gate opens (mirrors a successful login) — auth_gate_overlay.dart's
      // login paths call ref.invalidate(gpuInfoProvider) right alongside
      // ref.invalidate(authGateProvider) for exactly this reason: a plain
      // FutureProvider doesn't notice the gate changing on its own.
      gateState = const AuthGateInfo(AuthGateState.ok);
      container.invalidate(authGateProvider);
      // Let the gate's new stream value actually land before gpuInfoProvider
      // reads it — otherwise ref.read(authGateProvider) can still observe
      // the loading gap between invalidation and the new Stream.value(...)
      // delivering its event.
      await container.read(authGateProvider.future);
      container.invalidate(gpuInfoProvider);

      final resolved = await container.read(gpuInfoProvider.future);
      expect(resolved.ramTotalMb, 16384,
          reason: 'must reflect the real value once the gate is actually '
              'open — not stay permanently cached on the earlier blocked '
              'attempt\'s default');
      expect(adapter.gpuCalls, 1);
    });
  });
}
