import 'dart:io';

import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:path/path.dart' as p;
import 'package:record/record.dart';

import '../core/l10n.dart';
import 'chat_provider.dart';

enum RecordingState { idle, recording, transcribing }

final recordingProvider =
    StateNotifierProvider<RecordingNotifier, RecordingState>(
      RecordingNotifier.new,
    );

class RecordingNotifier extends StateNotifier<RecordingState> {
  final Ref _ref;
  final AudioRecorder _recorder = AudioRecorder();
  String? _recordingPath;

  RecordingNotifier(this._ref) : super(RecordingState.idle);

  Future<bool> canRecord() async {
    return await _recorder.hasPermission();
  }

  Future<String?> start() async {
    if (state != RecordingState.idle) return null;
    if (_ref.read(isSendingProvider)) return null;

    final hasPermission = await _recorder.hasPermission();
    if (!hasPermission) {
      debugPrint('recording: microphone permission denied');
      _ref.read(errorMessageProvider.notifier).state =
          L10n.t('mic_no_permission');
      return null;
    }

    try {
      _recordingPath = p.join(Directory.systemTemp.path, 'memo-stt-recording.wav');
      await _recorder.start(
        const RecordConfig(
          encoder: AudioEncoder.wav,
          sampleRate: 16000,
          numChannels: 1,
        ),
        path: _recordingPath!,
      );
      state = RecordingState.recording;
      return null;
    } catch (e) {
      debugPrint('recording: start error: $e');
      _ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: Kayıt başlatılamadı ($e)';
      state = RecordingState.idle;
      return null;
    }
  }

  Future<String?> stop() async {
    if (state != RecordingState.recording) return null;

    state = RecordingState.transcribing;

    try {
      final path = await _recorder.stop();
      if (path == null || _recordingPath == null) {
        state = RecordingState.idle;
        return null;
      }

      final file = File(_recordingPath!);
      if (!await file.exists()) {
        state = RecordingState.idle;
        return null;
      }

      final audioData = await file.readAsBytes();
      await file.delete();

      final api = _ref.read(apiClientProvider);
      final text = await api.transcribeAudio(audioData);

      state = RecordingState.idle;
      return text;
    } catch (e) {
      debugPrint('recording: transcribe error: $e');
      _ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: Ses metne dönüştürülemedi ($e)';
      state = RecordingState.idle;
      return null;
    }
  }

  @override
  void dispose() {
    _recorder.dispose();
    super.dispose();
  }
}
