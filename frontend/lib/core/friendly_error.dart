import 'package:dio/dio.dart';

import 'l10n.dart';

/// Turns a raw exception (Dio network error, or a Go backend error string
/// like "llama: server failed to become ready within 120s") into a short,
/// plain-language message. Mirrors the Go backend's own
/// `provider.ExtractErrorMessage` (unwraps a raw error into something a user
/// can act on) but one level up: this recognizes the handful of failure
/// shapes that actually reach the Flutter UI and replaces them outright,
/// the same way BUG-L4's "model/provider değişti" substitution already does
/// in the backend — a non-technical user can't do anything with a raw Dio
/// message or a Go error string, so there's no value in showing it.
class FriendlyError {
  static String describe(Object error) {
    if (error is DioException) {
      switch (error.type) {
        case DioExceptionType.connectionTimeout:
        case DioExceptionType.receiveTimeout:
        case DioExceptionType.sendTimeout:
        case DioExceptionType.connectionError:
          return L10n.t('friendly_error_network');
        default:
          break;
      }
    }

    final raw = error.toString().toLowerCase();

    if (raw.contains('failed to become ready') ||
        raw.contains('server process exited') ||
        raw.contains('start failed')) {
      return L10n.t('friendly_error_model_start');
    }
    if (raw.contains('out of memory') ||
        raw.contains('out-of-memory') ||
        raw.contains('killed')) {
      return L10n.t('friendly_error_oom');
    }
    if (raw.contains('download status') ||
        raw.contains('socketexception') ||
        raw.contains('no downloadable file') ||
        raw.contains('no space left')) {
      return L10n.t('friendly_error_download');
    }
    return L10n.t('friendly_error_generic');
  }
}
