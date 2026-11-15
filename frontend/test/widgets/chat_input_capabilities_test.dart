import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/models/provider_config.dart';
import 'package:memo_flutter/providers/auth_gate_provider.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/models_provider.dart';
import 'package:memo_flutter/providers/provider_provider.dart';
import 'package:memo_flutter/providers/settings_provider.dart';
import 'package:memo_flutter/widgets/chat_input.dart';
import 'package:shared_preferences/shared_preferences.dart';

class _TestWhisper extends WhisperEnabledNotifier {
  final bool value;
  _TestWhisper(this.value);
  @override
  Future<bool> build() async => value;
}

class _TestActiveProvider extends ActiveProviderNotifier {
  final String value;
  _TestActiveProvider(this.value);
  @override
  Future<String> build() async => value;
}

class _TestProviders extends ProviderListNotifier {
  final List<ProviderConfig> items;
  _TestProviders(this.items);
  @override
  Future<List<ProviderConfig>> build() async => items;
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  Future<void> pumpComposer(
    WidgetTester tester, {
    required bool whisperEnabled,
    required String activeProvider,
    required List<ProviderConfig> providers,
  }) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    final client = MemoApiClient(baseUrl: 'http://memo.test');

    await tester.pumpWidget(
      ProviderScope(
        key: UniqueKey(),
        overrides: [
          prefsProvider.overrideWithValue(prefs),
          apiClientProvider.overrideWithValue(client),
          authGateProvider.overrideWith(
            (ref) => Stream.value(const AuthGateInfo(AuthGateState.ok)),
          ),
          whisperEnabledProvider.overrideWith(() => _TestWhisper(whisperEnabled)),
          activeProviderTypeProvider.overrideWith(() => _TestActiveProvider(activeProvider)),
          providerListProvider.overrideWith(() => _TestProviders(providers)),
          modelStatusOnceProvider.overrideWith((ref) async => null),
        ],
        child: const MaterialApp(
          home: Scaffold(
            body: Column(
              mainAxisAlignment: MainAxisAlignment.end,
              children: [ChatInput()],
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
  }

  testWidgets('hides mic when whisper is off, shows mic when whisper is on', (tester) async {
    await pumpComposer(
      tester,
      whisperEnabled: false,
      activeProvider: 'OpenAI',
      providers: const [
        ProviderConfig(type: 'openai', name: 'OpenAI', model: 'gpt-4o', enabled: true),
      ],
    );
    expect(find.byIcon(Icons.mic_outlined), findsNothing);
    expect(find.byIcon(Icons.image_outlined), findsOneWidget);
    expect(find.byIcon(Icons.attach_file), findsOneWidget);

    await pumpComposer(
      tester,
      whisperEnabled: true,
      activeProvider: 'OpenAI',
      providers: const [
        ProviderConfig(type: 'openai', name: 'OpenAI', model: 'gpt-4o', enabled: true),
      ],
    );
    expect(find.byIcon(Icons.mic_outlined), findsOneWidget);
  });

  testWidgets('hides image button for non-vision custom model', (tester) async {
    await pumpComposer(
      tester,
      whisperEnabled: false,
      activeProvider: 'Custom',
      providers: const [
        ProviderConfig(
          type: 'custom',
          name: 'Custom',
          model: '/models/Moonlight-16B-A3B-Instruct-IQ4_NL.gguf',
          enabled: true,
        ),
      ],
    );
    expect(find.byIcon(Icons.image_outlined), findsNothing);
    expect(find.byIcon(Icons.attach_file), findsOneWidget);
  });
}
