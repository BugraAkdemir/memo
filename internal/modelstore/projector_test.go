package modelstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func waitDownload(t *testing.T, s *Store, repo, name string) DownloadProgress {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		s.mu.RLock()
		e := s.downloads[downloadKey(repo, name)]
		finished := e != nil && e.cancelFn == nil
		var p DownloadProgress
		if e != nil {
			p = *e.progress
		}
		s.mu.RUnlock()
		if finished {
			return p
		}
		select {
		case <-deadline.C:
			t.Fatal("download did not settle")
		case <-tick.C:
		}
	}
}

func TestDownloadModelIncludesProjectorAndKeepsModelsIsolated(t *testing.T) {
	mux := http.NewServeMux()
	for _, model := range []string{"one", "two"} {
		repo := "acme/" + model
		content := []byte("projector for " + model)
		hash := sha256.Sum256(content)
		mux.HandleFunc("/api/models/"+repo+"/tree/main", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("recursive") != "true" {
				t.Error("missing recursive tree query")
			}
			if r.URL.Query().Get("cursor") == "" {
				w.Header().Set("Link", fmt.Sprintf(`<https://huggingface.co/api/models/%s/tree/main?recursive=true&cursor=second>; rel="next"`, repo))
				json.NewEncoder(w).Encode([]hfTreeItem{{Path: "model.gguf", Size: 5}})
			} else {
				json.NewEncoder(w).Encode([]hfTreeItem{{Path: "vision/mmproj-F16.gguf", Size: int64(len(content)), LFS: &hfLFS{OID: hex.EncodeToString(hash[:])}}})
			}
		})
		mux.HandleFunc("/api/models/"+repo, func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(hfModelInfo{Tags: []string{"vision"}})
		})
		mux.HandleFunc("/"+repo+"/resolve/main/", func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "mmproj") {
				w.Write(content)
			} else {
				w.Write([]byte("model"))
			}
		})
	}
	store, _ := newTestStore(t, mux)
	for _, model := range []string{"one", "two"} {
		if err := store.DownloadModel("acme/"+model, "model.gguf", 5); err != nil {
			t.Fatal(err)
		}
	}
	for _, model := range []string{"one", "two"} {
		p := waitDownload(t, store, "acme/"+model, "model.gguf")
		if p.Active || p.Error != "" || p.Percent != 100 || p.Downloaded != p.TotalBytes || p.Downloaded != int64(5+len("projector for "+model)) {
			t.Fatalf("unexpected progress: %+v", p)
		}
		modelPath := ""
		for _, m := range store.ListLocalModels() {
			if m.RepoID == "acme/"+model {
				modelPath = m.Path
			}
		}
		if modelPath == "" {
			t.Fatalf("missing model for repo %s", model)
		}
		got, err := os.ReadFile(findMmproj(modelPath))
		if err != nil || string(got) != "projector for "+model {
			t.Fatalf("projector for %s: %q %v", model, got, err)
		}
	}
	models := store.ListLocalModels()
	if len(models) != 2 {
		t.Fatalf("projectors listed as models: %+v", models)
	}
	for _, m := range models {
		if m.MmprojPath == "" || !m.SupportsVision {
			t.Fatalf("missing local vision metadata: %+v", m)
		}
	}
}

func TestProjectorFailureIsVisibleAndRetrySucceeds(t *testing.T) {
	var fail atomic.Bool
	fail.Store(true)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/models/acme/vision/tree/main", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]hfTreeItem{{Path: "model.gguf", Size: 5}, {Path: "mmproj.gguf", Size: 4}})
	})
	mux.HandleFunc("/api/models/acme/vision", func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(hfModelInfo{}) })
	mux.HandleFunc("/acme/vision/resolve/main/mmproj.gguf", func(w http.ResponseWriter, r *http.Request) {
		if fail.Load() {
			http.Error(w, "failed", 503)
			return
		}
		w.Write([]byte("proj"))
	})
	mux.HandleFunc("/acme/vision/resolve/main/model.gguf", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("model")) })
	store, _ := newTestStore(t, mux)
	if err := store.DownloadModel("acme/vision", "model.gguf", 5); err != nil {
		t.Fatal(err)
	}
	p := waitDownload(t, store, "acme/vision", "model.gguf")
	if p.Error == "" || !p.Active || !strings.Contains(p.Error, "projector") {
		t.Fatalf("projector failure hidden: %+v", p)
	}
	if len(store.ListLocalModels()) != 0 {
		t.Fatal("model published despite projector failure")
	}
	store.CancelDownload("acme/vision", "model.gguf")
	fail.Store(false)
	if err := store.DownloadModel("acme/vision", "model.gguf", 5); err != nil {
		t.Fatal(err)
	}
	p = waitDownload(t, store, "acme/vision", "model.gguf")
	if p.Error != "" || p.Active || p.Percent != 100 {
		t.Fatalf("retry failed: %+v", p)
	}
}

