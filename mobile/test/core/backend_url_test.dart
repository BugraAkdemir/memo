import 'package:flutter_test/flutter_test.dart';

import 'package:memo_mobile/core/backend_url.dart';

void main() {
  group('normalizeBackendUrl', () {
    test('bare LAN IP with no scheme gets http:// prepended', () {
      expect(normalizeBackendUrl('192.168.1.106'), 'http://192.168.1.106');
    });

    test('bare LAN IP with an explicit port gets http:// prepended, port kept', () {
      expect(normalizeBackendUrl('192.168.1.106:9090'), 'http://192.168.1.106:9090');
    });

    test('a Tailscale Funnel host (*.ts.net) gets https://, not http://', () {
      expect(normalizeBackendUrl('myphone.tailxyz.ts.net'), 'https://myphone.tailxyz.ts.net');
    });

    test('an explicit http:// scheme is left alone', () {
      expect(normalizeBackendUrl('http://192.168.1.106:8090'), 'http://192.168.1.106:8090');
    });

    test('an explicit https:// scheme is left alone (no port forced on)', () {
      expect(normalizeBackendUrl('https://myphone.tailxyz.ts.net'), 'https://myphone.tailxyz.ts.net');
    });

    test('trailing slash is stripped', () {
      expect(normalizeBackendUrl('http://192.168.1.106:8090/'), 'http://192.168.1.106:8090');
    });

    test('surrounding whitespace is trimmed', () {
      expect(normalizeBackendUrl('  192.168.1.106:8090  '), 'http://192.168.1.106:8090');
    });

    test('empty input stays empty (caller decides what that means)', () {
      expect(normalizeBackendUrl(''), '');
      expect(normalizeBackendUrl('   '), '');
    });
  });
}
