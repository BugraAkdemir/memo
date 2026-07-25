// Package tts wraps the Piper text-to-speech binary. Unlike whisper.cpp's
// whisper-server (internal/whisper), Piper has no persistent HTTP server
// mode in the last standalone binary release (rhasspy/piper v1.2.0, tag
// 2023.11.14-2 — upstream development has since moved to a Python package,
// see docs/plans/PLAN_voice_live_mode_faz1.md's 1.1 notes) — it is a
// one-shot CLI that reads text on stdin and writes a WAV file per call.
// Synthesizer therefore spawns a fresh subprocess per Synthesize call
// rather than managing a long-lived server process/port like whisper.Server
// does.
package tts

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
)

// Synthesizer runs the Piper CLI to turn text into WAV audio bytes.
type Synthesizer struct {
	mu         sync.RWMutex
	binaryPath string
	modelPath  string
}

// NewSynthesizer creates a Synthesizer. binaryPath/modelPath may be empty —
// resolveBinary/resolveModel fall back to the standard binaries/ search
// paths (see whisper.resolveBinary/resolveModel, same convention here).
func NewSynthesizer(binaryPath, modelPath string) *Synthesizer {
	return &Synthesizer{binaryPath: binaryPath, modelPath: modelPath}
}

// Synthesize renders text to WAV-encoded audio bytes by invoking Piper as a
// one-shot subprocess. Safe for concurrent use — each call gets its own
// subprocess and temp file, no shared mutable state beyond the configured
// binary/model paths (read-locked).
func (s *Synthesizer) Synthesize(ctx context.Context, text string) ([]byte, error) {
	s.mu.RLock()
	binaryPath, modelPath := s.binaryPath, s.modelPath
	s.mu.RUnlock()

	bin, err := resolveBinary(binaryPath)
	if err != nil {
		return nil, fmt.Errorf("tts: %w", err)
	}
	model, err := resolveModel(modelPath)
	if err != nil {
		return nil, fmt.Errorf("tts: %w", err)
	}

	outFile, err := os.CreateTemp("", "memo-tts-*.wav")
	if err != nil {
		return nil, fmt.Errorf("tts: temp file: %w", err)
	}
	outPath := outFile.Name()
	outFile.Close()
	defer os.Remove(outPath)

	cmd := exec.CommandContext(ctx, bin, "--model", model, "--output_file", outPath)
	cmd.Stdin = bytes.NewReader([]byte(text))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("tts: piper run: %w: %s", err, stderr.String())
	}

	audio, err := os.ReadFile(outPath)
	if err != nil {
		return nil, fmt.Errorf("tts: read output: %w", err)
	}
	return audio, nil
}

func piperBinary() string {
	if runtime.GOOS == "windows" {
		return "piper.exe"
	}
	return "piper"
}

// resolveBinary mirrors whisper.resolveBinary's search order: an explicit
// configured path first, then binaries/{os}/{cpu,amd,nvidia}/ and bin/
// under each search base, then PATH.
func resolveBinary(configured string) (string, error) {
	if configured != "" {
		if _, err := os.Stat(configured); err == nil {
			return configured, nil
		}
	}

	for _, base := range binarySearchBases() {
		for _, dir := range []string{"cpu", "amd", "nvidia"} {
			p := filepath.Join(base, "binaries", runtime.GOOS, dir, piperBinary())
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
		p := filepath.Join(base, "bin", piperBinary())
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	if path, err := exec.LookPath("piper"); err == nil {
		return path, nil
	}

	return "", fmt.Errorf("piper binary not found — place it in binaries/{os}/cpu/")
}

// resolveModel finds a Piper voice model (.onnx file — its .onnx.json
// sidecar must sit next to it, Piper reads it implicitly by convention).
// Unlike whisper's fixed ggml-*.bin filenames, Piper voice files are named
// per-voice-per-language (e.g. en_US-lessac-medium.onnx, tr_TR-fahrettin-
// medium.onnx) with no single canonical default — Faz 1 has no voice
// auto-selection (that's the parent plan's Faz 2 "TTS Store"), so a
// configured path is required; this only validates it and looks for the
// sidecar.
func resolveModel(configured string) (string, error) {
	if configured == "" {
		return "", fmt.Errorf("no Piper voice model configured — set Voice.ModelPath to a .onnx file")
	}
	if _, err := os.Stat(configured); err != nil {
		return "", fmt.Errorf("configured Piper voice model not found: %s", configured)
	}
	sidecar := configured + ".json"
	if _, err := os.Stat(sidecar); err != nil {
		return "", fmt.Errorf("Piper voice model %s is missing its sidecar config %s", configured, sidecar)
	}
	return configured, nil
}

func binarySearchBases() []string {
	bases := []string{"."}
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		if exeDir != "." {
			bases = append(bases, exeDir)
		}
	}
	return bases
}
