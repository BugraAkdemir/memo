import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/providers/auth_gate_provider.dart';
import 'package:memo_flutter/providers/gate_guard.dart';

void main() {
  group('authGateBlocked', () {
    test('a null gate (still deciding) counts as blocked', () {
      expect(authGateBlocked(null), isTrue);
    });

    test('setupNeeded and loginNeeded count as blocked', () {
      expect(
        authGateBlocked(const AuthGateInfo(AuthGateState.setupNeeded)),
        isTrue,
      );
      expect(
        authGateBlocked(const AuthGateInfo(AuthGateState.loginNeeded)),
        isTrue,
      );
    });

    test('ok is the only non-blocked state', () {
      expect(authGateBlocked(const AuthGateInfo(AuthGateState.ok)), isFalse);
    });
  });
}