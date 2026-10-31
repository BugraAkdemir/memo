package autonomy

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os/exec"
	"time"
)

// VoiceWAV converts compressed voice to the PCM WAV Whisper expects. The fixed
// decoder cannot open files, playlists or network URLs supplied by the sender.
// No audio is written to disk. A missing decoder fails closed, never to cloud.
func VoiceWAV(ctx context.Context, data []byte) ([]byte, error) {
	if len(data) == 0 || len(data) > 20<<20 {
		return nil, fmt.Errorf("invalid voice size")
	}
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WAVE" {
		return data, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ffmpeg", "-nostdin", "-v", "error", "-threads", "1", "-protocol_whitelist", "pipe", "-format_whitelist", "ogg,wav,mp3,flac,mov,matroska,webm", "-i", "pipe:0", "-map", "0:a:0", "-vn", "-threads", "1", "-ac", "1", "-ar", "16000", "-c:a", "pcm_s16le", "-f", "s16le", "pipe:1")
	cmd.Stdin = bytes.NewReader(data)
	var pcm voiceBuffer
	cmd.Stdout = &pcm
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil || pcm.Len() == 0 {
		return nil, fmt.Errorf("local voice conversion unavailable or unsupported")
	}
	return pcmWAV(pcm.Bytes()), nil
}

type voiceBuffer struct{ bytes.Buffer }

func (b *voiceBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 20<<20 {
		return 0, fmt.Errorf("decoded voice exceeds limit")
	}
	return b.Buffer.Write(p)
}
func pcmWAV(pcm []byte) []byte {
	b := bytes.NewBuffer(make([]byte, 0, len(pcm)+44))
	b.WriteString("RIFF")
	binary.Write(b, binary.LittleEndian, uint32(len(pcm)+36))
	b.WriteString("WAVEfmt ")
	binary.Write(b, binary.LittleEndian, uint32(16))
	binary.Write(b, binary.LittleEndian, uint16(1))
	binary.Write(b, binary.LittleEndian, uint16(1))
	binary.Write(b, binary.LittleEndian, uint32(16000))
	binary.Write(b, binary.LittleEndian, uint32(32000))
	binary.Write(b, binary.LittleEndian, uint16(2))
	binary.Write(b, binary.LittleEndian, uint16(16))
	b.WriteString("data")
	binary.Write(b, binary.LittleEndian, uint32(len(pcm)))
	b.Write(pcm)
	return b.Bytes()
}
