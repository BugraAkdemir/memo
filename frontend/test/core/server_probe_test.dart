import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/server_probe.dart';

DioException _dio(DioExceptionType type, {Object? error, int? status}) {
  final req = RequestOptions(path: '/api/setup/status');
  return DioException(
    requestOptions: req,
    type: type,
    error: error,
    response: status == null
        ? null
        : Response(requestOptions: req, statusCode: status),
  );
}

class _Adapter implements HttpClientAdapter {
  _Adapter(this.status, this.body);
  final int status;
  final String body;
  @override
  Future<ResponseBody> fetch(RequestOptions o, Stream<Uint8List>? r,
          Future<void>? c) async =>
      ResponseBody.fromString(body, status,
          headers: {
            // A real 404 page is HTML; labelling it JSON would make Dio fail
            // to decode it and report a different error than the one tested.
            Headers.contentTypeHeader: [
              body.startsWith('{') || body.startsWith('[')
                  ? Headers.jsonContentType
                  : 'text/html',
            ],
          });
  @override
  void close({bool force = false}) {}
}

void main() {
  group('classifyProbeError', () {
    ServerProbeFailure? of(Object e) => classifyProbeError(e).failure;

    test('DNS failure', () {
      expect(
          of(_dio(DioExceptionType.connectionError,
              error: "SocketException: Failed host lookup: 'nope.example'")),
          ServerProbeFailure.dns);
    });

    test('connection refused (Android errno 111, Apple 61, Windows 10061)', () {
      for (final code in ['111', '61', '10061']) {
        expect(
            of(_dio(DioExceptionType.connectionError,
                error: 'SocketException: Connection refused, errno = $code')),
            ServerProbeFailure.refused,
            reason: 'errno $code');
      }
    });

    test('timeouts of every kind', () {
      for (final t in [
        DioExceptionType.connectionTimeout,
        DioExceptionType.sendTimeout,
        DioExceptionType.receiveTimeout,
      ]) {
        expect(of(_dio(t)), ServerProbeFailure.timeout);
      }
    });

    test('bad certificate and handshake failures are TLS', () {
      expect(of(_dio(DioExceptionType.badCertificate)), ServerProbeFailure.tls);
      expect(
          of(_dio(DioExceptionType.unknown,
              error: 'HandshakeException: Handshake error in client')),
          ServerProbeFailure.tls);
    });

    test('no route to host', () {
      expect(
          of(_dio(DioExceptionType.connectionError,
              error: 'SocketException: Network is unreachable, errno = 101')),
          ServerProbeFailure.unreachable);
    });

    test('an HTTP error status means a server that is not Memo', () {
      final r = classifyProbeError(_dio(DioExceptionType.badResponse, status: 404));
      expect(r.failure, ServerProbeFailure.notMemo);
      expect(r.detail, 'HTTP 404');
    });

    test('anything unrecognised is unknown, never a throw', () {
      expect(of(_dio(DioExceptionType.unknown, error: 'weird')),
          ServerProbeFailure.unknown);
      expect(of(StateError('x')), ServerProbeFailure.unknown);
    });
  });

  group('probeServer', () {
    MemoApiClient Function(String) clientWith(int status, String body) =>
        (url) {
          final c = MemoApiClient(baseUrl: url);
          c.dio.httpClientAdapter = _Adapter(status, body);
          return c;
        };

    test('a Memo backend answers with its setup status', () async {
      final r = await probeServer('http://memo.test:8090',
          clientFor: clientWith(
              200,
              jsonEncode({
                'needs_setup': false,
                'auth_mode': 'token_password',
                'install_id': 'abc',
              })));
      expect(r.ok, isTrue);
      expect(r.status!.authMode, 'token_password');
      expect(r.status!.needsSetup, isFalse);
    });

    test('a 200 that is not the setup JSON is "not Memo"', () async {
      final r = await probeServer('http://memo.test:8090',
          clientFor: clientWith(200, jsonEncode(['<html>'])));
      expect(r.failure, ServerProbeFailure.notMemo);
    });

    test('a 404 page is "not Memo"', () async {
      final r = await probeServer('http://memo.test:8090',
          clientFor: clientWith(404, 'nope'));
      expect(r.failure, ServerProbeFailure.notMemo);
    });
  });
}
