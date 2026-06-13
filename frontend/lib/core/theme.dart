import 'package:flutter/material.dart';
import 'package:google_fonts/google_fonts.dart';

/// Token bundle for one theme. Carried on [ThemeData] as a [ThemeExtension] so
/// `MemoTheme.of(context)` resolves correctly even though both Memo themes use
/// `Brightness.dark` (the mid-tone "Pewter" and the deeper "Night" share the
/// same brightness, so we can't tell them apart by brightness alone).
class ThemeColors extends ThemeExtension<ThemeColors> {
  final Color bgApp; // surface-0 (deepest)
  final Color bgPanel; // surface-1
  final Color bgElement; // surface-2
  final Color bgHover; // surface-3
  final Color textMain; // ink
  final Color textSecondary;
  final Color textMuted; // ink-muted
  final Color textDim; // ink-dim
  final Color textInverse; // text on accent
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

  @override
  ThemeColors copyWith({
    Color? bgApp,
    Color? bgPanel,
    Color? bgElement,
    Color? bgHover,
    Color? textMain,
    Color? textSecondary,
    Color? textMuted,
    Color? textDim,
    Color? textInverse,
    Color? borderSoft,
    Color? borderHover,
  }) {
    return ThemeColors(
      bgApp: bgApp ?? this.bgApp,
      bgPanel: bgPanel ?? this.bgPanel,
      bgElement: bgElement ?? this.bgElement,
      bgHover: bgHover ?? this.bgHover,
      textMain: textMain ?? this.textMain,
      textSecondary: textSecondary ?? this.textSecondary,
      textMuted: textMuted ?? this.textMuted,
      textDim: textDim ?? this.textDim,
      textInverse: textInverse ?? this.textInverse,
      borderSoft: borderSoft ?? this.borderSoft,
      borderHover: borderHover ?? this.borderHover,
    );
  }

  @override
  ThemeColors lerp(ThemeColors? other, double t) {
    if (other == null) return this;
    return ThemeColors(
      bgApp: Color.lerp(bgApp, other.bgApp, t)!,
      bgPanel: Color.lerp(bgPanel, other.bgPanel, t)!,
      bgElement: Color.lerp(bgElement, other.bgElement, t)!,
      bgHover: Color.lerp(bgHover, other.bgHover, t)!,
      textMain: Color.lerp(textMain, other.textMain, t)!,
      textSecondary: Color.lerp(textSecondary, other.textSecondary, t)!,
      textMuted: Color.lerp(textMuted, other.textMuted, t)!,
      textDim: Color.lerp(textDim, other.textDim, t)!,
      textInverse: Color.lerp(textInverse, other.textInverse, t)!,
      borderSoft: Color.lerp(borderSoft, other.borderSoft, t)!,
      borderHover: Color.lerp(borderHover, other.borderHover, t)!,
    );
  }
}

/// Memo "Pewter Study" theme — a warm graphite mid-tone identity (neither the
/// glare of a light theme nor the cave of a true dark one) with a single muted
/// bronze accent. A calm, premium workspace for a "second brain".
class MemoTheme {
  MemoTheme._();

  // ─── Pewter (default, mid-tone) ───────────────────────────────
  static const Color _pewterBgApp = Color(0xFF2B2E33);
  static const Color _pewterBgPanel = Color(0xFF33373D);
  static const Color _pewterBgElement = Color(0xFF3C4147);
  static const Color _pewterBgHover = Color(0xFF474D54);

  static const Color _inkMain = Color(0xFFECE9E3);
  static const Color _inkSecondary = Color(0xFFCFCBC3);
  static const Color _inkMuted = Color(0xFFB4B0A8);
  static const Color _inkDim = Color(0xFF85827B);
  static const Color _inkInverse = Color(0xFF241F18); // text on bronze

  static const Color _lineSoft = Color(0x14FFFFFF); // ~8% white
  static const Color _lineStrong = Color(0x29FFFFFF); // ~16% white

