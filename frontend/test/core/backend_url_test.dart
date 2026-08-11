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

  group('webBackendUrl (web build: page origin is the sane default)', () {
    const lanPage = 'http://192.168.1.106:8090';
    const loopbackPage = 'http://127.0.0.1:8090';

    test('no saved URL -> the page\'s own origin', () {
      expect(webBackendUrl('', lanPage), lanPage);
      expect(webBackendUrl('   ', lanPage), lanPage);
      expect(webBackendUrl('', loopbackPage), loopbackPage);
    });

    test('stale saved loopback URL on a LAN-served page is ignored', () {
      // The exact user-reported failure: page loaded from the LAN address
      // while a previous localhost session left http://127.0.0.1:8090
      // saved — every API call went to the client's own loopback and got
      // CORS-blocked ("Cross-Origin Request Blocked ... 127.0.0.1").
      expect(webBackendUrl('http://127.0.0.1:8090', lanPage), lanPage);
      expect(webBackendUrl('http://localhost:8090', lanPage), lanPage);
    });

    test('saved loopback URL on a loopback-served page is kept', () {
      expect(webBackendUrl('http://127.0.0.1:8090', loopbackPage),
          'http://127.0.0.1:8090');
    });

    test('a deliberately different server is respected', () {
      expect(webBackendUrl('http://192.168.1.50:8090', lanPage),
          'http://192.168.1.50:8090');
      expect(webBackendUrl('192.168.1.50', lanPage), 'http://192.168.1.50:8090');
    });

    test('bare host with no scheme or port still gets both defaults', () {
      expect(webBackendUrl('127.0.0.1', lanPage), lanPage);
    });
  });
}
