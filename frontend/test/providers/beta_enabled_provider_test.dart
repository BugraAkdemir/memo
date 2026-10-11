import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/providers/settings_provider.dart';

/// betaEnabledProvider gates the beta-only UI: Swarm, and now the browser
/// panel's entry points (the globe button, the overflow-menu row, the pane).
/// The rule it must keep is the one Swarm learned the hard way: the backend's
/// answer wins once it has arrived, and the local mirror only stands in until
/// then. A failed fetch becomes {'enabled': false} — an answer with no `beta`
/// key — and must count as "not arrived", not as "off".
Future<ProviderContainer> _container({
  required bool mirror,
  required Future<Map<String, dynamic>> Function() backend,
}) async {
  SharedPreferences.setMockInitialValues({'memo_beta_features': mirror});
  final prefs = await SharedPreferences.getInstance();
  final container = ProviderContainer(overrides: [
    prefsProvider.overrideWithValue(prefs),
    remoteAccessProvider.overrideWith((ref) => backend()),
  ]);
  addTearDown(container.dispose);
  return container;
}

void main() {
  test('the backend saying beta:false beats a stale local true', () async {
    final c = await _container(mirror: true, backend: () async => {'beta': false});
    await c.read(remoteAccessProvider.future);
    expect(c.read(betaEnabledProvider), isFalse);
  });

  test('the backend saying beta:true beats a local false', () async {
    final c = await _container(mirror: false, backend: () async => {'beta': true});
    await c.read(remoteAccessProvider.future);
    expect(c.read(betaEnabledProvider), isTrue);
  });

  test('before the backend has answered, the local mirror stands in', () async {
    // A request that never answers (a Completer, not a long delay: a pending
    // Timer would fail the test on its own).
    final never = Completer<Map<String, dynamic>>();
    final c = await _container(mirror: true, backend: () => never.future);
    expect(c.read(betaEnabledProvider), isTrue);
  });

  test('an answer without a beta key (a swallowed failure) is not an answer', () async {
    final c = await _container(mirror: true, backend: () async => {'enabled': false});
    await c.read(remoteAccessProvider.future);
    expect(c.read(betaEnabledProvider), isTrue,
        reason: 'no beta key means the server has not said anything about beta');
  });

  test('off everywhere is off', () async {
    final c = await _container(mirror: false, backend: () async => {'enabled': false});
    await c.read(remoteAccessProvider.future);
    expect(c.read(betaEnabledProvider), isFalse);
  });
}