func TestCancelModelDownloadCancelsItsProjector(t *testing.T) {
	started := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/api/models/acme/vision/tree/main", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]hfTreeItem{{Path: "mmproj.gguf", Size: 4}})
	})
	mux.HandleFunc("/acme/vision/resolve/main/mmproj.gguf", func(w http.ResponseWriter, r *http.Request) { close(started); <-r.Context().Done() })
	store, dir := newTestStore(t, mux)
	if err := store.DownloadModel("acme/vision", "model.gguf", 5); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("projector request not started")
	}
	store.CancelDownload("acme/vision", "model.gguf")
	p := waitDownload(t, store, "acme/vision", "model.gguf")
	if p.Error == "" || !p.Active {
		t.Fatalf("cancel did not settle: %+v", p)
	}
	if len(store.ListLocalModels()) != 0 {
		t.Fatal("model downloaded after cancel")
	}
	filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err == nil && strings.HasSuffix(p, ".downloading") {
			t.Errorf("partial file remains: %s", p)
		}
		return nil
	})
}

func TestSelectProjector(t *testing.T) {
	cases := []struct {
		name, model string
		files       []string
		want        string
		wantErr     bool
	}{
		{"text only", "model.gguf", []string{"model.gguf"}, "", false},
		{"one projector", "model.gguf", []string{"mmproj-F16.gguf"}, "mmproj-F16.gguf", false},
		{"precision variants", "model.Q4_K_M.gguf", []string{"mmproj-model-BF16.gguf", "mmproj-model-F16.gguf"}, "mmproj-model-F16.gguf", false},
		{"model name match", "model.Q4_K_M.gguf", []string{"mmproj-other-F16.gguf", "mmproj-model-F16.gguf"}, "mmproj-model-F16.gguf", false},
		{"folder match", "small/model.gguf", []string{"big/mmproj.gguf", "small/mmproj.gguf"}, "small/mmproj.gguf", false},
		{"ambiguous", "model.gguf", []string{"mmproj-one.gguf", "mmproj-two.gguf"}, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var tree []hfTreeItem
			for _, name := range tc.files {
				tree = append(tree, hfTreeItem{Path: name})
			}
			got, err := selectProjector(tc.model, tree)
			if (err != nil) != tc.wantErr {
				t.Fatalf("error: %v", err)
			}
			actual := ""
			if got != nil {
				actual = got.Path
			}
			if actual != tc.want {
				t.Fatalf("got %q want %q", actual, tc.want)
			}
		})
	}
}

func TestDownloadRejectsUnsafePathsAndTreePagination(t *testing.T) {
	store, _ := newTestStore(t, http.NewServeMux())
	for _, name := range []string{"../model.gguf", "/model.gguf", `C:\model.gguf`, `dir\model.gguf`, "dir/../model.gguf"} {
		if err := store.DownloadModel("acme/model", name, 1); err == nil {
			t.Errorf("accepted %q", name)
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/models/acme/model/tree/main", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Link", `<https://example.com/tree>; rel="next"`)
		w.Write([]byte("[]"))
	})
	store, _ = newTestStore(t, mux)
	if _, err := store.GetModelFiles("acme/model"); err == nil {
		t.Fatal("followed unsafe pagination")
	}
}

