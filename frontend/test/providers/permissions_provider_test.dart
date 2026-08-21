import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/providers/permissions_provider.dart';

// Faz 5.1.1 (yapacam.md): readMyPermissions/saveMyPermissions gate Settings
// UI on the current session's account permissions — these tests lock in
// the fail-open shape that makes the whole system safe to ship: anything
// other than a genuinely restricted "user" session must see everything.
void main() {
  test('readMyPermissions with no saved role at all fails open (local desktop)',
      () async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();

    expect(readMyPermissions(prefs), AccountPermissions.allTrue);
  });

  test('readMyPermissions for an admin session fails open regardless of any stored permissions value',
      () async {
    SharedPreferences.setMockInitialValues({
      'memo_session_role': 'admin',
      memoSessionPermissionsKey: '{"models":false,"memory":false}',
    });
    final prefs = await SharedPreferences.getInstance();

    expect(readMyPermissions(prefs), AccountPermissions.allTrue);
  });

  test('readMyPermissions for a user session with no saved permissions value fails open',
      () async {
    // A session persisted before this feature existed — role is saved,
    // permissions never were.
    SharedPreferences.setMockInitialValues({'memo_session_role': 'user'});
    final prefs = await SharedPreferences.getInstance();

    expect(readMyPermissions(prefs), AccountPermissions.allTrue);
  });

  test('readMyPermissions for a user session with corrupt saved JSON fails open',
      () async {
    SharedPreferences.setMockInitialValues({
      'memo_session_role': 'user',
      memoSessionPermissionsKey: 'not valid json',
    });
    final prefs = await SharedPreferences.getInstance();

    expect(readMyPermissions(prefs), AccountPermissions.allTrue);
  });

  test('readMyPermissions for a user session returns exactly its saved checkboxes',
      () async {
    SharedPreferences.setMockInitialValues({
      'memo_session_role': 'user',
      memoSessionPermissionsKey: '{"models":true,"whatsapp":true}',
    });
    final prefs = await SharedPreferences.getInstance();

    final got = readMyPermissions(prefs);
    expect(got.models, isTrue);
    expect(got.whatsapp, isTrue);
    expect(got.memory, isFalse);
    expect(got.agent, isFalse);
  });

  test('saveMyPermissions then readMyPermissions round-trips', () async {
    SharedPreferences.setMockInitialValues({'memo_session_role': 'user'});
    final prefs = await SharedPreferences.getInstance();
    const perms = AccountPermissions(memory: true, routines: true);

    await saveMyPermissions(prefs, perms);

    final got = readMyPermissions(prefs);
    expect(got.memory, isTrue);
    expect(got.routines, isTrue);
    expect(got.models, isFalse);
  });
}
