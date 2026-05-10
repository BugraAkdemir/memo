import 'package:flutter/material.dart';
import 'package:google_fonts/google_fonts.dart';

/// Memo Cream + Gold theme — matches the existing Svelte frontend design.
class MemoTheme {
  MemoTheme._();

  // ─── Cream + Gold Palette ──────────────────────────────────────

  static const Color bgApp = Color(0xFFFDFCF0);
  static const Color bgPanel = Color(0xFFF5F3E0);
  static const Color bgElement = Color(0xFFEAE8D5);
  static const Color bgHover = Color(0xFFDDD9C4);

  static const Color textMain = Color(0xFF1A1A1A);
  static const Color textSecondary = Color(0xFF2C2C2C);
  static const Color textMuted = Color(0xFF4A4A4A);
  static const Color textDim = Color(0xFF8A8A7A);
  static const Color textInverse = Color(0xFFFDFCF0);

  static const Color borderSoft = Color(0x141A1A1A); // 8%
  static const Color borderSubtle = Color(0x0F1A1A1A); // 6%
  static const Color borderHover = Color(0x2E1A1A1A); // 18%

  // ─── Gold Accent ───────────────────────────────────────────────
  static const Color accent = Color(0xFFC9A84C);
  static const Color accentLight = Color(0xFFE8C97A);
  static const Color accentPale = Color(0xFFF5E8C0);
  static const Color accentMuted = Color(0x1FC9A84C); // 12%
  static const Color accentHover = Color(0xFFB8943E);

  // ─── Functional ────────────────────────────────────────────────
  static const Color green = Color(0xFF51B576);
  static const Color red = Color(0xFFD35F5F);
  static const Color warmBrown = Color(0xFF8B6535);

  // ─── Radius ────────────────────────────────────────────────────
  static const double radiusSm = 8;
  static const double radiusMd = 14;
  static const double radiusLg = 20;

  // ─── Shadows ───────────────────────────────────────────────────
  static List<BoxShadow> get shadowSm => [
        BoxShadow(
          color: const Color(0x0A1A1A1A),
          blurRadius: 8,
          offset: const Offset(0, 2),
        ),
      ];

  static List<BoxShadow> get shadowMd => [
        BoxShadow(
          color: const Color(0x121A1A1A),
          blurRadius: 16,
          offset: const Offset(0, 4),
        ),
      ];

  static List<BoxShadow> get shadowLg => [
        BoxShadow(
          color: const Color(0x1A1A1A1A),
          blurRadius: 32,
          offset: const Offset(0, 8),
        ),
      ];

  // ─── ThemeData ─────────────────────────────────────────────────

  static ThemeData get themeData {
    final textTheme = GoogleFonts.interTextTheme().apply(
      bodyColor: textMain,
      displayColor: textMain,
    );

    return ThemeData(
      useMaterial3: true,
      brightness: Brightness.light,
      scaffoldBackgroundColor: bgApp,
      colorScheme: ColorScheme.light(
        surface: bgApp,
        primary: accent,
        onPrimary: textInverse,
        secondary: accentLight,
        onSecondary: textMain,
        tertiary: warmBrown,
        error: red,
        outline: borderSoft,
        surfaceContainerHighest: bgPanel,
      ),
      textTheme: textTheme,
      appBarTheme: AppBarTheme(
        backgroundColor: bgApp,
        foregroundColor: textMain,
        elevation: 0,
        scrolledUnderElevation: 0,
        titleTextStyle: textTheme.titleLarge?.copyWith(
          fontWeight: FontWeight.w600,
          color: textMain,
        ),
      ),
      cardTheme: CardThemeData(
        color: bgPanel,
        elevation: 0,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(radiusMd),
          side: BorderSide(color: borderSoft),
        ),
      ),
      inputDecorationTheme: InputDecorationTheme(
        filled: true,
        fillColor: bgApp,
        contentPadding:
            const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(radiusMd),
          borderSide: BorderSide(color: borderSoft),
        ),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(radiusMd),
          borderSide: BorderSide(color: borderSoft),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(radiusMd),
          borderSide: const BorderSide(color: accent, width: 1.5),
        ),
        hintStyle: textTheme.bodyMedium?.copyWith(color: textDim),
      ),
      elevatedButtonTheme: ElevatedButtonThemeData(
        style: ElevatedButton.styleFrom(
          backgroundColor: accent,
          foregroundColor: textInverse,
          elevation: 0,
          padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 14),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(radiusSm),
          ),
          textStyle:
              textTheme.labelLarge?.copyWith(fontWeight: FontWeight.w600),
        ),
      ),
      outlinedButtonTheme: OutlinedButtonThemeData(
        style: OutlinedButton.styleFrom(
          foregroundColor: textMain,
          side: BorderSide(color: borderSoft),
          padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 12),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(radiusSm),
          ),
        ),
      ),
      textButtonTheme: TextButtonThemeData(
        style: TextButton.styleFrom(
          foregroundColor: accent,
          textStyle:
              textTheme.labelLarge?.copyWith(fontWeight: FontWeight.w500),
        ),
      ),
      iconButtonTheme: IconButtonThemeData(
        style: IconButton.styleFrom(
          foregroundColor: textMuted,
        ),
      ),
      dividerTheme: const DividerThemeData(
        color: borderSoft,
        thickness: 1,
        space: 0,
      ),
      dialogTheme: DialogThemeData(
        backgroundColor: bgApp,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(radiusLg),
        ),
        titleTextStyle: textTheme.titleLarge?.copyWith(
          fontWeight: FontWeight.w600,
          color: textMain,
        ),
      ),
      tooltipTheme: TooltipThemeData(
        decoration: BoxDecoration(
          color: textMain,
          borderRadius: BorderRadius.circular(radiusSm),
        ),
        textStyle: textTheme.bodySmall?.copyWith(color: textInverse),
      ),
      scrollbarTheme: ScrollbarThemeData(
        thumbColor: WidgetStateProperty.all(bgHover),
        thickness: WidgetStateProperty.all(4),
        radius: const Radius.circular(999),
      ),
      snackBarTheme: SnackBarThemeData(
        backgroundColor: textMain,
        contentTextStyle: textTheme.bodyMedium?.copyWith(color: textInverse),
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(radiusSm),
        ),
        behavior: SnackBarBehavior.floating,
      ),
    );
  }
}
