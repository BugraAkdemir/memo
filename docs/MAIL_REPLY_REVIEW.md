# Prepared mail replies: owner review and delivery

The Mail screen has Inbox and Replies sections. Replies lists prepared actions
for the selected account, with Pending, Sent replies and All activity filters.
It is an action history (latest 100 records), not a general IMAP folder browser.
Account connection is under Add account; reply preferences, contacts and mail
security remain accessible from the toolbar menu. Below 850 logical pixels,
Inbox opens a message in the same pane with a Back to inbox control.

Open **Review and send** on a pending mail action, edit its subject/body, and
press **Send**. The account, recipient and In-Reply-To are taken from the stored
action, never accepted from the browser payload. The server rechecks the current
contact permission and outbound DLP. A contact with `never` cannot be sent to
through this workflow. Owner approval works for draft-only contacts too.
Automatic delivery, undo countdowns and owner-channel approvals are implemented
by the separate Step 6 bridge; see [DIGITAL_TWIN_APPROVAL.md](DIGITAL_TWIN_APPROVAL.md).
The existing IMAP Drafts copy is not updated or removed by this editor; the UI
states this explicitly.

## Atomic API contract

`POST /api/twin/send_action` (also `/api/v1/twin/send_action`) is admin-only.
Strict JSON input: `{"id":"action-id","subject":"Re: Meeting","body":"Owner-edited reply"}`.
Success: `{"ok":true,"id":"action-id","state":"sent"}`.
A successful response means SMTP accepted the message; it is not recipient-side
confirmation. The sent action and timestamp persist across restarts.

The serialized SQLite writer atomically reserves `pending → sending` before
SMTP. A parallel request cannot reserve it again; late preparation cannot
replace a sending/sent/uncertain record. Successful SMTP records `sent`;
transport or post-send storage failure is reported as `send_uncertain`.
Interrupted `sending` records become `send_uncertain` on startup. Neither state
is eligible for retry. This deliberately sacrifices automatic retry when SMTP's
outcome is ambiguous, since resending could duplicate a delivered reply.

Stable errors (private SMTP details are not returned):

| HTTP | Code | Owner action |
| --- | --- | --- |
| 400 | `twin_invalid` | Correct invalid subject/body |
| 403 | `mail_reply_blocked` | Remove sensitive content |
| 403 | `mail_reply_contact_denied` | Review contact permission |
| 404 | `twin_not_found` | Refresh; action/account is unavailable |
| 409 | `mail_reply_not_pending` | Refresh the recorded status |
| 409 | `mail_reply_sent_history` | Matching older outgoing evidence exists; do not resend |
| 500 | `mail_reply_send_uncertain` | Check the provider's Sent folder; no blind retry |

The legacy history guard compares original generated body, account, recipient
and an outgoing timestamp at/after action creation. It also applies when the
owner edits the text. This conservatively protects the previously approved live
test whose preparation record still says pending. It can reject a later action
whose identical body was sent through another path; it does not assert SMTP
exactly-once delivery or reconcile external mailbox history automatically.

## Verification

Store tests cover competing reservations, persisted edits/outcome, late updates,
restart recovery, revoked permission, cancellation, header injection and legacy
sent evidence. App tests use a real loopback SMTP server with the production
mail transport, verifying edited body, recipient envelope, thread headers,
sensitive-content blocking, duplicate rejection and SMTP failure. HTTP tests
cover the admin boundary and stable/redacted errors. Widget tests exercise the
real API client against a local adapter: three viewports, editing, double click,
sent filtering, uncertain outcomes and retained text.

No real recipient is involved in these automated tests. Prior approved Gmail
acceptance belongs to the preceding live-test sessions, not this API's test run.

Final verification (2 November 2026): Go build/vet exit 0; all 71 tested Go
packages pass with the race detector and sqlite_fts5. Flutter: `+886: All tests
passed!`; analyze reports only the three existing context infos. UI literal
check: 0 matches in all six staged Dart files. Rendered evidence checker passes.
The final access-revocation scenario also verifies clearing stale drafts and
recovering through Refresh.
