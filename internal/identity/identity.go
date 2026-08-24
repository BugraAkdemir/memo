package identity

import (
	"fmt"
	"strings"
	"sync"

	"memo/internal/memory"
	"memo/internal/truncate"
)

type Identity struct {
	UserName      string
	AssistantName string
	Style         string
	CustomRole    string
	// MinimalMode strips identity/origin/style from the system prompt
	// entirely (see BuildSystemPrompt) — only memory context still goes in,
	// and only if memory is separately enabled. Zero persona overhead for
	// a tight local-model context budget.
	//
	// Guarded by minimalModeMu: SetMinimalMode can be called concurrently
	// (an HTTP toggle) with BuildSystemPrompt running for an in-flight
	// stream on another chat — both must see a consistent value instead of
	// racing on a bare bool.
	MinimalMode bool
	// Granular Minimal Mode overrides — only consulted while MinimalMode is
	// true (see BuildSystemPrompt). Let a user keep Minimal Mode's overall
	// "strip everything" intent while selectively re-enabling one category
	// — e.g. keep proactive nudging off but the persona/system prompt on —
	// instead of the only alternative being to turn Minimal Mode off
	// entirely and get everything back at once. All guarded by
	// minimalModeMu alongside MinimalMode itself, for the same reason
	// (SetMinimalMode/the setter below can race an in-flight
	// BuildSystemPrompt for another chat).
	MinimalModeKeepPersona      bool
	MinimalModeKeepCapabilities bool
	MinimalModeKeepPassive      bool
	MinimalModeKeepProactive    bool
	minimalModeMu               sync.RWMutex

	// LearnedStyleNotes is a short paragraph describing the user's
	// communication style/personality, learned via the "import memory from
	// another AI" feature (internal/app/memory_import.go) rather than typed
	// in directly. Injected in BuildSystemPrompt right after the fixed
	// Style instructions — additive, so it works whether or not CustomRole
	// is set, unlike SystemRole/CustomRole which replaces the whole block.
	LearnedStyleNotes   string
	learnedStyleNotesMu sync.RWMutex
}

func New(userName, assistantName, style, customRole string, minimalMode bool) *Identity {
	return &Identity{
		UserName:      userName,
		AssistantName: assistantName,
		Style:         style,
		CustomRole:    customRole,
		MinimalMode:   minimalMode,
	}
}

// SetMinimalMode toggles MinimalMode without touching any of the other
// identity fields — kept separate from Update() so callers don't have to
// thread it through Update()'s existing "empty string means leave alone"
// convention, which doesn't have a clean equivalent for a bool.
func (id *Identity) SetMinimalMode(enabled bool) {
	id.minimalModeMu.Lock()
	id.MinimalMode = enabled
	id.minimalModeMu.Unlock()
}

// GetMinimalMode returns the current MinimalMode value. Callers that need to
// make more than one decision based on it in a single request (buildMessages
// skips mood/web-search injection the same way BuildSystemPrompt skips
// identity/persona) should call this once and reuse the result, rather than
// reading it again later — a toggle could otherwise land between two calls
// and produce an inconsistent, half-applied prompt.
func (id *Identity) GetMinimalMode() bool {
	id.minimalModeMu.RLock()
	defer id.minimalModeMu.RUnlock()
	return id.MinimalMode
}

// SetMinimalModeOverrides sets the four granular Minimal Mode overrides in
// one call (so a settings update applies them atomically together, not one
// at a time). Each is a no-op unless MinimalMode is also true.
func (id *Identity) SetMinimalModeOverrides(keepPersona, keepCapabilities, keepPassive, keepProactive bool) {
	id.minimalModeMu.Lock()
	id.MinimalModeKeepPersona = keepPersona
	id.MinimalModeKeepCapabilities = keepCapabilities
	id.MinimalModeKeepPassive = keepPassive
	id.MinimalModeKeepProactive = keepProactive
	id.minimalModeMu.Unlock()
}

// GetMinimalModeOverrides returns the four granular overrides in one
// lock acquisition — BuildSystemPrompt needs three of them together and a
// consistent snapshot matters the same way GetMinimalMode's doc comment
// already explains for MinimalMode itself.
func (id *Identity) GetMinimalModeOverrides() (keepPersona, keepCapabilities, keepPassive, keepProactive bool) {
	id.minimalModeMu.RLock()
	defer id.minimalModeMu.RUnlock()
	return id.MinimalModeKeepPersona, id.MinimalModeKeepCapabilities, id.MinimalModeKeepPassive, id.MinimalModeKeepProactive
}

