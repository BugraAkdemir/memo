import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:google_fonts/google_fonts.dart';
import 'package:memo_flutter/core/theme.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('MemoTheme disables runtime font fetching and resolves fonts from bundled assets', () async {
    final light = MemoTheme.themeData;
    final dark = MemoTheme.darkThemeData;
    expect(GoogleFonts.config.allowRuntimeFetching, isFalse);
    expect(dark.textTheme.bodyMedium?.fontFamily, contains('Inter'));
    expect(light.textTheme.displayLarge?.fontFamily, contains('SchibstedGrotesk'));

    // Also verify commonly styled weights load cleanly offline.
    GoogleFonts.inter(fontWeight: FontWeight.w400);
    GoogleFonts.inter(fontWeight: FontWeight.w500);
    GoogleFonts.inter(fontWeight: FontWeight.w600);
    GoogleFonts.inter(fontWeight: FontWeight.w700);
    GoogleFonts.schibstedGrotesk(fontWeight: FontWeight.w400);
    GoogleFonts.schibstedGrotesk(fontWeight: FontWeight.w500);
    GoogleFonts.schibstedGrotesk(fontWeight: FontWeight.w600);
    GoogleFonts.schibstedGrotesk(fontWeight: FontWeight.w700);

    await GoogleFonts.pendingFonts();
  });
}
