import 'package:flutter/material.dart';
import 'package:google_fonts/google_fonts.dart';

/// Memo — "Pewter Study"
/// A warm, lamp-lit graphite identity shared with the desktop app and website.
/// One bronze accent, spent only on primary action, active state, and progress.
class MemoTheme {
  MemoTheme._();

  // ── Surfaces — warm graphite, depth from elevation not colour ──
  static const bg = Color(0xFF1B1916);
  static const surface = Color(0xFF221F19);
  static const surfaceAlt = Color(0xFF2A261E);
  static const surfaceHi = Color(0xFF332D23);
  static const border = Color(0xFF332E25);
  static const borderHi = Color(0xFF453E31);

  // ── Ink — soft paper white ──
  static const text = Color(0xFFECE7DC);
  static const textDim = Color(0xFFB4AB99);
  static const textFaint = Color(0xFF837A6B);

  // ── Bronze — the single accent ──
  static const accent = Color(0xFFBC8A4E);
  static const accentBright = Color(0xFFD8AC6C);
  static const accentDeep = Color(0xFF8E6736);
  static const onAccent = Color(0xFF211A10);

  // ── Status ──
  static const sage = Color(0xFF9CB380); // "running" / online — used sparingly
  static const error = Color(0xFFCF6679);

  // ── Type roles ──
  static TextStyle display(double size, {FontWeight w = FontWeight.w700, double ls = -0.4, double? height, Color? color}) =>
      GoogleFonts.getFont('Schibsted Grotesk',
          fontSize: size, fontWeight: w, letterSpacing: ls, height: height, color: color ?? text);

  static TextStyle body(double size, {FontWeight w = FontWeight.w400, double? height, Color? color, double? ls}) =>
      GoogleFonts.inter(fontSize: size, fontWeight: w, height: height, color: color ?? text, letterSpacing: ls);

  static TextStyle mono(double size, {FontWeight w = FontWeight.w500, Color? color, double ls = 0.2}) =>
      GoogleFonts.jetBrainsMono(fontSize: size, fontWeight: w, color: color ?? textDim, letterSpacing: ls);

  static ThemeData get dark {
    const scheme = ColorScheme.dark(
      primary: accent,
      onPrimary: onAccent,
      secondary: accentBright,
      onSecondary: onAccent,
      surface: surface,
      onSurface: text,
      error: error,
      onError: bg,
      outline: border,
    );

    return ThemeData(
      useMaterial3: true,
      brightness: Brightness.dark,
      scaffoldBackgroundColor: bg,
      colorScheme: scheme,
      splashFactory: InkSparkle.splashFactory,
      textTheme: TextTheme(
        displayLarge: display(34, w: FontWeight.w800, ls: -0.8),
        headlineLarge: display(27, w: FontWeight.w700, ls: -0.5),
        headlineMedium: display(21, w: FontWeight.w700, ls: -0.3),
        titleLarge: display(18, w: FontWeight.w600, ls: -0.2),
        titleMedium: body(16, w: FontWeight.w600),
        bodyLarge: body(15.5, height: 1.5),
        bodyMedium: body(14, height: 1.5),
        bodySmall: body(13, color: textDim),
        labelLarge: body(14, w: FontWeight.w600),
        labelSmall: mono(11),
      ),
      appBarTheme: AppBarTheme(
        backgroundColor: bg,
        foregroundColor: text,
        elevation: 0,
        scrolledUnderElevation: 0,
        centerTitle: false,
        titleTextStyle: display(18, w: FontWeight.w700, ls: -0.3),
      ),
      inputDecorationTheme: InputDecorationTheme(
        filled: true,
        fillColor: surfaceAlt,
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(14),
          borderSide: const BorderSide(color: border),
        ),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(14),
          borderSide: const BorderSide(color: border),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(14),
          borderSide: const BorderSide(color: accent, width: 1.5),
        ),
        labelStyle: mono(12, color: textDim),
        floatingLabelStyle: mono(12, color: accent),
        hintStyle: body(15, color: textFaint),
        prefixIconColor: textFaint,
        contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 16),
      ),
      dividerTheme: const DividerThemeData(color: border, thickness: 1, space: 1),
      iconTheme: const IconThemeData(color: textDim, size: 22),
      snackBarTheme: SnackBarThemeData(
        backgroundColor: surfaceHi,
        contentTextStyle: body(14, color: text),
        actionTextColor: accentBright,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
        behavior: SnackBarBehavior.floating,
        insetPadding: const EdgeInsets.all(16),
      ),
      dialogTheme: DialogThemeData(
        backgroundColor: surface,
        surfaceTintColor: Colors.transparent,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(20)),
        titleTextStyle: display(18, w: FontWeight.w700),
        contentTextStyle: body(14.5, color: textDim, height: 1.5),
      ),
      bottomSheetTheme: const BottomSheetThemeData(
        backgroundColor: surface,
        surfaceTintColor: Colors.transparent,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.vertical(top: Radius.circular(24)),
        ),
      ),
      switchTheme: SwitchThemeData(
        thumbColor: WidgetStateProperty.resolveWith((s) =>
            s.contains(WidgetState.selected) ? onAccent : textDim),
        trackColor: WidgetStateProperty.resolveWith((s) =>
            s.contains(WidgetState.selected) ? accent : surfaceHi),
        trackOutlineColor: WidgetStateProperty.resolveWith((s) =>
            s.contains(WidgetState.selected) ? Colors.transparent : border),
      ),
    );
  }
}
