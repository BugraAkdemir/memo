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

  group('FriendlyError.describe (model start failures)', () {
    test('a permission error is never blamed on RAM', () {
      final msg = FriendlyError.describe(Exception(
          'llama: start failed: fork/exec /home/bugraa/.memo/binaries/linux/cpu/llama-server: permission denied'));
      expect(msg, L10n.t('friendly_error_model_permission'));
      expect(msg.contains('bellek'), isFalse);
    });

    test('a spawn failure that is not a permission problem is reported neutrally', () {
      final msg = FriendlyError.describe(Exception(
          'llama: start failed: exec: "llama-server": executable file not found in \$PATH'));
      expect(msg, L10n.t('friendly_error_model_spawn'));
      expect(msg.contains('bellek'), isFalse);
    });

    test('a server that ran but never became ready keeps the memory hint', () {
      final msg = FriendlyError.describe(
          Exception('llama: server failed to become ready within 120s'));
      expect(msg, L10n.t('friendly_error_model_start'));
    });
  });
}
