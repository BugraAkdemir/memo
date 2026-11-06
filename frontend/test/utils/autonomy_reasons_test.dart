import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/utils/autonomy_reasons.dart';

// Every reason code the backend can put on an autonomous action (taken from
// internal/autonomy: engine finish(...), shield and loop blocks, and the
// generation errors). Before this list was shared, three screens kept their
// own copies, two of them had drifted, and most codes fell through to one
// vague "evidence or model unavailable" line.
const backendReasonCodes = [
  'drafted',
  'follow_up',
  'user_cancelled',
  'no_action',
  'stale_message',
  'contact_manual',
  'contact_never',
  'unknown_contact',
  'automated_sender',
  'group_or_cc',
  'rate_limit',
  'message_too_large',
  'prompt_injection',
  'encoded_instruction',
  'secret_api_key',
  'secret_private_key',
  'secret_env',
  'secret_otp',
  'research_failed',
  'calendar_failed',
  'calendar_unavailable',
  'draft_upload_failed',
  'reply_timeout',
  'reply_cancelled',
  'reply_invalid_response',
  'reply_provider_failed',
  'review_required',
  'review_unavailable',
];

void main() {
  test('every backend reason has a readable text in both languages', () {
    final previous = L10n.locale;
    addTearDown(() => L10n.setLocale(previous));
    for (final locale in MemoLocale.values) {
      L10n.setLocale(locale);
      for (final code in backendReasonCodes) {
        final key = autonomyReasonKey(code);
        final text = L10n.t(key);
        expect(text, isNot(key), reason: '$code -> $key has no $locale text');
        expect(text.trim(), isNotEmpty);
      }
    }
  });

  test('specific causes are not collapsed into the generic review line', () {
    const generic = 'auto_reason_review';
    // These all used to fall through to the generic line in some or all screens.
    for (final code in [
      'research_failed',
      'group_or_cc',
      'rate_limit',
      'unknown_contact',
      'calendar_failed',
      'calendar_unavailable',
      'message_too_large',
      'prompt_injection',
      'secret_env',
      'drafted',
    ]) {
      expect(autonomyReasonKey(code), isNot(generic), reason: code);
    }
    // Distinct causes get distinct texts.
    final keys = {
      for (final c in backendReasonCodes) autonomyReasonKey(c),
    };
    expect(keys.length, greaterThan(15));
  });

  test('an unknown or missing code still shows the safe generic line', () {
    expect(autonomyReasonKey('something_new'), 'auto_reason_review');
    expect(autonomyReasonKey(null), 'auto_reason_review');
  });
}
