import 'package:dio/dio.dart';

import 'api_client.dart';

/// Why a server address did not answer like a Memo backend.
///
/// The mobile connect screen used to say one thing for every failure ("no
/// Memo found at that address"), which left a user whose phone, Wi-Fi, DNS or
/// server setting was the actual problem with nothing to act on.
enum ServerProbeFailure {
  /// The host name does not resolve.
  dns,

  /// The host answered but nothing listens on that port — typically the
  /// server's Remote Access is off, so it only listens on its own loopback.
  refused,

  /// No answer within the connect/receive timeout.
  timeout,

  /// The TLS handshake or certificate failed.
  tls,

  /// No route to the host (other network, VPN down, airplane mode).
  unreachable,

  /// Something answered, but not like a Memo backend.
  notMemo,

  /// Anything else.
  unknown,
}

class ServerProbeResult {
  const ServerProbeResult.ok(SetupStatus this.status)
      : failure = null,
        detail = '';
  const ServerProbeResult.failed(ServerProbeFailure this.failure, this.detail)
      : status = null;

  final SetupStatus? status;
  final ServerProbeFailure? failure;

  /// Short technical text for the failure, shown small under the
  /// plain-language reason so a bug report carries the real cause.
  final String detail;

  bool get ok => status != null;
}

/// Asks [baseUrl] for its unauthenticated `/api/setup/status` with a client of
/// its own — nothing is saved and the app's shared client is untouched, so a
/// wrong address costs the user nothing.
///
/// [clientFor] is the test seam; production builds a plain [MemoApiClient].
Future<ServerProbeResult> probeServer(
  String baseUrl, {
  MemoApiClient Function(String baseUrl)? clientFor,
}) async {
  try {
    final client = (clientFor ?? (url) => MemoApiClient(baseUrl: url))(baseUrl);
    final status = await client.fetchSetupStatus();
    return ServerProbeResult.ok(status);
  } catch (e) {
    return classifyProbeError(e);
  }
}

/// Maps a thrown error to a [ServerProbeResult.failed]. Pure and string-based
/// on the error text because the transport exceptions (`SocketException`,
/// `HandshakeException`) live in dart:io, which the web build must not import;
/// the wording matched here is stable across Android, iOS and desktop.
ServerProbeResult classifyProbeError(Object e) {
  if (e is DioException) {
    final raw = '${e.error ?? ''} ${e.message ?? ''}'.toLowerCase();
    final detail = _short(e.error?.toString() ?? e.message ?? e.type.name);
    switch (e.type) {
      case DioExceptionType.connectionTimeout:
      case DioExceptionType.sendTimeout:
      case DioExceptionType.receiveTimeout:
        return ServerProbeResult.failed(ServerProbeFailure.timeout, detail);
      case DioExceptionType.badCertificate:
        return ServerProbeResult.failed(ServerProbeFailure.tls, detail);
      case DioExceptionType.badResponse:
        // Reached a web server that is not Memo (or a proxy in front of a
        // dead Memo): setup/status is unauthenticated, so any status here
        // means it is not the Memo endpoint.
        return ServerProbeResult.failed(
          ServerProbeFailure.notMemo,
          'HTTP ${e.response?.statusCode ?? '?'}',
        );
      case DioExceptionType.connectionError:
      case DioExceptionType.unknown:
        if (_has(raw, ['failed host lookup', 'nodename nor servname',
            'name or service not known', 'no address associated'])) {
          return ServerProbeResult.failed(ServerProbeFailure.dns, detail);
        }
        if (_has(raw, ['connection refused', 'errno = 111', 'errno = 61',
            'errno = 10061'])) {
          return ServerProbeResult.failed(ServerProbeFailure.refused, detail);
        }
        if (_has(raw, ['handshake', 'certificate', 'tls', 'ssl'])) {
          return ServerProbeResult.failed(ServerProbeFailure.tls, detail);
        }
        if (_has(raw, ['network is unreachable', 'no route to host',
            'errno = 101', 'errno = 113'])) {
          return ServerProbeResult.failed(
              ServerProbeFailure.unreachable, detail);
        }
        if (_has(raw, ['timed out', 'timeout'])) {
          return ServerProbeResult.failed(ServerProbeFailure.timeout, detail);
        }
        return ServerProbeResult.failed(ServerProbeFailure.unknown, detail);
      default:
        // cancel and any timeout kind a newer Dio adds.
        return ServerProbeResult.failed(ServerProbeFailure.unknown, detail);
    }
  }
  // fetchSetupStatus's _guard threw: a 200 whose body is not the JSON object
  // Memo returns, i.e. some other service answered.
  final text = e.toString();
  if (text.startsWith('Exception: Expected')) {
    return ServerProbeResult.failed(ServerProbeFailure.notMemo, _short(text));
  }
  return ServerProbeResult.failed(ServerProbeFailure.unknown, _short(text));
}

bool _has(String haystack, List<String> needles) =>
    needles.any(haystack.contains);

String _short(String s) {
  final one = s.replaceAll(RegExp(r'\s+'), ' ').trim();
  return one.length <= 140 ? one : '${one.substring(0, 137)}...';
}
