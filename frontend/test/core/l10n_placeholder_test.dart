import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/core/l10n.dart';

void main() {
  tearDown(() => L10n.setLocale(MemoLocale.tr));

  // The task card shipped showing a literal "{n} tok" because its string was
  // written with a bare {n} while L10n.t only ever replaced the escaped
  // \${n} form. Both spellings must substitute.
  test('t() substitutes both {name} and \${name}', () {
    expect(L10n.t('__nonexistent_bare__', {'n': '42'}), '__nonexistent_bare__');
    // Unknown keys fall through to the key itself, so use real ones.
    expect(L10n.t('task_block_tokens', {'n': '1.4k'}), contains('1.4k'));
    expect(L10n.t('task_block_tokens', {'n': '1.4k'}), isNot(contains('{n}')));
  });

  test('no shipped string leaks an unsubstituted placeholder', () {
    for (final locale in MemoLocale.values) {
      L10n.setLocale(locale);
      for (final key in [
        'task_block_tokens',
        'task_block_silent',
        'task_block_show_log',
        'task_block_thinking',
        'accounts_delete_confirm_body',
        'accounts_password_dialog_title',
        'accounts_permissions_dialog_title',
        'auth_gate_error_generic',
      ]) {
        final out = L10n.t(key, {
          'n': '7',
          'd': '2dk',
          'err': 'boom',
          'name': 'ali',
        });
        expect(out, isNot(contains('{')),
            reason: '$key ($locale) still contains a placeholder: $out');
      }
    }
  });
}
