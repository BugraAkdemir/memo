import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../core/api_client.dart';

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
  final String? error;
  final String baseUrl;
  final String token;
  final bool remoteMode;

  const ConnectionState({
    this.connected = false,
    this.connecting = false,
    this.error,
    this.baseUrl = 'http://192.168.1.100:8090',
    this.token = '',
    this.remoteMode = false,
  });

  ConnectionState copyWith({
    bool? connected,
    bool? connecting,
    String? error,
    String? baseUrl,
    String? token,
    bool? remoteMode,
  }) {
    return ConnectionState(
      connected: connected ?? this.connected,
      connecting: connecting ?? this.connecting,
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

  Future<void> connect(String url,
      {String token = '', bool remote = false}) async {
    state = state.copyWith(
      connecting: true,
      error: null,
      baseUrl: url,
      token: token,
      remoteMode: remote,
    );

    final normalized = url.replaceAll(RegExp(r'/+$'), '');
    _client.updateBaseUrl(normalized);
    if (token.isNotEmpty) {
      _client.setToken(token);
    }

    try {
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
          error: 'Backend not reachable at $normalized\n'
              'Check:\n'
              '• Same network\n'
              '• Backend running\n'
              '• Token correct (if remote)',
        );
      }
    } catch (e) {
      state = state.copyWith(
        connecting: false,
        error: 'Connection failed: $e',
      );
    }
  }

  void disconnect() {
    state = state.copyWith(connected: false);
  }
}

class RemoteAccessNotifier extends StateNotifier<RemoteAccessState> {
  final MemoApiClient _client;

  RemoteAccessNotifier(this._client) : super(const RemoteAccessState());

  Future<void> loadStatus() async {
    state = state.copyWith(loading: true, error: null);
    try {
      final status = await _client.getRemoteAccess();
      state = RemoteAccessState(loading: false, status: status);
    } catch (e) {
      state = RemoteAccessState(
        loading: false,
        error: 'Failed to load remote access status: $e',
      );
    }
  }

  Future<void> enableNgrok(String ngrokToken) async {
    state = state.copyWith(enabling: true, error: null);
    try {
      await _client.setRemoteAccess(true, 8090,
          ngrokMode: true, ngrokToken: ngrokToken);
      await loadStatus();
    } catch (e) {
      state = state.copyWith(
        enabling: false,
        error: 'Failed to enable ngrok: $e',
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
        error: 'Failed to disable remote access: $e',
      );
    }
  }
}
