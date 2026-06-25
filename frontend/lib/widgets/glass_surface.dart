import 'dart:ui';
import 'package:flutter/material.dart';
import '../core/theme.dart';

/// Wraps [child] in a BackdropFilter blur (clipped to [borderRadius]) when the
/// active theme is glass; returns [child] unchanged otherwise.
///
/// Use this for surfaces that already build their own decorated container but
/// need real frosting — e.g. the chat input bar, which has content scrolling
/// behind it. The wrapped container should use a translucent fill in glass mode
/// for the blur to be visible.
class GlassBlur extends StatelessWidget {
  final Widget child;
  final BorderRadius borderRadius;

  const GlassBlur({
    super.key,
    required this.child,
    this.borderRadius = BorderRadius.zero,
  });

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    if (!c.isGlass) return child;
    return ClipRRect(
      borderRadius: borderRadius,
      child: BackdropFilter(
        filter: ImageFilter.blur(sigmaX: c.glassBlur, sigmaY: c.glassBlur),
        child: child,
      ),
    );
  }
}

/// A surface that renders as Apple-style frosted glass when the active theme is
/// the Glass Light theme, and as a plain opaque panel otherwise.
///
/// Dark themes have `isGlass == false`, so they take the opaque branch and look
/// exactly as before — only the Glass Light theme gets the BackdropFilter.
class GlassSurface extends StatelessWidget {
  final Widget child;
  final BorderRadius? borderRadius;
  final Border? border;

  /// Surface fill. Defaults to the theme's panel token (translucent in glass).
  final Color? color;
  final EdgeInsetsGeometry? padding;

  /// Blur sigma override; defaults to the theme's [ThemeColors.glassBlur].
  final double? blur;
  final List<BoxShadow>? shadow;

  const GlassSurface({
    super.key,
    required this.child,
    this.borderRadius,
    this.border,
    this.color,
    this.padding,
    this.blur,
    this.shadow,
  });

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final fill = color ?? c.bgPanel;

    if (!c.isGlass) {
      // Dark themes: plain opaque panel, unchanged.
      return Container(
        padding: padding,
        decoration: BoxDecoration(
          color: fill,
          borderRadius: borderRadius,
          border: border,
          boxShadow: shadow,
        ),
        child: child,
      );
    }

    final radius = borderRadius ?? BorderRadius.circular(MemoTheme.radiusLg);
    final sigma = blur ?? c.glassBlur;
    return DecoratedBox(
      decoration: BoxDecoration(borderRadius: radius, boxShadow: shadow),
      child: ClipRRect(
        borderRadius: radius,
        child: BackdropFilter(
          filter: ImageFilter.blur(sigmaX: sigma, sigmaY: sigma),
          child: Container(
            padding: padding,
            decoration: BoxDecoration(
              color: fill,
              borderRadius: radius,
              border: border ?? Border.all(color: c.borderSoft),
            ),
            child: child,
          ),
        ),
      ),
    );
  }
}
