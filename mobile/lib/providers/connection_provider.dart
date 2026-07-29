import 'dart:async';
import 'dart:io';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../core/api_client.dart';
import '../core/l10n.dart';

/// Turns a caught exception into a short, actionable message instead of
/// dumping the raw DioException text (stack-trace-like, unreadable) into the
/// UI. A 401 specifically means the phone's saved token no longer matches
/// the desktop's — e.g. remote access was just turned on, or its token was
/// rotated, after the phone last paired — so it gets its own message rather
/// than falling through to the generic case. Any other DioException (a
/// 400/500 with a structured `{"error": "..."}` body, say) reuses
/// MemoApiClient's own extractErrorMessage instead of dumping `e` raw — this
/// used to be a weaker, independent fallback that this function itself
/// warned against and then did anyway for every non-401/timeout case.
String friendlyErrorMessage(Object e, {required String action}) {
  if (e is DioException) {
    if (e.response?.statusCode == 401) {
      return L10n.t('err_token_invalid');
    }
    if (e.type == DioExceptionType.connectionTimeout ||
        e.type == DioExceptionType.receiveTimeout ||
        e.type == DioExceptionType.connectionError) {
      return L10n.t('err_backend_unreachable', {'action': action});
    }
    return '$action: ${MemoApiClient.extractErrorMessage(e)}';
  }
  return '$action: $e';
}

final apiClientProvider = Provider<MemoApiClient>((ref) {
  return MemoApiClient();
});

final connectionStateProvider =
    StateNotifierProvider<ConnectionNotifier, ConnectionState>((ref) {
  return ConnectionNotifier(ref.read(apiClientProvider));
});

final remoteAccessProvider =
    StateNotifierProvider<RemoteAccessNotifier, RemoteAccessState>((ref) {
  return RemoteAccessNotifier(ref.read(apiClientProvider));
});

class ConnectionState {
  final bool connected;
  final bool connecting;
  final bool discovering;
  final String? error;
  final String baseUrl;
  final String token;
  final bool remoteMode;

  const ConnectionState({
    this.connected = false,
    this.connecting = false,
    this.discovering = false,
    this.error,
    this.baseUrl = '',
    this.token = '',
    this.remoteMode = false,
  });

  ConnectionState copyWith({
    bool? connected,
    bool? connecting,
    bool? discovering,
    String? error,
    String? baseUrl,
    String? token,
    bool? remoteMode,
  }) {
    return ConnectionState(
      connected: connected ?? this.connected,
      connecting: connecting ?? this.connecting,
      discovering: discovering ?? this.discovering,
      error: error,
      baseUrl: baseUrl ?? this.baseUrl,
      token: token ?? this.token,
      remoteMode: remoteMode ?? this.remoteMode,
    );
  }
}

class RemoteAccessState {
  final bool loading;
  final RemoteAccessStatus? status;
  final bool enabling;
  final String? error;

  const RemoteAccessState({
    this.loading = false,
    this.status,
    this.enabling = false,
    this.error,
  });

  RemoteAccessState copyWith({
    bool? loading,
    RemoteAccessStatus? status,
    bool? enabling,
    String? error,
  }) {
    return RemoteAccessState(
      loading: loading ?? this.loading,
      status: status ?? this.status,
      enabling: enabling ?? this.enabling,
      error: error,
    );
  }
}

class ConnectionNotifier extends StateNotifier<ConnectionState> {
  final MemoApiClient _client;

  ConnectionNotifier(this._client) : super(const ConnectionState());

  Future<void> loadSavedUrl() async {
    final prefs = await SharedPreferences.getInstance();
    final url = prefs.getString('backend_url') ?? state.baseUrl;
    final token = prefs.getString('backend_token') ?? '';
    _client.updateBaseUrl(url);
    _client.setToken(token);
    state = state.copyWith(baseUrl: url, token: token);
  }

