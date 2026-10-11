// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"fmt"
	"testing"

	"memo/internal/memory"
)

func pinnedFact(id, content string) memory.MemoryResult {
	return memory.MemoryResult{ID: id, Content: content, Source: "explicit", Importance: 5, MatchType: "pinned"}
}

func chatHit(id, content string) memory.MemoryResult {
	return memory.MemoryResult{ID: id, Content: content, Source: "interaction", Importance: 3}
}

func countPinned(rs []memory.MemoryResult) int {
	n := 0
	for _, r := range rs {
		if r.Source == "explicit" && r.Importance >= 5 {
			n++
		}
	}
	return n
}

// The ranked pinned list is already the top PinnedFactsPerTurn of the pinned
// facts for the query. A pinned fact that also comes back from the similarity
// search is one that list left out; letting it in made the prompt carry N+1
// pinned facts. Facts that differ in a number are not "near duplicates" of each
// other, so nothing else was catching it — it surfaced only when the search
// happened to return one, about 2% of runs of the integration test.
func TestMergePinnedFirst_ASearchHitThatIsPinnedDoesNotExceedTheCap(t *testing.T) {
	var pinned []memory.MemoryResult
	for i := 0; i < 10; i++ {
		pinned = append(pinned, pinnedFact(fmt.Sprintf("p%d", i), fmt.Sprintf("kullanici hakkinda kalici gercek numara %d", i)))
	}
	retrieved := []memory.MemoryResult{
		pinnedFact("p10", "kullanici hakkinda kalici gercek numara 10"), // left out of the ranked top 10
		chatHit("c1", "bambaska bir konusma ozeti ve baska kelimeler"),
	}

	got := mergePinnedFirst(pinned, retrieved)

	if n := countPinned(got); n != 10 {
		t.Errorf("pinned facts in the merged memory = %d, want exactly the 10 ranked ones", n)
	}
	if len(got) != 11 {
		t.Errorf("merged length = %d, want the 10 pinned facts plus the one ordinary hit", len(got))
	}
	if got[len(got)-1].ID != "c1" {
		t.Errorf("the ordinary search hit was lost or misplaced: %+v", got[len(got)-1])
	}
}

func TestMergePinnedFirst_PinnedComeFirstAndDuplicatesGo(t *testing.T) {
	pinned := []memory.MemoryResult{pinnedFact("p1", "kedimin adi Zeytin ve yasi uc")}
	retrieved := []memory.MemoryResult{
		chatHit("c1", "alisveris listesi icin sut ekmek yumurta"),
		pinnedFact("p1", "kedimin adi Zeytin ve yasi uc"),    // same row: dropped
		chatHit("c2", "kedimin adi Zeytin ve yasi uc dedim"), // restates a pinned fact: dropped
	}

	got := mergePinnedFirst(pinned, retrieved)

	if len(got) != 2 || got[0].ID != "p1" || got[1].ID != "c1" {
		ids := make([]string, len(got))
		for i, r := range got {
			ids[i] = r.ID
		}
		t.Errorf("merged ids = %v, want [p1 c1] (pinned first; the same row and the restatement dropped)", ids)
	}
}

func TestMergePinnedFirst_NothingPinnedLeavesTheSearchResultsAlone(t *testing.T) {
	retrieved := []memory.MemoryResult{chatHit("c1", "birinci sohbet kaydi"), chatHit("c2", "ikinci baska kayit")}

	got := mergePinnedFirst(nil, retrieved)

	if len(got) != 2 || got[0].ID != "c1" || got[1].ID != "c2" {
		t.Errorf("merged = %+v, want the two search hits unchanged and in order", got)
	}
}
