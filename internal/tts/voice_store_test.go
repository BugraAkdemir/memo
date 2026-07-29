package tts

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestVoiceID_And_Language(t *testing.T) {
	v := Voice{Locale: "tr_TR", Name: "fahrettin", Quality: "medium"}
	if got := v.ID(); got != "tr_TR-fahrettin-medium" {
		t.Errorf("ID() = %q", got)
	}
	if got := v.Language(); got != "tr" {
		t.Errorf("Language() = %q", got)
	}
}

func TestVoiceRepoPath(t *testing.T) {
	v := Voice{Locale: "en_US", Name: "lessac", Quality: "medium"}
	want := "en/en_US/lessac/medium/en_US-lessac-medium.onnx"
	if got := v.repoPath(); got != want {
		t.Errorf("repoPath() = %q, want %q", got, want)
	}
}

func TestParseVoiceID_RoundTrip(t *testing.T) {
	for _, v := range CuratedVoices() {
		parsed := parseVoiceID(v.ID())
		if parsed != v {
			t.Errorf("parseVoiceID(%q) = %+v, want %+v", v.ID(), parsed, v)
		}
	}
}

func TestParseVoiceID_HyphenatedName(t *testing.T) {
	got := parseVoiceID("en_US-north-midwest-medium")
	want := Voice{Locale: "en_US", Name: "north-midwest", Quality: "medium"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestCuratedVoices_CoversTurkishAndEnglish(t *testing.T) {
	langs := map[string]bool{}
	for _, v := range CuratedVoices() {
		langs[v.Language()] = true
	}
	if !langs["tr"] || !langs["en"] {
		t.Errorf("expected both tr and en covered, got %v", langs)
	}
}

func TestVoiceStore_DownloadVoice_SuccessAndListLocalVoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/tr/tr_TR/fahrettin/medium/tr_TR-fahrettin-medium.onnx" {
			w.Write([]byte("fake-onnx-bytes"))
			return
		}
		if r.URL.Path == "/tr/tr_TR/fahrettin/medium/tr_TR-fahrettin-medium.onnx.json" {
			w.Write([]byte(`{"audio":{"sample_rate":22050}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	origBase := hfVoicesBaseURL
	hfVoicesBaseURL = srv.URL + "/"
	defer func() { hfVoicesBaseURL = origBase }()

	dir := t.TempDir()
	s := NewVoiceStore(dir)

	v := Voice{Locale: "tr_TR", Name: "fahrettin", Quality: "medium"}
	if err := s.DownloadVoice(v); err != nil {
		t.Fatalf("DownloadVoice: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		progress := s.GetDownloadProgress()
		if len(progress) == 1 && !progress[0].Active {
			if progress[0].Error != "" {
				t.Fatalf("download errored: %s", progress[0].Error)
			}
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	local := s.ListLocalVoices()
	if len(local) != 1 {
		t.Fatalf("expected 1 local voice, got %d", len(local))
	}
	if local[0].ID() != v.ID() {
		t.Errorf("expected %s, got %s", v.ID(), local[0].ID())
	}
	if local[0].Size == 0 {
		t.Error("expected non-zero size")
	}
}

func TestVoiceStore_DownloadVoice_AlreadyDownloadingRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("x"))
	}))
	defer srv.Close()

	origBase := hfVoicesBaseURL
	hfVoicesBaseURL = srv.URL + "/"
	defer func() { hfVoicesBaseURL = origBase }()

	s := NewVoiceStore(t.TempDir())
	v := Voice{Locale: "en_US", Name: "lessac", Quality: "medium"}

	if err := s.DownloadVoice(v); err != nil {
		t.Fatalf("first DownloadVoice: %v", err)
	}
	// Active is set synchronously (under lock) before DownloadVoice returns,
	// so this check is deterministic — no need to wait for the background
	// goroutine that actually fetches the bytes.
	if err := s.DownloadVoice(v); err == nil {
		t.Error("expected error for duplicate in-flight download")
	}

	// Drain the background download before the test returns and restores
	// hfVoicesBaseURL — otherwise the goroutine's read of that package var
	// races with this deferred restore (caught by -race in an earlier
	// version of this test that left the goroutine blocked indefinitely).
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		progress := s.GetDownloadProgress()
		if len(progress) == 1 && !progress[0].Active {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("download did not finish in time")
}

func TestVoiceStore_ListLocalVoices_SkipsIncompleteDownload(t *testing.T) {
	dir := t.TempDir()
	// .onnx with no .json sidecar — an interrupted/incomplete download.
	if err := os.WriteFile(filepath.Join(dir, "en_US-amy-medium.onnx"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	s := NewVoiceStore(dir)
	if got := s.ListLocalVoices(); len(got) != 0 {
		t.Errorf("expected incomplete download to be skipped, got %d", len(got))
	}
}

func TestVoiceStore_DeleteLocalVoice(t *testing.T) {
	dir := t.TempDir()
	id := "en_US-lessac-medium"
	if err := os.WriteFile(filepath.Join(dir, id+".onnx"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".onnx.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	s := NewVoiceStore(dir)
	if err := s.DeleteLocalVoice(id); err != nil {
		t.Fatalf("DeleteLocalVoice: %v", err)
	}
	if len(s.ListLocalVoices()) != 0 {
		t.Error("expected voice to be gone after delete")
	}
}

func TestVoiceStore_DeleteLocalVoice_RejectsPathTraversal(t *testing.T) {
	s := NewVoiceStore(t.TempDir())
	if err := s.DeleteLocalVoice("../../etc/passwd"); err == nil {
		t.Error("expected path traversal attempt to be rejected")
	}
}
