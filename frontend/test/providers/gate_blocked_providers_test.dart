import 'dart:async';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/providers/agent_provider.dart';
import 'package:memo_flutter/providers/auth_gate_provider.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/models_provider.dart';
import 'package:memo_flutter/providers/orchestra_provider.dart';
import 'package:memo_flutter/providers/provider_provider.dart';
import 'package:memo_flutter/providers/settings_provider.dart';
import 'package:memo_flutter/providers/skill_provider.dart';
import 'package:memo_flutter/providers/swarm_provider.dart';
import 'package:memo_flutter/providers/tasklist_provider.dart';

/// Answers every request with 401 and counts calls per path — a token-gated
/// backend's exact behavior while the login/setup gate is still up.
class _UnauthorizedAdapter implements HttpClientAdapter {
  final Map<String, int> calls = {};

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    calls[options.path] = (calls[options.path] ?? 0) + 1;
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

/// Answers 500 for one specific path, 200/empty for everything else —
/// models a genuine transient failure on a single ambiently-watched
/// endpoint while the rest of the (gate-open) app works normally.
class _FailingPathAdapter implements HttpClientAdapter {
  _FailingPathAdapter(this.failingPath);
  final String failingPath;
  final Map<String, int> calls = {};

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    calls[options.path] = (calls[options.path] ?? 0) + 1;
    if (options.path == failingPath) {
      return ResponseBody.fromString('{"error":"boom"}', 500, headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      });
    }
    return ResponseBody.fromString('{}', 200, headers: {
      Headers.contentTypeHeader: [Headers.jsonContentType],
    });
  }

  @override
  void close({bool force = false}) {}
}

// BUG-ONB6, systemic pass: reported live by the user across Settings and
// Developer Options, not just the chat screen — audited with codebase-
// memory and found in every AsyncNotifier below (plus swarmStatusProvider,
// a StateNotifier). Same root cause each time: a one-shot provider that
// calls the API directly in build()/its constructor with no auth-gate
// check, so a single attempt landing while the gate is still up 401s and
// gets permanently cached (as an AsyncError, or — for a couple that already
// had a try/catch — as an errorMessageProvider SnackBar toast fired on
// every connect). Fixed uniformly: check authGateBlocked() first and mount
// a safe default; app_shell.dart's centralized gate-transition listener
// invalidates all of them once the gate actually opens.
void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('no gate-sensitive provider touches the backend while the gate is blocked',
      () async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    final adapter = _UnauthorizedAdapter();
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

    final llama = await container.read(llamaSettingsProvider.future);
    final systemPrompt = await container.read(systemPromptProvider.future);
    final memoryFiles = await container.read(memoryFilesProvider.future);
    final memorySettings = await container.read(memorySettingsProvider.future);
    final usageStats = await container.read(usageStatsProvider.future);
    final devGateway = await container.read(devGatewayConfigProvider.future);
    final memoryEnabled = await container.read(memoryEnabledProvider.future);
    final minimalMode = await container.read(minimalModeProvider.future);
    final minimalModeOverrides =
        await container.read(minimalModeOverridesProvider.future);
    final agentPermissions = await container.read(agentPermissionsProvider.future);
    final skills = await container.read(skillListProvider.future);
    final localModels = await container.read(localModelsProvider.future);
    final taskLists = await container.read(taskListsProvider.future);
    final providers = await container.read(providerListProvider.future);
    final activeProvider = await container.read(activeProviderTypeProvider.future);
    final orchestra = await container.read(orchestraConfigProvider.future);
    // BUG-ONB11 additions: plain FutureProviders, the shape the original
    // BUG-ONB6 audit (AsyncNotifier.build) could not see. Both back an
    // IndexedStack screen built at app start, and neither has a retry loop
    // of its own, so a 401 here is permanent.
    final gatewayModels = await container.read(gatewayModelsProvider.future);
    final gpuInfo = await container.read(gpuInfoProvider.future);

    // Safe defaults, not errors.
    expect(llama.engineMode, 'auto');
    expect(systemPrompt, '');
    expect(memoryFiles, isEmpty);
    expect(memorySettings.topK, 5);
    expect(usageStats.totalRequests, 0);
    expect(devGateway.requireAPIKey, false);
    expect(memoryEnabled, true);
    expect(minimalMode, false);
    expect(minimalModeOverrides.keepPersona, false);
    expect(agentPermissions, isEmpty);
    expect(skills, isEmpty);
    expect(localModels, isEmpty);
    expect(taskLists, isEmpty);
    expect(providers, isEmpty);
    expect(activeProvider, '');
    expect(orchestra.enabled, false);
    expect(gatewayModels, isEmpty);
    expect(gpuInfo.ramTotalMb, 0);

    // The one and only assertion that actually matters: zero requests ever
    // reached the (401-answering) backend, for any of them.
    expect(adapter.calls, isEmpty,
        reason: 'no gate-sensitive provider should attempt a request while '
            'the login/setup gate is still up — got: ${adapter.calls}');
  });

  // Reported live (2026-08-13): even with the gate open, orchestraConfigProvider's
  // build() rethrew any fetch failure into errorMessageProvider — but it's
  // watched ambiently by the always-visible engine strip and chat input bar
  // (engine_strip.dart, chat_input.dart), not just the Orchestra dialog/
  // settings tab a user chose to open. A transient failure (flaky LAN link
  // to a self-hosted backend, a momentary 500) surfaced a SnackBar reading
  // "Orchestra yapılandırması alınamadı" with no relation to anything the
  // user was doing — including while Orchestra itself was off. Fixed the
  // same way activeProviderTypeProvider/remoteAccessProvider already were:
  // degrade to the safe default silently, no toast.
  test('orchestraConfigProvider stays silent on a non-gate fetch failure', () async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    final adapter = _FailingPathAdapter('/api/orchestra/config');
    final client = MemoApiClient(baseUrl: 'http://memo.test');
    client.dio.httpClientAdapter = adapter;

    final container = ProviderContainer(overrides: [
      apiClientProvider.overrideWithValue(client),
      prefsProvider.overrideWithValue(prefs),
      authGateProvider.overrideWith(
        (ref) => Stream.value(const AuthGateInfo(AuthGateState.ok)),
      ),
    ]);
    addTearDown(container.dispose);

    // Test trap (documented 2026-08-13, mood/Swarm default fix session):
    // authGateProvider's override stream hasn't delivered its first event
    // yet when build() runs synchronously on the very first read, so
    // authGateBlocked() sees null (== blocked) and orchestraConfigProvider
    // would short-circuit to the safe default *without ever calling the
    // API* — passing the assertions below for the wrong reason. Await the
    // gate's own future first so it's genuinely "ok" before exercising the
    // fetch-failure path.
    await container.read(authGateProvider.future);

    final orchestra = await container.read(orchestraConfigProvider.future);
    expect(orchestra.enabled, false);
    expect(adapter.calls['/api/orchestra/config'], greaterThan(0),
        reason: 'the fetch must actually have been attempted for this test '
            'to mean anything');
    expect(container.read(errorMessageProvider), isEmpty);
  });

  test('swarmStatusProvider (a StateNotifier, not AsyncNotifier) also stays blocked',
      () async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    final adapter = _UnauthorizedAdapter();
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

    // SwarmNotifier fetches synchronously in its constructor — give the
    // microtask queue a turn to let that settle before asserting.
    container.read(swarmStatusProvider);
    await Future<void>.delayed(Duration.zero);

    final status = container.read(swarmStatusProvider).valueOrNull;
    expect(status?.role, 'none');
    expect(adapter.calls, isEmpty);
  });
}
