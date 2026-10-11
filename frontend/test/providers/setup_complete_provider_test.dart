import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/settings_provider.dart';

/// Answers GET /api/onboarding from a script: `true`/`false` is the server's
/// answer, `null` is a failed request.
class _Api extends MemoApiClient {
  _Api(this.script) : super(baseUrl: 'http://memo.test');
  final List<bool?> script;
  int calls = 0;
  @override
  Future<bool> getOnboardingComplete() async {
    final r = script[calls < script.length ? calls : script.length - 1];
    calls++;
    if (r == null) throw Exception('401');
    return r;
  }
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  Future<(ProviderContainer, _Api)> make(List<bool?> script,
      {Map<String, Object> prefs = const {}}) async {
    SharedPreferences.setMockInitialValues(prefs);
    final p = await SharedPreferences.getInstance();
    final api = _Api(script);
    final c = ProviderContainer(overrides: [
      prefsProvider.overrideWithValue(p),
      apiClientProvider.overrideWithValue(api),
    ]);
    addTearDown(c.dispose);
    return (c, api);
  }

  test('a server that says "complete" corrects a fresh phone\'s local false',
      () async {
    final (c, _) = await make([true]);
    expect(c.read(setupCompleteProvider), isFalse,
        reason: 'the local preference seeds false on a fresh install');
    expect(c.read(onboardingCheckedProvider), isFalse);

    await Future<void>.delayed(const Duration(milliseconds: 20));

    expect(c.read(setupCompleteProvider), isTrue);
    expect(c.read(onboardingCheckedProvider), isTrue);
    final prefs = await SharedPreferences.getInstance();
    expect(prefs.getBool('memo_setup_complete'), isTrue);
  });

  test('a server that says "not complete" still marks the answer as known',
      () async {
    final (c, _) = await make([false]);
    c.read(setupCompleteProvider);
    await Future<void>.delayed(const Duration(milliseconds: 20));
    expect(c.read(setupCompleteProvider), isFalse);
    expect(c.read(onboardingCheckedProvider), isTrue);
  });

  test('a failed read leaves the answer unknown rather than claiming one',
      () async {
    final (c, _) = await make([null]);
    c.read(setupCompleteProvider);
    await Future<void>.delayed(const Duration(milliseconds: 20));
    expect(c.read(onboardingCheckedProvider), isFalse);
  });

  testWidgets('on a phone a failed read is retried, and the retry stops once answered',
      (tester) async {
    // flutter_test reports android by default: a phone.
    final (c, api) = await make([null, true]);
    c.read(setupCompleteProvider);
    await tester.pump();
    expect(api.calls, 1);
    expect(c.read(onboardingCheckedProvider), isFalse);

    await tester.pump(const Duration(seconds: 5));
    await tester.pump();
    expect(api.calls, 2);
    expect(c.read(setupCompleteProvider), isTrue);
    expect(c.read(onboardingCheckedProvider), isTrue);

    // Answered: no further requests.
    await tester.pump(const Duration(minutes: 1));
    expect(api.calls, 2);
  });

  testWidgets('disposing the notifier cancels a pending retry (no leaked timer)',
      (tester) async {
    final (c, api) = await make([null]);
    c.read(setupCompleteProvider);
    await tester.pump();
    expect(api.calls, 1);
    c.dispose();
    // A retry left armed would be reported by flutter_test as a pending
    // timer when the test ends, and would fire after dispose.
    await tester.pump(const Duration(seconds: 30));
    expect(api.calls, 1);
  });
}
