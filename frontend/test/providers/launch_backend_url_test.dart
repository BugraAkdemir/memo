import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/core/backend_url.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/settings_provider.dart';

// Also run this file with --dart-define=MEMO_BACKEND_URL=http://127.0.0.1:8090.
// That reproduces the dev launcher inheriting a stale saved remote address.
void main() {
  const launchUrl = String.fromEnvironment('MEMO_BACKEND_URL');
  const savedRemote = 'http://old-server.example.test:8090';

  Future<(SharedPreferences, ProviderContainer)> setup() async {
    SharedPreferences.setMockInitialValues({'memo_api_base_url': savedRemote});
    final prefs = await SharedPreferences.getInstance();
    final container = ProviderContainer(
      overrides: [prefsProvider.overrideWithValue(prefs)],
    );
    addTearDown(container.dispose);
    return (prefs, container);
  }

  test(
    'launch URL is temporary and explicit address edits reconnect to the saved choice',
    () async {
      final (prefs, container) = await setup();
      expect(
        container.read(apiClientProvider).baseUrl,
        launchUrl.isEmpty ? savedRemote : normalizeBackendUrl(launchUrl),
      );
      expect(
        container.read(backendUrlProvider),
        container.read(apiClientProvider).baseUrl,
      );
      expect(prefs.getString('memo_api_base_url'), savedRemote);
      await container.read(backendUrlProvider.notifier).save('192.0.2.20:9000');
      container.invalidate(apiClientProvider);
      expect(prefs.getString('memo_api_base_url'), 'http://192.0.2.20:9000');
      expect(container.read(backendUrlProvider), 'http://192.0.2.20:9000');
      expect(
        container.read(apiClientProvider).baseUrl,
        'http://192.0.2.20:9000',
      );
      await container.read(backendUrlProvider.notifier).save('');
      expect(prefs.containsKey('memo_api_base_url'), isFalse);
      expect(container.read(backendUrlProvider), normalizeBackendUrl(''));
    },
  );
}