  static const ThemeColors _pewter = ThemeColors(
    bgApp: _pewterBgApp,
    bgPanel: _pewterBgPanel,
    bgElement: _pewterBgElement,
    bgHover: _pewterBgHover,
    textMain: _inkMain,
    textSecondary: _inkSecondary,
    textMuted: _inkMuted,
    textDim: _inkDim,
    textInverse: _inkInverse,
    borderSoft: _lineSoft,
    borderHover: _lineStrong,
  );

  // ─── Night (deeper variant) ───────────────────────────────────
  static const ThemeColors _night = ThemeColors(
    bgApp: Color(0xFF1E2024),
    bgPanel: Color(0xFF25282D),
    bgElement: Color(0xFF2D3036),
    bgHover: Color(0xFF383C43),
    textMain: _inkMain,
    textSecondary: _inkSecondary,
    textMuted: Color(0xFFABA79F),
    textDim: Color(0xFF7C7A73),
    textInverse: _inkInverse,
    borderSoft: _lineSoft,
    borderHover: _lineStrong,
  );

  /// Active token set for the current theme, read from the ThemeExtension.
  static ThemeColors of(BuildContext context) {
    return Theme.of(context).extension<ThemeColors>() ?? _pewter;
  }

  /// Token sets for preview outside a build context (e.g. setup wizard).
  static const ThemeColors dark = _night;
  static const ThemeColors light = _pewter;

  // ─── Theme-independent constants ──────────────────────────────

  // Bronze accent — muted, premium, NOT neon and NOT the old bright gold.
  static const Color accent = Color(0xFFB08D57);
  static const Color accentLight = Color(0xFFC6A06A);
  static const Color accentPale = Color(0xFFC6A06A); // applied with alpha
  static const Color accentMuted = Color(0x24B08D57); // ~14% bronze
  static const Color accentHover = Color(0xFFC6A06A);

  // Functional — softened, no neon.
  static const Color green = Color(0xFF6FA07B);
  static const Color red = Color(0xFFC4736B);
  static const Color successGreen = Color(0xFF6FA07B);
  static const Color warningOrange = Color(0xFFC99A5B);
  static const Color warmBrown = Color(0xFF8A7B63);

  // Radius
  static const double radiusSm = 8;
  static const double radiusMd = 14;
  static const double radiusLg = 20;

  // ─── Shadows (deep, for a dark mid-tone surface) ──────────────
  static List<BoxShadow> get shadowSm => [
        const BoxShadow(
          color: Color(0x33000000),
          blurRadius: 8,
          offset: Offset(0, 2),
        ),
      ];

  static List<BoxShadow> get shadowMd => [
        const BoxShadow(
          color: Color(0x40000000),
          blurRadius: 16,
          offset: Offset(0, 4),
        ),
      ];

  static List<BoxShadow> get shadowLg => [
        const BoxShadow(
          color: Color(0x4D000000),
          blurRadius: 32,
          offset: Offset(0, 8),
        ),
      ];

  // ─── Typography ───────────────────────────────────────────────
  // Schibsted Grotesk for display/headline/title (characterful, premium,
  // strong numerals for the download %); Inter for body/label legibility.
  static TextTheme _buildTextTheme(ThemeColors c) {
    final base = ThemeData(brightness: Brightness.dark).textTheme;
    final body = GoogleFonts.interTextTheme(base);
    final display = GoogleFonts.schibstedGroteskTextTheme(base);

    return body
        .copyWith(
          displayLarge: display.displayLarge?.copyWith(fontWeight: FontWeight.w600),
          displayMedium: display.displayMedium?.copyWith(fontWeight: FontWeight.w600),
          displaySmall: display.displaySmall?.copyWith(fontWeight: FontWeight.w600),
          headlineLarge: display.headlineLarge?.copyWith(fontWeight: FontWeight.w600),
          headlineMedium: display.headlineMedium?.copyWith(fontWeight: FontWeight.w600),
          headlineSmall: display.headlineSmall?.copyWith(fontWeight: FontWeight.w600),
          titleLarge: display.titleLarge?.copyWith(fontWeight: FontWeight.w600),
        )
        .apply(bodyColor: c.textMain, displayColor: c.textMain);
  }

