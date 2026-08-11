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

    // The one and only assertion that actually matters: zero requests ever
    // reached the (401-answering) backend, for any of them.
    expect(adapter.calls, isEmpty,
        reason: 'no gate-sensitive provider should attempt a request while '
            'the login/setup gate is still up — got: ${adapter.calls}');
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
