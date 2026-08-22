import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/models/provider_config.dart';
import 'package:memo_flutter/widgets/settings/tabs/providers_tab.dart';

// Direct user feedback: the API Providers list looked cluttered with every
// known provider type shown as a disabled placeholder card, none of which
// the user had ever actually added. visibleProviders() is the fix on the
// display side (internal/provider/config.go's defaultConfigs() is the
// matching backend fix for fresh installs — this covers already-existing,
// already-cluttered installs without a destructive migration).
void main() {
  test('hides an untouched placeholder (disabled, no key, no base URL)', () {
    const placeholder = ProviderConfig(
      type: 'openai',
      name: 'OpenAI',
      model: 'gpt-4o',
      enabled: false,
    );
    expect(visibleProviders([placeholder]), isEmpty);
  });

  test('keeps an enabled provider even with no API key set', () {
    const p = ProviderConfig(
      type: 'ollama',
      name: 'Ollama',
      model: 'llama3',
      enabled: true,
    );
    expect(visibleProviders([p]), [p]);
  });

  test('keeps a disabled provider that has a real API key', () {
    const p = ProviderConfig(
      type: 'openai',
      name: 'OpenAI',
      model: 'gpt-4o',
      enabled: false,
      apiKey: 'sk-real-key',
    );
    expect(visibleProviders([p]), [p]);
  });

  test('keeps a disabled provider that has a custom base URL', () {
    const p = ProviderConfig(
      type: 'custom',
      name: 'My Proxy',
      model: 'some-model',
      enabled: false,
      baseUrl: 'https://my-proxy.example.com/v1',
    );
    expect(visibleProviders([p]), [p]);
  });

  test('an empty API key string does not count as "has a key"', () {
    const p = ProviderConfig(
      type: 'openai',
      name: 'OpenAI',
      model: 'gpt-4o',
      enabled: false,
      apiKey: '',
    );
    expect(visibleProviders([p]), isEmpty);
  });

  test('filters a mixed list down to only the actually-configured entries', () {
    const untouched = ProviderConfig(
        type: 'gemini', name: 'Google Gemini', model: 'gemini-2.0-flash');
    const configured = ProviderConfig(
      type: 'claude',
      name: 'Anthropic Claude',
      model: 'claude-sonnet-4-20250514',
      enabled: true,
      apiKey: 'sk-ant-real',
    );
    final result = visibleProviders([untouched, configured]);
    expect(result, [configured]);
  });
}
