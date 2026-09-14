import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/core/local_session_state.dart';
import 'package:memo_flutter/providers/permissions_provider.dart';

// The known-backend-swap scenario this module exists for (see its own
// module doc comment): a self-hosted Memo gets wiped and reinstalled at the
// same origin, and the browser's localStorage carries every server-coupled
// value over to a backend that never issued them. memo_session_permissions
// is the same class of value as memo_session_role/memo_session_username
// right next to it in the list (an account's cached effective permissions,
// scoped to one backend+account combination) — it belongs in
// serverCoupledPrefsKeys for the same reason they do.
void main() {
  test('serverCoupledPrefsKeys includes memo_session_permissions', () {
    expect(serverCoupledPrefsKeys, contains(memoSessionPermissionsKey));
  });

  test('clearServerCoupledState wipes a stale saved permissions blob along with role/username/token',
      () async {
    SharedPreferences.setMockInitialValues({
      'memo_session_role': 'user',
      'memo_session_username': 'alice',
      'memo_remote_access_token': 'stale-token',
      memoSessionPermissionsKey: '{"models":false,"memory":false}',
      // A device preference that must survive the reset.
      'memo_locale': 'tr',
    });
    final prefs = await SharedPreferences.getInstance();

    await clearServerCoupledState(prefs);

    expect(prefs.getString(memoSessionPermissionsKey), isNull);
    expect(prefs.getString('memo_session_role'), isNull);
    expect(prefs.getString('memo_locale'), 'tr');
  });
}
