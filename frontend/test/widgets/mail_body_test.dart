import 'package:flutter/material.dart';
import 'package:flutter_html/flutter_html.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/widgets/mail_body.dart';

void main() {
  test(
    'mail HTML preserves CSS while removing active content and all image sources',
    () {
      final document = prepareMailHtml('''
<style>.mail {color: #ff0000; font-weight: bold}</style>
<div class="mail" onclick="alert(1)">Türkçe İğüşöç</div>
<img src="https://tracker.example/pixel" srcset="https://tracker.example/2" alt="Logo">
<script>attack()</script><iframe src="https://tracker.example"></iframe>
<link rel="stylesheet" href="https://tracker.example/a.css">
<a href="javascript:alert(1)">Bad link</a><a href="https://example.com">Good link</a>
''');
      expect(document.querySelector('style')!.text, contains('#ff0000'));
      expect(
        document.querySelector('.mail')!.attributes.containsKey('onclick'),
        isFalse,
      );
      expect(document.querySelectorAll('img,script,iframe,link'), isEmpty);
      expect(document.querySelectorAll('a').first.attributes['href'], isNull);
      expect(
        document.querySelectorAll('a').last.attributes['href'],
        'https://example.com',
      );
      expect(document.body!.text, contains('Logo'));
      expect(safeMailLink('file:///etc/passwd'), isNull);
      expect(safeMailLink('data:text/html,attack'), isNull);
      expect(safeMailLink('mailto:a@example.com'), isNotNull);
    },
  );

  for (final lang in [MemoLocale.tr, MemoLocale.en]) {
    testWidgets('styled Turkish HTML, table and plain toggle ($lang)', (
      tester,
    ) async {
      L10n.setLocale(lang);
      await tester.pumpWidget(
        const MaterialApp(
          home: Scaffold(
            body: SingleChildScrollView(
              child: MailBody(
                textBody: 'Düz gövde ığüşöç',
                htmlBody: '''
<style>.red {color: #ff0000; font-weight: bold}</style>
<p class="red">Biçimli İğüşöç</p>
<table><tr><td>Fatura</td><td>125 TL</td></tr></table>
<img src="https://tracker.example/pixel">
''',
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.byType(Html), findsOneWidget);
      expect(
        find.textContaining('Biçimli İğüşöç', findRichText: true),
        findsWidgets,
      );
      expect(find.textContaining('125 TL', findRichText: true), findsWidgets);
      expect(find.byType(Image), findsNothing);
      bool redBold = false;
      void visit(InlineSpan span) {
        if (span is TextSpan) {
          if ((span.text ?? '').contains('Biçimli') &&
              span.style?.color == const Color(0xffff0000) &&
              span.style?.fontWeight == FontWeight.bold)
            redBold = true;
          for (final child in span.children ?? <InlineSpan>[]) {
            visit(child);
          }
        }
      }

      for (final rich in tester.widgetList<RichText>(find.byType(RichText))) {
        visit(rich.text);
      }
      expect(
        redBold,
        isTrue,
        reason: 'Embedded CSS must affect actual rendered text',
      );
      await tester.tap(find.text(L10n.t('mail_body_plain')));
      await tester.pumpAndSettle();
      expect(find.byType(Html), findsNothing);
      expect(find.text('Düz gövde ığüşöç'), findsOneWidget);
      expect(find.text(L10n.t('mail_body_formatted')), findsOneWidget);
      expect(tester.takeException(), isNull);
    });
  }
  tearDown(() => L10n.setLocale(MemoLocale.en));
}