  /// Restores the saved backend URL/token and, if there is one, silently
  /// tries to reconnect with it — instead of always landing on ConnectScreen
  /// and requiring a manual tap even though the fields are already prefilled.
  /// This matters most for a stable Tailscale (*.ts.net) URL, exactly the
  /// case where "enter it once, it just works" is the whole point: without
  /// this, every cold start still demanded a manual re-tap for no reason.
  /// A failed attempt (backend off, token rotated) falls through to the
  /// same ConnectScreen + prefilled-fields + error experience a manual
  /// connect attempt already has today — nothing new to handle there.
  Future<void> autoConnectIfPossible() async {
    await loadSavedUrl();
    if (state.baseUrl.isEmpty) return;
    await connect(state.baseUrl, token: state.token);
  }

  Future<void> connect(String url,
      {String token = '', bool remote = false}) async {
    state = state.copyWith(
      connecting: true,
      error: null,
      baseUrl: url,
      token: token,
      remoteMode: remote,
    );

    // Everything below can throw (URL parsing, Dio setup, the request
    // itself) — all of it must be inside this try, otherwise an exception
    // here leaves `connecting` stuck at true forever with no error shown.
    try {
      var normalized = url.trim().replaceAll(RegExp(r'/+$'), '');
      // Auto-add a scheme when the user typed a bare host. Tailscale/Funnel
      // hosts (*.ts.net) are HTTPS; everything else (LAN IPs) defaults to HTTP.
      if (!normalized.startsWith('http://') && !normalized.startsWith('https://')) {
        final host = normalized.split(':').first;
        normalized = (host.endsWith('.ts.net') ? 'https://' : 'http://') + normalized;
      }
      _client.updateBaseUrl(normalized);
      if (token.isNotEmpty) {
        _client.setToken(token);
      }

      final alive = await _client.isAlive().timeout(
        const Duration(seconds: 8),
        onTimeout: () => false,
      );
      if (alive) {
        final prefs = await SharedPreferences.getInstance();
        await prefs.setString('backend_url', normalized);
        await prefs.setString('backend_token', token);
        state = state.copyWith(
          connected: true,
          connecting: false,
          error: null,
          token: token,
          remoteMode: remote,
        );
      } else {
        state = state.copyWith(
          connecting: false,
          error: L10n.t('err_backend_not_reachable', {'url': normalized}),
        );
      }
    } catch (e) {
      state = state.copyWith(
        connecting: false,
        error: friendlyErrorMessage(e, action: L10n.t('err_connection_failed')),
      );
    }
  }

  void disconnect() {
    state = state.copyWith(connected: false);
  }

  /// Scans the local network for a running Memo backend.
  /// Returns the discovered base URL, or null if none found.
  Future<String?> discoverUrl() async {
    if (state.discovering) return null;
    state = state.copyWith(discovering: true, error: null);
    try {
      return await _scanLocalNetwork();
    } finally {
      if (mounted) state = state.copyWith(discovering: false);
    }
  }

  /// Scans the local network for a running Memo backend, then asks it for
  /// its ngrok remote-access status. Lets a phone on the same Wi-Fi as the
  /// desktop grab the current ngrok URL/token without copy-pasting it.
  /// Returns null if no backend is found or it has no active ngrok tunnel.
  Future<RemoteAccessStatus?> discoverNgrokStatus() async {
    if (state.discovering) return null;
    state = state.copyWith(discovering: true, error: null);
    try {
      final lanUrl = await _scanLocalNetwork();
      if (lanUrl == null) return null;

      final probeDio = Dio(BaseOptions(
        baseUrl: lanUrl,
        connectTimeout: const Duration(seconds: 4),
        receiveTimeout: const Duration(seconds: 4),
      ));
      try {
        final res = await probeDio.get('/api/remote-access');
        final status = RemoteAccessStatus.fromJson(res.data as Map<String, dynamic>);
        return status.ngrokUrl.isNotEmpty ? status : null;
      } catch (_) {
        return null;
      } finally {
        probeDio.close(force: true);
      }
    } finally {
      if (mounted) state = state.copyWith(discovering: false);
    }
  }

