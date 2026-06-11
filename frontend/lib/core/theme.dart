import 'package:flutter/material.dart';
import 'package:google_fonts/google_fonts.dart';

class ThemeColors {
  final Color bgApp;
  final Color bgPanel;
  final Color bgElement;
  final Color bgHover;
  final Color textMain;
  final Color textSecondary;
  final Color textMuted;
  final Color textDim;
  final Color textInverse;
  final Color borderSoft;
  final Color borderHover;

  const ThemeColors({
    required this.bgApp,
    required this.bgPanel,
    required this.bgElement,
    required this.bgHover,
    required this.textMain,
    required this.textSecondary,
    required this.textMuted,
    required this.textDim,
    required this.textInverse,
    required this.borderSoft,
    required this.borderHover,
  });
}

/// Memo Cream + Gold theme — matches the existing Svelte frontend design.
class MemoTheme {
  MemoTheme._();

  // ─── Light Palette (private) ───────────────────────────────────

  static const Color _lightBgApp = Color(0xFFFDFCF0);
  static const Color _lightBgPanel = Color(0xFFF5F3E0);
  static const Color _lightBgElement = Color(0xFFEAE8D5);
  static const Color _lightBgHover = Color(0xFFDDD9C4);

  static const Color _lightTextMain = Color(0xFF1A1A1A);
  static const Color _lightTextMuted = Color(0xFF4A4A4A);
  static const Color _lightTextDim = Color(0xFF8A8A7A);
  static const Color _lightTextInverse = Color(0xFFFDFCF0);

  static const Color _lightBorderSoft = Color(0x141A1A1A);
  static const Color _lightBorderHover = Color(0x2E1A1A1A);
  static const Color _lightTextSecondary = Color(0xFF2C2C2C);

  // ─── Dark Palette (private) ───────────────────────────────────

  static const Color _darkBgApp = Color(0xFF1C1C1E);
  static const Color _darkBgPanel = Color(0xFF252528);
  static const Color _darkBgElement = Color(0xFF2C2C30);
  static const Color _darkBgHover = Color(0xFF36363A);

  static const Color _darkTextMain = Color(0xFFF0EDE0);
  static const Color _darkTextMuted = Color(0xFFA09D90);
  static const Color _darkTextDim = Color(0xFF707060);
  static const Color _darkTextInverse = Color(0xFF1C1C1E);

  static const Color _darkBorderSoft = Color(0x30FFFFFF);
  static const Color _darkBorderHover = Color(0x50FFFFFF);
  static const Color _darkTextSecondary = Color(0xFFD0CDC0);

  // ─── Theme Color Instances ────────────────────────────────────

  static const ThemeColors _light = ThemeColors(
    bgApp: _lightBgApp,
    bgPanel: _lightBgPanel,
    bgElement: _lightBgElement,
    bgHover: _lightBgHover,
    textMain: _lightTextMain,
    textSecondary: _lightTextSecondary,
    textMuted: _lightTextMuted,
    textDim: _lightTextDim,
    textInverse: _lightTextInverse,
    borderSoft: _lightBorderSoft,
    borderHover: _lightBorderHover,
  );

  static const ThemeColors _dark = ThemeColors(
    bgApp: _darkBgApp,
    bgPanel: _darkBgPanel,
    bgElement: _darkBgElement,
    bgHover: _darkBgHover,
    textMain: _darkTextMain,
    textSecondary: _darkTextSecondary,
    textMuted: _darkTextMuted,
    textDim: _darkTextDim,
    textInverse: _darkTextInverse,
    borderSoft: _darkBorderSoft,
    borderHover: _darkBorderHover,
  );

  /// Returns the correct color set for the current theme brightness.
  static ThemeColors of(BuildContext context) {
    return Theme.of(context).brightness == Brightness.dark ? _dark : _light;
  }

  /// Dark theme colors (for preview purposes outside build context).
  static const ThemeColors dark = _dark;

  /// Light theme colors (for preview purposes outside build context).
  static const ThemeColors light = _light;

  // ─── Theme-independent Constants ──────────────────────────────

  // Gold Accent
  static const Color accent = Color(0xFFC9A84C);
  static const Color accentLight = Color(0xFFE8C97A);
  static const Color accentPale = Color(0xFFF5E8C0);
  static const Color accentMuted = Color(0x1FC9A84C);
  static const Color accentHover = Color(0xFFB8943E);

  // Functional
  static const Color green = Color(0xFF51B576);
  static const Color red = Color(0xFFD35F5F);
  static const Color successGreen = Color(0xFF51B576);
  static const Color warningOrange = Color(0xFFD4944F);
  static const Color warmBrown = Color(0xFF8B6535);

  // Radius
  static const double radiusSm = 8;
  static const double radiusMd = 14;
  static const double radiusLg = 20;

