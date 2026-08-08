import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/providers/settings_provider.dart';

/// Builds a ProviderContainer wired to a fresh, in-memory SharedPreferences
/// instance seeded with [initial] — no platform channel / network needed,
/// so these run as plain Dart logic tests.
Future<ProviderContainer> _containerWith(Map<String, Object> initial) async {
  SharedPreferences.setMockInitialValues(initial);
  final prefs = await SharedPreferences.getInstance();
  final container = ProviderContainer(
    overrides: [prefsProvider.overrideWithValue(prefs)],
  );
  return container;
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('SetupCompleteNotifier', () {
    test('defaults to false when nothing stored', () async {
      final container = await _containerWith({});
      addTearDown(container.dispose);

      expect(container.read(setupCompleteProvider), false);
    });

    test('reads a persisted true value on startup', () async {
      final container = await _containerWith({'memo_setup_complete': true});
      addTearDown(container.dispose);

      expect(container.read(setupCompleteProvider), true);
    });

    test('completeSetup() flips state and persists it', () async {
      final container = await _containerWith({});
      addTearDown(container.dispose);

      await container.read(setupCompleteProvider.notifier).completeSetup();

      expect(container.read(setupCompleteProvider), true);
      final prefs = container.read(prefsProvider);
      expect(prefs.getBool('memo_setup_complete'), true);
    });

    test('resetSetup() flips state back and persists it', () async {
      final container = await _containerWith({'memo_setup_complete': true});
      addTearDown(container.dispose);

      await container.read(setupCompleteProvider.notifier).resetSetup();

      expect(container.read(setupCompleteProvider), false);
      final prefs = container.read(prefsProvider);
      expect(prefs.getBool('memo_setup_complete'), false);
    });
  });

  group('BackendUrlNotifier', () {
    // Regression: MemoApiClient's Dio instance validates baseUrl eagerly
    // and synchronously — a bare "127.0.0.1" (no scheme) used to crash the
    // entire app with Flutter's red error screen the moment apiClientProvider
    // built, before any UI (including the screens meant to fix a bad
    // address) could render.
    test('self-heals an already-saved schemeless value on construction', () async {
      final container = await _containerWith({'memo_api_base_url': '127.0.0.1'});
      addTearDown(container.dispose);

      expect(container.read(backendUrlProvider), 'http://127.0.0.1:8090');
    });

    test('save() normalizes a schemeless, portless address before persisting', () async {
      final container = await _containerWith({});
      addTearDown(container.dispose);

      await container.read(backendUrlProvider.notifier).save('192.168.1.106');

      expect(container.read(backendUrlProvider), 'http://192.168.1.106:8090');
      final prefs = container.read(prefsProvider);
      expect(prefs.getString('memo_api_base_url'), 'http://192.168.1.106:8090');
    });

    test('save() respects an explicit port instead of defaulting it', () async {
      final container = await _containerWith({});
      addTearDown(container.dispose);

      await container.read(backendUrlProvider.notifier).save('192.168.1.106:1234');

      expect(container.read(backendUrlProvider), 'http://192.168.1.106:1234');
    });

    test('save("") clears the pref entirely and resets to the local default', () async {
      final container = await _containerWith({'memo_api_base_url': 'http://192.168.1.106:8090'});
      addTearDown(container.dispose);

      await container.read(backendUrlProvider.notifier).save('');

      expect(container.read(backendUrlProvider), 'http://127.0.0.1:8090');
      final prefs = container.read(prefsProvider);
      expect(prefs.containsKey('memo_api_base_url'), isFalse);
    });
  });

  group('LaunchpadSeenNotifier', () {
    test('defaults to false with forceShow false', () async {
      final container = await _containerWith({});
      addTearDown(container.dispose);

      expect(container.read(launchpadSeenProvider), false);
      expect(container.read(launchpadSeenProvider.notifier).forceShow, false);
    });

    test('reset() sets state false and forceShow true (re-show the tour)', () async {
      final container = await _containerWith({'memo_launchpad_seen': true});
      addTearDown(container.dispose);

      final notifier = container.read(launchpadSeenProvider.notifier);
      await notifier.reset();

      expect(container.read(launchpadSeenProvider), false);
      expect(notifier.forceShow, true);
      final prefs = container.read(prefsProvider);
      expect(prefs.getBool('memo_launchpad_seen'), false);
    });

    test('markSeen() sets state true and clears forceShow', () async {
      final container = await _containerWith({});
      addTearDown(container.dispose);

      final notifier = container.read(launchpadSeenProvider.notifier);
      await notifier.reset(); // simulate "reset from Settings" happening first
      await notifier.markSeen();

      expect(container.read(launchpadSeenProvider), true);
      expect(notifier.forceShow, false);
      final prefs = container.read(prefsProvider);
      expect(prefs.getBool('memo_launchpad_seen'), true);
    });
  });

  group('TourSeenNotifier', () {
    test('defaults to false', () async {
      final container = await _containerWith({});
      addTearDown(container.dispose);

      expect(container.read(tourSeenProvider), false);
    });

    test('markSeen() then resetTour() round-trips through persisted state', () async {
      final container = await _containerWith({});
      addTearDown(container.dispose);

      final notifier = container.read(tourSeenProvider.notifier);
      final prefs = container.read(prefsProvider);

      await notifier.markSeen();
      expect(container.read(tourSeenProvider), true);
      expect(prefs.getBool('memo_tour_seen'), true);

      await notifier.resetTour();
      expect(container.read(tourSeenProvider), false);
      expect(prefs.getBool('memo_tour_seen'), false);
    });
  });
}
