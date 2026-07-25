package tts

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestNewSynthesizer(t *testing.T) {
	s := NewSynthesizer("/some/bin", "/some/model.onnx")
	if s == nil {
		t.Fatal("NewSynthesizer returned nil")
	}
}

func TestPiperBinary(t *testing.T) {
	got := piperBinary()
	if got == "" {
		t.Error("piperBinary returned empty string")
	}
}

func TestResolveBinary_ExplicitPath(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "piper")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh"), 0755); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	got, err := resolveBinary(binPath)
	if err != nil {
		t.Fatalf("resolveBinary failed: %v", err)
	}
	if got != binPath {
		t.Errorf("got %q, want %q", got, binPath)
	}
}

func TestResolveBinary_NotFound(t *testing.T) {
	_, err := resolveBinary("/nonexistent/path/piper")
	if err == nil {
		t.Error("expected error for nonexistent binary")
	}
}

func TestResolveModel_ExplicitPathWithSidecar(t *testing.T) {
	dir := t.TempDir()
	modelPath := filepath.Join(dir, "en_US-lessac-medium.onnx")
	if err := os.WriteFile(modelPath, []byte("model data"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(modelPath+".json", []byte("{}"), 0644); err != nil {
		t.Fatalf("WriteFile sidecar failed: %v", err)
	}

	got, err := resolveModel(modelPath)
	if err != nil {
		t.Fatalf("resolveModel failed: %v", err)
	}
	if got != modelPath {
		t.Errorf("got %q, want %q", got, modelPath)
	}
}

// TestResolveModel_MissingSidecar is a regression test for a real Piper
// footgun: the .onnx file alone is not a usable voice — Piper reads the
// .onnx.json config sidecar implicitly, and a model directory missing it
// (e.g. only the big .onnx was copied, sidecar forgotten) fails at
// synthesis time with a confusing Piper-side error rather than a clear one
// here. resolveModel should catch this before ever spawning the subprocess.
func TestResolveModel_MissingSidecar(t *testing.T) {
	dir := t.TempDir()
	modelPath := filepath.Join(dir, "en_US-lessac-medium.onnx")
	if err := os.WriteFile(modelPath, []byte("model data"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	_, err := resolveModel(modelPath)
	if err == nil {
		t.Error("expected error when .onnx.json sidecar is missing")
	}
}

func TestResolveModel_NotFound(t *testing.T) {
	_, err := resolveModel("/nonexistent/model.onnx")
	if err == nil {
		t.Error("expected error for nonexistent model")
	}
}

func TestResolveModel_EmptyConfig(t *testing.T) {
	_, err := resolveModel("")
	if err == nil {
		t.Error("expected error when no model is configured — Faz 1 has no voice auto-selection")
	}
}

// TestSynthesize_MissingBinaryFailsFast ensures Synthesize surfaces a clear
// error (via resolveBinary) instead of hanging or panicking when Piper
// isn't installed — the common case in dev/CI environments that don't
// bundle the binary.
func TestSynthesize_MissingBinaryFailsFast(t *testing.T) {
	s := NewSynthesizer("/nonexistent/piper", "/nonexistent/model.onnx")
	_, err := s.Synthesize(context.Background(), "hello")
	if err == nil {
		t.Error("expected error when Piper binary is not found")
	}
}

// TestBinarySearchBasesFrom_IncludesParentOfExeDir is a regression test:
// this function previously copied whisper.resolveBinary's search (only "."
// and the exe's own directory), the same bug internal/llama's
// binarySearchBasesFrom was already fixed for — the installed CLI binary
// lives at ~/.memo/bin/memo, one level deeper than the bundled binaries/
// tree it ships next to (~/.memo/binaries/...), so without searching the
// parent, resolveBinary never finds the bundled Piper binary when running
// as the installed CLI.
func TestBinarySearchBasesFrom_IncludesParentOfExeDir(t *testing.T) {
	exePath := filepath.Join("/home/user/.memo/bin", "memo")

	bases := binarySearchBasesFrom(exePath)

	wantExeDir := filepath.Join("/home/user/.memo/bin")
	wantParent := filepath.Join("/home/user/.memo")
	if !slices.Contains(bases, wantExeDir) {
		t.Errorf("bases = %v, want to contain exe dir %q", bases, wantExeDir)
	}
	if !slices.Contains(bases, wantParent) {
		t.Errorf("bases = %v, want to contain parent dir %q", bases, wantParent)
	}
}

func TestBinarySearchBases(t *testing.T) {
	bases := binarySearchBases()
	if len(bases) == 0 {
		t.Error("expected at least one search base")
	}
	if bases[0] != "." {
		t.Errorf("expected first base to be \".\", got %q", bases[0])
	}
}
