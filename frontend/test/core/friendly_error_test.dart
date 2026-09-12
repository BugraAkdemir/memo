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

    // Reported live: a Router (internal/provider/router.go) fallback error
    // — "all providers failed: [opencode-zen] provider rate limited: Rate
    // limit exceeded. Please try again later." — reached the send-message
    // SnackBar completely verbatim, reading as an internal error dump for
    // what's actually just the external provider throttling, not a Memo
    // bug.
    test('a provider rate-limit error (plain-exception path) is replaced with a friendly message', () {
      final msg = FriendlyError.describeGeneric(Exception(
          'all providers failed: [opencode-zen] provider rate limited: Rate limit exceeded. Please try again later.'));
      expect(msg, L10n.t('friendly_error_provider_rate_limited'));
      expect(msg.contains('opencode-zen'), isFalse);
      expect(msg.contains('all providers failed'), isFalse);
    });

    test('a provider rate-limit error (Dio badResponse path) is also replaced', () {
      final req = RequestOptions(path: '/x');
      final res = Response(
        requestOptions: req,
        statusCode: 429,
        data: {'error': 'provider rate limited: Rate limit exceeded'},
      );
      final msg = FriendlyError.describeGeneric(
        DioException(requestOptions: req, type: DioExceptionType.badResponse, response: res),
      );
      expect(msg, L10n.t('friendly_error_provider_rate_limited'));
    });

    // Reported live: a small local model's context window overflowed on an
    // ordinary message once accumulated memory/history grew large enough —
    // internal/api's extractErrorMessage strips llama-server's raw JSON down
    // to one sentence ("request (N tokens) exceeds the available context
    // size (M tokens), try increasing it"), but that sentence is still
    // technical, not something a non-technical user can act on.
    test('a context-overflow error (plain-exception path) is replaced with a friendly message', () {
      final msg = FriendlyError.describeGeneric(Exception(
          'api.Stream: status 400: request (18147 tokens) exceeds the available context size (4096 tokens), try increasing it'));
      expect(msg, L10n.t('friendly_error_context_overflow'));
      expect(msg.contains('18147'), isFalse);
    });

    test('a context-overflow error (Dio badResponse path) is also replaced', () {
      final req = RequestOptions(path: '/x');
      final res = Response(
        requestOptions: req,
        statusCode: 400,
        data: {
          'error': {
            'code': 400,
            'message': 'request (18147 tokens) exceeds the available context size (4096 tokens), try increasing it',
            'type': 'exceed_context_size_error',
          },
        },
      );
      final msg = FriendlyError.describeGeneric(
        DioException(requestOptions: req, type: DioExceptionType.badResponse, response: res),
      );
      expect(msg, L10n.t('friendly_error_context_overflow'));
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