  // ─── Shadows (light-biased; okay for both) ───────────────────
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
      bodyColor: _lightTextMain,
      displayColor: _lightTextMain,
    );

    return ThemeData(
      useMaterial3: true,
      brightness: Brightness.light,
      scaffoldBackgroundColor: _lightBgApp,
      colorScheme: ColorScheme.light(
        surface: _lightBgApp,
        primary: accent,
        onPrimary: _lightTextInverse,
        secondary: accentLight,
        onSecondary: _lightTextMain,
        tertiary: warmBrown,
        error: red,
        outline: _lightBorderSoft,
        surfaceContainerHighest: _lightBgPanel,
      ),
      textTheme: textTheme,
      appBarTheme: AppBarTheme(
        backgroundColor: _lightBgApp,
        foregroundColor: _lightTextMain,
        elevation: 0,
        scrolledUnderElevation: 0,
        titleTextStyle: textTheme.titleLarge?.copyWith(
          fontWeight: FontWeight.w600,
          color: _lightTextMain,
        ),
      ),
      cardTheme: CardThemeData(
        color: _lightBgPanel,
        elevation: 0,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(radiusMd),
          side: BorderSide(color: _lightBorderSoft),
        ),
      ),
      inputDecorationTheme: InputDecorationTheme(
        filled: true,
        fillColor: _lightBgApp,
        contentPadding:
            const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(radiusMd),
          borderSide: BorderSide(color: _lightBorderSoft),
        ),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(radiusMd),
          borderSide: BorderSide(color: _lightBorderSoft),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(radiusMd),
          borderSide: const BorderSide(color: accent, width: 1.5),
        ),
        hintStyle: textTheme.bodyMedium?.copyWith(color: _lightTextDim),
      ),
      elevatedButtonTheme: ElevatedButtonThemeData(
        style: ElevatedButton.styleFrom(
          backgroundColor: accent,
          foregroundColor: _lightTextInverse,
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
          foregroundColor: _lightTextMain,
          side: BorderSide(color: _lightBorderSoft),
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
          foregroundColor: _lightTextMuted,
        ),
      ),
      dividerTheme: const DividerThemeData(
        color: _lightBorderSoft,
        thickness: 1,
        space: 0,
      ),
      dialogTheme: DialogThemeData(
        backgroundColor: _lightBgApp,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(radiusLg),
        ),
        titleTextStyle: textTheme.titleLarge?.copyWith(
          fontWeight: FontWeight.w600,
          color: _lightTextMain,
        ),
      ),
      tooltipTheme: TooltipThemeData(
        decoration: BoxDecoration(
          color: _lightTextMain,
          borderRadius: BorderRadius.circular(radiusSm),
        ),
        textStyle: textTheme.bodySmall?.copyWith(color: _lightTextInverse),
      ),
      scrollbarTheme: ScrollbarThemeData(
        thumbColor: WidgetStateProperty.all(_lightBgHover),
        thickness: WidgetStateProperty.all(4),
        radius: const Radius.circular(999),
      ),
      snackBarTheme: SnackBarThemeData(
        backgroundColor: _lightTextMain,
        contentTextStyle: textTheme.bodyMedium?.copyWith(color: _lightTextInverse),
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(radiusSm),
        ),
        behavior: SnackBarBehavior.floating,
      ),
    );
  }

  // ─── Dark ThemeData ─────────────────────────────────────────────

  static ThemeData get darkThemeData {
    final textTheme = GoogleFonts.interTextTheme().apply(
      bodyColor: _darkTextMain,
      displayColor: _darkTextMain,
    );

    return ThemeData(
      useMaterial3: true,
      brightness: Brightness.dark,
      scaffoldBackgroundColor: _darkBgApp,
      colorScheme: ColorScheme.dark(
        surface: _darkBgApp,
        primary: accent,
        onPrimary: _darkTextInverse,
        secondary: accentLight,
        onSecondary: _darkTextMain,
        tertiary: warmBrown,
        error: red,
        outline: _darkBorderSoft,
        surfaceContainerHighest: _darkBgPanel,
      ),
      textTheme: textTheme,
      appBarTheme: AppBarTheme(
        backgroundColor: _darkBgApp,
        foregroundColor: _darkTextMain,
        elevation: 0,
        scrolledUnderElevation: 0,
        titleTextStyle: textTheme.titleLarge?.copyWith(
          fontWeight: FontWeight.w600,
          color: _darkTextMain,
        ),
      ),
      cardTheme: CardThemeData(
        color: _darkBgPanel,
        elevation: 0,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(radiusMd),
          side: BorderSide(color: _darkBorderSoft),
        ),
      ),
      inputDecorationTheme: InputDecorationTheme(
        filled: true,
        fillColor: _darkBgApp,
        contentPadding:
            const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(radiusMd),
          borderSide: BorderSide(color: _darkBorderSoft),
        ),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(radiusMd),
          borderSide: BorderSide(color: _darkBorderSoft),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(radiusMd),
          borderSide: const BorderSide(color: accent, width: 1.5),
        ),
        hintStyle: textTheme.bodyMedium?.copyWith(color: _darkTextDim),
      ),
      elevatedButtonTheme: ElevatedButtonThemeData(
        style: ElevatedButton.styleFrom(
          backgroundColor: accent,
          foregroundColor: _darkTextInverse,
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
          foregroundColor: _darkTextMain,
          side: BorderSide(color: _darkBorderSoft),
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
          foregroundColor: _darkTextMuted,
        ),
      ),
      dividerTheme: DividerThemeData(
        color: _darkBorderSoft,
        thickness: 1,
        space: 0,
      ),
      dialogTheme: DialogThemeData(
        backgroundColor: _darkBgPanel,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(radiusLg),
        ),
        titleTextStyle: textTheme.titleLarge?.copyWith(
          fontWeight: FontWeight.w600,
          color: _darkTextMain,
        ),
      ),
      tooltipTheme: TooltipThemeData(
        decoration: BoxDecoration(
          color: _darkTextMain,
          borderRadius: BorderRadius.circular(radiusSm),
        ),
        textStyle: textTheme.bodySmall?.copyWith(color: _darkTextInverse),
      ),
      scrollbarTheme: ScrollbarThemeData(
        thumbColor: WidgetStateProperty.all(_darkBgHover),
        thickness: WidgetStateProperty.all(4),
        radius: const Radius.circular(999),
      ),
      snackBarTheme: SnackBarThemeData(
        backgroundColor: _darkTextMain,
        contentTextStyle: textTheme.bodyMedium?.copyWith(color: _darkTextInverse),
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(radiusSm),
        ),
        behavior: SnackBarBehavior.floating,
      ),
    );
  }
}
