import 'package:flutter/foundation.dart'
    show TargetPlatform, debugDefaultTargetPlatformOverride;
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

    test('an explicit http:// scheme is left alone, port default still applies', () {
      expect(normalizeBackendUrl('http://memo.example.com'), 'http://memo.example.com:8090');
    });

    // Ported from the retired mobile client, which reached Memo over
    // Tailscale Funnel routinely. Funnel serves over implicit 443, so
    // forcing Memo's own :8090 onto an https URL broke it outright — this
    // copy used to do exactly that.
    test('an explicit https:// scheme never gets a port forced onto it', () {
      expect(normalizeBackendUrl('https://memo.example.com'), 'https://memo.example.com');
    });

    test('an explicit port on an https URL is still respected', () {
      expect(normalizeBackendUrl('https://memo.example.com:8443'), 'https://memo.example.com:8443');
    });

    group('Tailscale Funnel hosts (*.ts.net)', () {
      test('a bare Funnel host gets https://, not http://, and no port', () {
        expect(normalizeBackendUrl('myphone.tailxyz.ts.net'), 'https://myphone.tailxyz.ts.net');
      });

      test('a Funnel host with an explicit port keeps https:// and the port', () {
        expect(normalizeBackendUrl('myphone.tailxyz.ts.net:8443'),
            'https://myphone.tailxyz.ts.net:8443');
      });

      test('an explicitly typed http:// on a Funnel host is respected', () {
        expect(normalizeBackendUrl('http://myphone.tailxyz.ts.net'),
            'http://myphone.tailxyz.ts.net:8090');
      });
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

    // flutter_test overrides defaultTargetPlatform to android for every
    // test, so isMobilePlatform is true by default in here and the empty
    // input takes the *mobile* branch. That is a harness artifact, not the
    // shipped behaviour — hence the explicit override on the desktop case
    // below. The platform decision itself is covered exhaustively, with no
    // override needed, in the defaultBackendUrl group further down, which
    // is exactly why that function takes its platform as parameters.
    test('empty input falls back to the local default (desktop)', () {
      debugDefaultTargetPlatformOverride = TargetPlatform.linux;
      addTearDown(() => debugDefaultTargetPlatformOverride = null);
      expect(normalizeBackendUrl(''), 'http://127.0.0.1:8090');
      expect(normalizeBackendUrl('   '), 'http://127.0.0.1:8090');
    });

    test('empty input stays empty on mobile (no local backend to guess)', () {
      debugDefaultTargetPlatformOverride = TargetPlatform.android;
      addTearDown(() => debugDefaultTargetPlatformOverride = null);
      expect(normalizeBackendUrl(''), '');
      expect(normalizeBackendUrl('   '), '');
    });

    test('trailing slashes are stripped even when repeated', () {
      expect(normalizeBackendUrl('http://127.0.0.1:8090///'), 'http://127.0.0.1:8090');
    });
  });

  group('defaultBackendUrl (platform-dependent, hence parameterized)', () {
    const lanPage = 'http://192.168.1.106:8090';

    test('web -> the page origin, whatever the device reports', () {
      // kIsWeb wins over the device platform on purpose: a browser on an
      // Android phone reports TargetPlatform.android while still being the
      // web build, which is served BY the backend it talks to.
      expect(
        defaultBackendUrl(isWeb: true, isMobile: false, pageOrigin: lanPage),
        lanPage,
      );
      expect(
        defaultBackendUrl(isWeb: true, isMobile: true, pageOrigin: lanPage),
        lanPage,
      );
    });

    test('mobile -> empty, because a phone never runs the Go backend', () {
      expect(
        defaultBackendUrl(isWeb: false, isMobile: true, pageOrigin: ''),
        '',
      );
    });

    test('desktop -> loopback, where the user may have started one', () {
      expect(
        defaultBackendUrl(isWeb: false, isMobile: false, pageOrigin: ''),
        'http://127.0.0.1:8090',
      );
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