  Future<String?> _scanLocalNetwork() async {
    List<NetworkInterface> interfaces;
    try {
      interfaces = await NetworkInterface.list(
        type: InternetAddressType.IPv4,
        includeLoopback: false,
      );
    } catch (_) {
      return null;
    }

    final subnets = <String>{};
    for (final iface in interfaces) {
      for (final addr in iface.addresses) {
        final parts = addr.address.split('.');
        if (parts.length == 4) {
          subnets.add('${parts[0]}.${parts[1]}.${parts[2]}');
        }
      }
    }

    if (subnets.isEmpty) return null;

    final candidates = <String>[];
    for (final subnet in subnets) {
      for (int i = 1; i < 255; i++) {
        candidates.add('http://$subnet.$i:8090');
      }
    }

    final scanDio = Dio(BaseOptions(
      connectTimeout: const Duration(seconds: 2),
      receiveTimeout: const Duration(seconds: 2),
    ));

    try {
      final completer = Completer<String?>();
      int pending = candidates.length;

      for (final url in candidates) {
        scanDio.get('$url/api/version').then((_) {
          if (!completer.isCompleted) completer.complete(url);
          pending--;
          if (pending <= 0 && !completer.isCompleted) completer.complete(null);
        }).catchError((_) {
          pending--;
          if (pending <= 0 && !completer.isCompleted) completer.complete(null);
        });
      }

      return await completer.future
          .timeout(const Duration(seconds: 10), onTimeout: () => null);
    } finally {
      scanDio.close(force: true);
    }
  }
}

class RemoteAccessNotifier extends StateNotifier<RemoteAccessState> {
  final MemoApiClient _client;

  RemoteAccessNotifier(this._client) : super(const RemoteAccessState());

  Future<void> loadStatus() async {
    // No saved backend yet (e.g. this fires before loadSavedUrl() finishes
    // restoring it on cold start) — nothing to query, so don't surface a
    // raw "No host specified in URI" DioException for it.
    if (_client.baseUrl.isEmpty) {
      state = const RemoteAccessState(loading: false);
      return;
    }
    state = state.copyWith(loading: true, error: null);
    try {
      final status = await _client.getRemoteAccess();
      state = RemoteAccessState(loading: false, status: status);
      // Cache the beta flag so the connect screen can decide whether to show
      // the Tailscale tab even before a connection exists.
      final prefs = await SharedPreferences.getInstance();
      await prefs.setBool('beta_enabled', status.beta);
    } catch (e) {
      state = RemoteAccessState(
        loading: false,
        error: friendlyErrorMessage(e, action: L10n.t('err_remote_status')),
      );
    }
  }

  Future<void> enableNgrok(String ngrokToken) async {
    state = state.copyWith(enabling: true, error: null);
    try {
      await _client.setRemoteAccess(true, 8090,
          ngrokMode: true, ngrokToken: ngrokToken);
      // Poll until ngrok URL appears (takes a few seconds)
      for (int i = 0; i < 30; i++) {
        await Future.delayed(const Duration(seconds: 1));
        try {
          final s = await _client.getRemoteAccess();
          if (s.ngrokUrl.isNotEmpty || s.ngrokError.isNotEmpty) break;
        } catch (_) {}
      }
      await loadStatus();
    } catch (e) {
      state = state.copyWith(
        enabling: false,
        error: friendlyErrorMessage(e, action: L10n.t('err_ngrok_enable')),
      );
    }
  }

  Future<void> disableNgrok() async {
    state = state.copyWith(enabling: true, error: null);
    try {
      await _client.setRemoteAccess(false, 8090);
      await loadStatus();
    } catch (e) {
      state = state.copyWith(
        enabling: false,
        error: friendlyErrorMessage(e, action: L10n.t('err_remote_disable')),
      );
    }
  }

  Future<void> setAutoStart(bool autoStart) async {
    try {
      await _client.setRemoteAccessAutoStart(autoStart);
      // If enabling, poll until ngrok URL appears
      if (autoStart) {
        for (int i = 0; i < 30; i++) {
          await Future.delayed(const Duration(seconds: 1));
          try {
            final s = await _client.getRemoteAccess();
            if (s.ngrokUrl.isNotEmpty || s.ngrokError.isNotEmpty) break;
          } catch (_) {}
        }
      }
      await loadStatus();
    } catch (e) {
      state = state.copyWith(
        error: friendlyErrorMessage(e, action: L10n.t('err_auto_start')),
      );
    }
  }
}
