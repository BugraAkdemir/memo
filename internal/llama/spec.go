// SPDX-License-Identifier: AGPL-3.0-or-later

package llama

import (
	"os"
	"strings"
	"sync"
)

// specNgramArgs turn on n-gram speculative decoding: the server drafts the next
// tokens by looking the last 16 up in the text it already has (prompt and own
// output) and verifies the draft in one batch, so it costs no extra model. Output
// is the same either way (multi-turn and tool-request transcripts compared
// identical); what changes is speed, because decode is memory-bandwidth bound and
// verifying several tokens costs about what one does.
//
// Measured on Phi-3 Q4, Ryzen 5 5600H (CPU + Vulkan prompt offload), greedy:
//
//	copying / rewriting code   14.6 -> 50.7 t/s   (76 of 96 drafted tokens accepted)
//	quoting from the prompt    15.3 -> 17.6 t/s
//	summarising, rewriting prose, free text: unchanged (no draft is made)
//
// The lookup length and hit count are not the defaults. With the defaults (12,
// one hit) prose tasks drafted 96 tokens and had none accepted, which cost 8-11%
// decode speed; 16 and two hits removed that without losing the code-copy gain.
// ngram-mod changed the output of a repeated request and ngram-map-k slowed free
// text to 11 t/s, so neither is used.
var specNgramArgs = []string{
	"--spec-type", "ngram-simple",
	"--spec-ngram-simple-size-n", "16",
	"--spec-ngram-simple-min-hits", "2",
}

var specFlagCache sync.Map // map[string]bool

// flagsAccepted probes (once per binary) whether bin gets past argument parsing
// with flags, remembering only a concluded answer.
func flagsAccepted(cache *sync.Map, bin string, flags []string) bool {
	key := probeCacheKey(bin)
	if cached, ok := cache.Load(key); ok {
		return cached.(bool)
	}
	supported, concluded := probeFlags(bin, flags)
	if !concluded {
		return false
	}
	cache.Store(key, supported)
	return supported
}

// speculativeArgs returns specNgramArgs for a chat model on a build that accepts
// them, or nil. Left out, because they were not verified there: the embedding
// sidecar (nothing to draft), a model with a multimodal projector, and the swarm
// coordinator. MEMO_LLAMA_SPECULATIVE=off switches it off without a rebuild.
func speculativeArgs(bin string, embedding, hasMmproj, rpc bool) []string {
	if embedding || hasMmproj || rpc || strings.EqualFold(os.Getenv("MEMO_LLAMA_SPECULATIVE"), "off") {
		return nil
	}
	if !flagsAccepted(&specFlagCache, bin, specNgramArgs) {
		return nil
	}
	return specNgramArgs
}
