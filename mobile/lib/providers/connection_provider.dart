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

  Future<void> connect(String url, {String token = '', bool remote = false}) async {
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
