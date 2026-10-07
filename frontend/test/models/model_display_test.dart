import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/models/model_display.dart';

void main() {
  // The twelve ids the Antigravity account really lists (2026-10-07).
  const cases = <String, List<String>>{
    'gpt-oss-120b-medium': ['GPT-OSS 120B', 'Medium'],
    'gemini-pro-agent': ['Gemini Pro', 'Agent'],
    'gemini-3.8-flash-high': ['Gemini 3.8 Flash', 'High'],
    'gemini-3.5-flash-lite': ['Gemini 3.5 Flash Lite', ''],
    'gemini-3.1-pro-low': ['Gemini 3.1 Pro', 'Low'],
    'gemini-3.1-flash-image': ['Gemini 3.1 Flash Image', ''],
    'gemini-3-flash': ['Gemini 3 Flash', ''],
    'claude-sonnet-4-6': ['Claude Sonnet 4.6', ''],
    'claude-opus-4-6-thinking': ['Claude Opus 4.6', 'Thinking'],
  };

  group('prettyModelName', () {
    cases.forEach((id, want) {
      test(id, () {
        final d = prettyModelName(id);
        expect(d.title, want[0]);
        expect(d.tag, want[1]);
      });
    });

    test('label joins the title and the tag', () {
      expect(prettyModelName('claude-opus-4-6-thinking').label, 'Claude Opus 4.6 · Thinking');
      expect(prettyModelName('claude-sonnet-4-6').label, 'Claude Sonnet 4.6');
    });

    test('only the model part of a vendor/model id is reshaped', () {
      expect(prettyModelName('anthropic/claude-sonnet-4-6').title, 'Claude Sonnet 4.6');
    });

    test('an id it cannot read comes back unchanged instead of mangled', () {
      expect(prettyModelName('weird id with spaces!').title, 'weird id with spaces!');
      expect(prettyModelName('').title, '');
      expect(prettyModelName('  ').title, '');
    });

    test('a lone descriptor word is a name, not a tag', () {
      final d = prettyModelName('high');
      expect(d.title, 'High');
      expect(d.tag, '');
    });
  });

  group('modelFamily', () {
    test('picks the logo family', () {
      expect(modelFamily('claude-opus-4-6-thinking'), 'claude');
      expect(modelFamily('gemini-3-flash'), 'gemini');
      expect(modelFamily('gpt-oss-120b-medium'), 'openai');
      expect(modelFamily('o3-mini'), 'openai');
      expect(modelFamily('stealth/union-alpha'), isNull);
    });
  });
}
