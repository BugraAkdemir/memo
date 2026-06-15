package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

var (
	recCmd   *exec.Cmd
	recStdin io.WriteCloser
	recFile  string
	recMu    sync.Mutex
)

// startSTTServer extracts and launches the embedded STT binary.
func (a *App) startSTTServer() {
	var binName string
	if runtime.GOOS == "windows" {
		binName = "stt_server_windows.exe"
	} else if runtime.GOOS == "linux" {
		binName = "stt_server_linux"
	} else {
		log.Printf("STT disabled: OS %s not specifically supported for bundled binary yet.", runtime.GOOS)
		return
	}

	binData, err := a.binaries.ReadFile("binaries/" + binName)
	if err != nil {
		log.Printf("STT: embedded binary %s not found in build. STT disabled.", binName)
		return
	}

	tempPath := filepath.Join(os.TempDir(), "memo_stt_server")
	if runtime.GOOS == "windows" {
		tempPath += ".exe"
	}

	// Always overwrite the temp file to ensure it is the latest bundled version
	err = os.WriteFile(tempPath, binData, 0700)
	if err != nil {
		log.Printf("STT server unpacking failed: %v", err)
		return
	}

	a.sttServer = exec.Command(tempPath, "tr", "9876")
	a.sttServer.Stdout = os.Stdout
	a.sttServer.Stderr = os.Stderr
	sttSetProcessGroup(a.sttServer)

	if err := a.sttServer.Start(); err != nil {
		log.Printf("STT server start failed: %v", err)
		a.sttServer = nil
		return
	}
	log.Println("STT server starting on :9876")
}

// stopRecordingProcess kills an in-flight microphone recording (arecord/sox/ffmpeg)
// so it doesn't outlive the app and keep writing to the temp WAV forever.
func stopRecordingProcess() {
	recMu.Lock()
	defer recMu.Unlock()
	if recCmd == nil {
		return
	}
	if recStdin != nil {
		recStdin.Close()
		recStdin = nil
	}
	if recCmd.Process != nil {
		recCmd.Process.Kill()
	}
	recCmd.Wait()
	recCmd = nil
	if recFile != "" {
		os.Remove(recFile)
		recFile = ""
	}
}

// getDefaultDshowDevice enumerates ffmpeg DirectShow audio devices and returns
// the first one, or "" if none is found.
func getDefaultDshowDevice() string {
	out, _ := exec.Command("ffmpeg", "-hide_banner", "-list_devices", "true", "-f", "dshow", "-i", "dummy").CombinedOutput()
	inAudioSection := false
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, "[dshow") {
			continue
		}
		if strings.Contains(line, "DirectShow audio devices") {
			inAudioSection = true
			continue
		}
		if strings.Contains(line, "DirectShow video devices") {
			inAudioSection = false
			continue
		}
		if strings.Contains(line, "Alternative name") {
			continue
		}
		isAudio := strings.Contains(line, "(audio)") || (inAudioSection && !strings.Contains(line, "(video)"))
		if !isAudio {
			continue
		}
		a := strings.Index(line, "\"")
		b := strings.LastIndex(line, "\"")
		if a != -1 && b > a+1 {
			return line[a+1 : b]
		}
	}
	return ""
}

// StartRecording begins capturing audio from the default microphone.
func (a *App) StartRecording() error {
	recMu.Lock()
	defer recMu.Unlock()

	if recCmd != nil {
		return fmt.Errorf("already recording")
	}

	tmpFile, err := os.CreateTemp("", "memo-stt-*.wav")
	if err != nil {
		return fmt.Errorf("temp file: %w", err)
	}
	tmpFile.Close()
	recFile = tmpFile.Name()

	var recordArgs []string
	var recorder string
	switch runtime.GOOS {
	case "windows":
		recorder = "ffmpeg"
		dev := getDefaultDshowDevice()
		if dev == "" {
			os.Remove(recFile)
			return fmt.Errorf("no DirectShow audio device found — is a microphone connected and ffmpeg installed?")
		}
		recordArgs = []string{"-y", "-f", "dshow", "-i", "audio=" + dev, "-ar", "16000", "-ac", "1", "-acodec", "pcm_s16le", recFile}
	case "darwin":
		recorder = "sox"
		recordArgs = []string{"-d", "-b", "16", "-r", "16000", "-c", "1", recFile}
	default:
		recorder = "arecord"
		recordArgs = []string{"-f", "S16_LE", "-r", "16000", "-c", "1", "-t", "wav", recFile}
	}
	recCmd = exec.Command(recorder, recordArgs...)
	if runtime.GOOS == "windows" {
		recStdin, _ = recCmd.StdinPipe()
	}
	if err := recCmd.Start(); err != nil {
		recCmd = nil
		recStdin = nil
		os.Remove(recFile)
		return fmt.Errorf("recording start (%s): %w", recorder, err)
	}

	log.Println("Recording started")
	return nil
}

// StopRecordingAndTranscribe stops the active recording and sends it to the STT server.
func (a *App) StopRecordingAndTranscribe() (string, error) {
	recMu.Lock()
	defer recMu.Unlock()

	if recCmd == nil {
		return "", fmt.Errorf("not recording")
	}

	// Stop recording gracefully
	if recCmd.Process != nil {
		if runtime.GOOS == "windows" {
			if recStdin != nil {
				io.WriteString(recStdin, "q")
				recStdin.Close()
				recStdin = nil
			}
			done := make(chan struct{})
			go func() { recCmd.Wait(); close(done) }()
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				recCmd.Process.Kill()
				<-done
			}
		} else {
			recCmd.Process.Signal(os.Interrupt)
			recCmd.Wait()
		}
	} else {
		recCmd.Wait()
	}
	recCmd = nil

	defer os.Remove(recFile)

	// Send WAV to the local STT server
	audioData, err := os.ReadFile(recFile)
	if err != nil {
		return "", fmt.Errorf("read recording: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://127.0.0.1:9876/transcribe", bytes.NewReader(audioData))
	if err != nil {
		return "", fmt.Errorf("stt request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("stt server unreachable (model may still be loading): %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("stt decode: %w", err)
	}

	log.Printf("STT result: %q", result.Text)
	return result.Text, nil
}

// TranscribeAudio sends raw audio bytes to the local STT server.
func (a *App) TranscribeAudio(audioData []byte) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://127.0.0.1:9876/transcribe", bytes.NewReader(audioData))
	if err != nil {
		return "", fmt.Errorf("stt request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("stt server unreachable: %w", err)
	}
	defer resp.Body.Close()
	var result struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("stt decode: %w", err)
	}
	return result.Text, nil
}
