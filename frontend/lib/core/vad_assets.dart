import 'package:flutter/foundation.dart';

/// The bundled Silero VAD v4 model's base path for the current platform.
///
/// Native implementations load this as a Flutter asset key. Flutter Web
/// serves the same declared asset below its generated `/assets/` directory.
String get vadModelBaseAssetPath =>
    kIsWeb ? 'assets/assets/vad/' : 'assets/vad/';