func TestCorruptProjectorIsNotPublished(t *testing.T) {
	mux := http.NewServeMux()
	hash := sha256.Sum256([]byte("good"))
	mux.HandleFunc("/api/models/acme/vision/tree/main", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]hfTreeItem{{Path: "mmproj.gguf", Size: 4, LFS: &hfLFS{OID: hex.EncodeToString(hash[:])}}})
	})
	mux.HandleFunc("/acme/vision/resolve/main/mmproj.gguf", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("oops")) })
	store, dir := newTestStore(t, mux)
	err := store.downloadWithProjector(context.Background(), &downloadEntry{progress: &DownloadProgress{}}, "acme/vision", "model.gguf", 5)
	if err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("corrupt projector accepted: %v", err)
	}
	filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err == nil && strings.HasSuffix(p, ".gguf") {
			t.Errorf("corrupt projector published: %s", p)
		}
		return nil
	})
}

func TestRepairExistingFlatModelDownloadsOnlyProjector(t *testing.T) {
	const repo = "acme/vision"
	content := []byte("existing model")
	hash := sha256.Sum256(content)
	var weightRequests atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/api/models/"+repo+"/tree/main", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]hfTreeItem{{Path: "model.gguf", Size: int64(len(content)), LFS: &hfLFS{OID: hex.EncodeToString(hash[:])}}, {Path: "mmproj.gguf", Size: 4}})
	})
	mux.HandleFunc("/api/models/"+repo, func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(hfModelInfo{}) })
	mux.HandleFunc("/"+repo+"/resolve/main/mmproj.gguf", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("proj")) })
	mux.HandleFunc("/"+repo+"/resolve/main/model.gguf", func(w http.ResponseWriter, r *http.Request) { weightRequests.Add(1); w.Write(content) })
	store, dir := newTestStore(t, mux)
	filename := filepath.Join(dir, "model.gguf")
	if err := os.WriteFile(filename, content, 0644); err != nil {
		t.Fatal(err)
	}
	if err := saveModelMeta(filename, &modelMeta{RepoID: repo}); err != nil {
		t.Fatal(err)
	}
	if err := store.DownloadModel(repo, "model.gguf", int64(len(content))); err != nil {
		t.Fatal(err)
	}
	p := waitDownload(t, store, repo, "model.gguf")
	if p.Error != "" || p.Active || p.Percent != 100 {
		t.Fatalf("repair failed: %+v", p)
	}
	if weightRequests.Load() != 0 {
		t.Fatal("valid existing weights were downloaded again")
	}
	if got, err := os.ReadFile(findMmproj(filename)); err != nil || string(got) != "proj" {
		t.Fatalf("projector: %q %v", got, err)
	}
	// A different same-size file must not pass the LFS hash check.
	if err := os.WriteFile(filename, []byte("corrupted data"), 0644); err != nil {
		t.Fatal(err)
	}
	if existingModelMatches(context.Background(), filename, hfTreeItem{Size: int64(len(content)), LFS: &hfLFS{OID: hex.EncodeToString(hash[:])}}) {
		t.Fatal("same-size corrupt weights considered reusable")
	}
	// Delete owns only its private projector directory, never a sibling's files.
	other := filepath.Join(dir, "other.gguf.mmproj")
	if err := os.MkdirAll(other, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, "mmproj.gguf"), []byte("other"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteLocalModel(filename); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filename + ".mmproj"); !os.IsNotExist(err) {
		t.Fatal("owned projector not removed")
	}
	if _, err := os.Stat(filename + ".meta.json"); !os.IsNotExist(err) {
		t.Fatal("metadata not removed")
	}
	if _, err := os.Stat(filepath.Join(other, "mmproj.gguf")); err != nil {
		t.Fatal("another model's projector removed")
	}
}

func TestBundleWithoutProjectorStillDownloadsTextModel(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/models/acme/text/tree/main", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]hfTreeItem{{Path: "nested/model.gguf", Size: 5}})
	})
	mux.HandleFunc("/api/models/acme/text", func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(hfModelInfo{}) })
	mux.HandleFunc("/acme/text/resolve/main/nested/model.gguf", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("model")) })
	store, dir := newTestStore(t, mux)
	if err := store.DownloadModel("acme/text", "nested/model.gguf", 5); err != nil {
		t.Fatal(err)
	}
	p := waitDownload(t, store, "acme/text", "nested/model.gguf")
	if p.Error != "" || p.Active {
		t.Fatalf("text model failed: %+v", p)
	}
	if _, err := os.Stat(filepath.Join(dir, "nested", "model.gguf")); err != nil {
		t.Fatal(err)
	}
}
