import 'dart:typed_data';

/// Encodes 16kHz mono float PCM samples (as produced by the `vad` package's
/// `onSpeechEnd` event — see docs/plans/PLAN_voice_live_mode_faz1.md's 1.5/1.6)
/// into a 16-bit PCM WAV file, ready to POST to the existing
/// `/api/transcribe` endpoint the same way a recorded .wav file already is
/// (see recording_provider.dart / api_client.dart's transcribeAudio).
///
/// Samples are expected in the standard [-1.0, 1.0] float range and are
/// clamped before scaling to int16 — a handful of VAD implementations can
/// hand back a sample marginally outside that range due to floating-point
/// rounding, and an unclamped multiply would wrap into loud noise instead
/// of a barely-clipped sample.
Uint8List encodePcm16Wav(List<double> samples, {int sampleRate = 16000}) {
  const bitsPerSample = 16;
  const numChannels = 1;
  final byteRate = sampleRate * numChannels * bitsPerSample ~/ 8;
  final blockAlign = numChannels * bitsPerSample ~/ 8;
  final dataSize = samples.length * 2;

  final bytes = Uint8List(44 + dataSize);
  final data = ByteData.view(bytes.buffer);

  void writeString(int offset, String s) {
    for (var i = 0; i < s.length; i++) {
      bytes[offset + i] = s.codeUnitAt(i);
    }
  }

  writeString(0, 'RIFF');
  data.setUint32(4, 36 + dataSize, Endian.little);
  writeString(8, 'WAVE');
  writeString(12, 'fmt ');
  data.setUint32(16, 16, Endian.little); // Subchunk1Size (PCM)
  data.setUint16(20, 1, Endian.little); // AudioFormat (1 = PCM)
  data.setUint16(22, numChannels, Endian.little);
  data.setUint32(24, sampleRate, Endian.little);
  data.setUint32(28, byteRate, Endian.little);
  data.setUint16(32, blockAlign, Endian.little);
  data.setUint16(34, bitsPerSample, Endian.little);
  writeString(36, 'data');
  data.setUint32(40, dataSize, Endian.little);

  var offset = 44;
  for (final sample in samples) {
    final clamped = sample.clamp(-1.0, 1.0);
    final intSample = (clamped * 32767).round();
    data.setInt16(offset, intSample, Endian.little);
    offset += 2;
  }

  return bytes;
}
