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

    if (raw.contains('permission denied')) {
      return L10n.t('friendly_error_model_permission');
    }
    if (raw.contains('start failed')) {
      return L10n.t('friendly_error_model_spawn');
    }
    if (raw.contains('failed to become ready') ||
        raw.contains('server process exited')) {
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

  /// General-purpose sibling of [describe] for everywhere else in the app
  /// (agent, WhatsApp, swarm, backup, settings tabs, ...) that isn't one of
  /// [describe]'s three model-loading callers. Same safe, type-based
  /// network-error classification, but deliberately without [describe]'s
  /// keyword heuristics: "killed"/"socketexception"/etc. as bare substrings
  /// were tuned for llama.cpp process-lifecycle errors specifically, and
  /// would misclassify an unrelated error elsewhere that happens to mention
  /// one of those words for its own reasons (e.g. "permission killed" in an
  /// agent context has nothing to do with running out of RAM).
  static String describeGeneric(Object error) {
    if (error is DioException) {
      switch (error.type) {
        case DioExceptionType.connectionTimeout:
        case DioExceptionType.receiveTimeout:
        case DioExceptionType.sendTimeout:
        case DioExceptionType.connectionError:
          return L10n.t('friendly_error_network');
        case DioExceptionType.badResponse:
          final msg = _messageFromResponseBody(error.response?.data);
          if (msg == null) return L10n.t('friendly_error_generic');
          return _classifyProviderMessage(msg) ?? msg;
        case DioExceptionType.cancel:
        case DioExceptionType.badCertificate:
        case DioExceptionType.transformTimeout:
        case DioExceptionType.unknown:
          return L10n.t('friendly_error_generic');
      }
    }
    final stripped = _stripExceptionPrefix(error.toString());
    return _classifyProviderMessage(stripped) ?? stripped;
  }

  /// Recognizes the handful of raw external-provider failure shapes that
  /// otherwise leak straight through describeGeneric's fallback (unlike
  /// describe() above, describeGeneric has no keyword heuristics of its own
  /// — see that method's doc comment for why). Reported live: a Router
  /// (internal/provider/router.go) "all providers failed: [opencode-zen]
  /// provider rate limited: Rate limit exceeded. Please try again later."
  /// error reached the send-message SnackBar completely verbatim — a
  /// legitimate, expected condition (the provider is throttling, not a
  /// Memo bug) read as a cryptic internal error dump, making the app look
  /// broken for something outside its control. Returns null (caller keeps
  /// showing the original message) for anything not recognized here.
  static String? _classifyProviderMessage(String message) {
    if (message.toLowerCase().contains('rate limit')) {
      return L10n.t('friendly_error_provider_rate_limited');
    }
    return null;
  }

  /// Unwraps the `{"error": "..."}` / `{"error": {"message": "..."}}` /
  /// `{"message": "..."}` shapes Memo's own backend error responses use
  /// (see internal/provider.ExtractErrorMessage on the Go side) so a
  /// genuine backend-reported failure still surfaces its actual message
  /// instead of a bare status code or nothing at all.
  static String? _messageFromResponseBody(Object? data) {
    if (data is Map) {
      final err = data['error'];
      if (err is String && err.isNotEmpty) return err;
      if (err is Map) {
        final msg = err['message'];
        if (msg is String && msg.isNotEmpty) return msg;
      }
      final msg = data['message'];
      if (msg is String && msg.isNotEmpty) return msg;
    }
    return null;
  }

  /// `Exception('some message').toString()` is `'Exception: some message'`
  /// — most non-Dio throws in this codebase are hand-written with an
  /// already-clean message, so stripping the mechanical type prefix is
  /// enough to make them read as prose instead of leaking a type name.
  static String _stripExceptionPrefix(String text) {
    // Dart's built-in Error/Exception types each format their own
    // toString() differently — Exception/FormatException prefix with their
    // own type name, but StateError's is "Bad state: ..." (its actual
    // wording, not "StateError: ...").
    const prefixes = ['Exception: ', 'FormatException: ', 'Bad state: '];
    for (final prefix in prefixes) {
      if (text.startsWith(prefix)) return text.substring(prefix.length);
    }
    return text;
  }
}