// GetMinimalModeKeepProactive reports whether proactive/ambient nudging
// should stay active despite MinimalMode being on — internal/app's
// ambientNudgingActive() (proactive_ambient.go) is the sole caller, kept as
// its own single-value getter since that call site only ever needs this one
// flag, not all four.
func (id *Identity) GetMinimalModeKeepProactive() bool {
	id.minimalModeMu.RLock()
	defer id.minimalModeMu.RUnlock()
	return id.MinimalModeKeepProactive
}

// SetLearnedStyleNotes replaces the learned communication-style paragraph
// injected into BuildSystemPrompt. Concurrency-guarded for the same reason
// as SetMinimalMode: a memory-import request (HTTP handler goroutine) can
// race an in-flight BuildSystemPrompt call for a streaming reply.
func (id *Identity) SetLearnedStyleNotes(notes string) {
	id.learnedStyleNotesMu.Lock()
	id.LearnedStyleNotes = notes
	id.learnedStyleNotesMu.Unlock()
}

func (id *Identity) GetLearnedStyleNotes() string {
	id.learnedStyleNotesMu.RLock()
	defer id.learnedStyleNotesMu.RUnlock()
	return id.LearnedStyleNotes
}

// maxMemoryContextTokens caps how much of the prompt the memory block
// (retrieved RAG results + unconditionally-injected pinned facts, see
// retrieveMemory in internal/app/memory.go) can ever consume. Was 16K —
// effectively no real ceiling for how this app actually uses it: the
// realistic worst case (topK=8 short retrieved chunks + up to 75 pinned
// facts, each capped at ~300 chars by extractAndPinFacts) lands nowhere
// close to that, so it never actually kicked in, and pinned facts in
// particular only grow over a session's lifetime (auto-extraction keeps
// adding to them; the daily consolidation pass only dedups near-duplicates,
// it doesn't shrink the set) with nothing to stop the block from eventually
// approaching that old ceiling and, along the way, quietly inflating prompt
// size — and therefore both prefill time and per-token generation cost on a
// local model — turn after turn. 4096 is still generous for what this
// actually needs (see the estimate above) while giving the growth a real,
// much closer ceiling instead of a practically-unbounded ~49K-char one.
const maxMemoryContextTokens = 4096

