import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/models/provider_models.dart';

void main() {
  group('model selection keys', () {
    test('round-trip a provider and a model', () {
      final key = encodeModelSelection('Subscriptions', 'claude-sonnet-4-6');
      final back = decodeModelSelection(key);
      expect(back, isNotNull);
      expect(back!.provider, 'Subscriptions');
      expect(back.model, 'claude-sonnet-4-6');
    });

    test('a model id containing slashes or colons survives', () {
      final key = encodeModelSelection('Subscriptions', 'vendor/model:v2');
      expect(decodeModelSelection(key)!.model, 'vendor/model:v2');
    });

    test('an ordinary provider name is not a model selection', () {
      expect(decodeModelSelection('Subscriptions'), isNull);
      expect(decodeModelSelection('local'), isNull);
      expect(decodeModelSelection(''), isNull);
    });

    test('malformed keys are rejected, not half-parsed', () {
      expect(decodeModelSelection('${kModelSelectPrefix}onlyprovider'), isNull);
      expect(decodeModelSelection('$kModelSelectPrefix\u0001model'), isNull);
      expect(decodeModelSelection('${kModelSelectPrefix}p\u0001'), isNull);
      expect(decodeModelSelection('${kModelSelectPrefix}p\u0001m\u0001extra'), isNull);
    });
  });

  group('ProviderModelList.fromJson', () {
    test('parses the backend response', () {
      final l = ProviderModelList.fromJson({
        'models': [
          {'id': 'claude-sonnet-4-6', 'owned_by': 'antigravity'},
          {'id': 'gpt-oss-120b-medium'},
        ],
        'current': 'claude-sonnet-4-6',
      });
      expect(l.models.map((m) => m.id), ['claude-sonnet-4-6', 'gpt-oss-120b-medium']);
      expect(l.models.first.ownedBy, 'antigravity');
      expect(l.models.last.ownedBy, '');
      expect(l.current, 'claude-sonnet-4-6');
      expect(l.error, '');
    });

    test('survives an unexpected payload instead of throwing', () {
      expect(ProviderModelList.fromJson({'models': 'nope'}).models, isEmpty);
      expect(ProviderModelList.fromJson({'models': [1, null, 'x', {'owned_by': 'y'}]}).models, isEmpty);
      expect(ProviderModelList.fromJson({}).models, isEmpty);
    });

    test('carries the backend error through', () {
      expect(ProviderModelList.fromJson({'models': [], 'error': 'boom'}).error, 'boom');
    });
  });

  test('vendorLabel names the known vendors and keeps unknown ones', () {
    expect(vendorLabel('antigravity'), 'Antigravity');
    expect(vendorLabel('claude'), 'Claude');
    expect(vendorLabel('codex'), 'Codex');
    expect(vendorLabel('xai'), 'Xai');
    expect(vendorLabel(''), '');
  });

  group('remaining allowance', () {
    test('is parsed from the model list and shown as a whole percentage', () {
      final m = ProviderModel.fromJson({
        'id': 'claude-sonnet-4-6',
        'remaining': 0.2549,
        'reset_at': '2026-10-13T20:54:16Z',
      });
      expect(m.remaining, closeTo(0.2549, 1e-9));
      expect(m.remainingPercent, 25);
      expect(m.resetAt, '2026-10-13T20:54:16Z');
    });

    test('absent means unknown, which is not the same as empty', () {
      final m = ProviderModel.fromJson({'id': 'gpt-5'});
      expect(m.remaining, isNull);
      expect(m.remainingPercent, isNull);
      expect(ProviderModel.fromJson({'id': 'x', 'remaining': 0}).remainingPercent, 0);
    });

    test('a sliver left never rounds down to "0%" and the value is clamped', () {
      expect(ProviderModel.fromJson({'id': 'x', 'remaining': 0.002}).remainingPercent, 1);
      expect(ProviderModel.fromJson({'id': 'x', 'remaining': 1.4}).remainingPercent, 100);
      expect(ProviderModel.fromJson({'id': 'x', 'remaining': -0.3}).remainingPercent, 0);
    });
  });

  group('vendor labels', () {
    test('the sidecar lists Codex models under owned_by "openai"', () {
      expect(vendorLabel('openai'), 'Codex');
      expect(vendorLabel('codex'), 'Codex');
      expect(vendorLabel('antigravity'), 'Antigravity');
      expect(vendorLabel(''), '');
    });
  });
}
