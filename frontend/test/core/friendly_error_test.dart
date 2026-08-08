import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/friendly_error.dart';
import 'package:memo_flutter/core/l10n.dart';

void main() {
  setUp(() => L10n.setLocale(MemoLocale.tr));

  group('FriendlyError.describeGeneric', () {
    test('network-type DioExceptions collapse to one plain sentence, never the raw dump', () {
      final req = RequestOptions(path: '/x');
      for (final type in [
        DioExceptionType.connectionError,
        DioExceptionType.connectionTimeout,
        DioExceptionType.receiveTimeout,
        DioExceptionType.sendTimeout,
      ]) {
        final msg = FriendlyError.describeGeneric(DioException(
          requestOptions: req,
          type: type,
          error: 'SocketException: Connection refused (OS Error: Connection refused, errno = 111)',
        ));
        expect(msg, L10n.t('friendly_error_network'));
        expect(msg.contains('SocketException'), isFalse);
        expect(msg.contains('errno'), isFalse);
      }
    });

    test('unwraps a backend JSON error body for a real (non-network) response', () {
      final req = RequestOptions(path: '/x');
      final res = Response(
        requestOptions: req,
        statusCode: 400,
        data: {
          'error': {'message': 'model dosyası bulunamadı'}
        },
      );
      final msg = FriendlyError.describeGeneric(
        DioException(requestOptions: req, type: DioExceptionType.badResponse, response: res),
      );
      expect(msg, 'model dosyası bulunamadı');
    });

    test('falls back to the generic message when the response body has no message', () {
      final req = RequestOptions(path: '/x');
      final res = Response(requestOptions: req, statusCode: 500, data: 'not json');
      final msg = FriendlyError.describeGeneric(
        DioException(requestOptions: req, type: DioExceptionType.badResponse, response: res),
      );
      expect(msg, L10n.t('friendly_error_generic'));
    });

    test('strips the mechanical "Exception: " prefix off a hand-thrown error', () {
      expect(
        FriendlyError.describeGeneric(Exception('Sağlayıcı test edilemedi')),
        'Sağlayıcı test edilemedi',
      );
    });

    test('strips StateError\'s own "Bad state: " prefix too', () {
      expect(FriendlyError.describeGeneric(StateError('kötü durum')), 'kötü durum');
    });
  });
}
