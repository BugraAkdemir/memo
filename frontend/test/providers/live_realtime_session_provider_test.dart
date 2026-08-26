import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/providers/live_realtime_session_provider.dart';

void main() {
  group('buildLiveModeSessionWsUrl', () {
    test('http -> ws, same host/port', () {
      expect(
        buildLiveModeSessionWsUrl('http://127.0.0.1:8090'),
        'ws://127.0.0.1:8090/api/livemode/session',
      );
    });

    test('https -> wss', () {
      expect(
        buildLiveModeSessionWsUrl('https://memo.example.com:8443'),
        'wss://memo.example.com:8443/api/livemode/session',
      );
    });

    test('replaces any existing path with the session path', () {
      expect(
        buildLiveModeSessionWsUrl('http://127.0.0.1:8090/some/other/path'),
        'ws://127.0.0.1:8090/api/livemode/session',
      );
    });
  });

  group('LiveRealtimeSessionState', () {
    test('defaults to idle with no error', () {
      const state = LiveRealtimeSessionState();
      expect(state.status, LiveRealtimeSessionStatus.idle);
      expect(state.error, isNull);
    });
  });
}
