import 'package:flutter/material.dart';

import '../../core/l10n.dart';
import '../../core/theme.dart';
import '../settings/image_model_section.dart';
import '../settings/stt_provider_section.dart';
import '../settings/tts_provider_section.dart';
import '../settings/tts_voice_section.dart';
import 'setup_widgets.dart';

/// Optional step: speech-to-text, text-to-speech (with the voice) and the image
/// model. Each part is the Settings section itself, so nothing has to be
/// configured twice.
class SetupVoiceImageStep extends StatelessWidget {
  final ThemeColors color;

  const SetupVoiceImageStep({super.key, required this.color});

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        SetupNote(color: color, message: L10n.t('setup_voice_image_tip')),
        const SizedBox(height: 12),
        SetupCard(color: color, child: const STTProviderSection()),
        const SizedBox(height: 12),
        SetupCard(color: color, child: const TTSProviderSection()),
        const SizedBox(height: 12),
        SetupCard(color: color, child: const TTSVoiceSection()),
        const SizedBox(height: 12),
        SetupCard(color: color, child: const ImageModelSection()),
      ],
    );
  }
}
