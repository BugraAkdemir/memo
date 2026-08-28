package livemode

import (
	"regexp"
	"strings"
)

// controlTokenRe matches pseudo-directives a realtime model sometimes
// emits into its spoken-text transcript instead of actually speaking —
// e.g. "text-to-speech:one_second_pause", observed when the model was
// nudged toward filler/pause sounds. They are meant as audio control, not
// words, but they arrive on the transcript stream and would otherwise be
// persisted and shown as a chat message. Narrow on purpose: only the
// observed "text-to-speech:<token>" shape, not a general SSML sweep.
var controlTokenRe = regexp.MustCompile(`(?i)\btext-to-speech\s*:\s*[a-z0-9_-]+`)

// toolCallLeakRe matches the model transcribing its own function call as
// spoken text — observed live as a chat bubble reading
// `response:delegate_to_main_model{instruction:User wants to hear a long
// poem. ...}`. The real function call is delivered separately as a
// toolCall message, so the transcribed copy is pure noise. Two forms: a
// closed `{...}` (keep whatever real speech surrounds it) and, because the
// leak is often truncated where real speech resumes, an unclosed `{` that
// runs to the end of this flush (accepts losing a bit of trailing filler
// — far better than showing the JSON). Anchored on `response:<name>{` or a
// bare `<name>_to_main_model{` so it can't eat ordinary prose containing a
// brace.
var (
	toolCallLeakClosedRe = regexp.MustCompile(`(?is)(?:response\s*:\s*)?\b[a-z_][a-z0-9_]*_to_main_model\{[^{}]*\}`)
	toolCallLeakOpenRe   = regexp.MustCompile(`(?is)(?:response\s*:\s*)?\b[a-z_][a-z0-9_]*_to_main_model\{[^}]*$`)
	// A generic `response:<tool>{...}` shape too (some engines prefix any
	// tool call this way), same closed/open pair. Brace must immediately
	// follow the tool name (no space) so it can't match "in response: yes {".
	responseCallClosedRe = regexp.MustCompile(`(?is)\bresponse\s*:\s*[a-z_][a-z0-9_]*\{[^{}]*\}`)
	responseCallOpenRe   = regexp.MustCompile(`(?is)\bresponse\s*:\s*[a-z_][a-z0-9_]*\{[^}]*$`)
)

// nulMarkerRe matches a stray NUL-wrapped internal sentinel (e.g.
// liveDelegateTimeoutMarker). One should never reach a transcript — the
// delegate stream that carries them is drained separately — this is pure
// belt-and-braces so a leak can't be rendered/spoken.
var nulMarkerRe = regexp.MustCompile("\x00[^\x00]*\x00")

// SanitizeModelTranscript strips, from a model-role transcript before it
// becomes an EventTranscript: control-token junk (controlTokenRe),
// verbalized function calls (toolCallLeak*/responseCall*), and any stray
// NUL-wrapped internal marker — then collapses the whitespace the removals
// leave behind. Returns "" when nothing but junk remains, so the caller's
// existing "drop empty transcript" guard skips it entirely.
func SanitizeModelTranscript(s string) string {
	s = nulMarkerRe.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "\x00", "") // any unpaired NUL left over
	s = controlTokenRe.ReplaceAllString(s, "")
	// Closed forms first (surgical), then the truncated-to-end forms.
	s = toolCallLeakClosedRe.ReplaceAllString(s, "")
	s = responseCallClosedRe.ReplaceAllString(s, "")
	s = toolCallLeakOpenRe.ReplaceAllString(s, "")
	s = responseCallOpenRe.ReplaceAllString(s, "")
	return strings.Join(strings.Fields(s), " ")
}
