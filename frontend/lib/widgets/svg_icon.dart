import 'package:flutter/widgets.dart';
import 'package:flutter_svg/flutter_svg.dart';

/// Renders one of the bundled Phosphor SVGs under `lib/icon/slash/` at a
/// square [size], tinted with [color] (falls back to the ambient
/// [DefaultTextStyle] colour, then black).
///
/// The SVG files are single-path, `viewBox="0 0 256 256"`,
/// `fill="currentColor"` — so a `srcIn` colour filter fully recolours them,
/// exactly like [Icon] does for an [IconData]. Use this instead of an emoji
/// literal for any on-screen glyph.
class SvgIcon extends StatelessWidget {
  /// File stem under `lib/icon/slash/`, e.g. `'puzzle-piece'`.
  final String name;
  final double size;
  final Color? color;

  const SvgIcon(this.name, {super.key, this.size = 20, this.color});

  @override
  Widget build(BuildContext context) {
    final tint = color ??
        DefaultTextStyle.of(context).style.color ??
        const Color(0xFF000000);
    return SizedBox(
      width: size,
      height: size,
      child: SvgPicture.asset(
        'lib/icon/slash/$name.svg',
        colorFilter: ColorFilter.mode(tint, BlendMode.srcIn),
      ),
    );
  }
}