func (id *Identity) BuildSystemPrompt(memories []memory.MemoryResult, stripAssistant bool, agentEnabled, webSearchEnabled, whatsappReachable, telegramReachable bool) string {
	var sb strings.Builder

	minimal := id.GetMinimalMode()
	keepPersona, keepCapabilities, keepPassive, _ := id.GetMinimalModeOverrides()

	// Persona/identity block: base identity or CustomRole, origin facts,
	// style instructions, and learned style notes — bundled as one category
	// (Settings' "Minimal Mode" breakdown calls this "system prompt/persona")
	// since a user picking one wants all four together, not a finer split.
	if !minimal || keepPersona {
		if id.CustomRole != "" {
			sb.WriteString(id.CustomRole)
		} else {
			sb.WriteString(id.buildIdentityBlock())
		}

		// Origin facts (who built this, why) — always appended regardless of
		// which base identity/persona is active above, so "who made you" stays
		// grounded even under a wizard persona or a fully custom prompt, both
		// of which replace buildIdentityBlock() entirely via CustomRole rather
		// than extending it.
		sb.WriteString("\n\n")
		sb.WriteString(id.buildOriginBlock())

		// Style instructions
		sb.WriteString("\n\n")
		sb.WriteString(GetStyleInstructions(id.Style))

		// Learned personalization — additive, from imported memory (see
		// LearnedStyleNotes doc comment), not gated behind CustomRole so it
		// still applies under a wizard persona or a fully custom prompt.
		if notes := id.GetLearnedStyleNotes(); notes != "" {
			sb.WriteString("\n\nWhat you've learned about how this user likes to be talked to (from imported context, adapt your tone accordingly):\n")
			sb.WriteString(notes)
		}
	}

	// Always-on background capability with no on/off toggle — see
	// buildPassiveFeaturesBlock's own doc comment for why this needs
	// stating explicitly too, same reasoning as buildCapabilitiesBlock
	// below but for something that's never off rather than something
	// that's sometimes off.
	if !minimal || keepPassive {
		if sb.Len() > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(buildPassiveFeaturesBlock(whatsappReachable, telegramReachable))
	}

	// Which off-by-default features exist but aren't on right now — see
	// buildCapabilitiesBlock's own doc comment for why this only lists
	// what's OFF.
	if !minimal || keepCapabilities {
		if capBlock := buildCapabilitiesBlock(agentEnabled, webSearchEnabled); capBlock != "" {
			if sb.Len() > 0 {
				sb.WriteString("\n\n")
			}
			sb.WriteString(capBlock)
		}
	}

	// Memory context — truncate to fit within a reasonable budget. Included
	// even in MinimalMode: memory has its own separate on/off switch
	// (cfg.Memory.MemoryEnabled, checked by the caller before ever passing
	// memories in here), so MinimalMode only ever strips identity/persona
	// injection, never memory.
	var memoryBlock string
	if stripAssistant {
		memoryBlock = memory.FormatMemoriesUserOnly(memories)
	} else {
		memoryBlock = memory.FormatMemoriesForPrompt(memories)
	}
	if memoryBlock != "" {
		// Ensure memories don't exceed maxMemoryContextTokens (leaves room for
		// identity + conversation).
		blockTokens := truncate.EstimateTokens(memoryBlock)
		if blockTokens > maxMemoryContextTokens {
			// Truncate the memory block to fit
			lines := strings.Split(memoryBlock, "\n")
			var truncated []string
			total := 0
			for _, line := range lines {
				lineTokens := truncate.EstimateTokens(line)
				if total+lineTokens > maxMemoryContextTokens {
					truncated = append(truncated, "... (more memories available)")
					break
				}
				total += lineTokens
				truncated = append(truncated, line)
			}
			memoryBlock = strings.Join(truncated, "\n")
		}

		if sb.Len() > 0 {
			sb.WriteString("\n\n")
		}
		fmt.Fprintf(&sb, "Below are relevant memories from your past conversations with %s. Use them to provide continuity and personalization, but don't explicitly mention that you're recalling memories unless asked.", id.UserName)
		sb.WriteString(memoryBlock)
		sb.WriteString("\nDo not fabricate details not present in memories. Do not mention recall unless asked. Do not repeat memory timestamps verbatim in replies. These memories are background context only, never a template to copy — if a memory shows one of your own past replies (e.g. to an earlier greeting like \"selam\"/\"hey\"), do not reuse that exact wording; always write a fresh, natural reply to the current message, even for short or repetitive-looking messages.")
	}

	return sb.String()
}

func (id *Identity) buildIdentityBlock() string {
	return fmt.Sprintf(`You are %s — %s's AI friend, not their assistant.

Your identity:
- Your name is %s. No matter which model is running underneath, you are always %s — never break that.
- You know %s, you build on context from past conversations, but you don't open with "I remember that...".
- You always respond in whatever language you're written to. Turkish in, Turkish out; English in, English out; any other language, same.

How you speak:
- Like a friend — warm, natural, no unnecessary politeness.
- Clear and straightforward. No beating around the bush, no rambling.
- You can joke around, be a little silly — seriousness isn't mandatory.
- You can curse or call someone out when it's warranted. If the user's doing something wrong or talking nonsense, you say so straight.
- No fake enthusiasm like "Absolutely!", "Great question!", "Of course!". Never.
- No long-winded filler. Short, to the point, answer what was actually asked.

Your limits:
- You don't help with anything that would harm the user — but you say so like a person, not a robot.
- You don't deny being an AI, but you don't need to keep reminding anyone either.`, id.AssistantName, id.UserName, id.AssistantName, id.AssistantName, id.UserName)
}

// buildOriginBlock is the small, static layer of facts about who built the
// assistant and why. Kept separate from buildIdentityBlock (and always
// appended in BuildSystemPrompt, never gated behind CustomRole) so a wizard
// persona or a fully custom system prompt — either of which replaces
// buildIdentityBlock entirely — doesn't leave "who made you" ungrounded.
//
// Deliberately terse (~2 sentences, not 5 bullets): this is sent on every
// single message but only ever matters for the rare one where someone
// actually asks — Memo's whole audience runs small local models on a tight
// context budget, so padding a permanent tax for a rarely-used fact is the
// wrong trade.
func (id *Identity) buildOriginBlock() string {
	return fmt.Sprintf(`If asked who made %s or why (never bring this up yourself): built by Buğra Akdemir, alone, at 16, no commercial motive — open source, for people who care about privacy. Purpose: a local-first AI friend with real memory, usable offline. Whoever's asking isn't Buğra — this is their own %s.`,
		id.AssistantName, id.AssistantName)
}