  // ─── ThemeData builder (shared by Pewter & Night) ─────────────
  static ThemeData _build(ThemeColors c) {
    final textTheme = _buildTextTheme(c);

    return ThemeData(
      useMaterial3: true,
      brightness: Brightness.dark,
      scaffoldBackgroundColor: c.bgApp,
      extensions: [c],
      colorScheme: ColorScheme.dark(
        surface: c.bgApp,
        onSurface: c.textMain,
        primary: accent,
        onPrimary: c.textInverse,
        secondary: accentLight,
        onSecondary: c.textInverse,
        tertiary: warmBrown,
        error: red,
        outline: c.borderSoft,
        surfaceContainerHighest: c.bgPanel,
      ),
      textTheme: textTheme,
      appBarTheme: AppBarTheme(
        backgroundColor: c.bgApp,
        foregroundColor: c.textMain,
        elevation: 0,
        scrolledUnderElevation: 0,
        titleTextStyle: textTheme.titleLarge?.copyWith(
          fontWeight: FontWeight.w600,
          color: c.textMain,
        ),
      ),
      cardTheme: CardThemeData(
        color: c.bgPanel,
        elevation: 0,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(radiusMd),
          side: BorderSide(color: c.borderSoft),
        ),
      ),
      inputDecorationTheme: InputDecorationTheme(
        filled: true,
        fillColor: c.bgElement,
        contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(radiusMd),
          borderSide: BorderSide(color: c.borderSoft),
        ),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(radiusMd),
          borderSide: BorderSide(color: c.borderSoft),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(radiusMd),
          borderSide: const BorderSide(color: accent, width: 1.5),
        ),
        hintStyle: textTheme.bodyMedium?.copyWith(color: c.textDim),
      ),
      elevatedButtonTheme: ElevatedButtonThemeData(
        style: ElevatedButton.styleFrom(
          backgroundColor: accent,
          foregroundColor: c.textInverse,
          elevation: 0,
          padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 14),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(radiusSm),
          ),
          textStyle: textTheme.labelLarge?.copyWith(fontWeight: FontWeight.w600),
        ),
      ),
      outlinedButtonTheme: OutlinedButtonThemeData(
        style: OutlinedButton.styleFrom(
          foregroundColor: c.textMain,
          side: BorderSide(color: c.borderSoft),
          padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 12),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(radiusSm),
          ),
        ),
      ),
      textButtonTheme: TextButtonThemeData(
        style: TextButton.styleFrom(
          foregroundColor: accent,
          textStyle: textTheme.labelLarge?.copyWith(fontWeight: FontWeight.w500),
        ),
      ),
      iconButtonTheme: IconButtonThemeData(
        style: IconButton.styleFrom(
          foregroundColor: c.textMuted,
        ),
      ),
      dividerTheme: DividerThemeData(
        color: c.borderSoft,
        thickness: 1,
        space: 0,
      ),
      dialogTheme: DialogThemeData(
        backgroundColor: c.bgPanel,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(radiusLg),
        ),
        titleTextStyle: textTheme.titleLarge?.copyWith(
          fontWeight: FontWeight.w600,
          color: c.textMain,
        ),
      ),
      tooltipTheme: TooltipThemeData(
        decoration: BoxDecoration(
          color: c.bgHover,
          borderRadius: BorderRadius.circular(radiusSm),
          border: Border.all(color: c.borderSoft),
        ),
        textStyle: textTheme.bodySmall?.copyWith(color: c.textMain),
      ),
      scrollbarTheme: ScrollbarThemeData(
        thumbColor: WidgetStateProperty.all(c.bgHover),
        thickness: WidgetStateProperty.all(4),
        radius: const Radius.circular(999),
      ),
      snackBarTheme: SnackBarThemeData(
        backgroundColor: c.bgHover,
        contentTextStyle: textTheme.bodyMedium?.copyWith(color: c.textMain),
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(radiusSm),
        ),
        behavior: SnackBarBehavior.floating,
      ),
    );
  }

  /// Default theme — Pewter (mid-tone). Shown for light/system-light modes.
  static ThemeData get themeData => _build(_pewter);

  /// Deeper variant — Night. Shown for dark/system-dark modes.
  static ThemeData get darkThemeData => _build(_night);
}
