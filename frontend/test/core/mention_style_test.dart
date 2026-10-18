import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/mention_style.dart';
import 'package:memo_flutter/models/chat.dart';
import 'package:memo_flutter/widgets/chat_message_list.dart';

void main() {
  group('mentionPattern', () {
    List<String> find(String s) => mentionPattern.allMatches(s).map((m) => m.group(0)!).toList();

    test('finds an @path at the start or after whitespace', () {
      expect(find('@AGENTS.md'), ['@AGENTS.md']);
      expect(find('bak @src/main.go ve @README.md\nsonra'), ['@src/main.go', '@README.md']);
    });

    test('does not treat an email or a mid-word @ as a mention', () {
      expect(find('mail bugra@example.com yaz'), isEmpty);
      expect(find('a@b'), isEmpty);
    });

    test('a lone @ (still typing) is not a mention yet', () {
      expect(find('@'), isEmpty);
      expect(find('bak @ şuna'), isEmpty);
    });
  });

  testWidgets('the composer paints a mention blue and the rest in the base style', (tester) async {
    final controller = MentionTextEditingController(text: 'bak @src/main.go şuna');
    late TextSpan span;
    late Color blue;
    await tester.pumpWidget(MaterialApp(
      home: Builder(builder: (context) {
        blue = mentionColor(context);
        span = controller.buildTextSpan(
          context: context,
          style: const TextStyle(fontSize: 14, color: Colors.white),
          withComposing: false,
        );
        return const SizedBox();
      }),
    ));
    final parts = span.children!.cast<TextSpan>();
    expect(parts.map((p) => p.text).join(), 'bak @src/main.go şuna', reason: 'the text itself is unchanged');
    final mention = parts.firstWhere((p) => p.text == '@src/main.go');
    expect(mention.style!.color, blue);
    expect(mention.style!.fontSize, 14, reason: 'the base style is kept');
    expect(parts.firstWhere((p) => p.text == 'bak ').style, isNull);
  });

  group('linkifyMentions', () {
    test('turns each mention into an inert markdown link', () {
      final out = linkifyMentions('bak @src/main.go ve selam');
      expect(out, startsWith('bak [@src/main.go]($mentionLinkScheme'));
      expect(out, endsWith(' ve selam'));
    });

    test('escapes markdown characters in the label so file_names are not italicised', () {
      final out = linkifyMentions('@my_file_name.go');
      expect(out, contains(r'[@my\_file\_name.go]'));
    });

    test('leaves text without a mention untouched', () {
      expect(linkifyMentions('merhaba dünya a@b.com'), 'merhaba dünya a@b.com');
    });

    test('isMentionLink recognises only its own scheme', () {
      expect(isMentionLink('${mentionLinkScheme}src%2Fmain.go'), isTrue);
      expect(isMentionLink('https://example.com'), isFalse);
    });
  });

  testWidgets('a sent message shows the mentioned file as a blue link', (tester) async {
    late Color blue;
    await tester.pumpWidget(MaterialApp(
      home: Scaffold(
        body: Builder(builder: (context) {
          blue = mentionColor(context);
          return ChatMessageList(
            apiBaseUrl: 'http://localhost:8090',
            messages: [
              ChatMessage(role: 'user', content: '@AGENTS.md oku', timestamp: '12:00'),
            ],
          );
        }),
      ),
    ));
    await tester.pumpAndSettle();

    // Selectable markdown renders into SelectableText, not RichText.
    final selectable = tester.widgetList<SelectableText>(find.byType(SelectableText)).firstWhere(
          (r) => (r.textSpan?.toPlainText() ?? '').contains('@AGENTS.md'),
        );
    final root = selectable.textSpan!;
    Color? colorOf(String needle) {
      Color? found;
      root.visitChildren((span) {
        if (span is TextSpan && span.text == needle) found = span.style?.color;
        return true;
      });
      return found;
    }

    expect(colorOf('@AGENTS.md'), blue, reason: 'the mention is drawn in the link blue');
    expect(root.toPlainText(), contains('oku'));
  });
}
