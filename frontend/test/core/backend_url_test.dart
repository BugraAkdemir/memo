import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/backend_url.dart';

void main() {
  group('normalizeBackendUrl', () {
    test('bare host with no scheme or port gets both defaults', () {
      expect(normalizeBackendUrl('127.0.0.1'), 'http://127.0.0.1:8090');
    });

    test('bare LAN IP with no scheme or port gets both defaults', () {
      expect(normalizeBackendUrl('192.168.1.106'), 'http://192.168.1.106:8090');
    });

    test('an explicit port is always respected, never overridden', () {
      expect(normalizeBackendUrl('192.168.1.106:1234'), 'http://192.168.1.106:1234');
    });

    test('an explicit scheme is left alone, port default still applies', () {
      expect(normalizeBackendUrl('https://memo.example.com'), 'https://memo.example.com:8090');
    });

    test('scheme and port both explicit — passed through unchanged', () {
      expect(normalizeBackendUrl('http://127.0.0.1:8090'), 'http://127.0.0.1:8090');
    });

    test('trailing slash is stripped', () {
      expect(normalizeBackendUrl('http://127.0.0.1:8090/'), 'http://127.0.0.1:8090');
    });

    test('surrounding whitespace is trimmed', () {
      expect(normalizeBackendUrl('  127.0.0.1:8090  '), 'http://127.0.0.1:8090');
    });

    test('empty input falls back to the local default', () {
      expect(normalizeBackendUrl(''), 'http://127.0.0.1:8090');
      expect(normalizeBackendUrl('   '), 'http://127.0.0.1:8090');
    });
  });
}
