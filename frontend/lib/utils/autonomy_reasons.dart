/// Maps the reason code the backend puts on an autonomous action (the Dijital
/// İkiz queue, the mail reply card, the mail list) to an L10n key.
///
/// One table on purpose: three screens used to keep their own copy of this
/// switch, two had drifted apart, and codes without a case all showed the same
/// vague "evidence or model unavailable" line. The codes come from
/// internal/autonomy (engine finish reasons, shield/loop blocks, and the
/// generation errors).
String autonomyReasonKey(dynamic code) => switch (code) {
  'drafted' => 'auto_reason_drafted',
  'follow_up' => 'auto_reason_follow_up',
  'user_cancelled' => 'auto_reason_user_cancelled',
  'no_action' => 'auto_reason_none',
  'stale_message' => 'auto_reason_stale',
  'contact_manual' => 'auto_reason_manual',
  'contact_never' => 'auto_reason_never',
  'unknown_contact' => 'auto_reason_unknown_contact',
  'automated_sender' => 'auto_reason_automated',
  'group_or_cc' => 'auto_reason_group',
  'rate_limit' => 'auto_reason_rate_limit',
  'message_too_large' => 'auto_reason_too_large',
  'prompt_injection' || 'encoded_instruction' => 'auto_reason_injection',
  'secret_api_key' ||
  'secret_private_key' ||
  'secret_env' ||
  'secret_otp' => 'auto_reason_dlp',
  'research_failed' => 'auto_reason_research',
  'calendar_failed' || 'calendar_unavailable' => 'auto_reason_calendar',
  'draft_upload_failed' => 'auto_reason_upload',
  'reply_timeout' => 'auto_reason_timeout',
  'reply_cancelled' => 'auto_reason_cancelled',
  'reply_invalid_response' => 'auto_reason_invalid_response',
  'reply_provider_failed' => 'auto_reason_provider_failed',
  _ => 'auto_reason_review',
};
