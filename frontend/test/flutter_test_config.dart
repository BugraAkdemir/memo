import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:google_fonts/google_fonts.dart';
import 'package:memo_flutter/core/theme.dart';

/// Runs before every test file in `frontend/test/`.
///
/// Pre-warms bundled GoogleFonts (`Inter` and `SchibstedGrotesk`) and awaits
/// [GoogleFonts.pendingFonts] before any [testWidgets] body starts, preventing
/// asynchronous `FontLoader.load()` completion from firing
/// `_SystemFontsNotifier.notifyListeners` -> `scheduleFrameCallback` mid-teardown
/// ("An animation is still running even after the widget tree was disposed").
Future<void> testExecutable(FutureOr<void> Function() testMain) async {
  TestWidgetsFlutterBinding.ensureInitialized();
  MemoTheme.themeData;
  MemoTheme.darkThemeData;
  GoogleFonts.inter(fontWeight: FontWeight.w400);
  GoogleFonts.inter(fontWeight: FontWeight.w500);
  GoogleFonts.inter(fontWeight: FontWeight.w600);
  GoogleFonts.inter(fontWeight: FontWeight.w700);
  GoogleFonts.schibstedGrotesk(fontWeight: FontWeight.w400);
  GoogleFonts.schibstedGrotesk(fontWeight: FontWeight.w500);
  GoogleFonts.schibstedGrotesk(fontWeight: FontWeight.w600);
  GoogleFonts.schibstedGrotesk(fontWeight: FontWeight.w700);
  await GoogleFonts.pendingFonts();
  await testMain();
}
