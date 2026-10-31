package autonomy

import (
	"bytes"
	"context"
	"os/exec"
	"testing"
)

func TestVoiceOpusIsConvertedLocallyWithoutFiles(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("optional local decoder not installed")
	}
	wav := pcmWAV(make([]byte, 3200))
	cmd := exec.Command("ffmpeg", "-nostdin", "-v", "error", "-f", "wav", "-i", "pipe:0", "-c:a", "libopus", "-f", "ogg", "pipe:1")
	cmd.Stdin = bytes.NewReader(wav)
	ogg, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := VoiceWAV(context.Background(), ogg)
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded) < 44 || string(decoded[8:12]) != "WAVE" {
		t.Fatal("invalid PCM WAV")
	}
	if _, err := VoiceWAV(context.Background(), []byte("#EXTM3U\nhttps://example.com/private")); err == nil {
		t.Fatal("playlist accepted")
	}
	if _, err := VoiceWAV(context.Background(), nil); err == nil {
		t.Fatal("empty accepted")
	}
}
