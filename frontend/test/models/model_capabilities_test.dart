import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/models/gpu_info.dart';
import 'package:memo_flutter/models/local_model.dart';
import 'package:memo_flutter/models/model_capabilities.dart';
import 'package:memo_flutter/models/provider_config.dart';
import 'package:memo_flutter/models/provider_models.dart';

void main() {
  group('resolveModelInputCapabilities', () {
    test('hides both image and file when orchestra is enabled', () {
      final caps = resolveModelInputCapabilities(
        activeProvider: 'OpenAI',
        cliType: '',
        providers: const [
          ProviderConfig(
            type: 'openai',
            name: 'OpenAI',
            model: 'gpt-4o',
            enabled: true,
          ),
        ],
        localStatus: null,
        localModels: const [],
        orchestraEnabled: true,
      );
      expect(caps.supportsImages, isFalse);
      expect(caps.supportsFiles, isFalse);
    });

    test('local running model without mmproj/vision hides image but keeps file', () {
      final caps = resolveModelInputCapabilities(
        activeProvider: '',
        cliType: '',
        providers: const [],
        localStatus: const ServerStatus(
          running: true,
          modelPath: '/models/phi-3-mini.gguf',
        ),
        localModels: const [
          LocalModel(
            repoId: '',
            filename: 'phi-3-mini.gguf',
            size: 2300,
            path: '/models/phi-3-mini.gguf',
            isEmbedding: false,
            isVision: false,
            supportsVision: false,
          ),
        ],
        orchestraEnabled: false,
      );
      expect(caps.supportsImages, isFalse);
      expect(caps.supportsFiles, isTrue);
    });

    test('local running model with mmproj supports both image and file', () {
      final caps = resolveModelInputCapabilities(
        activeProvider: '',
        cliType: '',
        providers: const [],
        localStatus: const ServerStatus(
          running: true,
          modelPath: '/models/gemma-4.gguf',
        ),
        localModels: const [
          LocalModel(
            repoId: '',
            filename: 'gemma-4.gguf',
            size: 10000,
            path: '/models/gemma-4.gguf',
            isEmbedding: false,
            isVision: true,
          ),
        ],
        orchestraEnabled: false,
      );
      expect(caps.supportsImages, isTrue);
      expect(caps.supportsFiles, isTrue);
    });

    test('custom provider running Moonlight or DeepSeek-Coder hides image', () {
      for (final model in [
        '/home/user/models/Moonlight-16B-A3B-Instruct-IQ4_NL.gguf',
        '/home/user/models/DeepSeek-Coder-V2-Lite-Instruct-Q4_K_S.gguf',
      ]) {
        final caps = resolveModelInputCapabilities(
          activeProvider: 'Özel (OpenAI uyumlu)',
          cliType: '',
          providers: [
            ProviderConfig(
              type: 'custom',
              name: 'Özel (OpenAI uyumlu)',
              model: model,
              enabled: true,
            ),
          ],
          localStatus: null,
          localModels: const [],
          orchestraEnabled: false,
        );
        expect(caps.supportsImages, isFalse, reason: model);
        expect(caps.supportsFiles, isTrue, reason: model);
      }
    });

    test('subscriptions/cloud vision models support image and file', () {
      for (final model in [
        'claude-sonnet-4-6',
        'gemini-3.8-flash-high',
        'gpt-4o',
      ]) {
        final caps = resolveModelInputCapabilities(
          activeProvider: kSubscriptionsProviderName,
          cliType: '',
          providers: [
            ProviderConfig(
              type: 'custom',
              name: kSubscriptionsProviderName,
              model: model,
              enabled: true,
            ),
          ],
          localStatus: null,
          localModels: const [],
          orchestraEnabled: false,
        );
        expect(caps.supportsImages, isTrue, reason: model);
        expect(caps.supportsFiles, isTrue, reason: model);
      }
    });

    test('auto-route supports both image and file', () {
      final caps = resolveModelInputCapabilities(
        activeProvider: kAutoRouteProviderName,
        cliType: '',
        providers: const [],
        localStatus: null,
        localModels: const [],
        orchestraEnabled: false,
      );
      expect(caps.supportsImages, isTrue);
      expect(caps.supportsFiles, isTrue);
    });
  });
}
