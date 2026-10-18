import 'package:flutter/material.dart';

/// "@file" mentions in a message: an "@" at the start of the text or after
/// whitespace, followed by the path (anything up to the next whitespace).
final RegExp mentionPattern = RegExp(r'(?<![^\s])@[^\s@]+');

/// The blue a mentioned file is drawn in, like a link. Lighter on a dark
/// surface, deeper on a light one, so it reads on both.
Color mentionColor(BuildContext context) =>
    Theme.of(context).brightness == Brightness.dark
        ? const Color(0xFF6CB6FF)
        : const Color(0xFF0B6BCB);

/// The composer's controller: identical to [TextEditingController] except that
/// every "@file" mention is painted in [mentionColor] while it is being typed.
class MentionTextEditingController extends TextEditingController {
  MentionTextEditingController({super.text});

  @override
  TextSpan buildTextSpan({
    required BuildContext context,
    TextStyle? style,
    required bool withComposing,
  }) {
    // Keep the IME underline for a composing region: styling by hand would lose
    // it, so only recolour when nothing is being composed.
    if (withComposing && value.composing.isValid && !value.composing.isCollapsed) {
      return super.buildTextSpan(context: context, style: style, withComposing: withComposing);
    }
    final blue = style?.copyWith(color: mentionColor(context)) ??
        TextStyle(color: mentionColor(context));
    final children = <TextSpan>[];
    var last = 0;
    for (final m in mentionPattern.allMatches(text)) {
      if (m.start > last) children.add(TextSpan(text: text.substring(last, m.start)));
      children.add(TextSpan(text: m.group(0), style: blue));
      last = m.end;
    }
    if (last < text.length) children.add(TextSpan(text: text.substring(last)));
    return TextSpan(style: style, children: children);
  }
}

const String mentionLinkScheme = 'memo-mention:';

/// Rewrites each "@file" mention of a sent message as a markdown link so the
/// message renders it in link style (blue). The target is inert — see
/// [isMentionLink].
String linkifyMentions(String content) {
  return content.replaceAllMapped(mentionPattern, (m) {
    final raw = m.group(0)!;
    final label = raw.replaceAllMapped(RegExp(r'([\\`*_{}\[\]()#+!|<>~])'), (c) => '\\${c[1]}');
    return '[$label]($mentionLinkScheme${Uri.encodeComponent(raw.substring(1))})';
  });
}

bool isMentionLink(String href) => href.startsWith(mentionLinkScheme);
