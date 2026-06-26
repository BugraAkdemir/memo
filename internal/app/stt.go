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
	"runtime"
	"strings"
	"sync"
	"time"

	"memo/internal/whisper"
)

var (
	recCmd   *exec.Cmd
	recStdin io.WriteCloser
	recFile  string
	recMu    sync.Mutex
)

// startSTTServer starts the whisper-server binary for speech-to-text.
func (a *App) startSTTServer() {
	cfg := a.cfg.Whisper
	if !cfg.Enabled {
		log.Println("STT: disabled by config")
		return
	}
	port := cfg.Port
	if port <= 0 {
		port = 9877
	}

	ws := whisper.NewServer(port)
	lang := cfg.Language
	if lang == "" {
		lang = "auto"
	}

	if err := ws.Start(cfg.BinaryPath, cfg.ModelPath, lang, port); err != nil {
		log.Printf("STT: whisper server start failed: %v", err)
		return
	}

	if err := ws.WaitReady(30 * time.Second); err != nil {
		log.Printf("STT: whisper server not ready: %v", err)
		ws.Stop()
		return
	}

	a.whisperServer = ws
	log.Printf("STT: whisper server ready on :%d", port)
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

	// Clear recFile before the defer so stopRecordingProcess (called on
	// shutdown) cannot accidentally delete the next session's recording.
	wavPath := recFile
	recFile = ""
	defer os.Remove(wavPath)

	// Send WAV to the local STT server
	audioData, err := os.ReadFile(wavPath)
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

// TranscribeAudio sends raw audio bytes to the whisper server for transcription.
func (a *App) TranscribeAudio(audioData []byte) (string, error) {
	if a.whisperServer == nil {
		return "", fmt.Errorf("STT: whisper server not started")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return a.whisperServer.Transcribe(ctx, audioData)
}