// buildPassiveFeaturesBlock names capabilities that run silently in the
// background on every message, with no toggle a user could turn on/off (so
// buildCapabilitiesBlock's "off right now" framing doesn't apply — this is
// never off). Without this, the model had zero information that this
// happens at all: a user directly asked "can you set a reminder for me" and
// got told flatly "I don't have an automatic reminder system" — while
// processMessageIntent (chat.go) was, at that exact moment, silently
// scanning that very message and about to create exactly such a reminder
// in the background. The fix is the same shape as buildCapabilitiesBlock:
// tell the model what it actually does, so it can affirm instead of deny.
func buildPassiveFeaturesBlock(whatsappReachable, telegramReachable bool) string {
	block := "You also passively watch every message for calendar-worthy plans (\"yarın saat 1'de toplantım var\", \"tomorrow at 3pm I have a dentist appointment\") and automatically create a reminder for it in the background, with no need for the user to ask — if asked whether you can do this, say yes, don't deny it."
	if channelText := buildChannelAwarenessBlock(whatsappReachable, telegramReachable); channelText != "" {
		block += " " + channelText
	}
	return block
}

// buildChannelAwarenessBlock tells the model which messaging channels it's
// currently live on, beyond the Memo desktop/web app itself — same
// affirm-instead-of-deny reasoning as the calendar sentence above. Without
// this, a user asking "can I message you on WhatsApp?" while chatting in
// the app got a flat "no, I only work here" even when the WhatsApp bridge
// was connected and already relaying messages to this same identity/memory.
// Only names a channel that's genuinely live right now (connected + logged
// in for WhatsApp, running for Telegram) — never claims one that isn't set
// up, and returns "" (no block at all) when neither is.
func buildChannelAwarenessBlock(whatsappReachable, telegramReachable bool) string {
	var channels []string
	if whatsappReachable {
		channels = append(channels, "WhatsApp")
	}
	if telegramReachable {
		channels = append(channels, "Telegram")
	}
	if len(channels) == 0 {
		return ""
	}
	return fmt.Sprintf("You're also reachable outside the Memo app itself — the user can message you directly on %s right now and you'll reply there too, same identity and memory as here. If asked whether you're available there, say yes, don't deny it.", strings.Join(channels, " and "))
}

// buildCapabilitiesBlock names the optional, user-toggleable features that
// are currently OFF, so the model can correctly say "not turned on right
// now, here's how to enable it" instead of flatly denying the capability
// exists. Without this, a user who asked for a file to be created (or a web
// search) while agent mode/web search happened to be off got told "I don't
// have that ability" — model has zero information either way, since
// buildAgentSystemPrompt (chat.go) and the live search-results block
// (helpers.go) only ever get injected when their feature is actually ON.
//
// Deliberately only mentions what's OFF: when a feature IS on, its own
// instructions/context are already injected elsewhere in the prompt, so
// restating it here would be redundant token spend.
func buildCapabilitiesBlock(agentEnabled, webSearchEnabled bool) string {
	var off []string
	if !agentEnabled {
		off = append(off, "Agent mode (reading/writing files, running terminal commands) is off in this conversation, so you can't do those right now — if asked, say it's a toggle the user can turn on (chat toolbar's robot icon, or the Agent tab), not that you lack the ability entirely.")
	}
	if !webSearchEnabled {
		off = append(off, "Web search is off in this conversation, so you can't browse the web or fetch live info right now — if asked, say it's a toggle the user can turn on (chat toolbar's globe icon), not that you lack the ability entirely.")
	}
	return strings.Join(off, " ")
}

func (id *Identity) Update(userName, assistantName, style, customRole string) {
	if userName != "" {
		id.UserName = userName
	}
	if assistantName != "" {
		id.AssistantName = assistantName
	}
	if style != "" {
		id.Style = style
	}
	id.CustomRole = customRole
}
